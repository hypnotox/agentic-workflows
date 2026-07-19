package bridge

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func validBridgeProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{".awf", "docs/decisions"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  disabled: false\n  sources: []\n"), 0o644)
	mustWrite(t, filepath.Join(root, ".awf", "current-state-migration.yaml"), []byte("version: 1\ninvariantApprovals: []\n"), 0o640)
	mustWrite(t, filepath.Join(root, "docs", "decisions", "ACTIVE.md"), []byte("legacy\n"), 0o644)
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "fixture")
	return root
}
func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func validLiveBridgeProject(t *testing.T) string {
	t.Helper()
	root := validBridgeProject(t)
	mustMkdir(t, filepath.Join(root, "src"))
	mustMkdir(t, filepath.Join(root, ".awf", "domains"))
	mustMkdir(t, filepath.Join(root, ".awf", "topics", "metadata", "core"))
	mustMkdir(t, filepath.Join(root, ".awf", "topics", "parts", "core", "contracts"))
	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: awf\ntargets: [claude]\ndomains: [core]\ninvariants:\n  sources:\n    - globs: ['src/**']\n      marker: //\n  testGlobs: ['src/**']\n"), 0o644)
	mustWrite(t, filepath.Join(root, ".awf", "domains", "core.yaml"), []byte("paths: ['src/**']\n"), 0o644)
	mustWrite(t, filepath.Join(root, ".awf", "topics", "metadata", "core", "contracts.yaml"), []byte("title: Contracts\nsummary: Current contracts.\npaths: ['src/x_test.go']\n"), 0o644)
	mustWrite(t, filepath.Join(root, ".awf", "topics", "parts", "core", "contracts", "current-state.md"), []byte("Current contracts.\n\n## Claims\n\n### `invariant: stable`\n\nStable.\n\nOrigin: ADR-0001\nBacking: test\n"), 0o644)
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "- `invariant: stable` - x.", "1. x.", "")
	mustWrite(t, filepath.Join(root, "src", "x_test.go"), []byte("package src\n// invariant: stable\n"), 0o644)
	mustWrite(t, filepath.Join(root, ".awf", "current-state-migration.yaml"), []byte("version: 1\ninvariantApprovals:\n  - key: ADR-0001#stable\n    destination: core/contracts:stable\n"), 0o640)
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "prepared")
	return root
}

