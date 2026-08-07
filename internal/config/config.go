// Package config loads and validates the per-project .awf/ configuration:
// a skeleton config.yaml plus per-target sidecar YAMLs and convention parts.
package config

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"gopkg.in/yaml.v3"
)

// DocsDir is the fixed root for awf-managed documentation.
const DocsDir = "docs"

// SectionOverride is a sidecar's per-section override. Body replacement is by
// convention part only; the field set is deliberately just Drop.
// touches-state: config/configuration:no-replacewith - SectionOverride field set omits replaceWith; proof in config_test.go
type SectionOverride struct {
	Drop bool `yaml:"drop"`
}

// Sidecar holds a single target's non-prose configuration: structured render
// data, per-section overrides, and the local flag. It lives at
// <awfDir>/<kind>/<name>.yaml (agents-doc: <awfDir>/agents-doc.yaml). An absent
// sidecar is the zero Sidecar (publication-safe: empty data/sections).
type Sidecar struct {
	Data         map[string]any             `yaml:"data"`
	DataDefaults map[string]bool            `yaml:"dataDefaults"`
	Sections     map[string]SectionOverride `yaml:"sections"`
	Local        bool                       `yaml:"local"`
	// Paths declares a domain's file territory as anchored path globs
	// (ADR-0077); read only from domain sidecars, inert on other kinds.
	Paths []string `yaml:"paths"`
}

// Config is the skeleton config.yaml: global fields plus flat enable arrays.
// Presence of a name in Skills/Agents/Docs enables that artifact; per-artifact
// data/sections/local live in sidecars, not here. Targets is the adapter-runtime
// enable array (default ["claude"]); adapter artifacts render once per entry.
type Config struct {
	Prefix  string `yaml:"prefix"`
	DocsDir string `yaml:"docsDir"`
	// IntegrationBranch names the branch effort work integrates into. It is
	// required-explicit and carries no in-code default (the Prefix precedent,
	// not the DocsDir one): the schema migration writes `integrationBranch:
	// main` visibly so no adopter silently inherits a branch name it never
	// chose (ADR-0202 Decision 6, keeping ADR-0127's silent-default removal).
	IntegrationBranch string              `yaml:"integrationBranch"`
	Vars              map[string]any      `yaml:"vars"`
	Skills            []string            `yaml:"skills"`
	Agents            []string            `yaml:"agents"`
	Docs              []string            `yaml:"docs"`
	Domains           []string            `yaml:"domains"`
	Tags              map[string]string   `yaml:"tags"`
	ContextIgnore     []string            `yaml:"contextIgnore"`
	Targets           []string            `yaml:"targets"`
	CurrentState      *CurrentStateConfig `yaml:"currentState"`
	Audit             *AuditConfig        `yaml:"audit"`
	Bootstrap         *BootstrapConfig    `yaml:"bootstrap"`
	ProseGate         *ProseGateConfig    `yaml:"proseGate"`
	MemoryCite        *MemoryCiteConfig   `yaml:"memoryCite"`
	CommitPolicy      *CommitPolicyConfig `yaml:"commitPolicy"`
	root              string              // <project>/.awf, for sidecar/part resolution
	raw               []byte              // the exact config.yaml bytes Load read, for in-place byte edits
	read              TreeReader          // selected filesystem or immutable snapshot universe
	filesystem        bool
}

// CommitPolicyConfig is an optional exact-commit provenance policy. Repository
// resolution and verification belong to later operation owners; this package
// validates only the authored structural contract.
type CommitPolicyConfig struct {
	GrandfatheredThrough string                 `yaml:"grandfatheredThrough"`
	AllowedIdentities    []CommitPolicyIdentity `yaml:"allowedIdentities"`
	RequireSignedCommits bool                   `yaml:"requireSignedCommits"`
	AllowedSigners       []CommitPolicySigner   `yaml:"allowedSigners"`
	allowedIdentitiesSet bool
	allowedSignersSet    bool
}

// CommitPolicyIdentity is one exact author/committer name and email pair.
type CommitPolicyIdentity struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// CommitPolicySigner is one SSH signing principal and public key pair.
type CommitPolicySigner struct {
	Principal string `yaml:"principal"`
	Key       string `yaml:"key"`
}

