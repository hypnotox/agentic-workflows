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
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"gopkg.in/yaml.v3"
)

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
	Data     map[string]any             `yaml:"data"`
	Sections map[string]SectionOverride `yaml:"sections"`
	Local    bool                       `yaml:"local"`
	// Paths declares a domain's file territory as anchored path globs
	// (ADR-0077); read only from domain sidecars, inert on other kinds.
	Paths []string `yaml:"paths"`
}

// Config is the skeleton config.yaml: global fields plus flat enable arrays.
// Presence of a name in Skills/Agents/Docs enables that artifact; per-artifact
// data/sections/local live in sidecars, not here. Targets is the adapter-runtime
// enable array (default ["claude"]); adapter artifacts render once per entry.
type Config struct {
	Prefix               string                  `yaml:"prefix"`
	DocsDir              string                  `yaml:"docsDir"`
	Vars                 map[string]any          `yaml:"vars"`
	Skills               []string                `yaml:"skills"`
	Agents               []string                `yaml:"agents"`
	Docs                 []string                `yaml:"docs"`
	Domains              []string                `yaml:"domains"`
	Tags                 map[string]string       `yaml:"tags"`
	ContextIgnore        []string                `yaml:"contextIgnore"`
	Targets              []string                `yaml:"targets"`
	CurrentState         *CurrentStateConfig     `yaml:"currentState"`
	Audit                *AuditConfig            `yaml:"audit"`
	Bootstrap            *BootstrapConfig        `yaml:"bootstrap"`
	Hooks                *HooksConfig            `yaml:"hooks"`
	Runner               *RunnerConfig           `yaml:"runner"`
	ProseGate            *ProseGateConfig        `yaml:"proseGate"`
	WorkflowTelemetry    WorkflowTelemetryConfig `yaml:"workflowTelemetry"`
	workflowTelemetrySet bool
	root                 string     // <project>/.awf, for sidecar/part resolution
	raw                  []byte     // the exact config.yaml bytes Load read, for in-place byte edits
	read                 TreeReader // selected filesystem or immutable snapshot universe
	filesystem           bool
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
	Sources           []CurrentStateSource `yaml:"sources"`
	TestGlobs         []string             `yaml:"testGlobs"`
	TopicCoverage     string               `yaml:"topicCoverage"`
	TopicFanout       string               `yaml:"topicFanout"`
	MaxTopicsPerPath  *int                 `yaml:"maxTopicsPerPath"`
	MaxClaimsPerTopic *int                 `yaml:"maxClaimsPerTopic"`
	coverageSet       bool
	fanoutSet         bool
}

// UnmarshalYAML retains severity presence while preserving strict nested field
// validation for the custom-decoded current-state mapping.
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
		case "topicCoverage":
			c.coverageSet = true
			if err := decodeStringScalar(value, &c.TopicCoverage, "currentState.topicCoverage"); err != nil {
				return err
			}
		case "topicFanout":
			c.fanoutSet = true
			if err := decodeStringScalar(value, &c.TopicFanout, "currentState.topicFanout"); err != nil {
				return err
			}
		case "maxTopicsPerPath":
			maximum, err := decodeIntegerScalar(value, "currentState.maxTopicsPerPath")
			if err != nil {
				return err
			}
			c.MaxTopicsPerPath = &maximum
		case "maxClaimsPerTopic":
			maximum, err := decodeIntegerScalar(value, "currentState.maxClaimsPerTopic")
			if err != nil {
				return err
			}
			c.MaxClaimsPerTopic = &maximum
		default:
			return fmt.Errorf("field %s not found in type config.CurrentStateConfig", key)
		}
	}
	return nil
}

// EffectiveMaxTopicsPerPath returns the configured fan-out budget, defaulting
// to eight without materializing that default into the decoded config.
func (c *CurrentStateConfig) EffectiveMaxTopicsPerPath() int {
	if c == nil || c.MaxTopicsPerPath == nil {
		return 8
	}
	return *c.MaxTopicsPerPath
}

// EffectiveMaxClaimsPerTopic returns the configured topic claim-count advisory
// threshold, defaulting to 20 without materializing it into decoded config.
func (c *CurrentStateConfig) EffectiveMaxClaimsPerTopic() int {
	if c == nil || c.MaxClaimsPerTopic == nil {
		return 20
	}
	return *c.MaxClaimsPerTopic
}

func decodeIntegerScalar(node *yaml.Node, field string) (int, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return 0, fmt.Errorf("%s must be an integer scalar", field)
	}
	var value int
	if err := node.Decode(&value); err != nil {
		return 0, fmt.Errorf("%s must be an integer scalar: %w", field, err)
	}
	return value, nil
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

// HooksConfig configures the rendered .awf/hooks/ payload singleton (ADR-0048):
// three inert git-hook payload scripts adopters wire into hook setups they own.
// BootstrapConfig semantics: a nil *HooksConfig (key absent) and Enabled false
// both mean "do not render"; only Enabled true renders the payloads. The key
// reuses the name the schema-4 drop-hooks migration stripped (ADR-0032); the
// legacy array shape never reaches this struct - gated commands migrate first,
// ungated ones fail loudly on the strict parser's type error.
type HooksConfig struct {
	Enabled bool `yaml:"enabled"`
}