func TestReadinessLiveInvariantAndCoverage(t *testing.T) {
	root := validLiveBridgeProject(t)
	r := Check(root)
	if !r.Ready {
		t.Fatalf("findings: %#v", r.Findings)
	}
	if len(r.InvariantAdjudications) != 1 || !r.InvariantAdjudications[0].Approved || r.InvariantAdjudications[0].Destination != "core/contracts:stable" {
		t.Fatalf("adjudications: %#v", r.InvariantAdjudications)
	}
	mustWrite(t, filepath.Join(root, "src", "gap.go"), []byte("package src\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "topic-coverage", "src/gap.go")
}

func TestReadinessCoverageUniverse(t *testing.T) {
	root := validLiveBridgeProject(t)
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	cfg, _ := os.ReadFile(cfgPath)
	cfg = []byte(strings.ReplaceAll(string(cfg), "domains: [core]", "domains: [core, other]"))
	mustWrite(t, cfgPath, cfg, 0o644)
	mustWrite(t, filepath.Join(root, ".awf", "domains", "other.yaml"), []byte("paths: ['src/**']\n"), 0o644)
	for _, spec := range []struct{ slug, meta, part string }{{"global", "title: Global\nsummary: Global.\napplies: global\n", "Intro.\n\n## Claims\n\n### `rule: global`\n\nGlobal.\n\nOrigin: ADR-0001\n"}, {"empty", "title: Empty\nsummary: Empty.\npaths: ['src/gap.go']\n", "Intro.\n\n## Claims\n"}} {
		mustMkdir(t, filepath.Join(root, ".awf", "topics", "parts", "core", spec.slug))
		mustWrite(t, filepath.Join(root, ".awf", "topics", "metadata", "core", spec.slug+".yaml"), []byte(spec.meta), 0o644)
		mustWrite(t, filepath.Join(root, ".awf", "topics", "parts", "core", spec.slug, "current-state.md"), []byte(spec.part), 0o644)
	}
	mustWrite(t, filepath.Join(root, ".gitignore"), []byte("src/ignored.go\n"), 0o644)
	mustWrite(t, filepath.Join(root, "src", "ignored.go"), []byte("package src\n"), 0o644)
	mustWrite(t, filepath.Join(root, "src", "deleted.go"), []byte("package src\n"), 0o644)
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "coverage fixture")
	mustRemove(t, filepath.Join(root, "src", "deleted.go"))
	mustWrite(t, filepath.Join(root, "src", "gap.go"), []byte("package src\n"), 0o644)
	mustMkdir(t, filepath.Join(root, "src", "nested", ".awf"))
	mustWrite(t, filepath.Join(root, "src", "nested", "x.go"), []byte("package nested\n"), 0o644)
	r := Check(root)
	count := 0
	for _, f := range r.Findings {
		if f.Code == "topic-coverage" && f.Path == "src/gap.go" {
			count++
		}
		if f.Path == "src/ignored.go" || f.Path == "src/deleted.go" || strings.Contains(f.Path, "nested") {
			t.Errorf("excluded path reported: %#v", f)
		}
	}
	if count != 2 {
		t.Fatalf("want one gap per owning domain, got %d: %#v", count, r.Findings)
	}
}

func TestReadinessValidAndReadOnly(t *testing.T) {
	root := validBridgeProject(t)
	before := treeDigest(t, root)
	report := Check(root)
	if !report.Ready {
		t.Fatalf("findings: %#v", report.Findings)
	}
	if len(report.InvariantAdjudications) != 0 {
		t.Fatalf("adjudications: %#v", report.InvariantAdjudications)
	}
	if len(report.PlannedMutations) == 0 {
		t.Fatal("expected prepared and terminal mutations")
	}
	after := treeDigest(t, root)
	if before != after {
		t.Fatalf("check mutated tree: %s != %s", before, after)
	}
	paths := []string{}
	for _, m := range report.PlannedMutations {
		paths = append(paths, m.Path)
		if m.Path == ApprovalPath {
			t.Fatal("unchanged approval entered mutations")
		}
	}
	if !slices.Contains(paths, "docs/decisions/ACTIVE.md") {
		t.Fatalf("legacy deletion absent: %v", paths)
	}
}
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "status", "--porcelain=v1", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(string(out))
	return string(b)
}