// UnmarshalYAML retains optional-list presence while preserving strict nested
// field validation for the commitPolicy mapping and its records.
func (c *CommitPolicyConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("commitPolicy must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if seen[key] {
			return fmt.Errorf("field %s already set in commitPolicy", key)
		}
		seen[key] = true
		switch key {
		case "grandfatheredThrough":
			if err := decodeStringScalar(value, &c.GrandfatheredThrough, "commitPolicy.grandfatheredThrough"); err != nil {
				return err
			}
		case "allowedIdentities":
			c.allowedIdentitiesSet = true
			if err := decodeCommitPolicyIdentities(value, &c.AllowedIdentities); err != nil {
				return err
			}
		case "requireSignedCommits":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!bool" {
				return errors.New("commitPolicy.requireSignedCommits must be a boolean scalar")
			}
			if err := value.Decode(&c.RequireSignedCommits); err != nil { // coverage-ignore: a yaml bool scalar is fully decoded by yaml.v3 after the kind and tag check above
				return fmt.Errorf("commitPolicy.requireSignedCommits must be a boolean scalar: %w", err)
			}
		case "allowedSigners":
			c.allowedSignersSet = true
			if err := decodeCommitPolicySigners(value, &c.AllowedSigners); err != nil {
				return err
			}
		default:
			return fmt.Errorf("field %s not found in type config.CommitPolicyConfig", key)
		}
	}
	return nil
}

func decodeCommitPolicyIdentities(node *yaml.Node, out *[]CommitPolicyIdentity) error {
	if node.Kind != yaml.SequenceNode {
		return errors.New("commitPolicy.allowedIdentities must be a list, not null")
	}
	identities := make([]CommitPolicyIdentity, len(node.Content))
	for i, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return fmt.Errorf("commitPolicy.allowedIdentities[%d] must be a mapping", i)
		}
		seen := map[string]bool{}
		for j := 0; j < len(item.Content); j += 2 {
			key, value := item.Content[j].Value, item.Content[j+1]
			if seen[key] {
				return fmt.Errorf("field %s already set in commitPolicy.allowedIdentities[%d]", key, i)
			}
			seen[key] = true
			switch key {
			case "name":
				if err := decodeStringScalar(value, &identities[i].Name, fmt.Sprintf("commitPolicy.allowedIdentities[%d].name", i)); err != nil {
					return err
				}
			case "email":
				if err := decodeStringScalar(value, &identities[i].Email, fmt.Sprintf("commitPolicy.allowedIdentities[%d].email", i)); err != nil {
					return err
				}
			default:
				return fmt.Errorf("field %s not found in type config.CommitPolicyIdentity", key)
			}
		}
	}
	*out = identities
	return nil
}

func decodeCommitPolicySigners(node *yaml.Node, out *[]CommitPolicySigner) error {
	if node.Kind != yaml.SequenceNode {
		return errors.New("commitPolicy.allowedSigners must be a list, not null")
	}
	signers := make([]CommitPolicySigner, len(node.Content))
	for i, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return fmt.Errorf("commitPolicy.allowedSigners[%d] must be a mapping", i)
		}
		seen := map[string]bool{}
		for j := 0; j < len(item.Content); j += 2 {
			key, value := item.Content[j].Value, item.Content[j+1]
			if seen[key] {
				return fmt.Errorf("field %s already set in commitPolicy.allowedSigners[%d]", key, i)
			}
			seen[key] = true
			switch key {
			case "principal":
				if err := decodeStringScalar(value, &signers[i].Principal, fmt.Sprintf("commitPolicy.allowedSigners[%d].principal", i)); err != nil {
					return err
				}
			case "key":
				if err := decodeStringScalar(value, &signers[i].Key, fmt.Sprintf("commitPolicy.allowedSigners[%d].key", i)); err != nil {
					return err
				}
			default:
				return fmt.Errorf("field %s not found in type config.CommitPolicySigner", key)
			}
		}
	}
	*out = signers
	return nil
}

// TreeReader supplies canonical config-tree-relative bytes without exposing a
// filesystem. Implementations return defensive copies.
type TreeReader interface {
	ReadFile(path string) ([]byte, bool)
	Paths(prefix string) []string
}

type filesystemTreeReader struct{ root string }

func (r filesystemTreeReader) ReadFile(path string) ([]byte, bool) {
	b, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(path)))
	if err != nil {
		return nil, false
	}
	return slices.Clone(b), true
}
func (r filesystemTreeReader) Paths(prefix string) []string { return []string{} }

