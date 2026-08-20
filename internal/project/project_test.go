package project

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// scaffold writes a .awf/config.yaml tree under a fresh temp root.
func scaffold(t *testing.T, configYAML string) string {
	return scaffoldFiles(t, configYAML, nil)
}

// testLayout returns a complete .layout template map - every key
// Layout.templateMap produces, with canonical docs/-rooted values - so a test
// that renders a template directly doesn't need to hand-pick which keys that
// template happens to reference today. A future Layout field addition only
// needs updating here, not at every hand-built fixture across the package.
func testLayout() map[string]any {
	return map[string]any{
		"docsDir":                "docs",
		"adrDir":                 "docs/decisions",
		"indexMd":                "docs/decisions/INDEX.md",
		"adrReadme":              "docs/decisions/README.md",
		"adrTemplate":            "docs/decisions/template.md",
		"plansDir":               "docs/plans",
		"plansReadme":            "docs/plans/README.md",
		"plansTemplate":          "docs/plans/template.md",
		"docs":                   map[string]any{},
		"workflowRef":            "docs/workflow.md",
		"docStandard":            "docs/doc-standard.md",
		"agentsMdStandard":       "docs/agents-md-standard.md",
		"workingWithAwf":         "docs/working-with-awf.md",
		"maintainableCodeDesign": "docs/maintainable-code-design.md",
		"configReference":        "docs/config-reference.md",
		"domainsDir":             "docs/domains",
	}
}

// scaffoldFiles writes config.yaml plus optional sidecar/part files keyed by path
// relative to .awf/ (e.g. "skills/tdd.yaml", "skills/parts/x/y.md").
func scaffoldFiles(t *testing.T, configYAML string, files map[string]string) string {
	t.Helper()
	return scaffoldFilesRaw(t, withTestGateCmd(withTestProfile(configYAML)), files)
}

func withTestProfile(configYAML string) string {
	if strings.Contains(configYAML, "profile:") {
		return configYAML
	}
	lines := strings.Split(strings.TrimSuffix(configYAML, "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "prefix:") {
			lines = slices.Insert(lines, i+1, "profile: full")
			break
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func scaffoldFilesRaw(t *testing.T, configYAML string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, configYAML)
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", rel), body)
	}
	return root
}

func withTestGateCmd(configYAML string) string {
	if strings.Contains(configYAML, "gateCmd:") {
		return configYAML
	}
	lines := strings.Split(strings.TrimSuffix(configYAML, "\n"), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "vars:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "vars:"))
		switch {
		case rest == "" || rest == "{}":
			lines[i] = "vars:"
			lines = slices.Insert(lines, i+1, "  gateCmd: test-gate")
		case strings.HasPrefix(rest, "{") && strings.HasSuffix(rest, "}"):
			inside := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rest, "{"), "}"))
			if inside != "" {
				inside += ", "
			}
			lines[i] = "vars: {" + inside + "gateCmd: test-gate}"
		}
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.Join(lines, "\n") + "\nvars:\n  gateCmd: test-gate\n"
}

func TestCommitPolicyManifestProjection(t *testing.T) {
	const base = "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: make gate}\n"
	root := scaffold(t, base)
	syncAndLoad := func() *manifest.Lock {
		t.Helper()
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Sync(); err != nil {
			t.Fatal(err)
		}
		lock, err := manifest.Load(lockFile(root))
		if err != nil {
			t.Fatal(err)
		}
		return lock
	}
	absent := syncAndLoad()
	absentAgain := syncAndLoad()
	if !reflect.DeepEqual(absent, absentAgain) {
		t.Fatal("repeated absent-policy sync changed the manifest")
	}
	consumerPath := "docs/architecture.md"
	unrelatedPath := "AGENTS.md"
	consumerBefore, ok := absent.Files[consumerPath]
	if !ok {
		t.Fatalf("manifest missing consumer %s", consumerPath)
	}
	unrelatedBefore, ok := absent.Files[unrelatedPath]
	if !ok {
		t.Fatalf("manifest missing unrelated output %s", unrelatedPath)
	}
	generateKey := func(name string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", path).CombinedOutput(); err != nil {
			t.Fatalf("generate SSH key: %v: %s", err, output)
		}
		body, err := os.ReadFile(path + ".pub")
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(body))
		if len(fields) < 2 {
			t.Fatalf("generated public key = %q", body)
		}
		return strings.Join(fields[:2], " ")
	}
	key1, key2 := generateKey("one"), generateKey("two")
	policyYAML := func(baseline, name, email string, signed bool, principal, key string) string {
		body := fmt.Sprintf("%scommitPolicy:\n  grandfatheredThrough: %s\n  allowedIdentities:\n    - name: %s\n      email: %s\n", base, baseline, name, email)
		if signed {
			body += fmt.Sprintf("  requireSignedCommits: true\n  allowedSigners:\n    - principal: %s\n      key: %s\n", principal, key)
		}
		return body
	}
	variants := []string{
		policyYAML(strings.Repeat("a", 40), "Ada", "ada@example.test", true, "ada@example.test", key1),
		policyYAML(strings.Repeat("b", 40), "Ada", "ada@example.test", true, "ada@example.test", key1),
		policyYAML(strings.Repeat("b", 40), "Grace", "grace@example.test", true, "ada@example.test", key1),
		policyYAML(strings.Repeat("b", 40), "Grace", "grace@example.test", false, "", ""),
		policyYAML(strings.Repeat("b", 40), "Grace", "grace@example.test", true, "grace@example.test", key2),
	}
	previous := consumerBefore.ConfigHash
	for i, policy := range variants {
		testsupport.WriteAwfConfig(t, root, policy)
		lock := syncAndLoad()
		if lock.Files[consumerPath].ConfigHash == previous {
			t.Fatalf("normalized policy mutation %d did not change consumer manifest hash", i)
		}
		if lock.Files[unrelatedPath].ConfigHash != unrelatedBefore.ConfigHash {
			t.Fatalf("unrelated manifest hash changed with policy mutation %d", i)
		}
		previous = lock.Files[consumerPath].ConfigHash
	}
}

// gitScaffold writes gitSampleYAML into a fresh git-backed root whose checkout
// sits on branch. A test that exercises branch-aware behaviour needs a real
// repository, because the branch is read through the git seam rather than
// injected (ADR-0193 keeps branch detection in one home).
func gitScaffold(t *testing.T, branch string) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	if branch != defaultFixtureBranch {
		gitfixture.NativeBranch(t, repo, branch)
		gitfixture.NativeCheckout(t, repo, branch)
	}
	testsupport.WriteAwfConfig(t, root, gitSampleYAML)
	return root
}

// defaultFixtureBranch is the branch a go-git fixture repository starts on.
const defaultFixtureBranch = "master"

// gitSampleYAML is sampleYAML with the integration branch pointed at the
// fixture default, so a plain git-backed scaffold is "on" the integration
// branch without any extra checkout.
const gitSampleYAML = `prefix: example
integrationBranch: master
vars:
  testCmd: go test ./...
  gateCmd: make gate
  gateCmdFull: make gate full
`

// lockFile is the relocated lock path under the tree.
func lockFile(root string) string {
	return filepath.Join(root, ".awf", "awf.lock")
}

// configPath is the tree config file path.
func configPath(root string) string {
	return filepath.Join(root, ".awf", "config.yaml")
}

const sampleYAML = `prefix: example
profile: full
integrationBranch: main
vars:
  testCmd: go test ./...
  gateCmd: make gate
  gateCmdFull: make gate full
`

func TestNewADRErrors(t *testing.T) {
	root := gitScaffold(t, defaultFixtureBranch)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.NewADR(testContext(t), "Missing Lock"); err == nil {
		t.Fatal("expected missing lock error")
	}
}

