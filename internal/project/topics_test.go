package project

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

const topicProjectConfig = "prefix: example\nskills: []\nagents: []\ndomains: [rendering]\n"

func writeProjectTopic(t *testing.T, root, slug, title, applies string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/rendering", slug+".yaml"), "title: "+title+"\nsummary: Current "+title+" contracts.\n"+applies)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering", slug, "current-state.md"), "<!-- awf:comment author note -->\nAuthored raw {{ .value }}.\n\n## Claims\n\n### `rule: stable`\nStable behavior.\nOrigin: ADR-0001\n")
}
func topicProject(t *testing.T) string {
	t.Helper()
	root := scaffoldFiles(t, topicProjectConfig, map[string]string{"domains/rendering.yaml": "paths: [\"internal/**\"]\n"})
	writeADR(t, root, "0001-topic.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Topic"), testsupport.WithBody("## Decision\n\n1. Topic.\n")))
	return root
}
func TestScaffoldedZeroClaimTopicPipeline(t *testing.T) {
	root := topicProject(t)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := topic.ScaffoldFiles(root, p.Cfg, "rendering", "Prepared Shell")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(file.Path)), string(file.Content))
	}
	adrs, err := adr.LoadCorpus(filepath.Join(root, "docs/decisions"))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := topic.LoadCorpus(root, p.Cfg, adrs)
	if err != nil {
		t.Fatalf("load scaffold corpus: %v", err)
	}
	shell, ok := corpus.ByTopicID("rendering/prepared-shell")
	if !ok || len(shell.Claims) != 0 {
		t.Fatalf("scaffold shell = %#v, found %v", shell, ok)
	}
	op, err := p.OutputPlan()
	if err != nil {
		t.Fatalf("output plan: %v", err)
	}
	found := false
	for _, node := range op.Nodes {
		if node.Path == "docs/topics/rendering/prepared-shell.md" {
			found = true
		}
	}
	if !found {
		t.Fatal("scaffolded topic missing from output plan")
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("render scaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/topics/rendering/prepared-shell.md")); err != nil {
		t.Fatal(err)
	}
	coverage := topic.CoverageForTopic(shell, corpus.DomainPaths["rendering"], corpus.Markers)
	if coverage.HasClaims || coverage.SatisfiesScopedCoverage || len(coverage.EffectiveSelectors) == 0 {
		t.Fatalf("zero-claim coverage = %#v", coverage)
	}
}

func TestTopicHashIsRepositoryRelative(t *testing.T) {
	model := topic.TopicRenderModel{Title: "Same", Summary: "Same.", Part: "Same part.\n"}
	var hashes []string
	for range 2 {
		root := t.TempDir()
		metadata := filepath.Join(root, ".awf/topics/metadata/rendering/same.yaml")
		part := filepath.Join(root, ".awf/topics/parts/rendering/same/current-state.md")
		testsupport.WriteFile(t, metadata, "title: Same\nsummary: Same.\npaths: [x]\n")
		testsupport.WriteFile(t, part, model.Part)
		hash, err := topicHash(root, model, metadata, part)
		if err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, hash)
	}
	if hashes[0] != hashes[1] {
		t.Fatalf("repository location changed topic hash: %q != %q", hashes[0], hashes[1])
	}
}

func TestTopicRenderLifecycle(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "zeta", "Zeta", "paths: [\"internal/**\"]\n")
	writeProjectTopic(t, root, "alpha", "Alpha", "applies: global\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := p.OutputPlan()
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"docs/topics/rendering/alpha.md": false, "docs/topics/rendering/zeta.md": false, "docs/topics/rendering/index.md": false}
	for _, n := range op.Nodes {
		if _, ok := wanted[n.Path]; ok {
			wanted[n.Path] = true
			if len(n.DependsOn) == 0 {
				t.Errorf("%s has no input dependencies", n.Path)
			}
		}
	}
	for path, ok := range wanted {
		if !ok {
			t.Errorf("missing %s", path)
		}
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	index := string(mustRead(t, filepath.Join(root, "docs/topics/rendering/index.md")))
	if strings.Index(index, "Alpha") > strings.Index(index, "Zeta") {
		t.Fatalf("index order: %s", index)
	}
	doc := string(mustRead(t, filepath.Join(root, "docs/topics/rendering/zeta.md")))
	if strings.Contains(doc, "awf:comment") || !strings.Contains(doc, "{{ .value }}") || !strings.Contains(doc, "Applicability") {
		t.Fatalf("topic output: %s", doc)
	}
	domain := readDomainDoc(t, root, "rendering")
	if !strings.Contains(domain, "## Topics") || !strings.Contains(domain, "## Decisions") || !strings.Contains(domain, "ADR-0001") {
		t.Fatalf("domain navigation lost authority: %s", domain)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	for path := range wanted {
		if _, ok := lock.Files[path]; !ok {
			t.Errorf("lock missing %s", path)
		}
	}
	// Metadata and part changes are both stale until sync.
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/rendering/zeta.yaml"), "title: Zeta changed\nsummary: Current Zeta contracts.\npaths: [\"internal/**\"]\n")
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	if !hasDrift(drift, "docs/topics/rendering/zeta.md", "stale") {
		t.Fatalf("metadata drift: %#v", drift)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering/zeta/current-state.md"), "Changed.\n\n## Claims\n")
	drift, err = p.Check()
	if err != nil {
		t.Fatal(err)
	}
	if !hasDrift(drift, "docs/topics/rendering/zeta.md", "stale") {
		t.Fatalf("part drift: %#v", drift)
	}
}
func TestTopicBrownfieldCollisionUsesSharedBackup(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "contracts", "Contracts", "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/topics/rendering/contracts.md"), "foreign\n")
	p, _ := Open(root)
	backups, _, _, err := p.SyncReport()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 || backups[0].Path != "docs/topics/rendering/contracts.md" {
		t.Fatalf("backups = %#v", backups)
	}
	if string(mustRead(t, filepath.Join(root, backups[0].Bak))) != "foreign\n" {
		t.Fatal("foreign topic output was not preserved")
	}
}

