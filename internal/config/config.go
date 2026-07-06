// Package config loads and validates the per-project .awf/ configuration:
// a skeleton config.yaml plus per-target sidecar YAMLs and convention parts.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"gopkg.in/yaml.v3"
)

// SectionOverride is a sidecar's per-section override. Body replacement is by
// convention part only; the field set is deliberately just Drop.
// invariant: no-replacewith
type SectionOverride struct {
	Drop bool `yaml:"drop"`
}

// Sidecar holds a single target's non-prose configuration: structured render
// data, per-section overrides, and the local flag. It lives at
// <awfDir>/<kind>/<name>.yaml (agents-doc: <awfDir>/agents-doc.yaml). An absent
// sidecar is the zero Sidecar (publication-safe: empty data/sections).
type Sidecar struct {
	Data     map[string]any             `yaml:"data"`
	Sections map[string]SectionOverride `yaml:"sections"`
	Local    bool                       `yaml:"local"`
}

// Config is the skeleton config.yaml: global fields plus flat enable arrays.
// Presence of a name in Skills/Agents/Docs enables that artifact; per-artifact
// data/sections/local live in sidecars, not here. Targets is the adapter-runtime
// enable array (default ["claude"]); adapter artifacts render once per entry.
type Config struct {
	Prefix     string           `yaml:"prefix"`
	DocsDir    string           `yaml:"docsDir"`
	Vars       map[string]any   `yaml:"vars"`
	Skills     []string         `yaml:"skills"`
	Agents     []string         `yaml:"agents"`
	Docs       []string         `yaml:"docs"`
	Domains    []string         `yaml:"domains"`
	Targets    []string         `yaml:"targets"`
	Invariants *InvariantConfig `yaml:"invariants"`
	Audit      *AuditConfig     `yaml:"audit"`
	Bootstrap  *BootstrapConfig `yaml:"bootstrap"`
	Hooks      *HooksConfig     `yaml:"hooks"`
	root       string           // <project>/.awf, for sidecar/part resolution
	raw        []byte           // the exact config.yaml bytes Load read, for in-place byte edits
}

// Source returns the exact config.yaml bytes Load read. A byte-level editor
// (SetArrayMember, SetArray, SetMappingScalar) reuses these instead of re-reading
// the file, which after a successful Load could only fail on a race.
func (c *Config) Source() []byte { return c.raw }

// InvariantConfig configures language-agnostic invariant backing. A nil
// *InvariantConfig (key absent) means "unchecked"; Disabled is the explicit
// opt-out; a non-empty Sources enables enforcement.
type InvariantConfig struct {
	Disabled bool              `yaml:"disabled"`
	Sources  []InvariantSource `yaml:"sources"`
}

// InvariantSource pairs filename globs (matched against a file's basename) with
// the literal comment marker that prefixes a backing `invariant: <slug>` tag.
type InvariantSource struct {
	Globs  []string `yaml:"globs"`
	Marker string   `yaml:"marker"`
}

// BootstrapConfig configures the rendered .awf/bootstrap.sh singleton (ADR-0040,
// relocated by ADR-0047). A
// nil *BootstrapConfig (key absent) and Enabled false both mean "do not render";
// only Enabled true renders the artifact — a nested enable entry rather than a
// top-level scalar bool (the Alternatives table rejected the bare bool).
type BootstrapConfig struct {
	Enabled bool `yaml:"enabled"`
}

// HooksConfig configures the rendered .awf/hooks/ payload singleton (ADR-0048):
// three inert git-hook payload scripts adopters wire into hook setups they own.
// BootstrapConfig semantics: a nil *HooksConfig (key absent) and Enabled false
// both mean "do not render"; only Enabled true renders the payloads. The key
// reuses the name the schema-4 drop-hooks migration stripped (ADR-0032); the
// legacy array shape never reaches this struct — gated commands migrate first,
// ungated ones fail loudly on the strict parser's type error.
type HooksConfig struct {
	Enabled bool `yaml:"enabled"`
}

// AuditConfig tunes `awf audit` (ADR-0017). A nil *AuditConfig means all
// defaults; within it, a nil slice means "use the default", an explicit empty
// slice means "accept any / disabled" per field. Resolution and defaults live in
// internal/audit (audit.Resolve), which owns the audit domain semantics.
type AuditConfig struct {
	BaseBranch          string      `yaml:"baseBranch"`
	AllowedTypes        []string    `yaml:"allowedTypes"`
	AllowedScopes       []ScopeSpec `yaml:"allowedScopes"`
	SubjectMaxLength    *int        `yaml:"subjectMaxLength"`
	DependencyManifests []string    `yaml:"dependencyManifests"`
	DiffThreshold       *int        `yaml:"diffThreshold"`
	DomainDocStaleness  *bool       `yaml:"domainDocStaleness"`
	UndocumentedDomain  *bool       `yaml:"undocumentedDomain"`
	UncommittedChanges  *bool       `yaml:"uncommittedChanges"`
}