// On the integration branch the scaffold allocates a number; off it - another
// branch, a detached HEAD, or a tree with no repository at all - it writes the
// slug-identified pending form (ADR-0202 item 5).
// invariant: adr-system/adr-lifecycle:adr-new-sequential-numbering (TestNewADRIsBranchAware)
func TestNewADRIsBranchAware(t *testing.T) {
	for _, tc := range []struct {
		name     string
		wantBase string
		setup    func(t *testing.T) string
	}{
		{
			name: "on the integration branch numbers", wantBase: "0001-branch-aware-title.md",
			setup: func(t *testing.T) string { return gitScaffold(t, defaultFixtureBranch) },
		},
		{
			name: "another branch is pending", wantBase: "branch-aware-title.md",
			setup: func(t *testing.T) string { return gitScaffold(t, "effort/side") },
		},
		{
			name: "detached HEAD is pending", wantBase: "branch-aware-title.md",
			setup: func(t *testing.T) string {
				root := gitScaffold(t, defaultFixtureBranch)
				repo := gitfixture.At(root)
				gitfixture.NativeCheckout(t, repo, gitfixture.NativeRevParse(t, repo, "HEAD"))
				return root
			},
		},
		{
			name: "no repository is pending", wantBase: "branch-aware-title.md",
			setup: func(t *testing.T) string { return scaffold(t, gitSampleYAML) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Sync(); err != nil {
				t.Fatal(err)
			}
			path, err := p.NewADR(testContext(t), "Branch Aware Title")
			if err != nil {
				t.Fatalf("NewADR: %v", err)
			}
			if got := filepath.Base(path); got != tc.wantBase {
				t.Fatalf("scaffolded %q, want %q", got, tc.wantBase)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "slug: branch-aware-title\n") {
				t.Errorf("scaffold missing the retained slug key:\n%s", body)
			}
		})
	}
}

// Generation 21 removes the obsolete workflow roots and generation 22 resets
// the standalone memory root, while the three roots awf still owns keep every
// dynamic descendant through migration, sync, and render alike.
//
// invariant: rendering/singletons-and-payloads:resident-output-preservation (TestResidentMigrationsPreserveOwnedRootsThroughProjectSync)
func TestResidentMigrationsPreserveOwnedRootsThroughProjectSync(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: "0.24.0", SchemaVersion: 20, Files: map[string]manifest.Entry{}, InitializedWithVersion: "0.24.0"}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"metrics", "assignments"} {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", name, "obsolete", "resident"), name)
	}
	for _, name := range []string{"efforts", "memory", "worktrees", "effort-archive"} {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", name, "retained", "nested", "resident.go"), name)
	}

	if _, _, err := migrate.Upgrade(testContext(t), root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"metrics", "assignments"} {
		if _, err := os.Lstat(filepath.Join(root, ".awf", name)); !os.IsNotExist(err) {
			t.Fatalf("obsolete %s root remains after migration: %v", name, err)
		}
	}
	for _, name := range []string{"efforts", "worktrees", "effort-archive"} {
		path := filepath.Join(root, ".awf", name, "retained", "nested", "resident.go")
		if got, err := os.ReadFile(path); err != nil || string(got) != name {
			t.Fatalf("retained %s resident changed: %q, %v", name, got, err)
		}
	}
	// The standalone memory root is reset, not migrated: generation 22 stops
	// owning it, so the whole root goes with the journaled schema advance.
	if _, err := os.Lstat(filepath.Join(root, ".awf", "memory")); !os.IsNotExist(err) {
		t.Fatalf("standalone memory root survived the schema-22 reset: %v", err)
	}

	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderAll(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"metrics", "assignments"} {
		if _, err := os.Lstat(filepath.Join(root, ".awf", name)); !os.IsNotExist(err) {
			t.Fatalf("obsolete %s root was recreated by sync/render: %v", name, err)
		}
	}
	for _, name := range []string{"efforts", "worktrees", "effort-archive"} {
		path := filepath.Join(root, ".awf", name, "retained", "nested", "resident.go")
		if got, err := os.ReadFile(path); err != nil || string(got) != name {
			t.Fatalf("retained %s resident changed after sync/render: %q, %v", name, got, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "memory")); !os.IsNotExist(err) {
		t.Fatalf("sync/render recreated the standalone memory root: %v", err)
	}
}

func TestRetiredPlanResyncDuplicateSelectionsUpgradeAndSync(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\nskills: [reviewing-plan-resync, reviewing-plan-resync]\nagents: []\ntargets: [claude]\n")
	lock := &manifest.Lock{AWFVersion: "0.31.0", SchemaVersion: 37, Files: map[string]manifest.Entry{}, InitializedWithVersion: "0.31.0"}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := migrate.Upgrade(testContext(t), root); err != nil {
		t.Fatal(err)
	}
	configBytes, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configBytes), "reviewing-plan-resync") {
		t.Fatalf("retired duplicate survived upgrade:\n%s", configBytes)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	applied, changes, err := migrate.Upgrade(testContext(t), root)
	if err != nil || len(applied) != 0 || len(changes) != 0 {
		t.Fatalf("second upgrade = %v, %v, %v", applied, changes, err)
	}
}

func TestSyncWritesFilesAndLock(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	b, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("skill not written: %v", err)
	}
	if !strings.Contains(string(b), "# example-tdd") || strings.Contains(string(b), "awf:section") {
		t.Errorf("rendered skill wrong:\n%s", b)
	}
	if !strings.Contains(string(b), "GENERATED by awf") || !strings.Contains(string(b), "<!-- awf:edit ") {
		t.Errorf("rendered skill missing provenance banner/pointer:\n%s", b)
	}
	for _, rel := range []string{".claude/agents/code-reviewer.md", ".awf/awf.lock"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

func TestCheckCleanAfterSync(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Errorf("expected clean, got drift: %#v", drift)
	}
}

func TestCheckDetectsHandEdit(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	_ = p.Sync()
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	_ = os.WriteFile(skill, []byte("hand edited\n"), 0o644)
	drift, _ := checkProject(p, testContext(t))
	if len(drift) == 0 || drift[0].Kind != "hand-edited" {
		t.Errorf("expected hand-edited drift, got %#v", drift)
	}
}

func TestCheckStaleTakesPrecedence(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skillPath := ".claude/skills/example-tdd/SKILL.md"
	// Make the lock entry stale by corrupting its TemplateHash.
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	e := lock.Files[skillPath]
	e.TemplateHash = "sha256:bogus"
	lock.Files[skillPath] = e
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	// Also hand-edit the rendered file so its on-disk content differs too.
	if err := os.WriteFile(filepath.Join(root, skillPath), []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	var forPath []manifest.Drift
	for _, d := range drift {
		if d.Path == skillPath {
			forPath = append(forPath, d)
		}
	}
	if len(forPath) != 1 {
		t.Fatalf("expected exactly one drift entry for %s, got %#v", skillPath, forPath)
	}
	if forPath[0].Kind != "stale" {
		t.Errorf("expected stale, got %q", forPath[0].Kind)
	}
}

func TestSyncRendersDeclaredDoc(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "architecture.md"))
	if err != nil {
		t.Fatalf("docs/architecture.md not written: %v", err)
	}
	if !strings.Contains(string(b), "# Architecture") {
		t.Errorf("docs/architecture.md missing heading:\n%s", b)
	}
}

// TestSyncAutoLinksDocsInAgentsDoc covers the project-level wiring that the
// template golden cannot: RenderAll injects resolvedDocs() into the agents-doc
// data map so the Document map auto-links every declared (non-local) doc with
// its catalog title/desc. A local doc must not appear.
func TestOpenRejectsMalformedRepository(t *testing.T) {
	root := scaffold(t, sampleYAML)
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(testContext(t), root); err == nil || errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("malformed repository error = %v", err)
	}
}

func TestOpenValidConfigSucceeds(t *testing.T) {
	root := scaffold(t, sampleYAML)
	_, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("expected valid config to open cleanly, got: %v", err)
	}
}

func TestSyncNeverPrunesResidentEffortsDescendants(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	const rel = ".awf/efforts/efforts/e/sessions/s.jsonl"
	path := filepath.Join(root, filepath.FromSlash(rel))
	testsupport.WriteFile(t, path, "resident\n")
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files[rel] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, pruned, err := p.SyncReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(pruned, rel) {
		t.Fatalf("resident path reported pruned: %v", pruned)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("resident path removed: %v", err)
	}
}

func TestSyncRejectsUnsafeResidentEffortsRoot(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	efforts := filepath.Join(root, ".awf", "efforts")
	if err := os.RemoveAll(efforts); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), efforts); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil {
		t.Fatal("sync accepted an unsafe resident efforts root")
	}
}

