package publisher

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

const topicProjectConfig = "prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: ./x gate\ndomains: [rendering]\n"

func TestGenerateTopicDocsPropagatesProjectTreeEnumerationFailure(t *testing.T) {
	calls := 0
	reader := failingPathsReader{memoryProjectReader: memoryProjectReader{}, failAt: 1, calls: &calls}
	state := testStateAt(t.TempDir())
	_, _, err := generateTopicDocs(newRenderInputs(state, &config.Config{}, reader, project.Version), topic.Corpus{})
	if err == nil || !strings.Contains(err.Error(), "enumeration fault") {
		t.Fatalf("generate topic docs error = %v, want enumeration fault", err)
	}
}

func writeProjectTopic(t *testing.T, root, slug, title, applies string) {
	t.Helper()
	writeProjectTopicDomain(t, root, "rendering", slug, title, applies)
}

func writeProjectTopicDomain(t *testing.T, root, domain, slug, title, applies string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata", domain, slug+".yaml"), "title: "+title+"\nsummary: Current "+title+" contracts.\n"+applies)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts", domain, slug, "current-state.md"), "<!-- awf:comment author note -->\nAuthored raw {{ .value }}.\n\n## Claims\n\n### `rule: stable`\nStable behavior.\n")
}
func topicProject(t *testing.T) string {
	t.Helper()
	root := scaffoldFiles(t, topicProjectConfig, map[string]string{"domains/rendering.yaml": "paths: [\"internal/**\"]\n"})
	writeADR(t, root, "0001-topic.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Topic"), testsupport.WithBody("## Decision\n\n1. Topic.\n")))
	return root
}
func TestRepositoryEffortManagementCoverage(t *testing.T) {
	p, err := loadTestSession(testContext(t), "../..")
	if err != nil {
		t.Fatal(err)
	}
	corpus := mustDeriveTopics(t, p)
	paths := map[string]string{
		"cmd/awf/effort.go":                         "tooling/effort-management",
		"cmd/awf/effort_test.go":                    "tooling/effort-management",
		"internal/effort/branches_test.go":          "tooling/effort-management",
		"internal/effort/durability_test.go":        "tooling/effort-management",
		"internal/effort/memory.go":                 "tooling/effort-management",
		"internal/effort/memory_test.go":            "tooling/effort-management",
		"internal/effort/paths.go":                  "tooling/effort-management",
		"internal/effort/paths_test.go":             "tooling/effort-management",
		"internal/effort/platform_test.go":          "tooling/effort-management",
		"internal/effort/repair_test.go":            "tooling/effort-management",
		"internal/effort/safeio.go":                 "tooling/effort-management",
		"internal/effort/safeio_darwin.go":          "tooling/effort-management",
		"internal/effort/safeio_linux.go":           "tooling/effort-management",
		"internal/effort/safeio_linux_test.go":      "tooling/effort-management",
		"internal/effort/safeio_unix.go":            "tooling/effort-management",
		"internal/effort/safety_test.go":            "tooling/effort-management",
		"internal/effort/testsys_unix_test.go":      "tooling/effort-management",
		"internal/effort/service.go":                "tooling/effort-management",
		"internal/effort/service_test.go":           "tooling/effort-management",
		"internal/effort/store.go":                  "tooling/effort-management",
		"internal/effort/store_test.go":             "tooling/effort-management",
		"internal/effort/types.go":                  "tooling/effort-management",
		"internal/effort/types_test.go":             "tooling/effort-management",
		"internal/git/controlroot.go":               "tooling/audit-and-snapshots",
		"internal/git/controlroot_internal_test.go": "tooling/audit-and-snapshots",
		"internal/git/controlroot_test.go":          "tooling/audit-and-snapshots",
		"internal/git/controlroot_unix.go":          "tooling/audit-and-snapshots",
	}
	for path, topicID := range paths {
		t.Run(strings.ReplaceAll(path, "/", "-"), func(t *testing.T) {
			if _, err := os.Stat(filepath.Join("../..", filepath.FromSlash(path))); err != nil {
				t.Fatalf("enumerated Phase 1 path does not exist: %v", err)
			}
			selected, ok := corpus.ByTopicID(topicID)
			if !ok {
				t.Fatalf("expected topic %s is absent", topicID)
			}
			applicability := topic.ApplicabilityForTopic(selected, corpus.DomainPaths["tooling"], corpus.Markers, []string{path})
			if len(applicability.DomainPaths) == 0 {
				t.Errorf("%s did not independently resolve to the tooling domain", path)
			}
			if len(applicability.TopicPaths) == 0 {
				t.Errorf("%s did not independently resolve to %s", path, topicID)
			}
		})
	}
}

