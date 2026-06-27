// Package config loads and validates the per-project .awf/ configuration:
// a skeleton config.yaml plus per-target sidecar YAMLs and convention parts.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
// Presence of a name in Skills/Agents/Docs/Hooks enables that target; per-target
// data/sections/local live in sidecars, not here.
type Config struct {
	Prefix     string           `yaml:"prefix"`
	DocsDir    string           `yaml:"docsDir"`
	Vars       map[string]any   `yaml:"vars"`
	Skills     []string         `yaml:"skills"`
	Agents     []string         `yaml:"agents"`
	Hooks      []string         `yaml:"hooks"`
	Docs       []string         `yaml:"docs"`
	Domains    []string         `yaml:"domains"`
	Invariants *InvariantConfig `yaml:"invariants"`
	Audit      *AuditConfig     `yaml:"audit"`
	root       string           // <project>/.awf, for sidecar/part resolution
}

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

// AuditConfig tunes `awf audit` (ADR-0017). A nil *AuditConfig means all
// defaults; within it, a nil slice means "use the default", an explicit empty
// slice means "accept any / disabled" per field (see AuditSettings).
type AuditConfig struct {
	BaseBranch          string   `yaml:"baseBranch"`
	AllowedTypes        []string `yaml:"allowedTypes"`
	AllowedScopes       []string `yaml:"allowedScopes"`
	SubjectMaxLength    *int     `yaml:"subjectMaxLength"`
	DependencyManifests []string `yaml:"dependencyManifests"`
	DiffThreshold       *int     `yaml:"diffThreshold"`
	DomainDocStaleness  *bool    `yaml:"domainDocStaleness"`
	UndocumentedDomain  *bool    `yaml:"undocumentedDomain"`
}

// AuditSettings resolves the effective audit settings, applying defaults.
// Returned slices/ints are ready for internal/audit to consume directly.
func (c *Config) AuditSettings() (baseBranch string, allowedTypes, allowedScopes, dependencyManifests []string, subjectMax, diffThreshold int, domainDocStaleness, undocumentedDomain bool) {
	a := c.Audit
	baseBranch = "main"
	allowedTypes = defaultAllowedTypes()
	dependencyManifests = defaultDependencyManifests()
	subjectMax, diffThreshold = 72, 400
	domainDocStaleness, undocumentedDomain = true, true
	if a == nil {
		return
	}
	if a.BaseBranch != "" {
		baseBranch = a.BaseBranch
	}
	if a.AllowedTypes != nil { // explicit (incl. empty = accept any)
		allowedTypes = a.AllowedTypes
	}
	allowedScopes = a.AllowedScopes // nil default = accept any
	if a.DependencyManifests != nil {
		dependencyManifests = a.DependencyManifests
	}
	if a.SubjectMaxLength != nil {
		subjectMax = *a.SubjectMaxLength
	}
	if a.DiffThreshold != nil {
		diffThreshold = *a.DiffThreshold
	}
	if a.DomainDocStaleness != nil {
		domainDocStaleness = *a.DomainDocStaleness
	}
	if a.UndocumentedDomain != nil {
		undocumentedDomain = *a.UndocumentedDomain
	}
	return
}

func defaultAllowedTypes() []string {
	return []string{"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert", "style", "test"}
}

func defaultDependencyManifests() []string {
	return []string{
		"go.mod", "package.json", "pyproject.toml", "setup.py", "requirements*.txt",
		"Cargo.toml", "Gemfile", "*.gemspec", "composer.json", "pom.xml", "build.gradle",
		"build.gradle.kts", "*.csproj", "Directory.Packages.props", "mix.exs",
		"Package.swift", "pubspec.yaml", "*.cabal", "package.yaml",
	}
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
	if c.DocsDir == "" {
		c.DocsDir = "docs"
	}
	return &c, nil
}

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

// IsSingletonKind reports whether kind is an always-on singleton whose sidecar lives at
// <root>/<kind>.yaml and whose parts live under <root>/parts/<kind>/ (ADR-0021).
func IsSingletonKind(kind string) bool {
	switch kind {
	case "agents-doc", "adr-readme", "adr-template", "plans-readme":
		return true
	}
	return false
}

// PartPath returns the convention part path for a section of a target.
func (c *Config) PartPath(kind, target, section string) string {
	if IsSingletonKind(kind) {
		return filepath.Join(c.root, "parts", kind, section+".md")
	}
	return filepath.Join(c.root, kind, "parts", target, section+".md")
}

func (c *Config) Validate() error {
	if c.Prefix == "" {
		return errors.New("prefix must not be empty")
	}
	if strings.ContainsAny(c.Prefix, "/\\") || strings.Contains(c.Prefix, "..") {
		return fmt.Errorf("prefix %q must not contain path separators", c.Prefix)
	}
	if strings.HasPrefix(c.DocsDir, "/") || strings.Contains(c.DocsDir, "..") {
		return fmt.Errorf("docsDir %q must be a relative path without \"..\"", c.DocsDir)
	}
	for _, d := range c.Domains {
		if d == "" {
			return errors.New("domain name must not be empty")
		}
		if strings.ContainsAny(d, "/\\") || strings.Contains(d, "..") {
			return fmt.Errorf("domain %q must not contain path separators or \"..\"", d)
		}
	}
	if c.Invariants != nil {
		for _, src := range c.Invariants.Sources {
			if src.Marker == "" {
				return fmt.Errorf("invariants source %v has an empty marker; set a literal comment marker (e.g. \"//\", \"#\")", src.Globs)
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
	return nil
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