func TestSyncPruneFailureKeepsLockEntry(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	const retired = "obsolete/generated.md"
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.Files[retired] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(retired))
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(path, "resident"), "keep\n")
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil || !strings.Contains(err.Error(), "remove retired output") {
		t.Fatalf("prune error = %v", err)
	}
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files[retired]; !ok {
		t.Fatal("failed prune disappeared from the saved lock")
	}
}

func TestSyncPruneReportSkipsAlreadyGoneFile(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	_ = p.Sync()
	// Hand-delete the rendered file before the pruning sync: the report must
	// not claim a removal the prune did not perform.
	if err := os.Remove(filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	noTDD := strings.Replace(sampleYAML, "  - tdd\n", "", 1)
	_ = os.WriteFile(configPath(root), []byte(noTDD), 0o644)
	p2, _ := Open(testContext(t), root)
	_, _, pruned, err := p2.SyncReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(pruned, ".claude/skills/example-tdd/SKILL.md") {
		t.Errorf("already-gone file must not be reported pruned: %v", pruned)
	}
}

type replacementFailureFilesystem struct {
	syncFilesystem
	path string
	err  error
}

func (f replacementFailureFilesystem) Replace(path string, contents []byte, mode os.FileMode) error {
	if path == f.path {
		return f.err
	}
	return f.syncFilesystem.Replace(path, contents, mode)
}

type publicationFailureFilesystem struct {
	syncFilesystem
	err   error
	calls *int
}

func (f publicationFailureFilesystem) Publish(string, []byte, os.FileMode) error {
	*f.calls++
	return f.err
}

type removalFailureFilesystem struct {
	syncFilesystem
	path string
	err  error
}

func (f removalFailureFilesystem) Remove(path string) error {
	if path == f.path {
		return f.err
	}
	return f.syncFilesystem.Remove(path)
}

type readFailureFilesystem struct {
	syncFilesystem
	err error
}

func (f readFailureFilesystem) Read(string) ([]byte, error) { return nil, f.err }

type pathReadWithModeFailureFilesystem struct {
	syncFilesystem
	path string
	err  error
}

func (f pathReadWithModeFailureFilesystem) ReadWithMode(path string) ([]byte, os.FileMode, error) {
	if path == f.path {
		return nil, 0, f.err
	}
	return f.syncFilesystem.ReadWithMode(path)
}

type linkInfoFailureFilesystem struct {
	syncFilesystem
	err error
}

func (f linkInfoFailureFilesystem) LinkInfo(string) (os.FileInfo, error) { return nil, f.err }

type pathLinkInfoFailureFilesystem struct {
	syncFilesystem
	path string
	err  error
}

func (f pathLinkInfoFailureFilesystem) LinkInfo(path string) (os.FileInfo, error) {
	if path == f.path {
		return nil, f.err
	}
	return f.syncFilesystem.LinkInfo(path)
}

type chmodFailureFilesystem struct {
	syncFilesystem
	err error
}

func (f chmodFailureFilesystem) Chmod(string, os.FileMode) error { return f.err }

type swapBeforePublishFilesystem struct {
	syncFilesystem
	root, outside string
	swapped       bool
}

func (f *swapBeforePublishFilesystem) Publish(path string, contents []byte, mode os.FileMode) error {
	if !f.swapped {
		dir := filepath.Dir(filepath.Join(f.root, filepath.FromSlash(path)))
		if err := os.Rename(dir, dir+"-saved"); err != nil {
			return err
		}
		if err := os.Symlink(f.outside, dir); err != nil {
			return err
		}
		f.swapped = true
	}
	return f.syncFilesystem.Publish(path, contents, mode)
}

type swapAfterPruneFilesystem struct {
	syncFilesystem
	root, outside string
	calls         []string
}

type swapBeforeLockReplaceFilesystem struct {
	syncFilesystem
	root, outside string
	swapped       bool
}

func (f *swapBeforeLockReplaceFilesystem) Replace(path string, contents []byte, mode os.FileMode) error {
	if path == ".awf/awf.lock" && !f.swapped {
		if err := os.Rename(filepath.Join(f.root, ".awf"), filepath.Join(f.root, "saved-awf")); err != nil {
			return err
		}
		if err := os.Symlink(f.outside, filepath.Join(f.root, ".awf")); err != nil {
			return err
		}
		f.swapped = true
	}
	return f.syncFilesystem.Replace(path, contents, mode)
}

func (f *swapAfterPruneFilesystem) Remove(path string) error {
	f.calls = append(f.calls, path)
	err := f.syncFilesystem.Remove(path)
	if path == "cleanup/child/file" && err == nil {
		dir := filepath.Join(f.root, "cleanup")
		if renameErr := os.Rename(dir, dir+"-saved"); renameErr != nil {
			return renameErr
		}
		if linkErr := os.Symlink(f.outside, dir); linkErr != nil {
			return linkErr
		}
	}
	return err
}

func TestSyncFilesystemsRouteUnchangedPaths(t *testing.T) {
	tracked := &readFailureFilesystem{}
	residentTree := &readFailureFilesystem{}
	filesystems := syncFilesystems{tracked: tracked, resident: residentTree}
	for _, tc := range []struct {
		path string
		want syncFilesystem
	}{
		{"AGENTS.md", tracked},
		{".awf/efforts/.gitignore", residentTree},
	} {
		got, path := filesystems.output(tc.path)
		if got != tc.want || path != tc.path {
			t.Fatalf("output(%q) = %T, %q; want %T, unchanged path", tc.path, got, path, tc.want)
		}
	}
}

func TestOpenSyncFilesystemsComposesDistinctRootsBeforeMutation(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	roots := p.state.roots
	setTestRoots(p, resident.NewRoots(roots.Tracked, t.TempDir()))
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	if filesystems.tracked == filesystems.resident {
		t.Fatal("distinct roots reused one handle")
	}
	roots = p.state.roots
	setTestRoots(p, resident.NewRoots(roots.Tracked, filepath.Join(root, "missing")))
	if _, _, err := openSyncFilesystems(renderInputsForTest(p)); err == nil {
		t.Fatal("missing resident root opened")
	}
	roots = p.state.roots
	setTestRoots(p, resident.NewRoots(filepath.Join(root, "missing-tracked"), roots.Resident))
	if _, _, err := openSyncFilesystems(renderInputsForTest(p)); err == nil {
		t.Fatal("missing tracked root opened")
	}
}

func TestSyncReportOpensDistinctResidentRootBeforeTrackedMutation(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("hand edit\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(agents, 0o640); err != nil {
		t.Fatal(err)
	}
	beforeLock, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	missingResident := t.TempDir()
	if err := os.Remove(missingResident); err != nil {
		t.Fatal(err)
	}
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	roots := p.state.roots
	setTestRoots(p, resident.NewRoots(roots.Tracked, missingResident))
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil {
		t.Fatal("sync accepted missing distinct resident root")
	}
	if got, err := os.ReadFile(agents); err != nil || string(got) != "hand edit\n" {
		t.Fatalf("tracked output changed before resident open refusal = %q, %v", got, err)
	}
	assertPerm(t, agents, 0o640)
	if got, err := os.ReadFile(lockFile(root)); err != nil || !reflect.DeepEqual(got, beforeLock) {
		t.Fatalf("lock changed before resident open refusal = %q, %v", got, err)
	}
}

func TestSyncFilesystemFailuresPreserveErrorIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		wrap func(syncFilesystems, error) syncFilesystems
	}{
		{"lock read", func(filesystems syncFilesystems, failure error) syncFilesystems {
			filesystems.tracked = readFailureFilesystem{syncFilesystem: filesystems.tracked, err: failure}
			return filesystems
		}},
		{"output link info", func(filesystems syncFilesystems, failure error) syncFilesystems {
			filesystems.tracked = linkInfoFailureFilesystem{syncFilesystem: filesystems.tracked, err: failure}
			return filesystems
		}},
		{"resident marker chmod", func(filesystems syncFilesystems, failure error) syncFilesystems {
			filesystems.resident = chmodFailureFilesystem{syncFilesystem: filesystems.resident, err: failure}
			return filesystems
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
				t.Fatal(err)
			}
			filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(p))
			if err != nil {
				t.Fatal(err)
			}
			defer closeAll()
			failure := errors.New(tc.name)
			corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, err = syncReportWithPitfalls(renderInputsForTest(p), testContext(t), nil, tc.wrap(filesystems, failure), corpus, pitfalls, topics, eff)
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v, want %v", err, failure)
			}
		})
	}
}

