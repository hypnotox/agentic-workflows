package project

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// stagedHeadFiles is the HEAD content the staged/range fixtures share: a config
// with a currentState policy, a domain, a one-claim topic scoped to
// internal/foo/**, and the Implemented ADR the claim cites.
func stagedHeadFiles() map[string]string {
	return map[string]string{
		".awf/awf.lock":                                `{"awfVersion":"0.18.0","schemaVersion":14,"files":{},"adrFormatV1From":2,"legacyAdrGaps":[]}`,
		".awf/config.yaml":                             csYAML,
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": csRuleTopic,
		"docs/decisions/0001-first.md": testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First"),
			testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
	}
}

// attestedLock returns the permanent cutoff used by staged fixtures.
func attestedLock() *manifest.Lock {
	return &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14, ADRFormatV1From: 2, LegacyADRGaps: []int{}}
}

// writeLock writes and stages the project's awf.lock.
func writeLock(t *testing.T, p *Project, lock *manifest.Lock) {
	t.Helper()
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, lockFile(p.Root), string(b))
	repo, err := gogit.PlainOpen(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add(".awf/awf.lock"); err != nil {
		t.Fatal(err)
	}
}

// TestCheckStagedCleanWithCoverage stages a new owned-but-unscoped file over an
// unchanged HEAD topic set: the transition is clean while the index coverage
// reports the uncovered path, proving both sides load and the HEAD-to-index diff
// runs.
func TestCheckStagedCleanWithCoverage(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "head", nil)
	gitfixture.Stage(t, repo, dir, map[string]string{"internal/bar.go": "package internalx\n"})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	// A different working lock must not contaminate the staged universe.
	testsupport.WriteFile(t, lockFile(p.Root), "{not json")

	report, err := p.CheckStaged()
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if len(report.Static) != 0 {
		t.Fatalf("static findings = %#v; want none for an unchanged topic set", report.Static)
	}
	findings := report.Findings()
	if len(findings) != 1 || !strings.Contains(findings[0], "internal/bar.go") {
		t.Fatalf("findings = %#v; want exactly the uncovered internal/bar.go", findings)
	}
}

func TestCheckStagedRejectsPermanentFormatAuthorityMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cutoff int
		gaps   []int
	}{
		{"raise cutoff", 3, []int{}},
		{"alter gaps", 2, []int{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, dir := gitfixture.InitRepo(t)
			gitfixture.Stage(t, repo, dir, stagedHeadFiles())
			gitfixture.Commit(t, repo, dir, "head", nil)
			p := openStaged(t, dir)
			writeLock(t, p, &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14, ADRFormatV1From: tc.cutoff, LegacyADRGaps: tc.gaps})
			if _, err := p.CheckStaged(); err == nil || !strings.Contains(err.Error(), "immutable") {
				t.Fatalf("CheckStaged mutation error = %v", err)
			}
		})
	}
}

func TestCheckStagedRejectsInitializedVersionMutation(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = `{"awfVersion":"0.18.0","schemaVersion":14,"files":{},"adrFormatV1From":2,"legacyAdrGaps":[],"initializedWithVersion":"0.18.0"}`
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "head", nil)
	p := openStaged(t, dir)
	writeLock(t, p, &manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 14, ADRFormatV1From: 2, LegacyADRGaps: []int{}, InitializedWithVersion: "0.19.0"})
	if _, err := p.CheckStaged(); err == nil || !strings.Contains(err.Error(), "initializedWithVersion") {
		t.Fatalf("CheckStaged init-version mutation error = %v", err)
	}
}

func TestCheckStagedAllowsSealedBridgePromotion(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = `{"awfVersion":"0.18.0","schemaVersion":14,"files":{},"bridgeAttestation":{"version":1,"preparedHead":"x","treeDigest":"sha256:x","adrFormatV1From":2,"legacyADRGaps":[]}}`
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "bridge", nil)
	p := openStaged(t, dir)
	writeLock(t, p, &manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 14, ADRFormatV1From: 2, LegacyADRGaps: []int{}})
	if _, err := p.CheckStaged(); err != nil {
		t.Fatalf("sealed promotion: %v", err)
	}
}