// Source returns the exact config.yaml bytes Load read. A byte-level editor
// (SetArrayMember, SetArray, SetMappingScalar) reuses these instead of re-reading
// the file, which after a successful Load could only fail on a race.
func (c *Config) Source() []byte { return c.raw }

// CurrentStateConfig configures bridge-preparation validation for canonical
// current-state topics. It is deliberately separate from the legacy invariant
// authority, which remains active throughout the bridge tranche.
type CurrentStateConfig struct {
	Sources   []CurrentStateSource `yaml:"sources"`
	TestGlobs []string             `yaml:"testGlobs"`
}

// UnmarshalYAML preserves strict nested field validation for the custom-decoded
// current-state mapping.
func (c *CurrentStateConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("currentState must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if seen[key] {
			return fmt.Errorf("field %s already set in currentState", key)
		}
		seen[key] = true
		switch key {
		case "sources":
			if err := value.Decode(&c.Sources); err != nil {
				return err
			}
		case "testGlobs":
			if err := decodeStringScalars(value, &c.TestGlobs, "currentState.testGlobs"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("field %s not found in type config.CurrentStateConfig", key)
		}
	}
	return nil
}

// CurrentStateSource describes one marker-bearing source family. closeSet
// distinguishes an omitted close token from an explicitly empty one.
type CurrentStateSource struct {
	Globs    []string `yaml:"globs"`
	Marker   string   `yaml:"marker"`
	Close    string   `yaml:"close"`
	closeSet bool
}

// UnmarshalYAML retains close-token presence while preserving strict nested
// field validation for the custom-decoded source mapping.
func (s *CurrentStateSource) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("currentState source must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if seen[key] {
			return fmt.Errorf("field %s already set in currentState source", key)
		}
		seen[key] = true
		switch key {
		case "globs":
			if err := decodeStringScalars(value, &s.Globs, "currentState source.globs"); err != nil {
				return err
			}
		case "marker":
			if err := decodeStringScalar(value, &s.Marker, "currentState source.marker"); err != nil {
				return err
			}
		case "close":
			s.closeSet = true
			if err := decodeStringScalar(value, &s.Close, "currentState source.close"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("field %s not found in type config.CurrentStateSource", key)
		}
	}
	return nil
}

func decodeStringScalar(node *yaml.Node, dst *string, field string) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("%s must be a string scalar", field)
	}
	*dst = node.Value
	return nil
}

func decodeStringScalars(node *yaml.Node, dst *[]string, field string) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s must be a sequence of string scalars", field)
	}
	decoded := make([]string, len(node.Content))
	for i, item := range node.Content {
		if err := decodeStringScalar(item, &decoded[i], fmt.Sprintf("%s[%d]", field, i)); err != nil {
			return err
		}
	}
	*dst = decoded
	return nil
}

// BootstrapConfig configures the rendered .awf/bootstrap.sh singleton (ADR-0040,
// relocated by ADR-0047). A
// nil *BootstrapConfig (key absent) and Enabled false both mean "do not render";
// only Enabled true renders the artifact - a nested enable entry rather than a
// top-level scalar bool (the Alternatives table rejected the bare bool).
type BootstrapConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ProseGateConfig configures exemptions for `awf check repo prose` (ADR-0119),
// which always scans every tracked text file for the seven banned typographic
// punctuation substitutes. A nil *ProseGateConfig means no paths or codepoints
// are exempt.
type ProseGateConfig struct {
	Exemptions []ProseExemption `yaml:"exemptions"`
}

// ProseExemption exempts one codepoint in one path. Codepoint is spelled
// "U+2014", never the character itself: config.yaml is a tracked file the scan
// reads, so a typed glyph here would be a finding against the file that
// configures the exemptions. A nil Count permits any number of occurrences; a
// non-nil Count pins the expected number, so an added occurrence in an exempt
// file still fails.
type ProseExemption struct {
	Path      string `yaml:"path"`
	Codepoint string `yaml:"codepoint"`
	Count     *int   `yaml:"count"`
}

// MemoryCiteConfig configures exemptions for `awf check repo memory`
// (ADR-0158), which always scans the staged decision-record directories and
// every cleaned commit-message body for a citation of a specific working-memory
// file. A nil *MemoryCiteConfig means no paths are exempt.
type MemoryCiteConfig struct {
	Exemptions []MemoryExemption `yaml:"exemptions"`
}