// invariant: rendering/sync-and-drift:local-doc-prune-preserved (TestLocalDocPruneUnreadableSourcePreservesRecoveryAndLock)
func TestLocalDocPruneUnreadableSourcePreservesRecoveryAndLock(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	const local = "docs/runbooks/incident.md"
	output := filepath.Join(root, filepath.FromSlash(local))
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	p.Cfg.LocalDocs = nil
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	failure := errors.New("unreadable local prune source")
	filesystems.tracked = pathReadWithModeFailureFilesystem{syncFilesystem: filesystems.tracked, path: local, err: failure}
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	backups, _, pruned, err := syncReportWithPitfalls(renderInputsForTest(p), testContext(t), nil, filesystems, corpus, pitfalls, topics, eff)
	if !errors.Is(err, failure) {
		t.Fatalf("sync error = %v, want %v", err, failure)
	}
	if len(backups) != 0 || len(pruned) != 0 {
		t.Fatalf("failed prune published backups=%v or pruned=%v", backups, pruned)
	}
	if got, readErr := os.ReadFile(output); readErr != nil || !bytes.Equal(got, before) {
		t.Fatalf("source = %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(output + ".awf-bak"); !os.IsNotExist(statErr) {
		t.Fatalf("backup published after unreadable source: %v", statErr)
	}
	if got, readErr := os.ReadFile(lockFile(root)); readErr != nil || !strings.Contains(string(got), local) {
		t.Fatalf("lock = %q, %v", got, readErr)
	}
}

// invariant: rendering/sync-and-drift:local-doc-prune-preserved (TestLocalDocPruneFaultsKeepRecoveryAndLock)
func TestLocalDocPruneFaultsKeepRecoveryAndLock(t *testing.T) {
	for _, tc := range []struct {
		name string
		wrap func(syncFilesystem, error) syncFilesystem
	}{
		{"backup publication", func(f syncFilesystem, err error) syncFilesystem {
			return publicationFailureFilesystem{syncFilesystem: f, err: err, calls: new(int)}
		}},
		{"inspection", func(f syncFilesystem, err error) syncFilesystem {
			return pathLinkInfoFailureFilesystem{syncFilesystem: f, path: "docs/runbooks/incident.md", err: err}
		}},
		{"removal after backup", func(f syncFilesystem, err error) syncFilesystem {
			return removalFailureFilesystem{syncFilesystem: f, path: "docs/runbooks/incident.md", err: err}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Sync(); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(root, "docs/runbooks/incident.md")
			before, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			p.Cfg.LocalDocs = nil
			filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(p))
			if err != nil {
				t.Fatal(err)
			}
			defer closeAll()
			failure := errors.New(tc.name)
			filesystems.tracked = tc.wrap(filesystems.tracked, failure)
			corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
			if err != nil {
				t.Fatal(err)
			}
			_, _, _, err = syncReportWithPitfalls(renderInputsForTest(p), testContext(t), nil, filesystems, corpus, pitfalls, topics, eff)
			if !errors.Is(err, failure) {
				t.Fatalf("sync error = %v, want %v", err, failure)
			}
			if got, readErr := os.ReadFile(lockFile(root)); readErr != nil || !strings.Contains(string(got), "docs/runbooks/incident.md") {
				t.Fatalf("lock = %q, %v", got, readErr)
			}
			if got, readErr := os.ReadFile(output); readErr != nil || !bytes.Equal(got, before) {
				t.Fatalf("source = %q, %v", got, readErr)
			}
			if tc.name == "removal after backup" {
				if got, readErr := os.ReadFile(output + ".awf-bak"); readErr != nil || !bytes.Equal(got, before) {
					t.Fatalf("recovery = %q, %v", got, readErr)
				}
			}
		})
	}
}

// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestBackupFileConfinedPropagatesPublicationFailure)
func TestBackupFileConfinedPropagatesPublicationFailure(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	failure := errors.New("publication failed")
	calls := 0
	_, err = backupFileConfined(".awf/config.yaml", publicationFailureFilesystem{syncFilesystem: filesystems.tracked, err: failure, calls: &calls})
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("backup error = %v, publication calls = %d", err, calls)
	}
}

func TestSyncReportDoesNotReportOutputWhenReplacementFails(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockPath(p.Root))
	if err != nil {
		t.Fatal(err)
	}
	e := lock.Files["AGENTS.md"]
	e.OutputHash = "different"
	lock.Files["AGENTS.md"] = e
	if err := lock.Save(lockPath(p.Root)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("hand edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	filesystems, closeAll, openErr := openSyncFilesystems(renderInputsForTest(p))
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer closeAll()
	failure := errors.New("replacement failed")
	filesystems.tracked = replacementFailureFilesystem{syncFilesystem: filesystems.tracked, path: "AGENTS.md", err: failure}
	corpus, pitfalls, topics, eff, deriveErr := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if deriveErr != nil {
		t.Fatal(deriveErr)
	}
	_, changes, _, err := syncReportWithPitfalls(renderInputsForTest(p), testContext(t), nil, filesystems, corpus, pitfalls, topics, eff)
	if !errors.Is(err, failure) || len(changes) != 0 {
		t.Fatalf("changes = %v, err = %v", changes, err)
	}
	if got, readErr := os.ReadFile(filepath.Join(root, "AGENTS.md")); readErr != nil || string(got) != "hand edit\n" {
		t.Fatalf("output = %q, err = %v", got, readErr)
	}
}

func TestSyncReportRetainsModeCorrectionWhenLaterWriteFails(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Chmod(agents, 0o600); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	filesystems, closeAll, openErr := openSyncFilesystems(renderInputsForTest(p))
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer closeAll()
	failure := errors.New("later replacement failed")
	filesystems.tracked = replacementFailureFilesystem{syncFilesystem: filesystems.tracked, path: "CLAUDE.md", err: failure}
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	_, changes, _, err := syncReportWithPitfalls(renderInputsForTest(p), testContext(t), nil, filesystems, corpus, pitfalls, topics, eff)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if want := []Change{{Path: "AGENTS.md", Cause: "internal"}}; !reflect.DeepEqual(changes, want) {
		t.Fatalf("changes = %v, want %v", changes, want)
	}
	assertPerm(t, agents, 0o644)
}

func TestSyncReportReportsContentAndModeOnce(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Chmod(agents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agents, []byte("hand edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	_, changes, _, err := p.SyncReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if want := []Change{{Path: "AGENTS.md", Cause: "internal"}}; !reflect.DeepEqual(changes, want) {
		t.Fatalf("changes = %v, want one record for both corrections: %v", changes, want)
	}
	assertPerm(t, agents, 0o644)
}

// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncReportForeignFinalSymlinkPolicy)
func TestSyncReportForeignFinalSymlinkPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		target  func(t *testing.T, root string) string
		wantErr bool
	}{
		{"in root", func(t *testing.T, root string) string {
			path := filepath.Join(root, "foreign")
			if err := os.WriteFile(path, []byte("foreign bytes\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			return "foreign"
		}, false},
		{"escaping", func(t *testing.T, _ string) string {
			path := filepath.Join(t.TempDir(), "foreign")
			if err := os.WriteFile(path, []byte("outside\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			return path
		}, true},
		{"broken", func(*testing.T, string) string { return "missing" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, sampleYAML)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
				t.Fatal(err)
			}
			lock, err := manifest.Load(lockPath(p.Root))
			if err != nil {
				t.Fatal(err)
			}
			delete(lock.Files, "AGENTS.md")
			if err := lock.Save(lockPath(p.Root)); err != nil {
				t.Fatal(err)
			}
			agents := filepath.Join(root, "AGENTS.md")
			if err := os.Remove(agents); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tc.target(t, root), agents); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			backups, _, _, err := p.SyncReport(testContext(t))
			if tc.wantErr {
				if err == nil {
					t.Fatal("foreign unsafe symlink was replaced")
				}
				if _, statErr := os.Lstat(agents); statErr != nil {
					t.Fatalf("unsafe link changed: %v", statErr)
				}
				if _, statErr := os.Stat(agents + ".awf-bak"); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("unsafe link backup = %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(backups) == 0 || backups[0].Bak != "AGENTS.md.awf-bak" {
				t.Fatalf("backups = %v", backups)
			}
			contents, readErr := os.ReadFile(agents + ".awf-bak")
			info, statErr := os.Stat(agents + ".awf-bak")
			if readErr != nil || statErr != nil || string(contents) != "foreign bytes\n" || info.Mode().Perm() != 0o640 {
				t.Fatalf("backup = %q, %v, %v", contents, info, errors.Join(readErr, statErr))
			}
			info, statErr = os.Lstat(agents)
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
				t.Fatalf("foreign link not replaced: %v, %v", info, statErr)
			}
		})
	}
}

func TestSyncReportReplacesManagedFinalSymlinkWithoutTargetAccess(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.Remove(agents); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-target", agents); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, _ = Open(testContext(t), root)
	if _, _, _, err := p.SyncReport(testContext(t)); err != nil {
		t.Fatalf("SyncReport: %v", err)
	}
	info, err := os.Lstat(agents)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("managed symlink = %v, %v", info, err)
	}
}