func TestReadinessStableFindings(t *testing.T) {
	r := Check(t.TempDir())
	assertFinding(t, r, "config-conversion", ".awf/config.yaml")
	root := validBridgeProject(t)
	mustRemove(t, filepath.Join(root, ".awf", "current-state-migration.yaml"))
	r = Check(root)
	assertFinding(t, r, "invariant-approval", ApprovalPath)
	root = validBridgeProject(t)
	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: ''\ntargets: [claude]\ninvariants:\n  sources: []\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "config-conversion", ".awf/config.yaml")
	root = validBridgeProject(t)
	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  disabled: true\ncurrentState:\n  topicCoverage: warn\n  topicFanout: warn\n  maxTopicsPerPath: 8\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "config-conversion", ".awf/config.yaml")
	assertFinding(t, r, "coverage-severity", ".awf/config.yaml")
	root = validBridgeProject(t)
	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: awf\ntargets: [claude]\ndomains: [Bad_Key]\ninvariants:\n  sources: []\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "domain-key", ".awf/domains/Bad_Key.yaml")
	root = validBridgeProject(t)
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Proposed", "", "1. x.", "")
	r = Check(root)
	assertFinding(t, r, "inflight-adr", "docs/decisions/0001-one.md")
	root = validBridgeProject(t)
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "- `invariant: x` - x.", "1. x.", "\n## Migration history\n\nbad\n")
	r = Check(root)
	assertFinding(t, r, "migration-history", "docs/decisions")
	root = validBridgeProject(t)
	mustWrite(t, filepath.Join(root, ".awf", "current-state-migration.yaml"), []byte("version: nope\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "invariant-approval", ApprovalPath)
	root = validBridgeProject(t)
	mustWrite(t, filepath.Join(root, "docs", "decisions", "0001-bad.md"), []byte("---\nstatus: [\n---\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "invariant-inventory", "docs/decisions")
	root = validBridgeProject(t)
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "- `invariant: x` - x.\n- `invariant: x` - x.", "1. x.", "")
	r = Check(root)
	assertFinding(t, r, "invariant-inventory", "docs/decisions")
	root = validBridgeProject(t)
	mustMkdir(t, filepath.Join(root, ".awf", "topics", "parts", "bad"))
	mustWrite(t, filepath.Join(root, ".awf", "topics", "parts", "bad", "current-state.md"), []byte("bad"), 0o644)
	r = Check(root)
	assertFinding(t, r, "topic-corpus", ".awf/topics/parts/bad/current-state.md")
	root = validLiveBridgeProject(t)
	os.RemoveAll(filepath.Join(root, ".awf", "topics"))
	r = Check(root)
	assertFinding(t, r, "claim-mapping", "docs/decisions/0001-one.md")
	root = validLiveBridgeProject(t)
	mustWrite(t, filepath.Join(root, "src", "x_test.go"), []byte("package src\n// invariant: stable\n// touches-invariant: stable\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "marker-mapping", "src/x_test.go")
	root = validLiveBridgeProject(t)
	mustWrite(t, filepath.Join(root, ".awf", "domains", "core.yaml"), []byte("unknown: x\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "topic-corpus", ".awf/config.yaml")
	root = validLiveBridgeProject(t)
	part := filepath.Join(root, ".awf", "topics", "parts", "core", "contracts", "current-state.md")
	b, _ := os.ReadFile(part)
	mustWrite(t, part, append(b, []byte("\n### `rule: extra`\n\nExtra.\n\nOrigin: ADR-9999\n")...), 0o644)
	r = Check(root)
	assertFinding(t, r, "topic-corpus", ".awf/topics")
	root = validLiveBridgeProject(t)
	part = filepath.Join(root, ".awf", "topics", "parts", "core", "contracts", "current-state.md")
	b, _ = os.ReadFile(part)
	mustWrite(t, part, append(b, []byte("\n### `invariant: extra`\n\nExtra.\n\nOrigin: ADR-0001\nBacking: test\n")...), 0o644)
	r = Check(root)
	assertFinding(t, r, "marker-mapping", ".awf/topics")
	root = validLiveBridgeProject(t)
	mustWrite(t, filepath.Join(root, "src", "x_test.go"), []byte("package src\n// invariant: stable\n// invariant: core/bad\n"), 0o644)
	r = Check(root)
	assertFinding(t, r, "marker-mapping", "src/x_test.go")
	root = validBridgeProject(t)
	writeBridgeADR(t, filepath.Join(root, "docs", "decisions"), "0001-one.md", "Implemented", "", "1. x.", "")
	r = Check(root)
	assertFinding(t, r, "invariant-inventory", "docs/decisions")
}
func TestReadinessHelpers(t *testing.T) {
	r := finish(Report{Findings: []Finding{{"z", "b", "x"}, {"z", "a", "y"}, {"a", "c", "z"}, {"z", "a", "a"}}})
	if r.Ready || r.Findings[0].Code != "a" || r.Findings[1].Path != "a" {
		t.Fatalf("sort: %#v", r)
	}
	if topicErrorPath(errors.New("parse /tmp/x/.awf/topics/parts/core/x/current-state.md: bad")) != ".awf/topics/parts/core/x/current-state.md" || topicErrorPath(errors.New("bad")) != ".awf/topics" {
		t.Fatal("topic path")
	}
	inv := Inventory{Entries: []LegacyInvariant{{Key: "ADR-0001#x", DeclarerPath: "/r/docs/decisions/0001-x.md"}}}
	if mappingErrorPath("/r", inv, errors.New("ADR-0001#x bad")) != "docs/decisions/0001-x.md" || mappingErrorPath("/r", inv, errors.New("other")) != "docs/decisions" {
		t.Fatal("mapping path")
	}
	for _, tc := range []struct{ msg, code, path string }{{"marker bad", "marker-mapping", "docs/decisions"}, {"config bad", "config-conversion", ".awf/config.yaml"}, {"date bad", "migration-history", "docs/decisions"}} {
		e := errors.New(tc.msg)
		if classifyNormalize(e) != tc.code || normalizePath(e) != tc.path {
			t.Errorf("%s", tc.msg)
		}
	}
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "x"), []byte("x"), 0o644)
	m, _ := newMutation(root, "x", nil, false, 0)
	if err := applyMutation(root, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "x")); !os.IsNotExist(err) {
		t.Fatal("delete")
	}
	merged := mergeMutations([]Mutation{{Path: "x", Before: []byte("old"), BeforePresent: true}}, []Mutation{{Path: "x", After: []byte("new"), AfterPresent: true}})
	if len(merged) != 1 || string(merged[0].Before) != "old" {
		t.Fatalf("merge %#v", merged)
	}
	if !excludedPath(".git/x", nil) || excludedPath("src/x", nil) {
		t.Fatal("exclude")
	}
	var adjudicated Report
	buildAdjudications(&adjudicated, Inventory{Entries: []LegacyInvariant{
		{Key: "ADR-0001#old", Declarer: "0001", Backing: "unbacked", Active: false, History: &MigrationHistoryEntry{Basis: "encoded"}},
		{Key: "ADR-0002#tok", Declarer: "0002", Backing: "test", Active: false, Carrier: "0003"},
	}}, nil)
	if !adjudicated.InvariantAdjudications[0].Approved || !adjudicated.InvariantAdjudications[1].Approved {
		t.Fatalf("retired approval: %#v", adjudicated.InvariantAdjudications)
	}
	copyRoot := t.TempDir()
	mustWrite(t, filepath.Join(copyRoot, "file"), []byte("x"), 0o644)
	if err := os.Symlink("file", filepath.Join(copyRoot, "link")); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(copyRoot, "nested", ".git"))
	tmp, err := copyPreparedTree(copyRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	if _, err := os.Stat(filepath.Join(tmp, "link")); !os.IsNotExist(err) {
		t.Fatal("symlink copied")
	}
	if _, err := os.Stat(filepath.Join(tmp, "nested")); !os.IsNotExist(err) {
		t.Fatal("nested repo copied")
	}
	if off, exists := migrationHistoryInsertOffset([]byte("```\n## Migration history\n```\n")); exists || off == 0 {
		t.Fatal("fenced history counted")
	}
	if err := applyMutation(t.TempDir(), Mutation{Path: "missing", AfterPresent: false}); !os.IsNotExist(err) {
		t.Fatalf("missing delete: %v", err)
	}
	rawRoot := t.TempDir()
	partRoot := filepath.Join(rawRoot, ".awf", "topics", "parts", "core", "x")
	metaRoot := filepath.Join(rawRoot, ".awf", "topics", "metadata", "core")
	mustMkdir(t, partRoot)
	mustWrite(t, filepath.Join(partRoot, "current-state.md"), []byte("Intro.\n\n## Claims\n"), 0o644)
	if _, err := loadRawTopics(rawRoot); err == nil {
		t.Fatal("missing metadata accepted")
	}
	mustMkdir(t, metaRoot)
	mustWrite(t, filepath.Join(metaRoot, "x.yaml"), []byte("bad: x\n"), 0o644)
	if _, err := loadRawTopics(rawRoot); err == nil {
		t.Fatal("bad metadata accepted")
	}
	mustWrite(t, filepath.Join(metaRoot, "x.yaml"), []byte("title: X\nsummary: X.\napplies: global\n"), 0o644)
	mustWrite(t, filepath.Join(partRoot, "current-state.md"), []byte("bad"), 0o644)
	if _, err := loadRawTopics(rawRoot); err == nil {
		t.Fatal("bad part accepted")
	}
	mustWrite(t, filepath.Join(partRoot, "current-state.md"), []byte("Intro.\n\n## Claims\n"), 0o644)
	mustMkdir(t, filepath.Join(rawRoot, ".awf", "topics", "parts", "core", "a"))
	mustWrite(t, filepath.Join(rawRoot, ".awf", "topics", "metadata", "core", "a.yaml"), []byte("title: A\nsummary: A.\napplies: global\n"), 0o644)
	mustWrite(t, filepath.Join(rawRoot, ".awf", "topics", "parts", "core", "a", "current-state.md"), []byte("Intro.\n\n## Claims\n"), 0o644)
	topics, err := loadRawTopics(rawRoot)
	if err != nil || len(topics) != 2 || topics[0].ID.Slug != "a" {
		t.Fatalf("raw sort: %#v %v", topics, err)
	}
	checkCoverage(&adjudicated, t.TempDir(), t.TempDir(), &config.Config{}, topic.Corpus{})
	if len(adjudicated.Findings) == 0 {
		t.Fatal("git coverage failure ignored")
	}
}

func assertFinding(t *testing.T, r Report, code, path string) {
	t.Helper()
	for _, f := range r.Findings {
		if f.Code == code && f.Path == path {
			return
		}
	}
	t.Fatalf("missing %s/%s in %#v", code, path, r.Findings)
}