// RunnerConfig configures the rendered command-runner singleton `x` (ADR-0101):
// a co-owned file (ADR-0100) whose awf-verb dispatch awf owns and whose project
// verbs live in in-place-editable sections the adopter fills. Like the
// bootstrap/hooks toggles, a nil *RunnerConfig (key absent) and Enabled false both
// mean "do not render"; only Enabled true renders the runner. Additive and
// default-off - no schema-generation migration, and adopters opt in explicitly.
type RunnerConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ProseGateConfig configures `awf prose-gate` (ADR-0119): a presence-level scan
// of every tracked text file for the seven banned typographic punctuation
// substitutes. BootstrapConfig semantics: a nil *ProseGateConfig (key absent)
// and Enabled false both mean "the command exits zero without scanning". The
// default is off because the scan blocks a commit, and a tree that has never
// been swept would fail it on the day it lands.
type ProseGateConfig struct {
	Enabled    bool             `yaml:"enabled"`
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

// WorkflowTelemetryConfig controls resident workflow metrics collection and display.
type WorkflowTelemetryConfig struct {
	Retention   TelemetryRetentionConfig   `yaml:"retention"`
	Widget      TelemetryWidgetConfig      `yaml:"widget"`
	Diagnostics TelemetryDiagnosticsConfig `yaml:"diagnostics"`
}

type TelemetryRetentionConfig struct {
	MaxCompletedEffortAgeDays int `yaml:"maxCompletedEffortAgeDays"`
	MaxCompletedEffortCount   int `yaml:"maxCompletedEffortCount"`
}

type TelemetryWidgetConfig struct {
	Enabled  bool `yaml:"enabled"`
	ShowCost bool `yaml:"showCost"`
}

type TelemetryDiagnosticsConfig struct {
	HeuristicsEnabled      bool                      `yaml:"heuristicsEnabled"`
	MinimumBaselineSamples int                       `yaml:"minimumBaselineSamples"`
	BaselinePercentile     int                       `yaml:"baselinePercentile"`
	Thresholds             TelemetryThresholdsConfig `yaml:"thresholds"`
}

type TelemetryThresholdsConfig struct {
	PhaseReentryCount         int `yaml:"phaseReentryCount"`
	PhaseDurationSeconds      int `yaml:"phaseDurationSeconds"`
	PhaseTokens               int `yaml:"phaseTokens"`
	CompactionCount           int `yaml:"compactionCount"`
	HandoffCount              int `yaml:"handoffCount"`
	ToolFailureCount          int `yaml:"toolFailureCount"`
	GateFailureCount          int `yaml:"gateFailureCount"`
	CacheReadPercentBelow     int `yaml:"cacheReadPercentBelow"`
	SubagentQueueWaitSeconds  int `yaml:"subagentQueueWaitSeconds"`
	ImplementationReworkCount int `yaml:"implementationReworkCount"`
}

// DefaultWorkflowTelemetryConfig returns the complete schema default.
func DefaultWorkflowTelemetryConfig() WorkflowTelemetryConfig {
	return WorkflowTelemetryConfig{
		Retention: TelemetryRetentionConfig{MaxCompletedEffortAgeDays: 90, MaxCompletedEffortCount: 100},
		Widget:    TelemetryWidgetConfig{Enabled: true, ShowCost: true},
		Diagnostics: TelemetryDiagnosticsConfig{
			HeuristicsEnabled: true, MinimumBaselineSamples: 10, BaselinePercentile: 95,
			Thresholds: TelemetryThresholdsConfig{
				PhaseReentryCount: 2, PhaseDurationSeconds: 14400, PhaseTokens: 200000,
				CompactionCount: 3, HandoffCount: 3, ToolFailureCount: 3, GateFailureCount: 2,
				CacheReadPercentBelow: 10, SubagentQueueWaitSeconds: 60, ImplementationReworkCount: 2,
			},
		},
	}
}

// AuditConfig tunes `awf audit` (ADR-0017). A nil *AuditConfig means all
// defaults; within it, a nil slice means "use the default", an explicit empty
// slice means "accept any / disabled" per field. Resolution and defaults live in
// internal/audit (audit.Resolve), which owns the audit domain semantics.
type AuditConfig struct {
	AllowedTypes        []string    `yaml:"allowedTypes"`
	AllowedScopes       []ScopeSpec `yaml:"allowedScopes"`
	SubjectMaxLength    *int        `yaml:"subjectMaxLength"`
	DependencyManifests []string    `yaml:"dependencyManifests"`
	DiffThreshold       *int        `yaml:"diffThreshold"`
	DomainDocStaleness  *bool       `yaml:"domainDocStaleness"`
	DomainCodeStaleness *bool       `yaml:"domainCodeStaleness"`
	UndocumentedDomain  *bool       `yaml:"undocumentedDomain"`
	PlainPunctuation    *bool       `yaml:"plainPunctuation"`
	UncommittedChanges  *bool       `yaml:"uncommittedChanges"`
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
	c := Config{WorkflowTelemetry: DefaultWorkflowTelemetryConfig()}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	var source yaml.Node
	if err := yaml.Unmarshal(b, &source); err != nil { // coverage-ignore: the strict decoder accepted the same YAML bytes above
		return nil, fmt.Errorf("parse config presence: %w", err)
	}
	if len(source.Content) > 0 {
		_, index := mapValue(source.Content[0], "workflowTelemetry")
		c.workflowTelemetrySet = index >= 0
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
	if !c.workflowTelemetrySet && c.WorkflowTelemetry == (WorkflowTelemetryConfig{}) {
		c.WorkflowTelemetry = DefaultWorkflowTelemetryConfig()
	}
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
	if c.CurrentState != nil {
		if !c.CurrentState.coverageSet {
			c.CurrentState.TopicCoverage = "error"
		}
		if !c.CurrentState.fanoutSet {
			c.CurrentState.TopicFanout = "warn"
		}
		for _, setting := range []struct{ name, severity string }{
			{"topicCoverage", c.CurrentState.TopicCoverage},
			{"topicFanout", c.CurrentState.TopicFanout},
		} {
			name, severity := setting.name, setting.severity
			if severity != "error" && severity != "warn" && severity != "off" {
				return fmt.Errorf("currentState.%s must be error, warn, or off; got %q", name, severity)
			}
		}
		for _, maximum := range []struct {
			name  string
			value *int
		}{
			{"maxTopicsPerPath", c.CurrentState.MaxTopicsPerPath},
			{"maxClaimsPerTopic", c.CurrentState.MaxClaimsPerTopic},
		} {
			if maximum.value != nil && *maximum.value <= 0 {
				return fmt.Errorf("currentState.%s must be positive; got %d", maximum.name, *maximum.value)
			}
		}
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
	t := c.WorkflowTelemetry
	for _, setting := range []struct {
		name      string
		value     int
		allowZero bool
	}{
		{"retention.maxCompletedEffortAgeDays", t.Retention.MaxCompletedEffortAgeDays, true},
		{"retention.maxCompletedEffortCount", t.Retention.MaxCompletedEffortCount, true},
		{"diagnostics.minimumBaselineSamples", t.Diagnostics.MinimumBaselineSamples, false},
		{"diagnostics.thresholds.phaseReentryCount", t.Diagnostics.Thresholds.PhaseReentryCount, false},
		{"diagnostics.thresholds.phaseDurationSeconds", t.Diagnostics.Thresholds.PhaseDurationSeconds, false},
		{"diagnostics.thresholds.phaseTokens", t.Diagnostics.Thresholds.PhaseTokens, false},
		{"diagnostics.thresholds.compactionCount", t.Diagnostics.Thresholds.CompactionCount, false},
		{"diagnostics.thresholds.handoffCount", t.Diagnostics.Thresholds.HandoffCount, false},
		{"diagnostics.thresholds.toolFailureCount", t.Diagnostics.Thresholds.ToolFailureCount, false},
		{"diagnostics.thresholds.gateFailureCount", t.Diagnostics.Thresholds.GateFailureCount, false},
		{"diagnostics.thresholds.subagentQueueWaitSeconds", t.Diagnostics.Thresholds.SubagentQueueWaitSeconds, false},
		{"diagnostics.thresholds.implementationReworkCount", t.Diagnostics.Thresholds.ImplementationReworkCount, false},
	} {
		if setting.value < 0 || (!setting.allowZero && setting.value == 0) {
			return fmt.Errorf("workflowTelemetry.%s must be %s; got %d", setting.name, map[bool]string{true: "non-negative", false: "positive"}[setting.allowZero], setting.value)
		}
	}
	if t.Diagnostics.BaselinePercentile < 1 || t.Diagnostics.BaselinePercentile > 100 {
		return fmt.Errorf("workflowTelemetry.diagnostics.baselinePercentile must be between 1 and 100; got %d", t.Diagnostics.BaselinePercentile)
	}
	if p := t.Diagnostics.Thresholds.CacheReadPercentBelow; p < 0 || p > 100 {
		return fmt.Errorf("workflowTelemetry.diagnostics.thresholds.cacheReadPercentBelow must be between 0 and 100; got %d", p)
	}
	if c.Audit != nil {
		for _, g := range c.Audit.DependencyManifests {
			if err := validatePathGlob(g); err != nil {
				return fmt.Errorf("audit.dependencyManifests: %w", err)
			}
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
// touches-state: config/configuration:local-name-validated - local skill/agent name charset validation; proof in config_test.go
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
// touches-state: config/configuration:local-doc-name-path-validated - path-aware local doc name validation; proof in docname_test.go
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