// MemoryExemption permits citations in one path. A nil Count permits any number
// of them; a non-nil Count pins the expected number, so an added citation in an
// exempt file still fails.
type MemoryExemption struct {
	Path  string `yaml:"path"`
	Count *int   `yaml:"count"`
}

// AuditConfig carries the repository-specific Conventional Commits scope
// vocabulary for `awf audit` (ADR-0017). Every audit rule and threshold is fixed
// in internal/audit; a nil *AuditConfig or an empty AllowedScopes accepts any
// scope.
type AuditConfig struct {
	AllowedScopes []ScopeSpec `yaml:"allowedScopes"`
}

// AuditScopes returns the configured scope vocabulary, if the audit block exists.
func AuditScopes(a *AuditConfig) []ScopeSpec {
	if a == nil {
		return nil
	}
	return a.AllowedScopes
}

// Load reads <awfDir>/config.yaml with the strict decoder, records awfDir as the
// sidecar/part resolution root, and defaults DocsDir.
func Load(awfDir string) (*Config, error) {
	b, err := os.ReadFile(filepath.Join(awfDir, "config.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not an awf project (run `awf init`): %w", err)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg, err := ParseTree(awfDir, b, filesystemTreeReader{root: awfDir})
	if cfg != nil {
		cfg.filesystem = true
	}
	return cfg, err
}

// Parse strictly decodes config.yaml bytes, records awfDir as the sidecar/part
// resolution root, and applies defaults.
func Parse(awfDir string, b []byte) (*Config, error) {
	cfg, err := ParseTree(awfDir, b, filesystemTreeReader{root: awfDir})
	if cfg != nil {
		cfg.filesystem = true
	}
	return cfg, err
}

// ParseTree decodes config bytes and injects the selected config-tree reader.
func ParseTree(awfDir string, b []byte, read TreeReader) (*Config, error) {
	c := Config{}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	var source yaml.Node
	if err := yaml.Unmarshal(b, &source); err != nil { // coverage-ignore: the strict decoder accepted the same YAML bytes above
		return nil, fmt.Errorf("parse config presence: %w", err)
	}

	c.root = awfDir
	c.raw = slices.Clone(b)
	c.read = read
	if c.DocsDir == "" {
		c.DocsDir = "docs"
	}
	if len(c.Targets) == 0 {
		c.Targets = []string{"claude"}
	}
	return &c, nil
}

// DirName is the config-tree directory name at the project root.
const DirName = ".awf"

// RootDir returns the config-tree directory for a project root (<root>/.awf).
func RootDir(root string) string { return filepath.Join(root, DirName) }

// ConfigPath returns the skeleton config.yaml path for a project root.
func ConfigPath(root string) string { return filepath.Join(RootDir(root), "config.yaml") }

// LockPath returns the awf.lock path for a project root.
func LockPath(root string) string { return filepath.Join(RootDir(root), "awf.lock") }

// Sidecar reads <root>/<kind>/<name>.yaml; agents-doc lives at <root>/agents-doc.yaml.
// A missing file yields a zero Sidecar (publication-safe: empty data/sections).
func (c *Config) Sidecar(kind, name string) (Sidecar, error) {
	var rel string
	if IsSingletonKind(kind) {
		rel = kind + ".yaml"
	} else {
		rel = filepath.Join(kind, name+".yaml")
	}
	var b []byte
	if c.filesystem {
		var err error
		b, err = os.ReadFile(filepath.Join(c.root, rel))
		if errors.Is(err, os.ErrNotExist) {
			return Sidecar{}, nil
		}
		if err != nil {
			return Sidecar{}, fmt.Errorf("read sidecar %s: %w", rel, err)
		}
	} else {
		var ok bool
		b, ok = c.ReadSidecar(rel)
		if !ok {
			return Sidecar{}, nil
		}
	}
	var s Sidecar
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return Sidecar{}, fmt.Errorf("parse sidecar %s: %w", rel, err)
	}
	return s, nil
}

// ReadSidecar returns selected-universe sidecar bytes by config-relative path.
func (c *Config) ReadSidecar(rel string) ([]byte, bool) {
	if c.read == nil {
		return nil, false
	}
	return c.read.ReadFile(filepath.ToSlash(rel))
}