func TestSyncReportDoesNotReportOutputWhenWriteFails(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(output, []byte("hand edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	filesystems, closeAll, openErr := openSyncFilesystems(renderInputsForTest(p))
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer closeAll()
	failure := errors.New("replacement failed")
	filesystems.tracked = replacementFailureFilesystem{syncFilesystem: filesystems.tracked, path: "AGENTS.md", err: failure}
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	_, changes, _, err := syncReportWithPitfalls(renderInputsForTest(p), testContext(t), nil, filesystems, corpus, pitfalls, topics, eff)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if len(changes) != 0 {
		t.Fatalf("changes = %v, want no output evidence before the failed write", changes)
	}
	got, readErr := os.ReadFile(output)
	if readErr != nil || string(got) != "hand edit\n" {
		t.Fatalf("output = %q, err = %v; want original bytes", got, readErr)
	}
}

// TestSyncReportClassifiesChangedOutput stages every provenance cause by
// authoring the prior lock directly - the classification compares the old
// entry against the fresh render, so a tweaked stored hash simulates the
// corresponding real change (an upstream template edit, a config edit, a
// non-hashed input) without mutating the embedded templates.
func TestSyncReportClassifiesChangedOutput(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(testContext(t), root)
	_, changes, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Errorf("first sync has no baseline and must report no changes, got %v", changes)
	}
	lock, err := manifest.Load(lockPath(p.Root))
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(path string, f func(e *manifest.Entry)) {
		t.Helper()
		e, ok := lock.Files[path]
		if !ok {
			t.Fatalf("no lock entry for %s; have %v", path, slices.Sorted(maps.Keys(lock.Files)))
		}
		f(&e)
		lock.Files[path] = e
	}
	// Output moved + template hash moved → upstream churn.
	mutate("AGENTS.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.TemplateHash = "x" })
	// Output moved + config hash moved → the project's own inputs.
	mutate(".claude/skills/example-tdd/SKILL.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.ConfigHash = "x" })
	// Both hashes moved.
	mutate("CLAUDE.md", func(e *manifest.Entry) { e.OutputHash = "x"; e.TemplateHash = "x"; e.ConfigHash = "x" })
	// Output moved, real hashes unmoved → a non-hashed input.
	mutate(".awf/efforts/.gitignore", func(e *manifest.Entry) { e.OutputHash = "x" })
	// Output moved on a generated index (no hashes by design) → regenerated.
	mutate("docs/decisions/INDEX.md", func(e *manifest.Entry) { e.OutputHash = "x" })
	// No prior entry → added.
	delete(lock.Files, "docs/workflow.md")
	if err := lock.Save(lockPath(p.Root)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"AGENTS.md", ".claude/skills/example-tdd/SKILL.md", "CLAUDE.md", ".awf/efforts/.gitignore", "docs/decisions/INDEX.md", "docs/workflow.md"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte("stale output\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p2, _ := Open(testContext(t), root)
	_, changes, _, err = p2.SyncReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{
		{Path: ".awf/efforts/.gitignore", Cause: "internal"},
		{Path: ".claude/skills/example-tdd/SKILL.md", Cause: "config"},
		{Path: "AGENTS.md", Cause: "template"},
		{Path: "CLAUDE.md", Cause: "template+config"},
		{Path: "docs/decisions/INDEX.md", Cause: "regenerated"},
		{Path: "docs/workflow.md", Cause: "added"},
	}
	if !slices.Equal(changes, want) {
		t.Errorf("changes = %v\nwant %v (path-sorted; untouched files silent)", changes, want)
	}
}

func TestOpenRejectsUnknownSectionOverride(t *testing.T) {
	// tdd in the catalog has sections [surfaces, notes]; "bogus" is not declared.
	cfg := "prefix: example\nprofile: full\nintegrationBranch: main\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/tdd.yaml": "sections:\n  bogus:\n    drop: true\n",
	})
	_, err := Open(testContext(t), root)
	if err == nil {
		t.Fatal("expected error for unknown section override 'bogus'")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus', got: %v", err)
	}
	// The label carries the artifact name for a named artifact (name != ""), so
	// the message identifies which skill; assert it so that branch is pinned.
	if !strings.Contains(err.Error(), `"tdd"`) {
		t.Errorf("error should name the offending skill \"tdd\", got: %v", err)
	}
}

func TestOpenAllowsValidSectionOverride(t *testing.T) {
	// "notes" is a declared section for tdd.
	cfg := "prefix: example\nprofile: full\nintegrationBranch: main\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/tdd.yaml": "sections:\n  notes:\n    drop: true\n",
	})
	_, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("valid section override 'notes' should succeed, got: %v", err)
	}
}

func TestOpenRejectsUnknownAgentSectionOverride(t *testing.T) {
	// code-reviewer in the catalog has sections universal-lenses/project-focus/doc-currency.
	cfg := "prefix: example\nprofile: full\nintegrationBranch: main\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"agents/code-reviewer.yaml": "sections:\n  bogus:\n    drop: true\n",
	})
	_, err := Open(testContext(t), root)
	if err == nil {
		t.Fatal("expected error for unknown agent section override 'bogus'")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus', got: %v", err)
	}
}

func TestSyncRendersAgentFromMap(t *testing.T) {
	root := scaffold(t, "prefix: myproject\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	agentPath := filepath.Join(root, ".claude/agents/code-reviewer.md")
	b, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("agent file not written: %v", err)
	}
	if !strings.Contains(string(b), "myproject") {
		t.Errorf("agent file should contain prefix 'myproject', got:\n%s", b)
	}
}

// TestSyncErrorsOnUnresolvedValueToken verifies the publication-safety net:
// Sync errors when rendered output contains the literal unresolved-value token.
// Since ADR-0045 every shipped var interpolation degrades gracefully, so the
// trigger here is content that carries the token itself (the ADR-0011/ADR-0014
// gotcha: prose containing the literal token trips the guard).
func TestSyncErrorsOnUnresolvedValueToken(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\n",
		map[string]string{
			"skills/tdd.yaml": "data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n",
		})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	err = p.Sync()
	if err == nil {
		t.Fatal("expected Sync to return an error on an unresolved-value token, got nil")
	}
	if !strings.Contains(err.Error(), "<no value>") {
		t.Errorf("error should mention \"<no value>\", got: %v", err)
	}
}