func TestTopicsPropagatesMalformedCorpus(t *testing.T) {
	root := topicProject(t)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/rendering/bad.yaml"), "title: [bad\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(p)); err == nil {
		t.Fatal("malformed topic corpus accepted")
	}

}

func TestScaffoldedZeroClaimTopicPipeline(t *testing.T) {
	root := topicProject(t)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := topic.ScaffoldFilesWithExists(testConfig(p), "rendering", "Prepared Shell", func(path string) (bool, error) {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return err == nil, err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(file.Path)), string(file.Content))
	}
	prepared, err := newPublisher(lowerForConfig(p, testConfig(p)), testConfig(p), NewFilesystemReader(root), project.Version).operationState()
	if err != nil {
		t.Fatalf("prepare scaffold corpus: %v", err)
	}
	corpus := prepared.topics.Clone()
	shell, ok := corpus.ByTopicID("rendering/prepared-shell")
	if !ok || len(shell.Claims) != 0 {
		t.Fatalf("scaffold shell = %#v, found %v", shell, ok)
	}
	op, err := outputPlanProject(p)
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
	if err := syncProject(p); err != nil {
		t.Fatalf("render scaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/topics/rendering/prepared-shell.md")); err != nil {
		t.Fatal(err)
	}
	applicability := topic.ApplicabilityForTopic(shell, corpus.DomainPaths["rendering"], corpus.Markers, []string{"internal/project/x.go"})
	if len(applicability.DomainPaths) == 0 || len(applicability.TopicPaths) == 0 {
		t.Fatalf("zero-claim coverage = %#v", applicability)
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
		hash, err := topicHash(root, filesystemProjectReader{root: root}, model, metadata, part)
		if err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, hash)
	}
	if hashes[0] != hashes[1] {
		t.Fatalf("repository location changed topic hash: %q != %q", hashes[0], hashes[1])
	}
}

func TestTopicHashReadsTheSelectedSnapshot(t *testing.T) {
	root := t.TempDir()
	metadataRel := ".awf/topics/metadata/rendering/same.yaml"
	partRel := ".awf/topics/parts/rendering/same/current-state.md"
	metadata := filepath.Join(root, filepath.FromSlash(metadataRel))
	part := filepath.Join(root, filepath.FromSlash(partRel))
	model := topic.TopicRenderModel{Title: "Snapshot", Summary: "Snapshot.", Part: "snapshot part\n"}
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: metadataRel, Mode: snapshot.Regular, Bytes: []byte("snapshot metadata\n")},
		{Path: partRel, Mode: snapshot.Regular, Bytes: []byte(model.Part)},
	})
	if err != nil {
		t.Fatal(err)
	}
	read := snapshotTreeReader{tree: tree}
	before, err := topicHash(root, read, model, metadata, part)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, metadata, "different working metadata\n")
	testsupport.WriteFile(t, part, "different working part\n")
	after, err := topicHash(root, read, model, metadata, part)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("working files changed snapshot topic hash: %q != %q", before, after)
	}
}

func TestTopicHashPropagatesReaderFault(t *testing.T) {
	root := t.TempDir()
	model := topic.TopicRenderModel{Title: "Fault", Summary: "Fault."}
	if _, err := topicHash(root, failingReadReader{}, model, filepath.Join(root, "metadata")); err == nil {
		t.Fatal("topic hash erased a project-tree read fault")
	}
}