// TestCheckStagedTransitionFinding stages a claim removal with no removing ADR:
// the HEAD-to-index diff surfaces the unmatched mutation.
func TestCheckStagedTransitionFinding(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "head", nil)
	// Re-stage the topic part with its claim removed.
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	report, err := p.CheckStaged()
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if len(report.Static) == 0 || !strings.Contains(report.Static[0].Message, "was removed with no ADR remove operation") {
		t.Fatalf("static = %#v; want the unmatched-removal finding", report.Static)
	}
}

// TestCheckStagedNestedAdopter validates HEAD/index snapshots through a project
// rooted inside a containing monorepo.
func TestCheckStagedNestedAdopter(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := map[string]string{}
	for path, body := range stagedHeadFiles() {
		files["examples/sundial/"+path] = body
	}
	lockBytes, err := attestedLock().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	files["examples/sundial/.awf/awf.lock"] = string(lockBytes)
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "nested head", nil)
	gitfixture.Stage(t, repo, dir, map[string]string{
		"examples/sundial/.awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n",
	})
	p := openStaged(t, filepath.Join(dir, "examples", "sundial"))
	report, err := p.CheckStaged()
	if err != nil {
		t.Fatalf("nested CheckStaged: %v", err)
	}
	if !strings.Contains(strings.Join(report.Findings(), "\n"), "was removed with no ADR remove operation") {
		t.Fatalf("nested findings = %#v; want staged transition finding", report.Findings())
	}
}

// TestCheckStagedUnmergedIndex rejects a conflicted index at the staged-check
// boundary rather than attempting to construct a partial after universe.
func TestCheckStagedUnmergedIndex(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "head", nil)
	idx, err := repo.Storer.Index()
	if err != nil {
		t.Fatal(err)
	}
	idx.Entries = append(idx.Entries, &index.Entry{Name: "conflict.md", Mode: filemode.Regular, Stage: index.OurMode})
	if err := repo.Storer.SetIndex(idx); err != nil {
		t.Fatal(err)
	}
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(); !errors.Is(err, awfgit.ErrIndexUnmerged) {
		t.Fatalf("CheckStaged unmerged index: got %v, want ErrIndexUnmerged", err)
	}
}

// TestCheckStagedNoHead covers the unborn-HEAD before side: a repository with no
// commit yet stages a complete covered universe.
func TestCheckStagedNoHead(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := stagedHeadFiles()
	files["internal/foo/x.go"] = "package foo\n"
	gitfixture.Stage(t, repo, dir, files)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	report, err := p.CheckStaged()
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if len(report.Findings()) != 0 {
		t.Fatalf("findings = %#v; want none (covered universe, bootstrap add)", report.Findings())
	}
}

// TestCheckStagedNoStagedConfig covers the missing index config: the working tree
// carries a config so Open succeeds, but it is never staged.
func TestCheckStagedNoStagedConfig(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nskills: [tdd]\nagents: [code-reviewer]\n")
	gitfixture.Stage(t, repo, dir, map[string]string{"internal/x.go": "package x\n"})
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(); err == nil || !strings.Contains(err.Error(), "no staged") {
		t.Fatalf("CheckStaged err = %v; want a no-staged-config error", err)
	}
}

// TestCheckStagedRequiresStagedLock proves an adopted staged universe cannot
// silently fall back to cutoff zero when its lock is deleted. The same staged
// slice also deletes a governed current-state-v1 ADR, which cutoff zero would
// misroute as legacy and fail to diagnose.
func TestCheckStagedRequiresStagedLock(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := stagedHeadFiles()
	files["docs/decisions/0002-v1.md"] = "---\nformat: current-state-v1\nstatus: Proposed\ndate: 2026-07-20\n---\n" +
		"# ADR-0002: V1\n\n## Context\n\nContext.\n\n## Decision\n\n1. Decide.\n\n" +
		"## State changes\n\nNone.\n\n## Consequences\n\nConsequence.\n\n" +
		"## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-20: Proposed\n"
	lockBytes, err := attestedLock().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	files[".awf/awf.lock"] = string(lockBytes)
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "head", nil)
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".awf/awf.lock", "docs/decisions/0002-v1.md"} {
		if _, err := wt.Remove(path); err != nil {
			t.Fatalf("stage deletion of %s: %v", path, err)
		}
	}
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(); err == nil || !strings.Contains(err.Error(), "no staged .awf/awf.lock") {
		t.Fatalf("CheckStaged err = %v; want required staged-lock diagnostic", err)
	}
}