func TestSyncRendersAgentsDoc(t *testing.T) {
	t.Run("always-on by default", func(t *testing.T) {
		root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\n")
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := p.Sync(); err != nil {
			t.Fatalf("Sync: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatalf("AGENTS.md not written: %v", err)
		}
		if !strings.Contains(string(b), "example") {
			t.Errorf("AGENTS.md should contain prefix 'example', got:\n%s", b)
		}
	})
}

// TestSyncPrunesEmptySkillDir verifies that after a skill is removed from config
// and Sync runs again, both the SKILL.md file and its now-empty parent directory
// are removed.

// invariant: rendering/doc-outputs:layout-derivation (TestLayoutUsesFixedDocsRootAndFullCatalog)
// invariant: rendering/doc-outputs:docs-root-fixed (TestLayoutUsesFixedDocsRootAndFullCatalog)
func TestLayoutUsesFixedDocsRootAndFullCatalog(t *testing.T) {
	p := &Project{Cfg: &config.Config{}, cat: catalog.NewView(catalog.Standard).Catalog()}
	l := layout(renderInputsForTest(p))
	if l.DocsDir != config.DocsDir || l.ADRDir != "docs/decisions" ||
		l.IndexMd != "docs/decisions/INDEX.md" || l.PlansDir != "docs/plans" {
		t.Errorf("layout = %+v", l)
	}
	// invariant: rendering/doc-outputs:domains-dir-given (TestLayoutUsesFixedDocsRootAndFullCatalog)
	if l.DomainsDir != "docs/domains" {
		t.Errorf("domainsDir = %q", l.DomainsDir)
	}
	// invariant: rendering/doc-outputs:layout-docs-profile-projection (TestLayoutUsesFixedDocsRootAndFullCatalog)
	if len(l.Docs) != len(catalog.Standard.Docs) {
		t.Errorf("Docs has %d entries, want full catalog of %d: %v", len(l.Docs), len(catalog.Standard.Docs), l.Docs)
	}
	for name, entry := range catalog.Standard.Docs {
		want := "docs/" + name + ".md"
		if entry.Path != "" {
			want = "docs/" + entry.Path
		}
		if entry.AgentsDoc {
			want = "AGENTS.md"
		}
		if got, ok := l.Docs[name]; !ok || got != want {
			t.Errorf("Docs[%q] = %q (present %t), want catalog-derived %q", name, got, ok, want)
		}
	}
	// templateMap reproduces the historical .layout map by literal value (ConfigHash
	// stability). The fixed directory keys are hand-built; the mandatory-singleton
	// keys derive from the catalog (ADR-0061) - assert each one's exact value so a
	// wrong derivation is caught, not just a present key.
	tm := l.templateMap()
	wantTM := map[string]string{
		"docsDir":                "docs",
		"adrDir":                 "docs/decisions",
		"indexMd":                "docs/decisions/INDEX.md",
		"plansDir":               "docs/plans",
		"domainsDir":             "docs/domains",
		"adrReadme":              "docs/decisions/README.md",
		"adrTemplate":            "docs/decisions/template.md",
		"plansReadme":            "docs/plans/README.md",
		"plansTemplate":          "docs/plans/template.md",
		"workflowRef":            "docs/workflow.md",
		"docStandard":            "docs/doc-standard.md",
		"agentsMdStandard":       "docs/agents-md-standard.md",
		"workingWithAwf":         "docs/working-with-awf.md",
		"maintainableCodeDesign": "docs/maintainable-code-design.md",
	}
	for k, want := range wantTM {
		if tm[k] != want {
			t.Errorf("templateMap[%q] = %v, want %q", k, tm[k], want)
		}
	}
	if got, ok := tm["docs"].(map[string]any); !ok || got["architecture"] != "docs/architecture.md" || got["debugging"] != "docs/debugging.md" {
		t.Errorf("templateMap[docs] = %v", tm["docs"])
	}
	// 5 fixed dir keys + docs + 11 mandatory-singleton keys = 17 (agents-doc has
	// no TemplateKey and is excluded; the generated config reference is
	// layout-exposed like its hash-checked siblings).
	if len(tm) != 17 {
		t.Errorf("templateMap has %d keys, want 17", len(tm))
	}
	if got := docOutPath(renderInputsForTest(p), "architecture"); got != "docs/architecture.md" {
		t.Errorf("docOutPath = %q", got)
	}
}

// invariant: rendering/project-output-plan:output-plan-complete (TestRenderAllRendersFullCatalogForBothTargets)
func TestRenderAllRendersFullCatalogForBothTargets(t *testing.T) {
	cfg := "prefix: example\nprofile: full\nintegrationBranch: main\n"
	root := scaffold(t, cfg)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := len(p.Targets); got != len(KnownTargets()) {
		t.Fatalf("targets = %d, want full built-in set of %d", got, len(KnownTargets()))
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	paths := map[string]bool{}
	for _, file := range files {
		paths[file.Path] = true
	}
	for _, target := range p.Targets {
		for name := range catalog.Standard.Skills {
			if path := target.SkillPath("example", name); !paths[path] {
				t.Errorf("missing catalog skill output %q", path)
			}
		}
		for name := range catalog.Standard.Agents {
			if path := target.AgentPath(name); !paths[path] {
				t.Errorf("missing catalog agent output %q", path)
			}
		}
	}
	for name, entry := range catalog.Standard.Docs {
		path := "docs/" + name + ".md"
		if entry.Path != "" {
			path = "docs/" + entry.Path
		}
		if entry.AgentsDoc {
			path = "AGENTS.md"
		}
		if !paths[path] {
			t.Errorf("missing catalog document %q at %q", name, path)
		}
	}
	for _, path := range []string{"docs/debugging.md", ".claude/skills/example-roadmap-graduation/SKILL.md", ".pi/skills/example-roadmap-graduation/SKILL.md"} {
		if !paths[path] {
			t.Errorf("missing unconditional catalog output %q", path)
		}
	}
	notes, err := p.AdvisoryNotes(testContext(t))
	if err != nil {
		t.Fatalf("AdvisoryNotes: %v", err)
	}
	joined := strings.Join(notes, "\n")
	for _, path := range []string{"docs/debugging.md", ".claude/skills/example-roadmap-graduation/SKILL.md", ".pi/skills/example-roadmap-graduation/SKILL.md"} {
		if !strings.Contains(joined, path+" has unauthored stub content") {
			t.Errorf("missing non-failing stub advisory for %s in:\n%s", path, joined)
		}
	}
}

// invariant: rendering/sync-and-drift:sync-always-writes-active-md (TestSyncGeneratesActiveMDAndCheckDetectsStaleness)
// invariant: rendering/sync-and-drift:check-active-md-stale (TestSyncGeneratesActiveMDAndCheckDetectsStaleness)
func TestSyncGeneratesActiveMDAndCheckDetectsStaleness(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adrBody := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Decision\n\n1. x.\n"))
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-first.md"), adrBody)

	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(adrDir, "INDEX.md"))
	if err != nil {
		t.Fatalf("INDEX.md not generated: %v", err)
	}
	// The Accepted ADR renders under the In flight status section.
	inflightPos := strings.Index(string(index), "## In flight")
	entryPos := strings.Index(string(index), "ADR-0001: First")
	if inflightPos < 0 || !strings.Contains(string(index), "## History") {
		t.Errorf("INDEX.md missing status sections:\n%s", index)
	}
	if entryPos < 0 || entryPos < inflightPos {
		t.Errorf("INDEX.md missing the ADR entry under In flight:\n%s", index)
	}
	if drift, err := checkProject(p, testContext(t)); err != nil || len(drift) != 0 {
		t.Fatalf("expected clean check after sync, got drift=%#v err=%v", drift, err)
	}

	// Change frontmatter status (Accepted In flight -> Implemented History); the
	// on-disk INDEX.md is now stale.
	adr2 := strings.Replace(adrBody, "status: Accepted", "status: Implemented", 1)
	testsupport.WriteFile(t, filepath.Join(adrDir, "0001-first.md"), adr2)
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, d := range drift {
		if strings.HasSuffix(d.Path, "decisions/INDEX.md") && d.Kind == "stale" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected stale drift for INDEX.md, got %#v", drift)
	}
}

// invariant: rendering/sync-and-drift:sync-always-writes-active-md (TestSyncRendersPlaceholderIndexMDWithoutADRs)
func TestSyncRendersPlaceholderIndexMDWithoutADRs(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "INDEX.md"))
	if err != nil {
		t.Fatalf("expected a placeholder INDEX.md when no ADRs exist: %v", err)
	}
	if !strings.Contains(string(got), "No decisions recorded yet") {
		t.Errorf("expected placeholder index, got:\n%s", got)
	}
	if drift, err := checkProject(p, testContext(t)); err != nil || len(drift) != 0 {
		t.Errorf("expected clean check with no ADRs, got drift=%#v err=%v", drift, err)
	}
}