func TestTopicPruneRemoveAndRename(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "old", "Old", "paths: [\"internal/**\"]\n")
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".awf/topics/metadata/rendering/old.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".awf/topics/parts/rendering/old")); err != nil {
		t.Fatal(err)
	}
	writeProjectTopic(t, root, "new", "New", "paths: [\"internal/**\"]\n")
	p2, _ := Open(root)
	_, _, pruned, err := p2.SyncReport()
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 || pruned[0] != "docs/topics/rendering/old.md" {
		t.Fatalf("pruned %v", pruned)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/topics/rendering/new.md")); err != nil {
		t.Fatal(err)
	}
}
func TestTopicOutputCollisions(t *testing.T) {
	t.Run("topic index", func(t *testing.T) {
		root := topicProject(t)
		writeProjectTopic(t, root, "index", "Index", "paths: [\"internal/**\"]\n")
		p, _ := Open(root)
		if _, err := p.OutputPlan(); err == nil || !strings.Contains(err.Error(), "same output path") {
			t.Fatalf("collision %v", err)
		}
	})
	t.Run("local doc", func(t *testing.T) {
		root := topicProject(t)
		testsupport.WriteFile(t, configPath(root), topicProjectConfig+"docs: [topics/rendering/index]\n")
		testsupport.WriteFile(t, filepath.Join(root, ".awf/docs/topics/rendering/index.yaml"), "local: true\n")
		writeProjectTopic(t, root, "x", "X", "paths: [\"internal/**\"]\n")
		p, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.OutputPlan(); err == nil || !strings.Contains(err.Error(), "local document") {
			t.Fatalf("collision %v", err)
		}
	})
}
func TestTopicRenderRejectsMalformedAuthoringComment(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "x", "X", "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering/x/current-state.md"), "<!-- awf:comment no close\n\n## Claims\n")
	p, _ := Open(root)
	if _, err := p.OutputPlan(); err == nil {
		t.Fatal("malformed authoring comment accepted")
	}
}