func TestTopicRenderLifecycle(t *testing.T) {
	// invariant: rendering/render-engine:source-marker-informational (TestTopicRenderLifecycle)
	root := topicProject(t)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), strings.Replace(topicProjectConfig, "domains: [rendering]", "domains: [rendering, tooling]", 1))
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/tooling.yaml"), "paths: [\"cmd/**\"]\n")
	writeProjectTopic(t, root, "zeta", "Zeta", "paths: [\"internal/**\"]\n")
	writeProjectTopic(t, root, "alpha", "Alpha", "applies: global\n")
	writeProjectTopicDomain(t, root, "tooling", "beta", "Beta", "applies: global\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{
		"docs/topics/rendering/alpha.md": false,
		"docs/topics/rendering/zeta.md":  false,
		"docs/topics/rendering/index.md": false,
		"docs/topics/tooling/beta.md":    false,
		"docs/topics/tooling/index.md":   false,
	}
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
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	index := string(mustRead(t, filepath.Join(root, "docs/topics/rendering/index.md")))
	if strings.Index(index, "Alpha") > strings.Index(index, "Zeta") {
		t.Fatalf("index order: %s", index)
	}
	indexMarker := "<!-- awf:source .awf/topics/metadata/rendering/*.yaml .awf/topics/parts/rendering/*/current-state.md -->"
	if !strings.Contains(index, "<!-- "+bannerText+" -->\n"+indexMarker+"\n") || strings.Count(index, "awf:source") != 1 {
		t.Fatalf("topic index marker = %s", index)
	}
	for _, tc := range []struct {
		path, marker string
	}{
		{"docs/topics/rendering/alpha.md", "<!-- awf:source .awf/topics/metadata/rendering/alpha.yaml .awf/topics/parts/rendering/alpha/current-state.md -->"},
		{"docs/topics/rendering/zeta.md", "<!-- awf:source .awf/topics/metadata/rendering/zeta.yaml .awf/topics/parts/rendering/zeta/current-state.md -->"},
		{"docs/topics/tooling/beta.md", "<!-- awf:source .awf/topics/metadata/tooling/beta.yaml .awf/topics/parts/tooling/beta/current-state.md -->"},
	} {
		page := string(mustRead(t, filepath.Join(root, tc.path)))
		if !strings.Contains(page, "<!-- "+bannerText+" -->\n"+tc.marker+"\n") || strings.Count(page, "awf:source") != 1 {
			t.Fatalf("topic marker for %s = %s", tc.path, page)
		}
	}
	toolingIndex := string(mustRead(t, filepath.Join(root, "docs/topics/tooling/index.md")))
	toolingMarker := "<!-- awf:source .awf/topics/metadata/tooling/*.yaml .awf/topics/parts/tooling/*/current-state.md -->"
	if !strings.Contains(toolingIndex, "<!-- "+bannerText+" -->\n"+toolingMarker+"\n") || strings.Count(toolingIndex, "awf:source") != 1 {
		t.Fatalf("tooling topic index marker = %s", toolingIndex)
	}
	doc := string(mustRead(t, filepath.Join(root, "docs/topics/rendering/zeta.md")))
	bodyAt := strings.Index(doc, "# Zeta")
	if bodyAt < 0 || strings.Contains(doc[bodyAt:], "awf:source") {
		t.Fatalf("topic body contains source marker: %s", doc)
	}
	if strings.Contains(doc, "awf:comment") || strings.Contains(doc, "<no value>") || !strings.Contains(doc, "{{ .value }}") || !strings.Contains(doc, "Applicability") {
		t.Fatalf("topic output: %s", doc)
	}
	domain := readDomainDoc(t, root, "rendering")
	// The domain doc navigates topics only; no ADR decisions index remains.
	if !strings.Contains(domain, "## Topics") || strings.Contains(domain, "## Decisions") || strings.Contains(domain, "ADR-0001") {
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
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDrift(drift, "docs/topics/rendering/zeta.md", "stale") {
		t.Fatalf("metadata drift: %#v", drift)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering/zeta/current-state.md"), "Changed.\n\n## Claims\n")
	drift, err = checkProject(p, testContext(t))
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
	p, _ := loadTestSession(testContext(t), root)
	backups, _, _, err := initializeReportProject(p, InitAuthority{InitializedWithVersion: Version})
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
	p, _ := loadTestSession(testContext(t), root)
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".awf/topics/metadata/rendering/old.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".awf/topics/parts/rendering/old")); err != nil {
		t.Fatal(err)
	}
	writeProjectTopic(t, root, "new", "New", "paths: [\"internal/**\"]\n")
	p2, _ := loadTestSession(testContext(t), root)
	_, pruned, err := syncReportProject(p2)
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
		p, _ := loadTestSession(testContext(t), root)
		if _, err := outputPlanProject(p); err == nil || !strings.Contains(err.Error(), "same output path") {
			t.Fatalf("collision %v", err)
		}
	})
}
func TestTopicRenderRejectsMalformedAuthoringComment(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "x", "X", "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering/x/current-state.md"), "<!-- awf:comment no close\n\n## Claims\n")
	p, _ := loadTestSession(testContext(t), root)
	if _, err := outputPlanProject(p); err == nil {
		t.Fatal("malformed authoring comment accepted")
	}
}

