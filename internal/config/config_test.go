package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
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

// invariant: config/configuration:no-artifact-selection-surface (TestConfigRejectsSelectionKeys)
func TestConfigRejectsSelectionKeys(t *testing.T) {
	for _, key := range []string{"skills", "agents", "docs", "targets", "docsDir"} {
		if _, err := Load(writeConfig(t, "prefix: awf\nintegrationBranch: main\n"+key+": []\n")); err == nil {
			t.Fatalf("Load accepted retired %s key", key)
		}
	}
}

func TestLoadParsesRepositoryFacts(t *testing.T) {
	dir := writeConfig(t, "prefix: example\nintegrationBranch: main\nvars:\n  testCmd: go test ./...\ndomains: [rendering]\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Prefix != "example" || c.Vars["testCmd"] != "go test ./..." || len(c.Domains) != 1 || c.Domains[0] != "rendering" {
		t.Errorf("Config = %#v", c)
	}
}

func TestParseRetainsSuppliedSourceAndStrictness(t *testing.T) {
	body := []byte("prefix: example\n")
	c, err := Parse("staged/.awf", body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if string(c.Source()) != string(body) {
		t.Errorf("Parse source = %q", c.Source())
	}
	if _, err := Parse("staged/.awf", []byte("unknown: true\n")); err == nil {
		t.Error("Parse must retain strict decoding")
	}
}

func TestLoadRetainsSource(t *testing.T) {
	// Load keeps the exact bytes it read, so a byte-level editor can reuse them
	// instead of re-reading config.yaml (and defending against a read that cannot
	// fail after Load already succeeded).
	body := "prefix: example\n"
	dir := writeConfig(t, body)
	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(c.Source()) != body {
		t.Errorf("Source() = %q, want %q", c.Source(), body)
	}
}

// invariant: config/configuration:root-sidecar-keys-rejected (TestRootAndSidecarRetiredKeysAreRejected)
func TestRootAndSidecarRetiredKeysAreRejected(t *testing.T) {
	for _, bad := range []string{"data", "sections", "local"} {
		d := writeConfig(t, "prefix: example\n"+bad+": {}\n")
		if _, err := Load(d); err == nil {
			t.Errorf("expected root %q to be rejected", bad)
		}
	}
	dir := writeConfig(t, "prefix: example\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "tdd.yaml"), []byte("local: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := Load(dir)
	if _, err := c.Sidecar("skills", "tdd"); err == nil {
		t.Fatal("sidecar local must be rejected")
	}
}

// invariant: config/configuration:awf-config-root (TestLoadReadsTreeRoot)
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

func TestSidecarReadsDataSections(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	sidecar := "data:\n  foo: bar\nsections:\n  notes:\n    drop: true\n"
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

// invariant: rendering/render-engine:sidecar-optional (TestSidecarAbsentIsEmpty)
func TestSidecarAbsentIsEmpty(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sc, err := c.Sidecar("skills", "tdd")
	if err != nil {
		t.Fatalf("absent sidecar should be empty, not an error: %v", err)
	}
	if sc.Data != nil || sc.Sections != nil {
		t.Errorf("absent sidecar should be the zero Sidecar, got %#v", sc)
	}
}

// A stale schema-1 sidecar carrying replaceWith fails closed at the strict
// decoder (the migration converts it before load); see ADR-0015.
func TestSidecarRejectsReplaceWith(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "sections:\n  notes:\n    replaceWith: parts/x.md\n"
	if err := os.WriteFile(filepath.Join(dir, "skills", "tdd.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := Load(dir)
	_, err := c.Sidecar("skills", "tdd")
	// invariant: config/configuration:no-replacewith (TestSidecarRejectsReplaceWith)
	if err == nil || !strings.Contains(err.Error(), "replaceWith") {
		t.Errorf("expected a strict-decoder error mentioning replaceWith, got: %v", err)
	}
}

func TestSidecarRejectsUnknownKey(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
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

// invariant: rendering/render-engine:sidecar-optional (TestSidecarAgentsDocSingleton)
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
	if sc.Data != nil || sc.Sections != nil {
		t.Errorf("absent agents-doc sidecar should be the zero Sidecar, got %#v", sc)
	}
	if err := os.WriteFile(filepath.Join(dir, "agents-doc.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Sidecar("agents-doc", ""); err == nil {
		t.Fatal("agents-doc local must be rejected")
	}
}

func TestSidecarReadErrorWhenPathIsDir(t *testing.T) {
	dir := writeConfig(t, "prefix: example\n")
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
	dir := writeConfig(t, "prefix: \"\"\n")
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
		dir := writeConfig(t, "prefix: "+prefix+"\n")
		c, err := Load(dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := c.Validate(); err == nil {
			t.Errorf("expected error for prefix %q containing path separator", prefix)
		}
	}
}

// invariant: config/validation:domain-name-validated (TestValidateRejectsBadDomainName)
func TestValidateRejectsBadDomainName(t *testing.T) {
	for _, bad := range []string{"", "../evil", "foo/bar", "a\\b"} {
		c := &Config{Prefix: "x", IntegrationBranch: "main", Domains: []string{bad}}
		if err := c.Validate(); err == nil {
			t.Errorf("expected error for domain name %q", bad)
		}
	}
	ok := &Config{Prefix: "x", IntegrationBranch: "main", Domains: []string{"rendering", "config"}}
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

func TestRetiredTargetsAndDocsDirAreRejected(t *testing.T) {
	for _, body := range []string{"prefix: example\ntargets: [claude]\n", "prefix: example\ndocsDir: docs\n"} {
		if _, err := Parse("staged/.awf", []byte(body)); err == nil {
			t.Fatalf("retired key accepted: %q", body)
		}
	}
}

func TestCurrentStatePresence(t *testing.T) {
	absent, err := Parse("staged/.awf", []byte("prefix: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if absent.CurrentState != nil {
		t.Fatalf("absent currentState = %#v", absent.CurrentState)
	}
	cfg, err := Parse("staged/.awf", []byte("prefix: x\nintegrationBranch: main\ncurrentState: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentState == nil {
		t.Fatal("present currentState decoded as nil")
	}
}

// invariant: config/validation:testglobs-anchored-validated (TestCurrentStateStrictValidation)
// invariant: config/configuration:severity-not-configurable (TestCurrentStateStrictValidation)
func TestCurrentStateStrictValidation(t *testing.T) {
	valid := `prefix: x
integrationBranch: main
currentState:
  sources:
    - globs: ['**/*.go']
      marker: '//'
      close: '*/'
  testGlobs: ['**/*_test.go']
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
			parsed, err := Parse("staged/.awf", []byte("prefix: x\nintegrationBranch: main\ncurrentState:\n"+tc.fragment))
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
		// ADR-0183 removed both severity keys, so a tree still carrying either is
		// rejected by the unknown-field path rather than honoured.
		{"prefix: x\ncurrentState:\n  topicCoverage: error\n", "topicCoverage"},
		{"prefix: x\ncurrentState:\n  topicFanout: warn\n", "topicFanout"},
		{"prefix: x\ncurrentState:\n  sources:\n    - globs: ['**/*.go']\n      marker: '//'\n      unknown: true\n", "unknown"},
		{"prefix: x\ncurrentState:\n  maxTopicsPerPath: 20\n", "maxTopicsPerPath"},
		{"prefix: x\ncurrentState:\n  testGlobs: ['**/*.go']\n  testGlobs: ['**/*.md']\n", "already set"},
		{"prefix: x\ncurrentState:\n  sources:\n    - globs: ['**/*.go']\n      marker: '//'\n      marker: '#'\n", "already set"},
	} {
		if _, err := Parse("staged/.awf", []byte(tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("strict nested field was accepted: %v", err)
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
		"prefix: x\ncurrentState:\n  maxTopicsPerPath: 8\n",
		"prefix: x\ncurrentState:\n  sources:\n    - globs: {}\n",
		"prefix: x\ncurrentState:\n  sources:\n    - marker: []\n",
		"prefix: x\ncurrentState:\n  sources:\n    - close: []\n",
	} {
		if _, err := Parse("staged/.awf", []byte(body)); err == nil {
			t.Errorf("wrong currentState value type was accepted:\n%s", body)
		}
	}
}

// integrationBranch is required and has no in-code default: an absent, empty,
// whitespace-bearing, or dash-leading value is rejected, while an ordinary or
// slashed branch name is accepted (ADR-0202 Decision 6).
// invariant: config/configuration:integration-branch-explicit (TestIntegrationBranchValidation)
func TestIntegrationBranchValidation(t *testing.T) {
	for _, tc := range []struct {
		name, branch, want string
	}{
		{"absent", "", "integrationBranch must not be empty"},
		{"inner whitespace", "my branch", "must not contain whitespace"},
		{"tab", "my\tbranch", "must not contain whitespace"},
		{"leading dash", "-force", `must not start with "-"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Prefix: "x", IntegrationBranch: tc.branch}
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate = %v, want error containing %q", err, tc.want)
			}
		})
	}
	for _, branch := range []string{"main", "master", "release/1.0"} {
		c := &Config{Prefix: "x", IntegrationBranch: branch}
		if err := c.Validate(); err != nil {
			t.Errorf("branch %q rejected: %v", branch, err)
		}
	}
	// Absence is absence: no default is materialized at parse time.
	parsed, err := Parse("staged/.awf", []byte("prefix: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.IntegrationBranch != "" {
		t.Errorf("ParseTree materialized an in-code default %q", parsed.IntegrationBranch)
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

// invariant: config/validation:commit-policy (TestCommitPolicyValidation)
func TestCommitPolicyValidation(t *testing.T) {
	validKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFSyHgjX4Y74rFN//IDMW2HBGkTMn5JF1Ls6VJr4pojt"
	valid := &CommitPolicyConfig{
		GrandfatheredThrough: strings.Repeat("a", 40),
		AllowedIdentities:    []CommitPolicyIdentity{{Name: "Ada", Email: "ada@example.test"}},
		RequireSignedCommits: true,
		AllowedSigners:       []CommitPolicySigner{{Principal: "ada@example.test", Key: validKey}},
	}
	if err := validateCommitPolicy(valid, validateOpenSSHPublicKey); err != nil {
		t.Fatalf("valid commit policy rejected: %v", err)
	}
	cfg := &Config{Prefix: "x", IntegrationBranch: "main", CommitPolicy: valid}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("config rejected valid commit policy: %v", err)
	}
	invalidPolicy := *valid
	invalidPolicy.GrandfatheredThrough = "abbreviated"
	cfg.CommitPolicy = &invalidPolicy
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "commitPolicy.grandfatheredThrough") {
		t.Fatalf("config validation = %v, want commit-policy error", err)
	}
	base := "prefix: x\nintegrationBranch: main\n"
	decodeCases := []struct {
		name string
		body string
		want string
	}{
		{name: "absent policy", body: base},
		{name: "identity only", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities:\n    - name: Ada\n      email: ada@example.test\n"},
		{name: "signer only", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: ada@example.test\n      key: " + validKey + "\n"},
		{name: "null identities", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities: null\n", want: "commitPolicy.allowedIdentities"},
		{name: "empty identities", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities: []\n", want: "commitPolicy.allowedIdentities"},
		{name: "null signers disabled", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedSigners: null\n", want: "commitPolicy.allowedSigners"},
		{name: "empty signers disabled", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedSigners: []\n", want: "commitPolicy.allowedSigners"},
		{name: "null signers required", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners: null\n", want: "commitPolicy.allowedSigners"},
		{name: "policy not mapping", body: base + "commitPolicy: nope\n", want: "commitPolicy must be a mapping"},
		{name: "duplicate policy key", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  grandfatheredThrough: " + strings.Repeat("b", 40) + "\n", want: "grandfatheredThrough"},
		{name: "baseline not string", body: base + "commitPolicy:\n  grandfatheredThrough: 123\n", want: "commitPolicy.grandfatheredThrough"},
		{name: "signing flag not bool", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: nope\n", want: "commitPolicy.requireSignedCommits"},
		{name: "unknown policy key", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  unknown: value\n", want: "unknown"},
		{name: "identity not mapping", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities:\n    - nope\n", want: "allowedIdentities[0]"},
		{name: "duplicate identity key", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities:\n    - name: Ada\n      name: Grace\n      email: ada@example.test\n", want: "name"},
		{name: "identity name not string", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities:\n    - name: 123\n      email: ada@example.test\n", want: "allowedIdentities[0].name"},
		{name: "identity email not string", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities:\n    - name: Ada\n      email: 123\n", want: "allowedIdentities[0].email"},
		{name: "unknown identity key", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities:\n    - name: Ada\n      email: ada@example.test\n      handle: ada\n", want: "handle"},
		{name: "identity name omitted", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities:\n    - email: ada@example.test\n", want: "allowedIdentities[0].name"},
		{name: "identity email omitted", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  allowedIdentities:\n    - name: Ada\n", want: "allowedIdentities[0].email"},
		{name: "signer not mapping", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners:\n    - nope\n", want: "allowedSigners[0]"},
		{name: "duplicate signer key", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: ada@example.test\n      principal: grace@example.test\n      key: " + validKey + "\n", want: "principal"},
		{name: "signer principal not string", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: 123\n      key: " + validKey + "\n", want: "allowedSigners[0].principal"},
		{name: "signer key not string", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: ada@example.test\n      key: 123\n", want: "allowedSigners[0].key"},
		{name: "unknown signer key", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: ada@example.test\n      key: " + validKey + "\n      comment: no\n", want: "comment"},
		{name: "signer principal omitted", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners:\n    - key: " + validKey + "\n", want: "allowedSigners[0].principal"},
		{name: "signer key omitted", body: base + "commitPolicy:\n  grandfatheredThrough: " + strings.Repeat("a", 40) + "\n  requireSignedCommits: true\n  allowedSigners:\n    - principal: ada@example.test\n", want: "allowedSigners[0].key"},
	}
	for _, tc := range decodeCases {
		t.Run("decode "+tc.name, func(t *testing.T) {
			parsed, err := Parse("staged/.awf", []byte(tc.body))
			if err == nil {
				err = parsed.Validate()
			}
			if tc.want == "" && err != nil {
				t.Fatalf("valid config rejected: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("validation = %v, want key %q", err, tc.want)
			}
		})
	}
	cases := []struct {
		name string
		edit func(*CommitPolicyConfig)
		want string
	}{
		{"abbreviated baseline", func(p *CommitPolicyConfig) { p.GrandfatheredThrough = "abcdef" }, "grandfatheredThrough"},
		{"uppercase baseline", func(p *CommitPolicyConfig) { p.GrandfatheredThrough = strings.Repeat("A", 40) }, "grandfatheredThrough"},
		{"empty identities", func(p *CommitPolicyConfig) { p.AllowedIdentities = []CommitPolicyIdentity{} }, "allowedIdentities"},
		{"empty name", func(p *CommitPolicyConfig) { p.AllowedIdentities[0].Name = "" }, "allowedIdentities[0].name"},
		{"empty email", func(p *CommitPolicyConfig) { p.AllowedIdentities[0].Email = "" }, "allowedIdentities[0].email"},
		{"space name", func(p *CommitPolicyConfig) { p.AllowedIdentities[0].Name = " Ada" }, "allowedIdentities[0].name"},
		{"control email", func(p *CommitPolicyConfig) { p.AllowedIdentities[0].Email = "a\n@example.test" }, "allowedIdentities[0].email"},
		{"invalid UTF-8 email", func(p *CommitPolicyConfig) { p.AllowedIdentities[0].Email = string([]byte{0xff}) }, "allowedIdentities[0].email"},
		{"duplicate identity", func(p *CommitPolicyConfig) { p.AllowedIdentities = append(p.AllowedIdentities, p.AllowedIdentities[0]) }, "allowedIdentities[1]"},
		{"missing signers", func(p *CommitPolicyConfig) { p.AllowedSigners = nil }, "allowedSigners"},
		{"signers without requirement", func(p *CommitPolicyConfig) { p.RequireSignedCommits = false }, "allowedSigners"},
		{"empty principal", func(p *CommitPolicyConfig) { p.AllowedSigners[0].Principal = "" }, "allowedSigners[0].principal"},
		{"bad principal", func(p *CommitPolicyConfig) { p.AllowedSigners[0].Principal = "bad space" }, "allowedSigners[0].principal"},
		{"duplicate signer", func(p *CommitPolicyConfig) { p.AllowedSigners = append(p.AllowedSigners, p.AllowedSigners[0]) }, "allowedSigners[1]"},
		{"empty key", func(p *CommitPolicyConfig) { p.AllowedSigners[0].Key = "" }, "allowedSigners[0].key"},
		{"key comment", func(p *CommitPolicyConfig) { p.AllowedSigners[0].Key += " comment" }, "allowedSigners[0].key"},
		{"key newline", func(p *CommitPolicyConfig) { p.AllowedSigners[0].Key += "\nnext" }, "allowedSigners[0].key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := *valid
			got.AllowedIdentities = append([]CommitPolicyIdentity(nil), valid.AllowedIdentities...)
			got.AllowedSigners = append([]CommitPolicySigner(nil), valid.AllowedSigners...)
			tc.edit(&got)
			if err := validateCommitPolicy(&got, validateOpenSSHPublicKey); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validation = %v, want key %q", err, tc.want)
			}
		})
	}
	called := false
	if err := validateCommitPolicy(valid, func(string) error { called = true; return nil }); err != nil || !called {
		t.Fatalf("operation key seam = %v, called=%v", err, called)
	}
	for _, key := range []string{"ssh-rsa AAAA", "ssh-ed25519 !!!", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFSyHgjX4Y74rFN//IDMW2HBGkTMn5JF1Ls6VJr4pojt\textra", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFSyHgjX4Y74rFN//IDMW2HBGkTMn5JF1Ls6VJr4pojt\n", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5"} {
		if err := validateOpenSSHPublicKey(key); err == nil {
			t.Errorf("invalid key %q accepted", key)
		}
	}
	rsaPath := filepath.Join(t.TempDir(), "rsa")
	if output, err := exec.Command("ssh-keygen", "-q", "-t", "rsa", "-b", "2048", "-N", "", "-f", rsaPath).CombinedOutput(); err != nil {
		t.Fatalf("generate unsupported RSA key: %v: %s", err, output)
	}
	rsaPublic, err := os.ReadFile(rsaPath + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	rsaFields := strings.Fields(string(rsaPublic))
	if len(rsaFields) < 2 {
		t.Fatalf("generated RSA public key = %q", rsaPublic)
	}
	if err := validateOpenSSHPublicKey(strings.Join(rsaFields[:2], " ")); err == nil {
		t.Fatal("well-formed unsupported RSA key accepted")
	}
	if !isFullOID(strings.Repeat("b", 64)) || isFullOID(strings.Repeat("g", 40)) || validPrincipal("") || validPrincipal("bad space") {
		t.Fatal("OID or principal helper accepted an invalid value")
	}
	if matchingSSHKeyBlob("ssh-ed25519", "AAAAIA==") || matchingSSHKeyBlob("ssh-ed25519", "!!!") {
		t.Fatal("malformed SSH key blob accepted")
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

// invariant: config/configuration:config-serialization-owned (TestConfigSerializationFunnelOwnsEncoding)
func TestConfigSerializationFunnelOwnsEncoding(t *testing.T) {
	// One source carrying both shapes the funnel must render identically on every
	// path: a nested mapping (audit) holding a nested array (allowedScopes), plus a
	// nested mapping SetMappingString can edit in place (it is total, so it leaves an
	// absent key untouched rather than creating one).
	const src = "prefix: ex\naudit:\n  allowedScopes:\n    - adr\nrunner:\n  awfInvokeCmd: old\n"

	// The untouched nested block every src-taking editor must round-trip byte for
	// byte. A second encoder at any other indent, or in flow style, changes it.
	const untouched = "audit:\n  allowedScopes:\n    - adr\n"

	cases := []struct {
		name  string
		edit  func() ([]byte, error)
		wrote string
	}{
		{"SetArrayMember", func() ([]byte, error) { return SetArrayMember([]byte(src), "skills", "tdd", true) }, "skills:\n  - tdd\n"},
		{"SetMappingScalar", func() ([]byte, error) { return SetMappingScalar([]byte(src), "bootstrap", "enabled", true) }, "bootstrap:\n  enabled: true\n"},
		{"SetMappingInteger", func() ([]byte, error) { return SetMappingInteger([]byte(src), "currentState", "maxTopicsPerPath", 8) }, "currentState:\n  maxTopicsPerPath: 8\n"},
		{"SetMappingString", func() ([]byte, error) { return SetMappingString([]byte(src), "runner", "awfInvokeCmd", "./awf") }, "runner:\n  awfInvokeCmd: ./awf\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.edit()
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !strings.Contains(string(got), untouched) {
				t.Errorf("%s did not round-trip the nested block through the shared funnel:\n%s", tc.name, got)
			}
			if !strings.Contains(string(got), tc.wrote) {
				t.Errorf("%s did not write %q at the funnel's two-space indent:\n%s", tc.name, tc.wrote, got)
			}
		})
	}

	// MarshalSkeleton is the construction half and takes no source bytes, so it is
	// driven separately and asserted against the same expected nesting.
	built, err := MarshalSkeleton(Skeleton{Prefix: "ex", Audit: &SkeletonAudit{AllowedScopes: []string{"adr"}}})
	if err != nil {
		t.Fatalf("MarshalSkeleton: %v", err)
	}
	if !strings.Contains(string(built), untouched) {
		t.Errorf("MarshalSkeleton did not render the nested block at the funnel's two-space indent:\n%s", built)
	}
}