// ReadPart returns selected-universe convention-part bytes.
func (c *Config) ReadPart(kind, artifact, section string) ([]byte, bool, error) {
	if c.read == nil {
		return nil, false, nil
	}
	var rel string
	if IsSingletonKind(kind) {
		rel = filepath.Join("parts", kind, section+".md")
	} else {
		rel = filepath.Join(kind, "parts", artifact, section+".md")
	}
	if c.filesystem {
		b, err := os.ReadFile(filepath.Join(c.root, rel))
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("read part %s: %w", rel, err)
		}
		return slices.Clone(b), true, nil
	}
	b, ok := c.read.ReadFile(filepath.ToSlash(rel))
	return b, ok, nil
}

// ReadPartPath reads a consumed absolute part path through the selected reader.
func (c *Config) ReadPartPath(full string) ([]byte, error) {
	rel, err := filepath.Rel(c.root, full)
	if err != nil { // coverage-ignore: selected config root and consumed part paths are absolute paths on the same volume
		return nil, err
	}
	if c.filesystem {
		return os.ReadFile(full)
	}
	if b, ok := c.read.ReadFile(filepath.ToSlash(rel)); ok {
		return b, nil
	}
	return nil, os.ErrNotExist
}

// HasSidecar reports whether a declaring sidecar file exists for an artifact -
// the presence signal that marks a non-catalog name as an intentional local
// artifact rather than a typo (ADR-0068).
func (c *Config) HasSidecar(kind, name string) (bool, error) {
	var rel string
	if IsSingletonKind(kind) {
		rel = kind + ".yaml"
	} else {
		rel = filepath.Join(kind, name+".yaml")
	}
	if c.filesystem {
		_, err := os.Stat(filepath.Join(c.root, rel))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("stat sidecar %s: %w", rel, err)
		}
		return true, nil
	}
	_, ok := c.ReadSidecar(filepath.ToSlash(rel))
	return ok, nil
}

// IsSingletonKind reports whether kind is an always-on singleton whose sidecar lives at
// <root>/<kind>.yaml and whose parts live under <root>/parts/<kind>/ (ADR-0021, ADR-0043).
func IsSingletonKind(kind string) bool {
	return slices.Contains(catalog.SingletonKinds(), kind)
}

// PartPath returns the convention part path for a section of an artifact.
func (c *Config) PartPath(kind, artifact, section string) string {
	if IsSingletonKind(kind) {
		return filepath.Join(c.root, "parts", kind, section+".md")
	}
	return filepath.Join(c.root, kind, "parts", artifact, section+".md")
}

func (c *Config) Validate() error {
	if c.Prefix == "" {
		return errors.New("prefix must not be empty")
	}
	if hasPathSep(c.Prefix) {
		return fmt.Errorf("prefix %q must not contain path separators", c.Prefix)
	}
	if strings.HasPrefix(c.DocsDir, "/") || strings.Contains(c.DocsDir, "..") {
		return fmt.Errorf("docsDir %q must be a relative path without \"..\"", c.DocsDir)
	}
	if err := validateIntegrationBranch(c.IntegrationBranch); err != nil {
		return err
	}
	for _, d := range c.Domains {
		if err := ValidateDomainName(d); err != nil {
			return err
		}
	}
	if c.CurrentState != nil {
		for i, src := range c.CurrentState.Sources {
			if len(src.Globs) == 0 {
				return fmt.Errorf("currentState.sources[%d] has no globs; list at least one path glob", i)
			}
			if src.Marker == "" {
				return fmt.Errorf("currentState.sources[%d] has an empty marker", i)
			}
			if src.closeSet && src.Close == "" {
				return fmt.Errorf("currentState.sources[%d] has an explicitly empty close token", i)
			}
			if err := validateUniquePathGlobs(src.Globs); err != nil {
				return fmt.Errorf("currentState.sources[%d].globs: %w", i, err)
			}
		}
		if err := validateUniquePathGlobs(c.CurrentState.TestGlobs); err != nil {
			return fmt.Errorf("currentState.testGlobs: %w", err)
		}
	}

	if c.CommitPolicy != nil {
		if err := validateCommitPolicy(c.CommitPolicy, validateOpenSSHPublicKey); err != nil {
			return err
		}
	}
	// Targets: sanity only - the unknown-adapter-name check lives in project.Open
	// (resolveTargets), where the adapter registry is, to keep config free of a
	// project import cycle (ADR-0037).
	if len(c.Targets) == 0 {
		return errors.New("targets must not be empty")
	}
	seenTargets := map[string]bool{}
	for _, t := range c.Targets {
		if t == "" || hasPathSep(t) {
			return fmt.Errorf("target %q must be a non-empty name without path separators", t)
		}
		if seenTargets[t] {
			return fmt.Errorf("duplicate target %q", t)
		}
		seenTargets[t] = true
	}
	return nil
}