// Load reads <awfDir>/config.yaml with the strict decoder, records awfDir as the
// sidecar/part resolution root, and defaults DocsDir.
func Load(awfDir string) (*Config, error) {
	b, err := os.ReadFile(filepath.Join(awfDir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.root = awfDir
	c.raw = b
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
	b, err := os.ReadFile(filepath.Join(c.root, rel))
	if errors.Is(err, os.ErrNotExist) {
		return Sidecar{}, nil
	}
	if err != nil {
		return Sidecar{}, fmt.Errorf("read sidecar %s: %w", rel, err)
	}
	var s Sidecar
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return Sidecar{}, fmt.Errorf("parse sidecar %s: %w", rel, err)
	}
	return s, nil
}

// HasSidecar reports whether a declaring sidecar file exists for an artifact —
// the presence signal that marks a non-catalog name as an intentional local
// artifact rather than a typo (ADR-0068).
func (c *Config) HasSidecar(kind, name string) (bool, error) {
	var rel string
	if IsSingletonKind(kind) {
		rel = kind + ".yaml"
	} else {
		rel = filepath.Join(kind, name+".yaml")
	}
	_, err := os.Stat(filepath.Join(c.root, rel))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat sidecar %s: %w", rel, err) // coverage-ignore: Stat fails here only on a permission fault a test cannot trigger
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
	for _, d := range c.Domains {
		if err := ValidateDomainName(d); err != nil {
			return err
		}
	}
	if c.Invariants != nil {
		for _, src := range c.Invariants.Sources {
			if src.Marker == "" {
				return fmt.Errorf("invariants source %v has an empty marker; set a literal comment marker (e.g. \"//\", \"#\")", src.Globs)
			}
			if len(src.Globs) == 0 {
				return fmt.Errorf("invariants source with marker %q has no globs; list at least one filename glob (e.g. \"*.go\")", src.Marker)
			}
			for _, g := range src.Globs {
				if err := validateBasenameGlob(g); err != nil {
					return fmt.Errorf("invariants glob: %w", err)
				}
			}
		}
	}
	if c.Audit != nil {
		for _, g := range c.Audit.DependencyManifests {
			if err := validateBasenameGlob(g); err != nil {
				return fmt.Errorf("audit.dependencyManifests: %w", err)
			}
		}
	}
	// Targets: sanity only — the unknown-adapter-name check lives in project.Open
	// (resolveTargets), where the adapter registry is, to keep config free of a
	// project import cycle (ADR-0037).
	if len(c.Targets) == 0 {
		return errors.New("targets must not be empty")
	}
	for _, t := range c.Targets {
		if t == "" || hasPathSep(t) {
			return fmt.Errorf("target %q must be a non-empty name without path separators", t)
		}
	}
	return nil
}

// ValidateDomainName reports whether name is a usable domain key: non-empty and
// free of path separators or "..". Shared by Validate and the `awf add domain`
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
// name (ADR-0068): non-empty, free of path separators or "..", and not in awf's
// reserved "_"-prefixed namespace (which the base templates occupy).
// invariant: local-name-validated
func ValidateArtifactName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s name must not be empty", kind)
	}
	if hasPathSep(name) {
		return fmt.Errorf("%s %q must not contain path separators or \"..\"", kind, name)
	}
	if strings.HasPrefix(name, "_") {
		return fmt.Errorf("%s %q must not start with \"_\" (reserved)", kind, name)
	}
	return nil
}

// hasPathSep reports whether s contains a path separator or a ".." segment — the
// shared reject condition for prefix/target/domain names.
func hasPathSep(s string) bool {
	return strings.ContainsAny(s, "/\\") || strings.Contains(s, "..")
}

// validateBasenameGlob rejects a glob that contains a path separator (it must be
// a filename pattern matched against a basename) or is a malformed pattern.
func validateBasenameGlob(g string) error {
	if strings.Contains(g, "/") {
		return fmt.Errorf("glob %q must be a filename pattern, not a path", g)
	}
	if _, err := filepath.Match(g, "x"); err != nil {
		return fmt.Errorf("glob %q is malformed: %w", g, err)
	}
	return nil
}