// TestCheckStagedLockError covers the lock-read failure in the staged check.
func TestCheckStagedLockError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "head", nil)
	p := openStaged(t, dir)
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/awf.lock": "{not json"})
	if _, err := p.CheckStaged(); err == nil {
		t.Fatal("expected a lock parse error")
	}
}

func TestCheckStagedRejectsCorruptHeadLock(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = "{not json"
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "head", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	if _, err := p.CheckStaged(); err == nil || !strings.Contains(err.Error(), "snapshot lock") {
		t.Fatalf("corrupt HEAD lock error = %v", err)
	}
}

// TestCheckStagedHeadLoadError covers a load failure on the HEAD (before) side: a
// committed ADR whose frontmatter does not parse.
func TestCheckStagedHeadLoadError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := stagedHeadFiles()
	files["docs/decisions/0001-first.md"] = "---\nstatus: [unterminated\n---\n# X\n"
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "head", nil)
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(); err == nil {
		t.Fatal("expected a HEAD-side corpus load error")
	}
}

func TestCheckStagedIndexConfigValidationError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "head", nil)
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/config.yaml": "prefix: \"\"\n"})
	testsupport.WriteFile(t, filepath.Join(dir, ".awf/config.yaml"), csYAML)
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(); err == nil || !strings.Contains(err.Error(), "prefix") {
		t.Fatalf("validation error = %v", err)
	}
}

// TestCheckStagedIndexLoadError covers a load failure on the index (after) side:
// HEAD is clean, but a malformed ADR is staged.
func TestCheckStagedIndexLoadError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "head", nil)
	gitfixture.Stage(t, repo, dir, map[string]string{"docs/decisions/0002-bad.md": "---\nstatus: [unterminated\n---\n# X\n"})
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(); err == nil {
		t.Fatal("expected an index-side corpus load error")
	}
}

// TestCheckStagedOutsideRepo covers the before-side HEAD probe failing: a
// scaffolded project that is not a git repository.
func TestCheckStagedOutsideRepo(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [tdd]\nagents: [code-reviewer]\n", nil)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.CheckStaged(); err == nil {
		t.Fatal("expected an error outside a git repository")
	}
}

// TestCheckStagedHeadConfigParseError covers loadTreeCurrentState's config parse
// failure: the committed HEAD config is malformed while the working tree carries
// a valid one, so Open succeeds but the HEAD universe cannot load.
func TestCheckStagedHeadConfigParseError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/config.yaml": "prefix: example\nskills: [tdd\n"})
	gitfixture.Commit(t, repo, dir, "head", nil)
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nskills: [tdd]\nagents: []\n")
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(); err == nil {
		t.Fatal("expected a HEAD-side config parse error")
	}
}

// TestRangePairUniversesErrors covers the two error branches: an unresolvable rev
// (RangePair fails) and a commit whose first-parent tree cannot load.
func TestRangePairUniversesErrors(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nskills: [tdd]\nagents: []\n")
	p := openStaged(t, dir)
	if _, _, err := p.rangePairUniverses("does-not-exist", 0, nil); err == nil {
		t.Fatal("expected an unresolvable-rev error")
	}
	// A child whose first-parent commit carries a malformed config.
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/config.yaml": "prefix: example\nskills: [tdd\n"})
	gitfixture.Commit(t, repo, dir, "bad parent", nil)
	child := gitfixture.Commit(t, repo, dir, "child", map[string]string{"note.txt": "x"})
	if _, _, err := p.rangePairUniverses(child.String(), 0, nil); err == nil {
		t.Fatal("expected a before-side load error from the malformed parent")
	}
}

// openStaged opens a project whose config is on disk (staged or untracked),
// failing the test on error.
func openStaged(t *testing.T, dir string) *Project {
	t.Helper()
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestAuditTransitionsClean accepts a range whose only mutation is a bootstrap
// claim first appearing below the cutoff: no add operation is owed.
func TestAuditTransitionsClean(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "cutover", nil)
	p := openStaged(t, dir)

	findings, err := p.auditTransitions(base.String(), "HEAD", 2, nil)
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %#v; want none", findings)
	}
}