// invariant: rendering/sync-and-drift:check-invalid-frontmatter (TestCheckDetectsInvalidFrontmatter)
func TestCheckDetectsInvalidFrontmatter(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	const skillPath = ".claude/skills/example-tdd/SKILL.md"
	broken := "---\nname: \"\"\ndescription: \"\"\n---\nbody\n"
	// Fresh planned bytes, the locked hash, and observed bytes all agree, so
	// frontmatter validation is the first applicable finding.
	testsupport.WriteFile(t, filepath.Join(root, skillPath), broken)
	file := RenderedFile{Path: skillPath, Content: broken, Policy: OutputPolicy{ValidateFrontmatter: true}, Encoder: MarkdownAgentDialect}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		skillPath: {OutputHash: manifest.Hash([]byte(broken))},
	}}
	drift := checkLockedFiles(renderInputsForTest(p).residentRoots(), lock, map[string]RenderedFile{skillPath: file}, nil)
	want := []manifest.Drift{{Path: skillPath, Kind: "invalid-frontmatter", Detail: "frontmatter name is empty"}}
	if !slices.Equal(drift, want) {
		t.Errorf("invalid-frontmatter drift = %#v, want %#v", drift, want)
	}
}

// invariant: rendering/singletons-and-payloads:adr-system-singletons-rendered (TestSyncReportBacksUpForeignIndexNotManaged)
// invariant: rendering/singletons-and-payloads:plain-singleton-via-renderkind (TestSyncReportBacksUpForeignIndexNotManaged)
// invariant: rendering/doc-outputs:working-with-awf-mandatory (TestSyncReportBacksUpForeignIndexNotManaged)
func TestSyncReportBacksUpForeignIndexNotManaged(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	lay := layout(renderInputsForTest(p))
	// Plant a foreign ADR index with hand content before the first sync (no lock yet),
	// so its path is absent from the prior lock and therefore foreign.
	foreign := filepath.Join(root, lay.IndexMd)
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	backups, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version})
	if err != nil {
		t.Fatalf("InitializeReport: %v", err)
	}
	var got *Backup
	for i := range backups {
		if backups[i].Path == lay.IndexMd {
			got = &backups[i]
		}
	}
	// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncReportBacksUpForeignIndexNotManaged)
	if got == nil {
		t.Fatalf("foreign INDEX.md not backed up; backups=%#v", backups)
	}
	if !got.Index {
		t.Errorf("INDEX.md backup must be flagged Index=true")
	}
	if b, _ := os.ReadFile(filepath.Join(root, got.Bak)); string(b) != "hand index\n" {
		t.Errorf("backup = %q, want original hand content", b)
	}
	// A path recorded in the prior lock is awf-managed: a second sync backs up
	// nothing and prunes nothing.
	again, _, pruned, err := p.SyncReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("re-sync of awf-managed output must not back up, got %#v", again)
	}
	if len(pruned) != 0 {
		t.Errorf("re-sync of awf-managed output must not prune, got %v", pruned)
	}
}

