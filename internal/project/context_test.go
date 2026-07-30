package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const ctxConfig = `prefix: example
skills: [tdd]
agents: [code-reviewer]
domains: [alpha, core]
currentState:
  sources:
    - globs: ["internal/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`

func ctxFiles() map[string]string {
	return map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/foo/**\n",
		".awf/domains/core.yaml":                       "paths: []\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder is deterministic.\nSummary: Deterministic order.\nOrigin: ADR-0001\n\n### `invariant: tested`\nTests protect output.\nOrigin: ADR-0001\nBacking: test\n\n### `invariant: stable`\nOutput is stable.\nOrigin: ADR-0001\nBacking: unbacked\nVerify: by hand.\n",
		".awf/topics/metadata/core/g.yaml":             "title: Global\nsummary: Global rules.\napplies: global\n",
		".awf/topics/parts/core/g/current-state.md":    "Intro.\n\n## Claims\n\n### `rule: everywhere`\nApplies everywhere.\nOrigin: ADR-0001\n",
		"internal/foo/x.go":                            "package foo\n// state: alpha/one:order\n",
		"internal/foo/y.go":                            "package foo\n",
		"internal/foo/y_test.go":                       "package foo\n// invariant: alpha/one:tested\n",
	}
}

// invariant: tooling/context-and-topic:context-read-only
// invariant: tooling/context-and-topic:context-path-attribution
func TestContextRequestUniverse(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	before := snapshotTreeForContext(t, p.Root)
	res, err := p.ContextForOptions([]string{"internal/foo", "internal/foo/x.go", "internal/foo/x.go"}, ContextOptions{Selection: SelectionExplicit})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Requests) != 3 || res.Requests[0].Directory == nil || res.Requests[0].Directory.Included != 3 || res.Requests[1].Argument != "internal/foo"+"/x.go" {
		t.Fatalf("requests=%#v", res.Requests)
	}
	if len(res.Topics) != 2 {
		t.Fatalf("topics=%#v", res.Topics)
	}
	wantDirectoryRelationships := ContextRelationships{State: []string{"alpha/one:order"}, Touches: []string{}, Proofs: []string{"alpha/one:tested"}}
	if !reflect.DeepEqual(res.Requests[0].Directory.Relationships, wantDirectoryRelationships) {
		t.Fatalf("directory relationships=%#v", res.Requests[0].Directory.Relationships)
	}
	if !reflect.DeepEqual(res.Requests[1].Exact.Context.Relationships, (ContextRelationships{State: []string{"alpha/one:order"}, Touches: []string{}, Proofs: []string{}})) {
		t.Fatalf("file relationships=%#v", res.Requests[1].Exact.Context.Relationships)
	}
	var alpha TopicImpact
	for _, impact := range res.Topics {
		if impact.ID == "alpha/one" {
			alpha = impact
		}
	}
	wantSources := []ContextRelationshipSource{{RequestIndex: 2, Kinds: []string{"State"}}, {RequestIndex: 3, Kinds: []string{"State"}}}
	if len(alpha.Direct) != 1 || alpha.Direct[0].ID != "alpha/one:order" || !reflect.DeepEqual(alpha.Direct[0].Sources, wantSources) || len(alpha.Invariants) != 0 {
		t.Fatalf("mixed request authority=%#v", alpha)
	}
	glob, err := p.ContextForOptions([]string{"internal/foo/*.go"}, ContextOptions{})
	if err != nil || len(glob.Requests) != 1 || len(glob.Requests[0].Exact.Context.Warnings) != 1 {
		t.Fatalf("glob=%#v err=%v", glob, err)
	}
	if after := snapshotTreeForContext(t, p.Root); before != after {
		t.Fatal("context changed repository")
	}
}

func TestContextWorkingIndexDivergenceAndErrors(t *testing.T) {
	p := csRepo(t, ctxConfig, ctxFiles())
	lock := &manifest.Lock{AWFVersion: "0.0.0", SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(p.Root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = p.Root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	testsupport.WriteFile(t, filepath.Join(p.Root, "internal/foo/new.go"), "package foo\n")
	working, err := p.ContextForOptions([]string{"internal/foo"}, ContextOptions{Selection: SelectionExplicit})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := StagedContextRootOptions(p.Root, []string{"internal/foo"}, ContextOptions{Selection: SelectionStaged})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StagedUncoveredRoot(p.Root, nil); err != nil {
		t.Fatal(err)
	}
	if working.Requests[0].Directory.Included != staged.Requests[0].Directory.Included+1 {
		t.Fatalf("working=%d staged=%d", working.Requests[0].Directory.Included, staged.Requests[0].Directory.Included)
	}
	if _, err := (&Project{Root: t.TempDir()}).ContextForOptions([]string{"x"}, ContextOptions{}); err == nil {
		t.Fatal("outside repo accepted")
	}
	if _, err := StagedContextRootOptions(t.TempDir(), []string{"x"}, ContextOptions{}); err == nil {
		t.Fatal("staged outside repo accepted")
	}
}

func TestStagedContextInputErrors(t *testing.T) {
	_, root := gitfixture.InitRepo(t)
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: "0", SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := StagedContextRootOptions(root, []string{"x"}, ContextOptions{}); err == nil || !strings.Contains(err.Error(), "no staged") {
		t.Fatalf("missing config err=%v", err)
	}
	for _, tc := range []struct {
		name, cfg string
		extra     map[string]string
	}{{"unknown-target", ctxConfig + "targets: [unknown]\n", nil}, {"bad-local", strings.Replace(ctxConfig, "skills: [tdd]", "skills: [mine]", 1), map[string]string{".awf/skills/mine.yaml": "local: [bad"}}, {"bad-topic", ctxConfig, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "broken"}}} {
		t.Run(tc.name, func(t *testing.T) {
			p := csRepo(t, ctxConfig, ctxFiles())
			if err := os.WriteFile(filepath.Join(p.Root, ".awf", "config.yaml"), []byte(tc.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			for rel, body := range tc.extra {
				testsupport.WriteFile(t, filepath.Join(p.Root, filepath.FromSlash(rel)), body)
			}
			lock := &manifest.Lock{AWFVersion: "0", SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
			if err := lock.Save(filepath.Join(p.Root, ".awf", "awf.lock")); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("git", "add", ".")
			cmd.Dir = p.Root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("add: %v %s", err, out)
			}
			if _, err := StagedContextRootOptions(p.Root, []string{"x"}, ContextOptions{}); err == nil {
				t.Fatal("invalid staged state accepted")
			}
		})
	}
	p := csRepo(t, ctxConfig, ctxFiles())
	testsupport.WriteFile(t, filepath.Join(p.Root, ".awf", "awf.lock"), "bad")
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = p.Root
	_ = cmd.Run()
	if _, err := StagedContextRootOptions(p.Root, []string{"x"}, ContextOptions{}); err == nil {
		t.Fatal("corrupt staged lock accepted")
	}
}

func TestContextUniverseSetupErrors(t *testing.T) {
	bad := csRepo(t, ctxConfig, ctxFiles())
	if err := os.WriteFile(filepath.Join(bad.Root, ".awf", "config.yaml"), []byte(strings.Replace(ctxConfig, "skills: [tdd]", "skills: [mine]", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(bad.Root, ".awf", "skills", "mine.yaml"), "local: [bad")
	if _, err := bad.ContextForOptions([]string{"x"}, ContextOptions{}); err == nil {
		t.Fatal("malformed catalog accepted")
	}
	p := csRepo(t, ctxConfig, ctxFiles())
	if err := os.WriteFile(filepath.Join(p.Root, ".awf/config.yaml"), []byte(ctxConfig+"targets: [unknown]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ContextForOptions([]string{"x"}, ContextOptions{}); err == nil {
		t.Fatal("invalid target accepted")
	}
}

func TestNormalizeContextRequestPaths(t *testing.T) {
	got := NormalizeContextPaths([]string{"", "b/../a", "a", "b"})
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatal(got)
	}
	if topicOfClaim("topic-only") != "topic-only" {
		t.Fatal("topic fallback")
	}
}

func snapshotTreeForContext(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, path)
		b.WriteString(rel + info.Mode().String())
		if !info.IsDir() {
			data, _ := os.ReadFile(path)
			b.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

const uncoveredConfig = `prefix: example
skills:
  - tdd
agents:
  - code-reviewer
domains:
  - alpha
contextIgnore:
  - .awf/**
currentState:
  maxTopicsPerPath: 8
`

func uncoveredFiles() map[string]string {
	return map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: The one topic.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: order`\nOrder.\nOrigin: ADR-0001\n",
		"internal/foo/x.go":                            "package foo\n",
		"internal/bar.go":                              "package internalx\n",
		"docs/thing.md":                                "doc\n",
	}
}

// TestUncovered proves the report lists domain-owned paths with no scoped topic
// and, separately, the eligible paths owned by no domain (collapsed).
// invariant: invariants/current-state-authority:uncovered-lists-unowned-unignored
// invariant: tooling/context-and-topic:uncovered-collapses-directories
// The selection clause is marked here because internal/topic's marker cannot
// reach assembleUncovered: this is where "the uncovered report requests coverage
// only" actually fails if the policy gains Fanout (ADR-0180 item 5).
// invariant: invariants/topics-and-markers:coverage-evaluation-selects-checks
func TestUncovered(t *testing.T) {
	cfg := strings.Replace(uncoveredConfig, "contextIgnore:\n  - .awf/**", "contextIgnore:\n  - .awf/**\n  - gen/skipped.md", 1)
	files := uncoveredFiles()
	files["gen/real.md"] = "unowned eligible\n"
	files["gen/output.md"] = "generated\n"
	files["gen/skipped.md"] = "ignored\n"
	p := csRepo(t, cfg, files)
	lock := &manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 14, Files: map[string]manifest.Entry{"gen/output.md": {}}}
	if err := lock.Save(lockFile(p.Root)); err != nil {
		t.Fatal(err)
	}
	res, err := p.Uncovered(nil)
	if err != nil {
		t.Fatalf("Uncovered: %v", err)
	}
	if len(res.Uncovered) != 1 || res.Uncovered[0].Path != "internal/bar.go" || res.Uncovered[0].Domain != "alpha" {
		t.Fatalf("uncovered = %#v; want internal/bar.go owned by alpha", res.Uncovered)
	}
	// docs/ (two unowned files beneath) and the committed README.md are owned by
	// no domain; gen/ collapses around one unowned file while its generated and
	// ignored siblings count as excluded, never listed.
	want := []UnownedEntry{
		{Path: "README.md", UnownedCount: 1, ExcludedCount: 0},
		{Path: "docs/", UnownedCount: 2, ExcludedCount: 0},
		{Path: "gen/", UnownedCount: 1, ExcludedCount: 2},
	}
	if !reflect.DeepEqual(res.Unowned, want) {
		t.Errorf("unowned = %#v; want %#v", res.Unowned, want)
	}
}

// TestUncoveredCollapsesToRoot covers the root-collapse branch: when a domain
// owns nothing present, no path seeds the repository root as covered, so a
// whole-repo scan folds every unowned path up to ".".
func TestUncoveredCollapsesToRoot(t *testing.T) {
	cfg := "prefix: example\ndomains:\n  - alpha\ncontextIgnore:\n  - .awf/**\ncurrentState:\n  maxTopicsPerPath: 8\n"
	files := map[string]string{
		".awf/domains/alpha.yaml": "paths:\n  - nonexistent/**\n",
		"top.txt":                 "x\n",
	}
	res, err := csRepo(t, cfg, files).Uncovered(nil)
	if err != nil {
		t.Fatalf("Uncovered: %v", err)
	}
	// top.txt, the committed README.md, and the auto-added decision record are
	// unowned; the .awf config tree is context-ignored and counts as excluded.
	want := []UnownedEntry{{Path: ".", UnownedCount: 3, ExcludedCount: 2}}
	if !reflect.DeepEqual(res.Unowned, want) {
		t.Errorf("unowned = %#v; want %#v", res.Unowned, want)
	}
}

// TestUncoveredScanRoot restricts the report to a scan root on segment boundaries.
func TestUncoveredScanRoot(t *testing.T) {
	p := csRepo(t, uncoveredConfig, uncoveredFiles())
	res, err := p.Uncovered([]string{"internal"})
	if err != nil {
		t.Fatalf("Uncovered: %v", err)
	}
	if len(res.Uncovered) != 1 || res.Uncovered[0].Path != "internal/bar.go" {
		t.Fatalf("uncovered = %#v; want just internal/bar.go", res.Uncovered)
	}
	// With scan roots restricting the report, out-of-scope unowned and excluded
	// files produce no entry and inflate no entry's counts.
	if len(res.Unowned) != 0 {
		t.Errorf("unowned = %v; want none in scope (docs/ and README.md are out of scope)", res.Unowned)
	}
	if strings.Join(res.ScanRoots, ",") != "internal" {
		t.Errorf("scanRoots = %v", res.ScanRoots)
	}
}

func TestStagedUncoveredOutsideRepo(t *testing.T) {
	if _, err := StagedUncoveredRoot(t.TempDir(), nil); err == nil {
		t.Fatal("expected staged uncovered to reject a non-repository")
	}
}

// TestUncoveredOutsideRepo covers the working-Tree failure in Uncovered.
func TestUncoveredOutsideRepo(t *testing.T) {
	root := scaffoldFiles(t, uncoveredConfig, map[string]string{".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Uncovered(nil); err == nil {
		t.Fatal("expected a working-tree error outside a git repository")
	}
}
