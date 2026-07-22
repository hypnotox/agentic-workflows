package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"gopkg.in/yaml.v3"
)

// writeConfig writes config.yaml into a fresh awf dir and returns that dir.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// invariant: config/configuration:duplicate-target-rejected
func TestConfigRejectsDuplicateTargets(t *testing.T) {
	cfg, err := Load(writeConfig(t, "prefix: awf\nskills: []\nagents: []\ntargets: [claude, claude]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate target") {
		t.Fatalf("Validate = %v", err)
	}
}

func TestLoadParsesSkeletonFields(t *testing.T) {
	dir := writeConfig(t, `prefix: example
vars:
  testCmd: go test ./...
skills:
  - tdd
  - bugfix
agents:
  - code-reviewer
docs:
  - architecture
`)
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Prefix != "example" {
		t.Errorf("prefix = %q", c.Prefix)
	}
	if c.Vars["testCmd"] != "go test ./..." {
		t.Errorf("vars.testCmd = %v", c.Vars["testCmd"])
	}
	if strings.Join(c.Skills, ",") != "tdd,bugfix" {
		t.Errorf("skills = %v", c.Skills)
	}
	if strings.Join(c.Agents, ",") != "code-reviewer" {
		t.Errorf("agents = %v", c.Agents)
	}
	if strings.Join(c.Docs, ",") != "architecture" {
		t.Errorf("docs = %v", c.Docs)
	}
}

func TestParseRetainsSuppliedSourceAndDefaults(t *testing.T) {
	body := []byte("prefix: example\nskills: []\n")
	c, err := Parse("staged/.awf", body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if string(c.Source()) != string(body) || c.DocsDir != "docs" || strings.Join(c.Targets, ",") != "claude" {
		t.Errorf("Parse = %+v, source %q", c, c.Source())
	}
	if _, err := Parse("staged/.awf", []byte("unknown: true\n")); err == nil {
		t.Error("Parse must retain strict decoding")
	}
}

func TestLoadRetainsSource(t *testing.T) {
	// Load keeps the exact bytes it read, so a byte-level editor can reuse them
	// instead of re-reading config.yaml (and defending against a read that cannot
	// fail after Load already succeeded).
	body := "prefix: example\nskills:\n  - tdd\n"
	dir := writeConfig(t, body)
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(c.Source()) != body {
		t.Errorf("Source() = %q, want %q", c.Source(), body)
	}
}

// invariant: config/configuration:enable-arrays
func TestEnableListsAreArrays(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n  - bugfix\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Skills) != 2 || c.Skills[0] != "tdd" || c.Skills[1] != "bugfix" {
		t.Errorf("skills did not parse to []string{tdd,bugfix}: %v", c.Skills)
	}
	// A per-target data/sections/local key at the root is rejected (KnownFields).
	for _, bad := range []string{"data", "sections", "local"} {
		d := writeConfig(t, "prefix: example\n"+bad+": {}\n")
		if _, err := Load(d); err == nil {
			t.Errorf("expected a root %q key to be rejected", bad)
		}
	}
	// The map enable shape (name: {}) no longer parses as a string array.
	d := writeConfig(t, "prefix: example\nskills:\n  tdd: {}\n")
	if _, err := Load(d); err == nil {
		t.Error("expected map-shaped skills to be rejected (must be a string array)")
	}
}

// invariant: config/configuration:awf-config-root
// TestLoadReadsTreeRoot pins the config root to .awf/config.yaml and
// co-owns (with the migrate package's TestLegacyReadOnlyInMigrate, ADR-0010
// inv: legacy-read-isolation) the exemption that ONLY internal/migrate reads the
// legacy .claude/awf.yaml: the import-graph assertion below scans the repo and
// fails if any non-migrate, non-test source references the legacy path.
func TestLoadReadsTreeRoot(t *testing.T) {
	root := t.TempDir()
	awfDir := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awfDir, "config.yaml"), []byte("prefix: tree-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A legacy decoy sibling; Load must ignore it.
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"), []byte("prefix: legacy-decoy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(awfDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Prefix != "tree-root" {
		t.Errorf("Load read the wrong file: prefix = %q, want tree-root", c.Prefix)
	}

	repo := testsupport.RepoRoot(t)
	legacyRefs := scanLegacyRefs(t, repo)
	if len(legacyRefs) != 0 {
		t.Errorf("only internal/migrate may reference the legacy .claude/awf.yaml; found refs in: %v", legacyRefs)
	}
}

// scanLegacyRefs returns non-test, non-migrate Go files that mention the legacy
// awf.yaml filename. The repo-walk boundary (hidden trees, nested checkouts,
// test files) is owned by testsupport.WalkRepoSources.
func scanLegacyRefs(t *testing.T, repo string) []string {
	t.Helper()
	var hits []string
	testsupport.WalkRepoSources(t, repo, func(rel string, body []byte) {
		if strings.HasPrefix(rel, "internal/migrate/") {
			return
		}
		if strings.Contains(string(body), "awf.yaml") {
			hits = append(hits, rel)
		}
	})
	return hits
}

func TestSidecarReadsDataSectionsLocal(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	sidecar := "data:\n  foo: bar\nsections:\n  notes:\n    drop: true\nlocal: false\n"
	if err := os.WriteFile(filepath.Join(dir, "skills", "tdd.yaml"), []byte(sidecar), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sc, err := c.Sidecar("skills", "tdd")
	if err != nil {
		t.Fatalf("Sidecar: %v", err)
	}
	if sc.Data["foo"] != "bar" {
		t.Errorf("sidecar data.foo = %v", sc.Data["foo"])
	}
	if !sc.Sections["notes"].Drop {
		t.Errorf("sidecar sections.notes.drop should be true")
	}
}

func TestSidecarReadsDomainPaths(t *testing.T) {
	dir := writeConfig(t, "prefix: example\ndomains:\n  - tooling\n")
	if err := os.MkdirAll(filepath.Join(dir, "domains"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "domains", "tooling.yaml"), []byte("paths:\n  - cmd/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sc, err := c.Sidecar("domains", "tooling")
	if err != nil {
		t.Fatalf("Sidecar: %v", err)
	}
	if len(sc.Paths) != 1 || sc.Paths[0] != "cmd/**" {
		t.Errorf("sidecar paths = %v, want [cmd/**]", sc.Paths)
	}
}

// invariant: rendering/render-engine:sidecar-optional
func TestSidecarAbsentIsEmpty(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := c.Sidecar("skills", "tdd")
	if err != nil {
		t.Fatalf("absent sidecar should be empty, not an error: %v", err)
	}
	if sc.Data != nil || sc.Sections != nil || sc.Local {
		t.Errorf("absent sidecar should be the zero Sidecar, got %#v", sc)
	}
}

// A stale schema-1 sidecar carrying replaceWith fails closed at the strict
// decoder (the migration converts it before load); see ADR-0015.
func TestSidecarRejectsReplaceWith(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "sections:\n  notes:\n    replaceWith: parts/x.md\n"
	if err := os.WriteFile(filepath.Join(dir, "skills", "tdd.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := Load(dir)
	_, err := c.Sidecar("skills", "tdd")
	// invariant: config/configuration:no-replacewith
	if err == nil || !strings.Contains(err.Error(), "replaceWith") {
		t.Errorf("expected a strict-decoder error mentioning replaceWith, got: %v", err)
	}
}

func TestSidecarRejectsUnknownKey(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "tdd.yaml"), []byte("dat: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := Load(dir)
	if _, err := c.Sidecar("skills", "tdd"); err == nil {
		t.Error("expected error for unknown sidecar key 'dat'")
	}
}

func TestLoadMissingConfigErrors(t *testing.T) {
	dir := t.TempDir() // no config.yaml written
	// Missing config.yaml → the no-project hint (ADR-0076 Decision 5).
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "not an awf project (run `awf init`)") {
		t.Fatalf("missing: %v", err)
	}
	// Present but unreadable (a directory at the path) → the plain read wrap.
	if err := os.Mkdir(filepath.Join(dir, "config.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "read config") || strings.Contains(err.Error(), "awf init") {
		t.Fatalf("unreadable-but-present: %v", err)
	}
}

// invariant: rendering/render-engine:sidecar-optional
func TestSidecarAgentsDocSingleton(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Absent agents-doc.yaml resolves via the singleton branch to a zero Sidecar.
	sc, err := c.Sidecar("agents-doc", "")
	if err != nil {
		t.Fatalf("absent agents-doc sidecar should be empty, not an error: %v", err)
	}
	if sc.Data != nil || sc.Sections != nil || sc.Local {
		t.Errorf("absent agents-doc sidecar should be the zero Sidecar, got %#v", sc)
	}
	// Present singleton is read from <root>/agents-doc.yaml (not a kind subdir).
	if err := os.WriteFile(filepath.Join(dir, "agents-doc.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err = c.Sidecar("agents-doc", "")
	if err != nil {
		t.Fatalf("Sidecar agents-doc: %v", err)
	}
	if !sc.Local {
		t.Errorf("agents-doc sidecar local = %v, want true", sc.Local)
	}
}

func TestSidecarReadErrorWhenPathIsDir(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskills:\n  - tdd\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A directory squatting on the expected sidecar file path makes ReadFile
	// fail with a non-ErrNotExist error (EISDIR), exercising the wrap branch.
	if err := os.MkdirAll(filepath.Join(dir, "skills", "tdd.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Sidecar("skills", "tdd"); err == nil {
		t.Fatal("expected a read error when the sidecar path is a directory")
	} else if !strings.Contains(err.Error(), "read sidecar") {
		t.Errorf("error = %v, want it wrapped with \"read sidecar\"", err)
	}
}

func TestPartPath(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
	c, _ := Load(dir)
	if got := c.PartPath("skills", "debugging", "surfaces"); got != filepath.Join(dir, "skills", "parts", "debugging", "surfaces.md") {
		t.Errorf("PartPath skills = %q", got)
	}
	if got := c.PartPath("agents-doc", "", "identity"); got != filepath.Join(dir, "parts", "agents-doc", "identity.md") {
		t.Errorf("PartPath agents-doc = %q", got)
	}
}

func TestValidateRejectsEmptyPrefix(t *testing.T) {
	dir := writeConfig(t, "prefix: \"\"\nskills: []\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err == nil {
		t.Errorf("expected error for empty prefix")
	}
}

func TestValidateRejectsPathInPrefix(t *testing.T) {
	cases := []string{"../evil", "foo/bar", "a\\b"}
	for _, prefix := range cases {
		dir := writeConfig(t, "prefix: "+prefix+"\nskills: []\n")
		c, err := Load(dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := c.Validate(); err == nil {
			t.Errorf("expected error for prefix %q containing path separator", prefix)
		}
	}
}

// invariant: config/configuration:domain-name-validated
func TestValidateRejectsBadDomainName(t *testing.T) {
	for _, bad := range []string{"", "../evil", "foo/bar", "a\\b"} {
		c := &Config{Prefix: "x", DocsDir: "docs", Domains: []string{bad}}
		if err := c.Validate(); err == nil {
			t.Errorf("expected error for domain name %q", bad)
		}
	}
	ok := &Config{Prefix: "x", DocsDir: "docs", Targets: []string{"claude"}, Domains: []string{"rendering", "config"}}
	if err := ok.Validate(); err != nil {
		t.Errorf("clean domain names should validate, got: %v", err)
	}
}

func TestLoadRejectsUnknownTopLevelKey(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nskils: []\n")
	if _, err := Load(dir); err == nil {
		t.Errorf("expected error for unknown top-level key 'skils'")
	}
}

// invariant: config/configuration:targets-default-claude
func TestTargetsDefaultAndValidation(t *testing.T) {
	// An absent targets: key loads as ["claude"] (the unknown-name check itself
	// lives in project.Open/resolveTargets - config stays registry-free).
	dir := writeConfig(t, "prefix: example\nskills: []\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Targets) != 1 || c.Targets[0] != "claude" {
		t.Errorf("absent targets should default to [claude], got %v", c.Targets)
	}
	// An explicitly-empty list is rejected by Validate.
	empty := &Config{Prefix: "x", DocsDir: "docs", Targets: []string{}}
	if err := empty.Validate(); err == nil {
		t.Error("expected empty targets list to be rejected")
	}
	// A path-separator name is rejected by Validate.
	bad := &Config{Prefix: "x", DocsDir: "docs", Targets: []string{"a/b"}}
	if err := bad.Validate(); err == nil {
		t.Error("expected path-separator target name to be rejected")
	}
}

// invariant: config/configuration:docsdir-default
func TestDocsDirDefaultsToDocs(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want \"docs\"", c.DocsDir)
	}
}

func TestDocsDirExplicitValue(t *testing.T) {
	dir := writeConfig(t, "prefix: example\ndocsDir: documentation\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DocsDir != "documentation" {
		t.Errorf("DocsDir = %q, want \"documentation\"", c.DocsDir)
	}
}

func TestDocsDirRejectsEscapingPath(t *testing.T) {
	c := &Config{Prefix: "example", DocsDir: "../escape"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for escaping docsDir")
	}
}

// invariant: config/configuration:topic-claim-budget-configured
func TestCurrentStateDefaultsAndPresence(t *testing.T) {
	absent, err := Parse("staged/.awf", []byte("prefix: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if absent.CurrentState != nil || absent.CurrentState.EffectiveMaxTopicsPerPath() != 8 || absent.CurrentState.EffectiveMaxClaimsPerTopic() != 20 {
		t.Fatalf("absent currentState = %#v, effective topic max = %d, effective claim max = %d", absent.CurrentState, absent.CurrentState.EffectiveMaxTopicsPerPath(), absent.CurrentState.EffectiveMaxClaimsPerTopic())
	}

	cfg, err := Parse("staged/.awf", []byte("prefix: x\ncurrentState: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentState == nil {
		t.Fatal("present currentState decoded as nil")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentState.TopicCoverage != "error" || cfg.CurrentState.TopicFanout != "warn" || cfg.CurrentState.MaxTopicsPerPath != nil || cfg.CurrentState.MaxClaimsPerTopic != nil || cfg.CurrentState.EffectiveMaxTopicsPerPath() != 8 || cfg.CurrentState.EffectiveMaxClaimsPerTopic() != 20 {
		t.Errorf("defaults = %#v, effective topic max = %d, effective claim max = %d", cfg.CurrentState, cfg.CurrentState.EffectiveMaxTopicsPerPath(), cfg.CurrentState.EffectiveMaxClaimsPerTopic())
	}

	max := 3
	claimMax := 7
	direct := &Config{Prefix: "x", DocsDir: "docs", Targets: []string{"claude"}, CurrentState: &CurrentStateConfig{MaxTopicsPerPath: &max, MaxClaimsPerTopic: &claimMax}}
	if err := direct.Validate(); err != nil {
		t.Fatal(err)
	}
	if direct.CurrentState.MaxTopicsPerPath != &max || direct.CurrentState.EffectiveMaxTopicsPerPath() != 3 || direct.CurrentState.MaxClaimsPerTopic != &claimMax || direct.CurrentState.EffectiveMaxClaimsPerTopic() != 7 {
		t.Errorf("explicit maximum was replaced: topics=%#v claims=%#v", direct.CurrentState.MaxTopicsPerPath, direct.CurrentState.MaxClaimsPerTopic)
	}
}

func TestCurrentStateSeverityValidation(t *testing.T) {
	for _, field := range []string{"topicCoverage", "topicFanout"} {
		for _, value := range []string{"error", "warn", "off"} {
			t.Run(field+"_"+value, func(t *testing.T) {
				body := "prefix: x\ncurrentState:\n  " + field + ": " + value + "\n"
				cfg, err := Parse("staged/.awf", []byte(body))
				if err != nil {
					t.Fatal(err)
				}
				if err := cfg.Validate(); err != nil {
					t.Errorf("legal severity rejected: %v", err)
				}
			})
		}
	}
	for _, tc := range []struct{ field, value string }{{"topicCoverage", "fatal"}, {"topicFanout", "quiet"}, {"topicCoverage", "''"}, {"topicFanout", "''"}} {
		t.Run(tc.field+"_invalid", func(t *testing.T) {
			body := "prefix: x\ncurrentState:\n  " + tc.field + ": " + tc.value + "\n"
			cfg, err := Parse("staged/.awf", []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Errorf("Validate = %v", err)
			}
		})
	}
}

// invariant: config/configuration:testglobs-anchored-validated
func TestCurrentStateStrictValidation(t *testing.T) {
	valid := `prefix: x
currentState:
  sources:
    - globs: ['**/*.go']
      marker: '//'
      close: '*/'
  testGlobs: ['**/*_test.go']
  maxTopicsPerPath: 4
  maxClaimsPerTopic: 20
`
	cfg, err := Parse("staged/.awf", []byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid currentState rejected: %v", err)
	}

	for _, tc := range []struct {
		name, fragment, want string
	}{
		{"zero maximum", "  maxTopicsPerPath: 0\n", "must be positive"},
		{"negative maximum", "  maxTopicsPerPath: -1\n", "must be positive"},
		{"zero claim maximum", "  maxClaimsPerTopic: 0\n", "must be positive"},
		{"negative claim maximum", "  maxClaimsPerTopic: -1\n", "must be positive"},
		{"empty source globs", "  sources:\n    - globs: []\n      marker: '//'\n", "has no globs"},
		{"duplicate source glob", "  sources:\n    - globs: ['**/*.go', '**/*.go']\n      marker: '//'\n", "duplicate glob"},
		{"empty source glob", "  sources:\n    - globs: ['']\n      marker: '//'\n", "empty"},
		{"malformed source glob", "  sources:\n    - globs: ['[']\n      marker: '//'\n", "malformed"},
		{"empty marker", "  sources:\n    - globs: ['**/*.go']\n      marker: ''\n", "empty marker"},
		{"empty close", "  sources:\n    - globs: ['**/*.go']\n      marker: '//'\n      close: ''\n", "empty close"},
		{"duplicate test glob", "  testGlobs: ['**/*_test.go', '**/*_test.go']\n", "duplicate glob"},
		{"empty test glob", "  testGlobs: ['']\n", "empty"},
		{"malformed test glob", "  testGlobs: ['[']\n", "malformed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := Parse("staged/.awf", []byte("prefix: x\ncurrentState:\n"+tc.fragment))
			if err != nil {
				t.Fatal(err)
			}
			if err := parsed.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate = %v, want error containing %q", err, tc.want)
			}
		})
	}

	for _, tc := range []struct{ body, want string }{
		{"prefix: x\ncurrentState:\n  unknown: true\n", "unknown"},
		{"prefix: x\ncurrentState:\n  sources:\n    - globs: ['**/*.go']\n      marker: '//'\n      unknown: true\n", "unknown"},
		{"prefix: x\ncurrentState:\n  topicCoverage: warn\n  topicCoverage: error\n", "already set"},
		{"prefix: x\ncurrentState:\n  maxClaimsPerTopic: 20\n  maxClaimsPerTopic: 21\n", "already set"},
		{"prefix: x\ncurrentState:\n  sources:\n    - globs: ['**/*.go']\n      marker: '//'\n      marker: '#'\n", "already set"},
	} {
		if _, err := Parse("staged/.awf", []byte(tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("strict nested field was accepted: %v", err)
		}
	}
}

func TestCurrentStateMaximumIntegerOverflow(t *testing.T) {
	for _, field := range []string{"maxTopicsPerPath", "maxClaimsPerTopic"} {
		node := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: field},
			{Kind: yaml.ScalarNode, Tag: "!!int", Value: "999999999999999999999999999999999999"},
		}}
		var cfg CurrentStateConfig
		if err := cfg.UnmarshalYAML(node); err == nil || !strings.Contains(err.Error(), "integer scalar") {
			t.Fatalf("UnmarshalYAML(%s) = %v", field, err)
		}
	}
}

func TestCurrentStateMappingsRequired(t *testing.T) {
	for _, body := range []string{
		"prefix: x\ncurrentState: not-a-map\n",
		"prefix: x\ncurrentState:\n  sources: [not-a-map]\n",
	} {
		if _, err := Parse("staged/.awf", []byte(body)); err == nil || !strings.Contains(err.Error(), "must be a mapping") {
			t.Errorf("Parse = %v", err)
		}
	}
}

func TestCurrentStateRejectsNonStringScalars(t *testing.T) {
	fields := []struct {
		name, yaml, want string
	}{
		{"source_glob", "  sources:\n    - globs: [%s]\n", "currentState source.globs[0] must be a string scalar"},
		{"marker", "  sources:\n    - marker: %s\n", "currentState source.marker must be a string scalar"},
		{"close", "  sources:\n    - close: %s\n", "currentState source.close must be a string scalar"},
		{"test_glob", "  testGlobs: [%s]\n", "currentState.testGlobs[0] must be a string scalar"},
		{"topic_coverage", "  topicCoverage: %s\n", "currentState.topicCoverage must be a string scalar"},
		{"topic_fanout", "  topicFanout: %s\n", "currentState.topicFanout must be a string scalar"},
	}
	values := []struct{ name, yaml string }{
		{"numeric", "123"},
		{"boolean", "true"},
		{"null", "null"},
	}
	for _, field := range fields {
		for _, value := range values {
			t.Run(field.name+"_"+value.name, func(t *testing.T) {
				body := "prefix: x\ncurrentState:\n" + fmt.Sprintf(field.yaml, value.yaml)
				_, err := Parse("staged/.awf", []byte(body))
				if err == nil || !strings.Contains(err.Error(), field.want) {
					t.Fatalf("Parse = %v, want error containing %q", err, field.want)
				}
			})
		}
	}
}

func TestCurrentStateRejectsWrongValueTypes(t *testing.T) {
	for _, body := range []string{
		"prefix: x\ncurrentState:\n  testGlobs: {}\n",
		"prefix: x\ncurrentState:\n  topicCoverage: []\n",
		"prefix: x\ncurrentState:\n  topicFanout: []\n",
		"prefix: x\ncurrentState:\n  maxTopicsPerPath: null\n",
		"prefix: x\ncurrentState:\n  maxTopicsPerPath: nope\n",
		"prefix: x\ncurrentState:\n  maxTopicsPerPath: true\n",
		"prefix: x\ncurrentState:\n  maxTopicsPerPath: 1.5\n",
		"prefix: x\ncurrentState:\n  maxTopicsPerPath: 999999999999999999999999999999999999\n",
		"prefix: x\ncurrentState:\n  maxClaimsPerTopic: null\n",
		"prefix: x\ncurrentState:\n  maxClaimsPerTopic: nope\n",
		"prefix: x\ncurrentState:\n  maxClaimsPerTopic: true\n",
		"prefix: x\ncurrentState:\n  maxClaimsPerTopic: 1.5\n",
		"prefix: x\ncurrentState:\n  maxClaimsPerTopic: 999999999999999999999999999999999999\n",
		"prefix: x\ncurrentState:\n  sources:\n    - globs: {}\n",
		"prefix: x\ncurrentState:\n  sources:\n    - marker: []\n",
		"prefix: x\ncurrentState:\n  sources:\n    - close: []\n",
	} {
		if _, err := Parse("staged/.awf", []byte(body)); err == nil {
			t.Errorf("wrong currentState value type was accepted:\n%s", body)
		}
	}
}

func TestAuditDependencyManifestValidation(t *testing.T) {
	ok := &Config{Prefix: "x", DocsDir: "docs", Targets: []string{"claude"}, Audit: &AuditConfig{
		DependencyManifests: []string{"go.mod", "**/*.csproj", "src/go.mod"},
	}}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid manifest globs (path globs included, ADR-0077) rejected: %v", err)
	}
	bad := &Config{Prefix: "x", DocsDir: "docs", Audit: &AuditConfig{
		DependencyManifests: []string{"["},
	}}
	if err := bad.Validate(); err == nil {
		t.Error("expected malformed manifest glob to be rejected")
	}
}

func TestBootstrapConfigDecode(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nbootstrap:\n  enabled: true\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Bootstrap == nil || !c.Bootstrap.Enabled {
		t.Errorf("bootstrap = %+v, want enabled true", c.Bootstrap)
	}

	absent := writeConfig(t, "prefix: example\n")
	c2, err := Load(absent)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c2.Bootstrap != nil {
		t.Errorf("bootstrap = %+v, want nil when key absent", c2.Bootstrap)
	}
}

func TestHooksConfigDecode(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nhooks:\n  enabled: true\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Hooks == nil || !c.Hooks.Enabled {
		t.Errorf("hooks = %+v, want enabled true", c.Hooks)
	}

	absent := writeConfig(t, "prefix: example\n")
	c2, err := Load(absent)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c2.Hooks != nil {
		t.Errorf("hooks = %+v, want nil when key absent", c2.Hooks)
	}

	// The legacy pre-ADR-0032 array shape must fail loudly on the strict
	// parser, never silently misparse (ADR-0048).
	legacy := writeConfig(t, "prefix: example\nhooks:\n  - pre-commit\n")
	if _, err := Load(legacy); err == nil {
		t.Error("expected legacy hooks array shape to be rejected")
	}
}

func TestPathHelpers(t *testing.T) {
	root := filepath.Join("x", "y")
	if got, want := RootDir(root), filepath.Join("x", "y", ".awf"); got != want {
		t.Errorf("RootDir = %q, want %q", got, want)
	}
	if got, want := ConfigPath(root), filepath.Join("x", "y", ".awf", "config.yaml"); got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
	if got, want := LockPath(root), filepath.Join("x", "y", ".awf", "awf.lock"); got != want {
		t.Errorf("LockPath = %q, want %q", got, want)
	}
}

func TestValidateArtifactName(t *testing.T) {
	if err := ValidateArtifactName("skill", "good-name"); err != nil {
		t.Errorf("valid name rejected: %v", err)
	}
	// invariant: config/configuration:local-name-validated
	for _, bad := range []string{"", "a/b", "a\\b", "..", "a..b", "_reserved", "Foo", "foo bar", "foo: bar", "foo.bar", "über"} {
		if err := ValidateArtifactName("skill", bad); err == nil {
			t.Errorf("expected %q rejected", bad)
		}
	}
}

func TestHasSidecar(t *testing.T) {
	dir := writeConfig(t, "prefix: x\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Absent (non-singleton).
	if has, err := c.HasSidecar("skills", "nope"); err != nil || has {
		t.Fatalf("expected absent, got has=%v err=%v", has, err)
	}
	// Present (non-singleton).
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "yep.yaml"), []byte("data: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if has, err := c.HasSidecar("skills", "yep"); err != nil || !has {
		t.Fatalf("expected present, got has=%v err=%v", has, err)
	}
	// Singleton kind branch: sidecar lives at <root>/<kind>.yaml.
	if err := os.WriteFile(filepath.Join(dir, "agents-doc.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if has, err := c.HasSidecar("agents-doc", ""); err != nil || !has {
		t.Fatalf("expected singleton present, got has=%v err=%v", has, err)
	}

	brokenDir := writeConfig(t, "prefix: x\n")
	if err := os.WriteFile(filepath.Join(brokenDir, "skills"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken, err := Load(brokenDir)
	if err != nil {
		t.Fatal(err)
	}
	if has, err := broken.HasSidecar("skills", "fault"); err == nil || has || !strings.Contains(err.Error(), "stat sidecar skills/fault.yaml") {
		t.Fatalf("filesystem I/O error was not propagated: has=%v err=%v", has, err)
	}

	snapshot, err := ParseTree(".awf", []byte("prefix: x\n"), memoryTreeReader{"skills/yep.yaml": []byte("data: {}\n")})
	if err != nil {
		t.Fatal(err)
	}
	if has, err := snapshot.HasSidecar("skills", "yep"); err != nil || !has {
		t.Fatalf("snapshot sidecar behavior changed: has=%v err=%v", has, err)
	}
}

func TestWorkflowTelemetryConfigContract(t *testing.T) {
	// invariant: config/configuration:workflow-telemetry-settings
	cfg, err := Parse(".", []byte("prefix: x\ntargets: [claude]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkflowTelemetry != DefaultWorkflowTelemetryConfig() {
		t.Fatalf("defaults = %#v", cfg.WorkflowTelemetry)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}

	valid := "prefix: x\ntargets: [claude]\nworkflowTelemetry:\n  retention:\n    maxCompletedEffortAgeDays: 0\n    maxCompletedEffortCount: 0\n  widget:\n    enabled: false\n    showCost: false\n  diagnostics:\n    heuristicsEnabled: false\n    minimumBaselineSamples: 1\n    baselinePercentile: 1\n    thresholds:\n      phaseReentryCount: 1\n      phaseDurationSeconds: 1\n      phaseTokens: 1\n      compactionCount: 1\n      handoffCount: 1\n      toolFailureCount: 1\n      gateFailureCount: 1\n      cacheReadPercentBelow: 0\n      subagentQueueWaitSeconds: 1\n      implementationReworkCount: 1\n"
	cfg, err = Parse(".", []byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	explicit := DefaultWorkflowTelemetryConfig()
	explicit.Retention = TelemetryRetentionConfig{}
	explicit.Widget = TelemetryWidgetConfig{}
	explicit.Diagnostics.HeuristicsEnabled = false
	explicit.Diagnostics.MinimumBaselineSamples = 1
	explicit.Diagnostics.BaselinePercentile = 1
	explicit.Diagnostics.Thresholds = TelemetryThresholdsConfig{PhaseReentryCount: 1, PhaseDurationSeconds: 1, PhaseTokens: 1, CompactionCount: 1, HandoffCount: 1, ToolFailureCount: 1, GateFailureCount: 1, SubagentQueueWaitSeconds: 1, ImplementationReworkCount: 1}
	if cfg.WorkflowTelemetry != explicit {
		t.Fatalf("explicit values = %#v, want %#v", cfg.WorkflowTelemetry, explicit)
	}
	fresh, err := MarshalSkeleton(Skeleton{Prefix: "x"})
	if err != nil {
		t.Fatal(err)
	}
	const exactBlock = "workflowTelemetry:\n  retention:\n    maxCompletedEffortAgeDays: 90\n    maxCompletedEffortCount: 100\n  widget:\n    enabled: true\n    showCost: true\n  diagnostics:\n    heuristicsEnabled: true\n    minimumBaselineSamples: 10\n    baselinePercentile: 95\n    thresholds:\n      phaseReentryCount: 2\n      phaseDurationSeconds: 14400\n      phaseTokens: 200000\n      compactionCount: 3\n      handoffCount: 3\n      toolFailureCount: 3\n      gateFailureCount: 2\n      cacheReadPercentBelow: 10\n      subagentQueueWaitSeconds: 60\n      implementationReworkCount: 2\n"
	if !strings.HasSuffix(string(fresh), exactBlock) {
		t.Fatalf("fresh telemetry block:\n%s", fresh)
	}
	for _, bad := range []string{
		"workflowTelemetry:\n  unknown: true\n",
		"workflowTelemetry:\n  retention:\n    unknown: 1\n",
		"workflowTelemetry:\n  diagnostics:\n    thresholds:\n      unknown: 1\n",
	} {
		if _, err := Parse(".", []byte("prefix: x\ntargets: [claude]\n"+bad)); err == nil {
			t.Errorf("accepted unknown field in %q", bad)
		}
	}
	for _, mutate := range []struct{ old, new string }{
		{"maxCompletedEffortAgeDays: 0", "maxCompletedEffortAgeDays: -1"},
		{"maxCompletedEffortCount: 0", "maxCompletedEffortCount: -1"},
		{"minimumBaselineSamples: 1", "minimumBaselineSamples: 0"},
		{"baselinePercentile: 1", "baselinePercentile: 0"},
		{"baselinePercentile: 1", "baselinePercentile: 101"},
		{"phaseReentryCount: 1", "phaseReentryCount: 0"},
		{"phaseDurationSeconds: 1", "phaseDurationSeconds: 0"},
		{"phaseTokens: 1", "phaseTokens: 0"},
		{"compactionCount: 1", "compactionCount: 0"},
		{"handoffCount: 1", "handoffCount: 0"},
		{"toolFailureCount: 1", "toolFailureCount: 0"},
		{"gateFailureCount: 1", "gateFailureCount: 0"},
		{"cacheReadPercentBelow: 0", "cacheReadPercentBelow: -1"},
		{"cacheReadPercentBelow: 0", "cacheReadPercentBelow: 101"},
		{"subagentQueueWaitSeconds: 1", "subagentQueueWaitSeconds: 0"},
		{"implementationReworkCount: 1", "implementationReworkCount: 0"},
	} {
		cfg, err := Parse(".", []byte(strings.Replace(valid, mutate.old, mutate.new, 1)))
		if err != nil {
			t.Fatal(err)
		}
		if err := cfg.Validate(); err == nil {
			t.Errorf("accepted %s", mutate.new)
		}
	}
	upperBounds := strings.Replace(strings.Replace(valid, "baselinePercentile: 1", "baselinePercentile: 100", 1), "cacheReadPercentBelow: 0", "cacheReadPercentBelow: 100", 1)
	cfg, err = Parse(".", []byte(upperBounds))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("inclusive upper boundaries rejected: %v", err)
	}

	allZero := strings.ReplaceAll(valid, ": 1\n", ": 0\n")
	cfg, err = Parse(".", []byte(allZero))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err == nil {
		t.Error("explicit all-zero workflowTelemetry was treated as omitted")
	}
}