// ValidateDomainName reports whether name is a usable domain key: non-empty and
// free of path separators or "..". Shared by Validate and the `awf enable domain`
// path so a freeform domain name is rejected the same way in both.
func ValidateDomainName(name string) error {
	if name == "" {
		return errors.New("domain name must not be empty")
	}
	if hasPathSep(name) {
		return fmt.Errorf("domain %q must not contain path separators or \"..\"", name)
	}
	return nil
}

// ValidateArtifactName reports whether name is usable as a local skill/agent
// name (ADR-0068): non-empty lowercase kebab-case (letters, digits, hyphens).
// The charset is frontmatter-safe - it excludes the path separators and ".." the
// invariant requires, awf's reserved "_" namespace, and the colon/space/quote
// characters that would otherwise interpolate into the base template's name: line
// and break its YAML frontmatter. It mirrors every catalog artifact's naming.
// touches-state: config/validation:local-name-validated - local skill/agent name charset validation; proof in config_test.go
func ValidateArtifactName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s name must not be empty", kind)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return fmt.Errorf("%s %q must be lowercase kebab-case: letters, digits, and hyphens only", kind, name)
		}
	}
	return nil
}

// ValidateDocName validates a path-aware local doc name (ADR-0091): one or more
// lowercase-kebab segments joined by "/", rejecting a path escape, an empty or
// leading/trailing segment, a ".md" suffix, and any segment (e.g. the reserved
// "_base" stem) carrying a non-kebab character. Skill/agent names stay flat.
// touches-state: config/validation:local-doc-name-path-validated - path-aware local doc name validation; proof in docname_test.go
func ValidateDocName(name string) error {
	if name == "" {
		return errors.New("doc name must not be empty")
	}
	if strings.HasSuffix(name, ".md") {
		return fmt.Errorf("doc %q must not end in .md", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("doc %q must not contain a .. path escape", name)
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return fmt.Errorf("doc %q must not have a leading or trailing slash", name)
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" {
			return fmt.Errorf("doc %q must not have an empty path segment", name)
		}
		alnum := false
		for _, r := range seg {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				alnum = true
			case r == '-':
			default:
				return fmt.Errorf("doc %q segment %q must be lowercase kebab-case (the reserved _base stem is rejected here)", name, seg)
			}
		}
		// An all-hyphen segment derives an empty title, which would breach
		// inv: local-doc-map-fields (a non-empty document-map label).
		if !alnum {
			return fmt.Errorf("doc %q segment %q must contain a letter or digit", name, seg)
		}
	}
	return nil
}

// hasPathSep reports whether s contains a path separator or a ".." segment - the
// shared reject condition for prefix/target/domain names.
func hasPathSep(s string) bool {
	return strings.ContainsAny(s, "/\\") || strings.Contains(s, "..")
}

// validatePathGlob rejects a malformed anchored path-glob pattern (ADR-0077).
// Patterns are matched against slash-separated repo-relative paths; `**/` is
// the any-depth form.
func validatePathGlob(g string) error {
	return pathglob.Validate(g)
}

// validateIntegrationBranch enforces the required-explicit integrationBranch
// key's shape: non-empty, free of whitespace, and not starting with `-` (git
// reads a leading dash as an option). Slashes stay legal, so `release/1.0` is
// an accepted branch name.
func validateIntegrationBranch(b string) error {
	if b == "" {
		return errors.New("integrationBranch must not be empty; set it explicitly (for example `integrationBranch: main`)")
	}
	if strings.ContainsFunc(b, unicode.IsSpace) {
		return fmt.Errorf("integrationBranch %q must not contain whitespace", b)
	}
	if strings.HasPrefix(b, "-") {
		return fmt.Errorf("integrationBranch %q must not start with %q", b, "-")
	}
	return nil
}