// TestAuditTransitionsFinding reports the unmatched claim removal at the commit
// that removed it while leaving the bootstrap-add commit clean.
func TestAuditTransitionsFinding(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "cutover", nil)
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	gitfixture.Commit(t, repo, dir, "drop claim", nil)
	p := openStaged(t, dir)

	findings, err := p.auditTransitions(base.String(), "HEAD", 2, nil)
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
	}
	var errs []audit.Finding
	for _, f := range findings {
		if f.Severity != audit.Error || f.Rule != "current-state-transition" {
			t.Fatalf("unexpected finding %#v", f)
		}
		errs = append(errs, f)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Detail, "was removed with no ADR remove operation") {
		t.Fatalf("findings = %#v; want one unmatched-removal error", findings)
	}
}

// TestAuditTransitionsWarning warns, rather than aborting, when a commit's
// current-state universes cannot load (a malformed staged ADR).
func TestAuditTransitionsWarning(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/config.yaml": csYAML, ".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"})
	gitfixture.Commit(t, repo, dir, "config", nil)
	gitfixture.Stage(t, repo, dir, map[string]string{"docs/decisions/0001-bad.md": "---\nstatus: [unterminated\n---\n# X\n"})
	gitfixture.Commit(t, repo, dir, "bad adr", nil)
	p := openStaged(t, dir)

	findings, err := p.auditTransitions(base.String(), "HEAD", 2, nil)
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
	}
	var warned bool
	for _, f := range findings {
		if f.Severity == audit.Warning && strings.Contains(f.Detail, "could not load the current-state universes") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("findings = %#v; want a load warning", findings)
	}
}

// TestAuditTransitionsCollectError propagates an unresolvable range.
func TestAuditTransitionsCollectError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nskills: [tdd]\nagents: [code-reviewer]\n")
	p := openStaged(t, dir)
	if _, err := p.auditTransitions("does-not-exist", "HEAD", 2, nil); err == nil {
		t.Fatal("expected an unresolvable-range error")
	}
}

// TestCheckStagedIgnoresWorkingTree proves the staged check reads the index and
// HEAD, never the working tree: a garbage working-tree topic part that would fail
// to parse leaves the staged result clean.
func TestCheckStagedIgnoresWorkingTree(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "head", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	// Corrupt the topic part on disk only; the index and HEAD keep the valid one.
	testsupport.WriteFile(t, filepath.Join(dir, ".awf/topics/parts/alpha/one/current-state.md"), "garbage, no Claims section\n")

	report, err := p.CheckStaged()
	if err != nil {
		t.Fatalf("CheckStaged must ignore the dirty working tree, got: %v", err)
	}
	if len(report.Static) != 0 || len(report.Findings()) != 0 {
		t.Fatalf("expected a clean staged result despite the dirty working tree, got static=%#v findings=%#v", report.Static, report.Findings())
	}
}

// TestAuditTransitionsMerge proves first-parent merge integration: a claim
// removed on a branch is validated at the merge commit against its first parent,
// so the merge's transition pair reports the unmatched removal.
func TestAuditTransitionsMerge(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	b0 := gitfixture.Commit(t, repo, dir, "mainline", nil)
	// Branch work: remove the claim.
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	f1 := gitfixture.Commit(t, repo, dir, "remove claim", nil)
	// Merge f1 into mainline: first parent b0 (claim present), tree = f1 (claim removed).
	merge := gitfixture.Merge(t, repo, "merge", b0, f1)
	p := openStaged(t, dir)

	findings, err := p.auditTransitions(b0.String(), merge.String(), 2, nil)
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
	}
	var mergeReported bool
	for _, f := range findings {
		if f.Subject == "merge" && f.Severity == audit.Error && strings.Contains(f.Detail, "was removed with no ADR remove operation") {
			mergeReported = true
		}
	}
	if !mergeReported {
		t.Fatalf("findings = %#v; want the merge commit's first-parent removal reported", findings)
	}
}

// TestAuditIncludesTransitions proves p.Audit appends the transition findings to
// the range-rule findings.
func TestAuditIncludesTransitions(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "cutover", nil)
	gitfixture.Stage(t, repo, dir, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	gitfixture.Commit(t, repo, dir, "drop claim", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	findings, _, err := p.Audit(base.String(), "HEAD")
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	var found bool
	for _, f := range findings {
		if f.Rule == "current-state-transition" && strings.Contains(f.Detail, "was removed with no ADR remove operation") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Audit findings = %#v; want the transition finding appended", findings)
	}
}