func TestTopicCorpusRefusalAndSweep(t *testing.T) {
	root := topicProject(t)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/rendering/orphan.yaml"), "title: X\nsummary: X.\npaths: [x]\n")
	p, _ := loadTestSession(testContext(t), root)
	if _, err := outputPlanProject(p); err == nil {
		t.Fatal("orphan corpus accepted")
	}
	if err := os.Remove(filepath.Join(root, ".awf/topics/metadata/rendering/orphan.yaml")); err != nil {
		t.Fatal(err)
	}
	writeProjectTopic(t, root, "x", "X", "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering/x/extra.md"), "stray\n")
	p, _ = loadTestSession(testContext(t), root)
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDrift(drift, ".awf/topics/parts/rendering/x/extra.md", "orphaned") {
		t.Fatalf("sweep: %#v", drift)
	}
}
func TestTopicSubstrateEndToEnd(t *testing.T) {
	root := scaffoldFiles(t, `prefix: example
integrationBranch: main
domains: [schedule]
currentState:
  sources:
    - globs: ["internal/schedule*.go"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`, map[string]string{"domains/schedule.yaml": "paths: [\"internal/**\"]\n"})
	gitfixture.InitRepoAt(t, root)
	writeADR(t, root, "0001-scheduling.md", testsupport.ADR("Implemented", testsupport.WithDomains("schedule"), testsupport.WithTitle("0001: Scheduling contracts"), testsupport.WithBody("## Decision\n\n1. Define scheduling.\n\n## Invariants\n\n- `invariant: legacy-scheduling` - legacy authority remains stable.\n")))
	testsupport.WriteFile(t, filepath.Join(root, "internal/legacy_test.go"), "package internal\n// invariant: legacy-scheduling\n// invariant: schedule\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	scaffold, err := topic.ScaffoldFilesWithExists(testConfig(p), "schedule", "Contracts", func(path string) (bool, error) {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return err == nil, err
	})
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

### `+"`invariant: stable-output`"+`
Scheduling output is stable.
Backing: test
`)
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule.go"), "package schedule\n// state: schedule/contracts:deterministic-order\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule_test.go"), "package schedule\n// invariant: schedule/contracts:stable-output (TestStableOutput)\nfunc TestStableOutput() {}\n")

	p, err = loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	corpus := mustDeriveTopics(t, p)
	completed, ok := corpus.ByTopicID("schedule/contracts")
	if !ok || len(completed.Claims) != 2 {
		t.Fatalf("completed topic = %#v, found %v", completed, ok)
	}
	if err := syncProject(p); err != nil {
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
		cmd := exec.Command(binary, append([]string{"read", "topic"}, args...)...)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("awf read topic %v: %v: %s", args, err, output)
		}
		return string(output)
	}
	human := runQuery("schedule/contracts", "--references", "--coverage")
	for _, value := range []string{"title: Scheduling", "summary: Current scheduling contracts.", "identity: schedule/contracts:deterministic-order", "prose: Scheduling output is stable.", "marker: internal/schedule_test.go:2"} {
		if !strings.Contains(human, value) {
			t.Errorf("topic text missing %q:\n%s", value, human)
		}
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
	p, err = loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, pruned, err := syncReportProject(p)
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
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