func validateCommitPolicy(policy *CommitPolicyConfig, validateKey func(string) error) error {
	if !isFullOID(policy.GrandfatheredThrough) {
		return errors.New("commitPolicy.grandfatheredThrough must be a lowercase full object ID")
	}
	identitiesPresent := policy.allowedIdentitiesSet || policy.AllowedIdentities != nil
	if identitiesPresent && len(policy.AllowedIdentities) == 0 {
		return errors.New("commitPolicy.allowedIdentities must be non-empty when present")
	}
	identities := map[CommitPolicyIdentity]bool{}
	for i, identity := range policy.AllowedIdentities {
		if err := validateIdentityField(identity.Name); err != nil {
			return fmt.Errorf("commitPolicy.allowedIdentities[%d].name %w", i, err)
		}
		if err := validateIdentityField(identity.Email); err != nil {
			return fmt.Errorf("commitPolicy.allowedIdentities[%d].email %w", i, err)
		}
		if identities[identity] {
			return fmt.Errorf("commitPolicy.allowedIdentities[%d] duplicates an earlier identity", i)
		}
		identities[identity] = true
	}
	signersPresent := policy.allowedSignersSet || policy.AllowedSigners != nil
	if policy.RequireSignedCommits && len(policy.AllowedSigners) == 0 {
		return errors.New("commitPolicy.allowedSigners must be non-empty when commitPolicy.requireSignedCommits is true")
	}
	if !policy.RequireSignedCommits && signersPresent {
		return errors.New("commitPolicy.allowedSigners requires commitPolicy.requireSignedCommits to be true")
	}
	signers := map[CommitPolicySigner]bool{}
	for i, signer := range policy.AllowedSigners {
		if !validPrincipal(signer.Principal) {
			return fmt.Errorf("commitPolicy.allowedSigners[%d].principal must contain only [A-Za-z0-9._@+-]", i)
		}
		if signers[signer] {
			return fmt.Errorf("commitPolicy.allowedSigners[%d] duplicates an earlier signer", i)
		}
		if err := validateKey(signer.Key); err != nil {
			return fmt.Errorf("commitPolicy.allowedSigners[%d].key is not a supported OpenSSH public key", i)
		}
		signers[signer] = true
	}
	return nil
}

func validateIdentityField(value string) error {
	if value == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return errors.New("must be non-empty UTF-8 without leading/trailing whitespace or control characters")
	}
	return nil
}

func isFullOID(value string) bool {
	// Git currently supports SHA-1 and SHA-256 object formats. Runtime resolution
	// later compares the configured width to the opened repository.
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, r := range value {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'f' {
			continue
		}
		return false
	}
	return true
}

func validPrincipal(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._@+-", r) {
			continue
		}
		return false
	}
	return true
}

// validateOpenSSHPublicKey is the operation-scoped subprocess seam. Unit tests
// pass a controlled function to validateCommitPolicy rather than replacing a
// process package global.
func validateOpenSSHPublicKey(key string) error {
	if strings.ContainsAny(key, "\r\n") || strings.TrimSpace(key) != key {
		return errors.New("not a single record")
	}
	fields := strings.Fields(key)
	if len(fields) != 2 || fields[0] == "" {
		return errors.New("must have algorithm and key only")
	}
	if !supportedSSHKeyAlgorithm(fields[0]) || !matchingSSHKeyBlob(fields[0], fields[1]) {
		return errors.New("unsupported or malformed key")
	}
	cmd := exec.Command("ssh-keygen", "-lf", "-")
	cmd.Stdin = strings.NewReader(key)
	if err := cmd.Run(); err != nil {
		return errors.New("ssh-keygen rejected key")
	}
	return nil
}

func supportedSSHKeyAlgorithm(algorithm string) bool {
	switch algorithm {
	case "ssh-ed25519", "ecdsa-sha2-nistp256", "sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com":
		return true
	default:
		return false
	}
}

func matchingSSHKeyBlob(algorithm, encoded string) bool {
	blob, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(blob) < 4 {
		return false
	}
	n := binary.BigEndian.Uint32(blob[:4])
	if int(n) > len(blob)-4 {
		return false
	}
	return string(blob[4:4+int(n)]) == algorithm
}

func validateUniquePathGlobs(globs []string) error {
	seen := map[string]bool{}
	for _, g := range globs {
		if g == "" {
			return errors.New("glob must not be empty")
		}
		if seen[g] {
			return fmt.Errorf("duplicate glob %q", g)
		}
		seen[g] = true
		if err := validatePathGlob(g); err != nil {
			return err
		}
	}
	return nil
}