// The generated indexes carry RegenChecked=true (drift checked by regeneration,
// not the frozen OutputHash); an ordinary rendered file carries false. This is the
// single source of truth that replaced the hardcoded index-path literals.
func TestRegenCheckedAttribute(t *testing.T) {
	root := scaffold(t, domainCfg)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	// invariant: rendering/sync-and-drift:regeneration-checked-attribute (TestRegenCheckedAttribute)
	amd := generateIndexMD(renderInputsForTest(p), mustDeriveCorpus(t, p))
	if !amd.RegenChecked {
		t.Errorf("INDEX.md must be regeneration-checked")
	}
	dds, err := generateDomainDocs(renderInputsForTest(p), mustDeriveTopics(t, p), mustDeriveSkills(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if len(dds) == 0 {
		t.Fatal("fixture must declare at least one domain")
	}
	for _, dd := range dds {
		if !dd.RegenChecked {
			t.Errorf("domain doc %s must be regeneration-checked", dd.Path)
		}
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	cref, ok, err := generateConfigReference(renderInputsForTest(p), slices.Concat(files, dds), mustDeriveSkills(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !cref.RegenChecked {
		t.Errorf("config reference must be regeneration-checked (ok=%v)", ok)
	}
	// Ordinary planned writes are frozen-OutputHash-checked; generated plan
	// nodes are explicitly regeneration-checked.
	if len(files) == 0 {
		t.Fatal("RenderAll produced no files")
	}
	for _, f := range files {
		if f.Policy.Regenerate != f.RegenChecked {
			t.Errorf("plan policy and RegenChecked disagree for %s: policy=%v regenChecked=%v", f.Path, f.Policy.Regenerate, f.RegenChecked)
		}
	}
}

// invariant: rendering/guide-and-doc-templates:document-map-lists-mandatory-docs (TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally)
func TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	got := string(b)
	// Iterate the catalog's DocumentMap entries (default docsDir "docs") so a new
	// mandatory document-map doc cannot silently stop being covered (ADR-0061).
	mapped := 0
	for name, e := range catalog.Standard.Docs {
		if !e.DocumentMap {
			continue
		}
		mapped++
		// Assert the whole rendered line - title, link, and catalog desc - so a
		// mandatory doc is cited with its data-driven title/desc, not just linked.
		line := fmt.Sprintf("- **%s:** [docs/%s](docs/%s), %s", e.Title, e.Path, e.Path, e.Desc)
		if !strings.Contains(got, line) {
			t.Errorf("Document map should unconditionally cite %q (%s; %s", line, name, got)
		}
	}
	if mapped != 7 {
		t.Errorf("expected 7 DocumentMap entries, iterated %d", mapped)
	}
}

// A skill enabled without its dispatched agent fails project open - the error
// names both sides and the fix. The fixture carries reviewing-impl's skill
// closure so the agent edge is the failing one, and validation walks the
// declared config order, so reviewing-impl's missing code-reviewer is reported
// rather than executing-plans' missing implementer (ADR-0050, generalized by
// ADR-0081's closure validation).
func TestSyncRecordsTopicOutputsInManifest(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "contracts", "Contracts", "paths: [\"internal/**\"]\n")
	p, _ := Open(testContext(t), root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files["docs/topics/rendering/contracts.md"]; !ok {
		t.Fatal("topic output missing from manifest")
	}
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncMutationsStayWithinSelectedRoots)
// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncMutationsStayWithinSelectedRoots)
func TestSyncMutationsStayWithinSelectedRoots(t *testing.T) {
	root := scaffold(t, sampleYAML)
	residentRoot := t.TempDir()
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	roots := p.state.roots
	setTestRoots(p, resident.NewRoots(roots.Tracked, residentRoot))
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ root, path string }{
		{root, "AGENTS.md"},
		{residentRoot, ".awf/efforts/.gitignore"},
	} {
		if _, err := os.Stat(filepath.Join(tc.root, tc.path)); err != nil {
			t.Fatalf("selected-root output %s missing: %v", tc.path, err)
		}
	}

	// A foreign ordinary output keeps its bytes, mode, and report through the
	// tracked handle before replacement.
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	delete(lock.Files, "AGENTS.md")
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("foreign\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0o640); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	p.roots.Resident = residentRoot
	backups, _, _, err := p.SyncReport(testContext(t))
	if err != nil || !slices.Contains(backups, Backup{Path: "AGENTS.md", Bak: "AGENTS.md.awf-bak"}) {
		t.Fatalf("foreign backup = %v, error = %v", backups, err)
	}
	backup := filepath.Join(root, "AGENTS.md.awf-bak")
	if got, err := os.ReadFile(backup); err != nil || string(got) != "foreign\n" {
		t.Fatalf("backup bytes = %q, %v", got, err)
	}
	assertPerm(t, backup, 0o640)

	// A foreign resident output keeps the same lock-relative report while its
	// backup, replacement bytes, and final mode stay under the resident root.
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const residentOutput = ".awf/efforts/.gitignore"
	delete(lock.Files, residentOutput)
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	residentFile := filepath.Join(residentRoot, filepath.FromSlash(residentOutput))
	if err := os.WriteFile(residentFile, []byte("resident foreign\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(residentFile, 0o640); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	roots = p.state.roots
	setTestRoots(p, resident.NewRoots(roots.Tracked, residentRoot))
	backups, _, _, err = p.SyncReport(testContext(t))
	wantResidentBackup := Backup{Path: residentOutput, Bak: residentOutput + ".awf-bak"}
	if err != nil || !slices.Contains(backups, wantResidentBackup) {
		t.Fatalf("resident backup = %v, error = %v", backups, err)
	}
	residentBackup := residentFile + ".awf-bak"
	residentBackupBytes, residentBackupErr := os.ReadFile(residentBackup)
	residentOutputBytes, residentOutputErr := os.ReadFile(residentFile)
	if residentBackupErr != nil || residentOutputErr != nil || string(residentBackupBytes) != "resident foreign\n" || string(residentOutputBytes) == "resident foreign\n" {
		t.Fatalf("resident publication = backup %q output %q errors %v", residentBackupBytes, residentOutputBytes, errors.Join(residentBackupErr, residentOutputErr))
	}
	assertPerm(t, residentBackup, 0o640)
	assertPerm(t, residentFile, 0o644)

	// A managed final symlink is pruned as its entry, not by touching its target.
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const retired = "obsolete/managed"
	lock.Files[retired] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "obsolete"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, filepath.FromSlash(retired))); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, _ = Open(testContext(t), root)
	p.roots.Resident = residentRoot
	_, _, pruned, err := p.SyncReport(testContext(t))
	if err != nil || !slices.Contains(pruned, retired) {
		t.Fatalf("managed symlink prune = %v, %v", pruned, err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "outside\n" {
		t.Fatalf("symlink target changed = %q, %v", got, err)
	}

	// An escaping prune parent refuses at the converted removal path, preserves
	// outside bytes and mode, and leaves the old lock entry for retry.
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const escapingPrune = "escape-prune/victim"
	lock.Files[escapingPrune] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	beforePruneLock, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	outsidePrune := t.TempDir()
	outsideVictim := filepath.Join(outsidePrune, "victim")
	if err := os.WriteFile(outsideVictim, []byte("outside prune\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePrune, filepath.Join(root, "escape-prune")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, _ = Open(testContext(t), root)
	p.roots.Resident = residentRoot
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil {
		t.Fatal("sync accepted escaping prune parent")
	}
	if got, err := os.ReadFile(outsideVictim); err != nil || string(got) != "outside prune\n" {
		t.Fatalf("outside prune target changed = %q, %v", got, err)
	}
	assertPerm(t, outsideVictim, 0o600)
	if got, err := os.ReadFile(lockFile(root)); err != nil || !reflect.DeepEqual(got, beforePruneLock) {
		t.Fatalf("lock advanced after failed prune = %q, %v", got, err)
	}
	if err := os.Remove(filepath.Join(root, "escape-prune")); err != nil {
		t.Fatal(err)
	}
	lock, err = manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	delete(lock.Files, escapingPrune)
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}

	// An escaping output parent refuses before replacement or lock
	// advance, preserving outside bytes and modes.
	beforeLock, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	outsideRoot := t.TempDir()
	sentinel := filepath.Join(outsideRoot, "sentinel")
	if err := os.WriteFile(sentinel, []byte("outside bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "docs"), filepath.Join(root, "saved-docs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(root, "docs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, _ = Open(testContext(t), root)
	p.roots.Resident = residentRoot
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil {
		t.Fatal("sync accepted escaping output parent")
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "outside bytes\n" {
		t.Fatalf("outside output parent changed = %q, %v", got, err)
	}
	assertPerm(t, sentinel, 0o600)
	if got, err := os.ReadFile(lockFile(root)); err != nil || !reflect.DeepEqual(got, beforeLock) {
		t.Fatalf("lock advanced after incomplete output mutation = %q, %v", got, err)
	}
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncBackupPublicationRefusesParentSwap)
// invariant: rendering/sync-and-drift:sync-backs-up-foreign (TestSyncBackupPublicationRefusesParentSwap)
// invariant: rendering/sync-and-drift:local-doc-prune-preserved (TestSyncBackupPublicationRefusesParentSwap)
func TestSyncBackupPublicationRefusesParentSwap(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "collision"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "collision", "source"), []byte("source bytes\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	if err := os.WriteFile(sentinel, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	filesystem := &swapBeforePublishFilesystem{syncFilesystem: filesystems.tracked, root: root, outside: outside}
	if _, err := backupFileConfined("collision/source", filesystem); err == nil {
		t.Fatal("backup publication accepted swapped escaping parent")
	}
	if got, err := os.ReadFile(filepath.Join(root, "collision-saved", "source")); err != nil || string(got) != "source bytes\n" {
		t.Fatalf("backup source changed = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "source.awf-bak")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside backup published: %v", err)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "outside\n" {
		t.Fatalf("outside sentinel changed = %q, %v", got, err)
	}
	assertPerm(t, sentinel, 0o600)
}

// invariant: rendering/sync-and-drift:sync-mutations-root-confined (TestSyncAncestorCleanupRefusesParentSwap)
func TestSyncAncestorCleanupRefusesParentSwap(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	const retired = "cleanup/child/file"
	lock.Files[retired] = manifest.Entry{}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cleanup", "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(retired)), []byte("retired\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	if err := os.WriteFile(sentinel, []byte("outside cleanup\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, _ = Open(testContext(t), root)
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	swapping := &swapAfterPruneFilesystem{syncFilesystem: filesystems.tracked, root: root, outside: outside}
	filesystems.tracked = swapping
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	_, _, pruned, err := syncReportWithPitfalls(renderInputsForTest(p), testContext(t), nil, filesystems, corpus, pitfalls, topics, eff)
	if err != nil || !slices.Contains(pruned, retired) {
		t.Fatalf("cleanup sync = pruned %v, error %v", pruned, err)
	}
	for _, want := range []string{"cleanup/child/file", "cleanup/child", "cleanup"} {
		if !slices.Contains(swapping.calls, want) {
			t.Fatalf("cleanup calls = %v, missing %q", swapping.calls, want)
		}
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "outside cleanup\n" {
		t.Fatalf("outside cleanup changed = %q, %v", got, err)
	}
	assertPerm(t, sentinel, 0o600)
	updated, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := updated.Files[retired]; ok {
		t.Fatal("successfully pruned path remained in lock")
	}
}

// invariant: config/migrations-and-locks:lock-atomic-save (TestSyncLockSaveRefusesParentSwap)
func TestSyncLockSaveRefusesParentSwap(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	filesystems, closeAll, err := openSyncFilesystems(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	filesystems.tracked = &swapBeforeLockReplaceFilesystem{syncFilesystem: filesystems.tracked, root: root, outside: outside}
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := syncReportWithPitfalls(renderInputsForTest(p), testContext(t), nil, filesystems, corpus, pitfalls, topics, eff); err == nil {
		t.Fatal("sync accepted swapped lock parent during final save")
	}
	if _, err := os.Stat(filepath.Join(outside, "awf.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside lock = %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "saved-awf", "awf.lock")); err != nil || !reflect.DeepEqual(got, before) {
		t.Fatalf("saved lock advanced = %q, %v", got, err)
	}
}

func TestSyncLockRefusesEscapingParent(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Rename(filepath.Join(root, ".awf"), filepath.Join(root, "saved-awf")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".awf")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	// The selected handle refuses the symlinked lock parent before it can load
	// or advance authority; outside receives neither lock bytes nor a mode change.
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil {
		t.Fatal("sync accepted escaping lock parent")
	}
	if _, err := os.Stat(filepath.Join(outside, "awf.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside lock = %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "saved-awf", "awf.lock")); err != nil || !reflect.DeepEqual(got, before) {
		t.Fatalf("saved lock advanced = %q, %v", got, err)
	}
}

func TestInitializeAndSyncAuthorityRefusals(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil || !strings.Contains(err.Error(), "pre-tracking") {
		t.Fatalf("missing lock: %v", err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.InitializeReport(testContext(t), InitAuthority{InitializedWithVersion: Version}); err == nil || !strings.Contains(err.Error(), "absent lock") {
		t.Fatalf("repeat init: %v", err)
	}
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: 31, Files: map[string]manifest.Entry{}, BridgeAttestation: &manifest.BridgeAttestation{Version: 1, PreparedHead: "h", TreeDigest: "sha256:x", ADRFormatV1From: 1, LegacyADRGaps: []int{}}}
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.SyncReport(testContext(t)); err == nil || !strings.Contains(err.Error(), "permanent") {
		t.Fatalf("bridge sync: %v", err)
	}
}