func TestTopicCorpusRefusalAndSweep(t *testing.T) {
	root := topicProject(t)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/rendering/orphan.yaml"), "title: X\nsummary: X.\npaths: [x]\n")
	p, _ := Open(root)
	if _, err := p.OutputPlan(); err == nil {
		t.Fatal("orphan corpus accepted")
	}
	if err := os.Remove(filepath.Join(root, ".awf/topics/metadata/rendering/orphan.yaml")); err != nil {
		t.Fatal(err)
	}
	writeProjectTopic(t, root, "x", "X", "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering/x/extra.md"), "stray\n")
	p, _ = Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	if !hasDrift(drift, ".awf/topics/parts/rendering/x/extra.md", "orphaned") {
		t.Fatalf("sweep: %#v", drift)
	}
}
func TestQueryTopicLoadErrors(t *testing.T) {
	badADRRoot := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ndomains: []\n", nil)
	testsupport.WriteFile(t, filepath.Join(badADRRoot, "docs/decisions/0001-bad.md"), "---\nstatus: [\n---\n")
	p, err := Open(badADRRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.QueryTopic("schedule/contracts", topic.QueryOptions{}); err == nil {
		t.Fatal("QueryTopic accepted malformed ADR corpus")
	}

	badTopicRoot := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ndomains: [schedule]\n", map[string]string{"domains/schedule.yaml": "paths: [\"internal/**\"]\n"})
	writeADR(t, badTopicRoot, "0001-scheduling.md", testsupport.ADR("Implemented", testsupport.WithDomains("schedule")))
	testsupport.WriteFile(t, filepath.Join(badTopicRoot, ".awf/topics/metadata/schedule/contracts.yaml"), "title: Contracts\n")
	p, err = Open(badTopicRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.QueryTopic("schedule/contracts", topic.QueryOptions{}); err == nil {
		t.Fatal("QueryTopic accepted malformed topic corpus")
	}
}

func TestTopicSubstrateEndToEnd(t *testing.T) {
	root := scaffoldFiles(t, `prefix: example
skills: []
agents: []
domains: [schedule]
invariants:
  sources:
    - globs: ["internal/**"]
      marker: "//"
currentState:
  sources:
    - globs: ["internal/schedule*.go"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`, map[string]string{"domains/schedule.yaml": "paths: [\"internal/**\"]\n"})
	writeADR(t, root, "0001-scheduling.md", testsupport.ADR("Implemented", testsupport.WithDomains("schedule"), testsupport.WithTitle("0001: Scheduling contracts"), testsupport.WithBody("## Decision\n\n1. Define scheduling.\n\n## Invariants\n\n- `invariant: legacy-scheduling` - legacy authority remains stable.\n")))
	testsupport.WriteFile(t, filepath.Join(root, "internal/legacy_test.go"), "package internal\n// invariant: legacy-scheduling\n// invariant: schedule\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	scaffold, err := topic.ScaffoldFiles(root, p.Cfg, "schedule", "Contracts")
	if err != nil {
		t.Fatal(err)
	}
	if len(scaffold) != 2 || strings.Contains(string(scaffold[1].Content), "Origin:") {
		t.Fatalf("scaffold invented claims: %#v", scaffold)
	}
	for _, file := range scaffold {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(file.Path)), string(file.Content))
	}
	metadataPath := filepath.Join(root, ".awf/topics/metadata/schedule/contracts.yaml")
	partPath := filepath.Join(root, ".awf/topics/parts/schedule/contracts/current-state.md")
	testsupport.WriteFile(t, metadataPath, "title: Scheduling\nsummary: Current scheduling contracts.\npaths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, partPath, `Scheduling contracts are explicit.

## Claims

### `+"`rule: deterministic-order`"+`
Scheduling order is deterministic.
Origin: ADR-0001

### `+"`invariant: stable-output`"+`
Scheduling output is stable.
Origin: ADR-0001
Backing: test
`)
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule.go"), "package schedule\n// state: schedule/contracts:deterministic-order\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule_test.go"), "package schedule\n// invariant: schedule/contracts:stable-output\n")

	p, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := p.Topics()
	if err != nil {
		t.Fatalf("completed corpus: %v", err)
	}
	completed, ok := corpus.ByTopicID("schedule/contracts")
	if !ok || len(completed.Claims) != 2 {
		t.Fatalf("completed topic = %#v, found %v", completed, ok)
	}
	legacyBefore := legacyTopicBoundary(t, p)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"docs/topics/schedule/contracts.md", "docs/topics/schedule/index.md"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("missing generated topic path %s: %v", path, err)
		}
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"docs/topics/schedule/contracts.md", "docs/topics/schedule/index.md"} {
		if _, ok := lock.Files[path]; !ok {
			t.Fatalf("lock missing %s", path)
		}
	}

	binary := filepath.Join(t.TempDir(), "awf")
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/awf")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build awf: %v: %s", err, output)
	}
	runQuery := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(binary, append([]string{"topic"}, args...)...)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("awf topic %v: %v: %s", args, err, output)
		}
		return string(output)
	}
	human := runQuery("schedule/contracts", "--history", "--references", "--coverage")
	encoded := runQuery("schedule/contracts", "--history", "--references", "--coverage", "--json")
	var result topic.QueryResult
	if err := json.Unmarshal([]byte(encoded), &result); err != nil {
		t.Fatalf("query JSON: %v: %s", err, encoded)
	}
	for _, value := range []string{result.Title, result.Summary, result.Claims[0].ID, result.Claims[1].Prose, result.History[0].Origin.Title, result.Coverage.MarkerSites[0].Path} {
		if !strings.Contains(human, value) {
			t.Errorf("human/JSON query parity missing %q:\n%s", value, human)
		}
	}
	if legacyAfter := legacyTopicBoundary(t, p); legacyAfter != legacyBefore {
		t.Fatalf("topic sync changed legacy context/invariants:\nbefore %s\nafter  %s", legacyBefore, legacyAfter)
	}

	if err := os.Remove(metadataPath); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(partPath)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"internal/schedule.go", "internal/schedule_test.go"} {
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	p, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, pruned, err := p.SyncReport()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pruned, ",") != "docs/topics/schedule/contracts.md,docs/topics/schedule/index.md" {
		t.Fatalf("topic prune = %v", pruned)
	}
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, topicPresent := lock.Files["docs/topics/schedule/contracts.md"]; topicPresent {
		t.Fatal("pruned topic remains in lock")
	}
	if legacyAfter := legacyTopicBoundary(t, p); legacyAfter != legacyBefore {
		t.Fatalf("topic prune changed legacy context/invariants:\nbefore %s\nafter  %s", legacyBefore, legacyAfter)
	}
}

func legacyTopicBoundary(t *testing.T, p *Project) string {
	t.Helper()
	contextResult, err := p.ContextFor([]string{"internal/schedule.go"})
	if err != nil {
		t.Fatal(err)
	}
	findings, notes, err := p.CheckInvariants()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(struct {
		Context  ContextResult
		Findings any
		Notes    any
	}{contextResult, findings, notes})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
