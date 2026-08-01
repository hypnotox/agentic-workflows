package project

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
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
	return &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14}
}

func boundaryADR(format, title string) string {
	return "---\nformat: " + format + "\nstatus: Proposed\ndate: 2026-07-21\n---\n" +
		"# ADR-0002: " + title + "\n\n## Context\n\nContext.\n\n## Decision\n\n1. Decide.\n\n" +
		"## State changes\n\nNone.\n\n## Consequences\n\nConsequence.\n\n" +
		"## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-21: Proposed\n"
}

func lockJSON(t *testing.T, lock *manifest.Lock) string {
	t.Helper()
	data, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func findBoundaryADR(records []adr.ADR) (adr.ADR, bool) {
	for _, record := range records {
		if record.Number == "0002" {
			return record, true
		}
	}
	return adr.ADR{}, false
}

// writeLock writes and stages the project's awf.lock.
func writeLock(t *testing.T, p *Project, lock *manifest.Lock) {
	t.Helper()
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, lockFile(p.Root), string(b))
	gitfixture.Add(t, gitfixture.At(p.Root), ".awf/awf.lock")
}

// TestCheckStagedCleanWithCoverage stages a new owned-but-unscoped file over an
// unchanged HEAD topic set: the transition is clean while the index coverage
// reports the uncovered path, proving both sides load and the HEAD-to-index diff
// runs.
func TestCheckStagedCleanWithCoverage(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{"internal/bar.go": "package internalx\n"})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	// A different working lock must not contaminate the staged universe.
	testsupport.WriteFile(t, lockFile(p.Root), "{not json")

	report, err := p.CheckStaged(testContext(t))
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

// TestCheckStagedNoPolicy proves the staged path evaluates coverage and fan-out
// for a tree that declares no currentState block, the staged half of the
// contract TestCheckCurrentStateNoPolicy pins for the working tree (ADR-0192).
// stagedHeadFiles already scopes one claim-bearing topic to internal/foo/**, so
// eight more claimless topics take that path over the nil-receiver budget of 8.
// invariant: rendering/sync-and-drift:coverage-evaluation-unconditional (TestCheckStagedNoPolicy)
func TestCheckStagedNoPolicy(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	head := stagedHeadFiles()
	head[".awf/config.yaml"] = csNoPolicyYAML
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("fan%d", i)
		head[".awf/topics/metadata/alpha/"+name+".yaml"] = fmt.Sprintf("title: Fan %d\nsummary: Fan-out fixture topic %d.\npaths:\n  - internal/foo/**\n", i, i)
		head[".awf/topics/parts/alpha/"+name+"/current-state.md"] = "Intro.\n\n## Claims\n"
	}
	gitfixture.Stage(t, repo, head)
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{
		"internal/bar.go":   "package internalx\n",
		"internal/foo/x.go": "package foo\n",
	})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	report, err := p.CheckStaged(testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if report.Coverage == nil {
		t.Fatal("staged coverage = nil; want evaluation without a currentState policy")
	}
	want := []topic.CoverageFinding{
		{Path: "internal/bar.go", Domain: "alpha", Kind: topic.Uncovered, Severity: severity.Error},
		{Path: "internal/foo/x.go", Kind: topic.Fanout, Severity: severity.Warn, Topics: 9},
	}
	if !reflect.DeepEqual(report.Coverage, want) {
		t.Fatalf("staged coverage:\n got %#v\nwant %#v", report.Coverage, want)
	}
}

// TestCheckStagedRejectsBridgePromotionWithArbitraryV2Boundary uses snapshots
// whose ADR-0002 bytes are valid only under each side's own lock: V1 under the
// bridge HEAD and V2 under the staged permanent lock. Phase 3 must reject that
// arbitrary V2 activation rather than treating it as the sealed V1 promotion.
// TestCheckStagedTransitionFinding stages a claim removal with no removing ADR:
// the HEAD-to-index diff surfaces the unmatched mutation.
func TestCheckStagedTransitionFinding(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	// Re-stage the topic part with its claim removed.
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	report, err := p.CheckStaged(testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if len(report.Static) == 0 || !strings.Contains(report.Static[0].Message, "was removed with no ADR remove operation") {
		t.Fatalf("static = %#v; want the unmatched-removal finding", report.Static)
	}
}

// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestCheckStagedMarksOlderIntroductionsProvisionalWithoutSuppressingFindings)
func TestCheckStagedMarksOlderIntroductionsProvisionalWithoutSuppressingFindings(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	head := stagedHeadFiles()
	head["docs/decisions/0003-existing.md"] = boundaryADR(adr.V1FormatMarker, "Existing")
	head["docs/decisions/0004-aggregate.md"] = publicV2ADR(t, "0004", "Aggregate", "Proposed", "- add `alpha/one:x`\n- add `alpha/one:y`\n- add `alpha/one:z`", "")
	gitfixture.Stage(t, repo, head)
	gitfixture.Commit(t, repo, "head", nil)

	gitfixture.Stage(t, repo, map[string]string{"docs/decisions/0002-stale.md": boundaryADR(adr.V2FormatMarker, "Stale")})
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	clean, err := p.CheckStaged(testContext(t))
	if err != nil {
		t.Fatalf("clean CheckStaged: %v", err)
	}
	want := []currentstate.Introduction{{Identity: "0002", Format: adr.CurrentStateV2}}
	if !reflect.DeepEqual(clean.Provisional, want) || len(clean.Findings()) != 0 {
		t.Fatalf("clean provisional report = %#v, findings = %#v; want %#v and no findings", clean.Provisional, clean.Findings(), want)
	}

	aggregate := publicV2ADR(t, "0004", "Aggregate", "Implementing", "- add `alpha/one:x`\n- add `alpha/one:y`\n- add `alpha/one:z`",
		"- 2026-07-22: Implementing; content-sha256: %s\n- 2026-07-22: Applied; operations: add `alpha/one:x`\n- 2026-07-22: Applied; operations: add `alpha/one:y`")
	gitfixture.Stage(t, repo, map[string]string{
		"docs/decisions/0003-existing.md":              boundaryADR(adr.V2FormatMarker, "Existing"),
		"docs/decisions/0004-aggregate.md":             aggregate,
		".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n",
		"internal/bar.go":                              "package internalx\n",
	})
	report, err := p.CheckStaged(testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged with unrelated violations: %v", err)
	}
	if !reflect.DeepEqual(report.Provisional, want) {
		t.Fatalf("provisional = %#v, want %#v", report.Provisional, want)
	}
	findings := strings.Join(report.Findings(), "\n")
	for _, wantFinding := range []string{
		"was removed with no ADR remove operation",
		"internal/bar.go",
		"changed governed format across this transition",
		"appends 2 application batches",
	} {
		if !strings.Contains(findings, wantFinding) {
			t.Fatalf("unrelated blocking finding %q was suppressed:\n%s", wantFinding, findings)
		}
	}
	if notes := strings.Join(report.Notes(), "\n"); !strings.Contains(notes, "provisional older-format ADR-0002") {
		t.Fatalf("provisional note missing:\n%s", notes)
	}
}

// TestCheckStagedNestedAdopter validates HEAD/index snapshots through a project
// rooted inside a containing monorepo.
func TestCheckStagedNestedAdopter(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := map[string]string{}
	for path, body := range stagedHeadFiles() {
		files["examples/sundial/"+path] = body
	}
	lockBytes, err := attestedLock().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	files["examples/sundial/.awf/awf.lock"] = string(lockBytes)
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "nested head", nil)
	gitfixture.Stage(t, repo, map[string]string{
		"examples/sundial/.awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n",
	})
	p := openStaged(t, filepath.Join(dir, "examples", "sundial"))
	report, err := p.CheckStaged(testContext(t))
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
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.StageUnmerged(t, repo, "conflict.md")
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(testContext(t)); !errors.Is(err, awfgit.ErrIndexUnmerged) {
		t.Fatalf("CheckStaged unmerged index: got %v, want ErrIndexUnmerged", err)
	}
}

// TestCheckStagedNoHead covers the unborn-HEAD before side: a repository with no
// commit yet stages a complete covered universe.
func TestCheckStagedNoHead(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files["internal/foo/x.go"] = "package foo\n"
	gitfixture.Stage(t, repo, files)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	report, err := p.CheckStaged(testContext(t))
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
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: [code-reviewer]\n")
	gitfixture.Stage(t, repo, map[string]string{"internal/x.go": "package x\n"})
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(testContext(t)); err == nil || !strings.Contains(err.Error(), "no staged") {
		t.Fatalf("CheckStaged err = %v; want a no-staged-config error", err)
	}
}

// TestCheckStagedRequiresStagedLock proves an adopted staged universe cannot
// silently fall back to cutoff zero when its lock is deleted. The same staged
// slice also deletes a governed current-state-v1 ADR, which cutoff zero would
// misroute as legacy and fail to diagnose.
func TestCheckStagedRequiresStagedLock(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
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
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.StageRemoval(t, repo, ".awf/awf.lock", "docs/decisions/0002-v1.md")
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(testContext(t)); err == nil || !strings.Contains(err.Error(), "no staged .awf/awf.lock") {
		t.Fatalf("CheckStaged err = %v; want required staged-lock diagnostic", err)
	}
}

// TestCheckStagedLockError covers the lock-read failure in the staged check.
func TestCheckStagedLockError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": "{not json"})
	if _, err := p.CheckStaged(testContext(t)); err == nil {
		t.Fatal("expected a lock parse error")
	}
}

func TestCheckStagedRejectsCorruptHeadLock(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = "{not json"
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	if _, err := p.CheckStaged(testContext(t)); err == nil || !strings.Contains(err.Error(), "snapshot lock") {
		t.Fatalf("corrupt HEAD lock error = %v", err)
	}
}

// TestCheckStagedHeadLoadError covers a load failure on the HEAD (before) side: a
// committed ADR whose frontmatter does not parse.
func TestCheckStagedHeadLoadError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files["docs/decisions/0001-first.md"] = "---\nstatus: [unterminated\n---\n# X\n"
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(testContext(t)); err == nil {
		t.Fatal("expected a HEAD-side corpus load error")
	}
}

func TestCheckStagedIndexConfigValidationError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "prefix: \"\"\n"})
	testsupport.WriteFile(t, filepath.Join(dir, ".awf/config.yaml"), csYAML)
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(testContext(t)); err == nil || !strings.Contains(err.Error(), "prefix") {
		t.Fatalf("validation error = %v", err)
	}
}

// TestCheckStagedIndexLoadError covers a load failure on the index (after) side:
// HEAD is clean, but a malformed ADR is staged.
func TestCheckStagedIndexLoadError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{"docs/decisions/0002-bad.md": "---\nstatus: [unterminated\n---\n# X\n"})
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(testContext(t)); err == nil {
		t.Fatal("expected an index-side corpus load error")
	}
}

// TestCheckStagedOutsideRepo covers the before-side HEAD probe failing: a
// scaffolded project that is not a git repository.
func TestCheckStagedOutsideRepo(t *testing.T) {
	t.Parallel()
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: [code-reviewer]\n", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.CheckStaged(testContext(t)); err == nil {
		t.Fatal("expected an error outside a git repository")
	}
}

// TestCheckStagedMigratesHistoricalWorkflowTelemetry compares a generation-19
// HEAD against the generation-20 index. The historical block is removed only
// from the immutable before-side bytes before the current strict parser runs.
func TestCheckStagedMigratesHistoricalWorkflowTelemetry(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, map[string]string{
		".awf/config.yaml": "prefix: example\nintegrationBranch: main\nworkflowTelemetry:\n  retention: {}\nrunner:\n  enabled: true\n",
		".awf/awf.lock":    `{"awfVersion":"0.20.0","schemaVersion":19,"files":{}}`,
	})
	gitfixture.Commit(t, repo, "generation 19", nil)
	gitfixture.Stage(t, repo, map[string]string{
		".awf/config.yaml": "prefix: example\nintegrationBranch: main\nrunner:\n  enabled: true\n",
		".awf/awf.lock":    `{"awfVersion":"0.20.0","schemaVersion":20,"files":{}}`,
	})
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(testContext(t)); err != nil {
		t.Fatalf("CheckStaged historical migration: %v", err)
	}
}

func TestCheckStagedRefusesHistoricalMalformedOrDuplicateConfigAndCurrentObsoleteBlock(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, headConfig, headLock, stagedConfig string
	}{
		{"historical malformed", "prefix: [\nworkflowTelemetry: {}\n", `{"schemaVersion":19,"files":{}}`, "prefix: example\nintegrationBranch: main\n"},
		{"historical duplicate", "prefix: example\nintegrationBranch: main\nworkflowTelemetry: {}\nworkflowTelemetry: {}\n", `{"schemaVersion":19,"files":{}}`, "prefix: example\nintegrationBranch: main\n"},
		{"current obsolete", "prefix: example\nintegrationBranch: main\n", `{"schemaVersion":20,"files":{}}`, "prefix: example\nintegrationBranch: main\nworkflowTelemetry: {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gitfixture.InitRepo(t)
			dir := repo.Root()
			gitfixture.Stage(t, repo, map[string]string{
				".awf/config.yaml": tc.headConfig,
				".awf/awf.lock":    tc.headLock,
			})
			gitfixture.Commit(t, repo, "head", nil)
			gitfixture.Stage(t, repo, map[string]string{
				".awf/config.yaml": tc.stagedConfig,
				".awf/awf.lock":    `{"schemaVersion":20,"files":{}}`,
			})
			p := &Project{Root: dir}
			if _, err := p.CheckStaged(testContext(t)); err == nil {
				t.Fatal("CheckStaged accepted invalid config")
			}
		})
	}
}

// TestCheckStagedHeadConfigParseError covers loadTreeCurrentState's config parse
// failure: the committed HEAD config is malformed while the working tree carries
// a valid one, so Open succeeds but the HEAD universe cannot load.
func TestCheckStagedHeadConfigParseError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "prefix: example\nintegrationBranch: main\nskills: [tdd\n"})
	gitfixture.Commit(t, repo, "head", nil)
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n")
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(testContext(t)); err == nil {
		t.Fatal("expected a HEAD-side config parse error")
	}
}

// TestRangePairUniversesErrors covers the two error branches: an unresolvable rev
// (RangePair fails) and a commit whose first-parent tree cannot load.
func TestRangePairUniversesErrors(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n")
	p := openStaged(t, dir)
	if _, _, err := p.rangePairUniverses(testContext(t), "does-not-exist"); err == nil {
		t.Fatal("expected an unresolvable-rev error")
	}
	// A child whose first-parent commit carries a malformed config.
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "prefix: example\nintegrationBranch: main\nskills: [tdd\n"})
	gitfixture.Commit(t, repo, "bad parent", nil)
	child := gitfixture.Commit(t, repo, "child", map[string]string{"note.txt": "x"})
	if _, _, err := p.rangePairUniverses(testContext(t), child); err == nil {
		t.Fatal("expected a before-side load error from the malformed parent")
	}
	gitfixture.Commit(t, repo, "restore config", map[string]string{".awf/config.yaml": "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n"})
	badLock := gitfixture.Commit(t, repo, "bad lock", map[string]string{".awf/awf.lock": "{"})
	if _, _, err := p.rangePairUniverses(testContext(t), badLock); err == nil {
		t.Fatal("expected an after-side lock parse error")
	}
	lockChild := gitfixture.Commit(t, repo, "bad lock child", map[string]string{"note.txt": "y"})
	if _, _, err := p.rangePairUniverses(testContext(t), lockChild); err == nil {
		t.Fatal("expected a before-side lock parse error")
	}
}

func TestCheckCommitAuthorizationPropagatesEvidenceErrors(t *testing.T) {
	msg := commitmsg.Message{}
	openRoot := func(t *testing.T, root string) *Project {
		t.Helper()
		p, err := openRootProject(root)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	t.Run("no checkout", func(t *testing.T) {
		if _, err := (&Project{Root: t.TempDir()}).CheckCommitAuthorization(testContext(t), msg); err == nil {
			t.Fatal("missing checkout succeeded")
		}
	})
	t.Run("malformed repository", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, ".git", "HEAD"), "broken\n")
		if _, err := (&Project{Root: root}).CheckCommitAuthorization(testContext(t), msg); err == nil {
			t.Fatal("malformed repository succeeded")
		}
	})
	t.Run("unborn HEAD", func(t *testing.T) {
		root := gitfixture.InitRepo(t).Root()
		if _, err := openRoot(t, root).CheckCommitAuthorization(testContext(t), msg); err == nil {
			t.Fatal("unborn HEAD succeeded")
		}
	})
	t.Run("unmerged index", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.StageUnmerged(t, repo, "conflict.md")
		if _, err := openRoot(t, repo.Root()).CheckCommitAuthorization(testContext(t), msg); err == nil {
			t.Fatal("unmerged index succeeded")
		}
	})
	t.Run("missing incoming object", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		testsupport.WriteFile(t, filepath.Join(repo.Root(), ".git", "MERGE_HEAD"), "0123456789012345678901234567890123456789\n")
		if _, err := openRoot(t, repo.Root()).CheckCommitAuthorization(testContext(t), msg); err == nil {
			t.Fatal("missing incoming object succeeded")
		}
	})
	t.Run("malformed first-parent lock", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.Commit(t, repo, "bad lock", map[string]string{".awf/awf.lock": "{"})
		if _, err := openRoot(t, repo.Root()).CheckCommitAuthorization(testContext(t), msg); err == nil {
			t.Fatal("malformed first-parent lock succeeded")
		}
	})
	t.Run("malformed result lock", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": "{"})
		if _, err := openRoot(t, repo.Root()).CheckCommitAuthorization(testContext(t), msg); err == nil {
			t.Fatal("malformed result lock succeeded")
		}
	})
	t.Run("malformed incoming lock", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		base := gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.CheckoutNewBranch(t, repo, "bad-parent", base)
		bad := gitfixture.Commit(t, repo, "bad lock", map[string]string{".awf/awf.lock": "{"})
		gitfixture.CheckoutNewBranch(t, repo, "integration", base)
		testsupport.WriteFile(t, filepath.Join(repo.Root(), ".git", "MERGE_HEAD"), bad+"\n")
		if _, err := openRoot(t, repo.Root()).CheckCommitAuthorization(testContext(t), msg); err == nil {
			t.Fatal("malformed incoming lock succeeded")
		}
	})
}

// openStaged opens a project whose config is on disk (staged or untracked),
// failing the test on error.
func openStaged(t *testing.T, dir string) *Project {
	t.Helper()
	p, err := Open(testContext(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestRangePairUniversesUsesEachFirstParentSnapshotBoundary changes both the
// lock boundary and ADR-0002 bytes across one commit. Each ADR is deliberately
// invalid under the other side's lock, so a parsed format-transition finding
// proves audit loading derived boundaries independently from both snapshots.
func TestRangePairUniversesUsesEachFirstParentSnapshotBoundary(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files["docs/decisions/0002-boundary.md"] = boundaryADR(adr.V1FormatMarker, "V1 side")
	gitfixture.Stage(t, repo, files)
	base := gitfixture.Commit(t, repo, "v1 boundary", nil)
	gitfixture.Stage(t, repo, map[string]string{
		".awf/awf.lock":                   lockJSON(t, &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14}),
		"docs/decisions/0002-boundary.md": boundaryADR(adr.V2FormatMarker, "V2 side"),
	})
	head := gitfixture.Commit(t, repo, "v2 boundary", nil)
	p := openStaged(t, dir)
	before, after, err := p.rangePairUniverses(testContext(t), head)
	if err != nil {
		t.Fatalf("rangePairUniverses: %v", err)
	}
	if got, ok := findBoundaryADR(before.ADRs); !ok || !got.IsV1() {
		t.Fatalf("before ADR-0002 = %#v, found=%v; want V1", got, ok)
	}
	if got, ok := findBoundaryADR(after.ADRs); !ok || !got.IsV2() {
		t.Fatalf("after ADR-0002 = %#v, found=%v; want V2", got, ok)
	}
	findings, err := p.auditTransitions(testContext(t), base, head)
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != severity.Error || !strings.Contains(findings[0].Detail, "changed governed format") {
		t.Fatalf("audit transitions findings=%#v; want the parsed format-transition error, not a snapshot-load warning", findings)
	}
}

// TestAuditTransitionsClean retains the live V1-only-lock case: its only
// mutation is a bootstrap claim first appearing below the cutoff, so no add
// operation is owed.
func TestAuditTransitionsClean(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "cutover", nil)
	p := openStaged(t, dir)

	findings, err := p.auditTransitions(testContext(t), base, "HEAD")
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
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "cutover", nil)
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	gitfixture.Commit(t, repo, "drop claim", nil)
	p := openStaged(t, dir)

	findings, err := p.auditTransitions(testContext(t), base, "HEAD")
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
	}
	var errs []audit.Finding
	for _, f := range findings {
		if f.Severity != severity.Error || f.Rule != "current-state-transition" {
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
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": csYAML, ".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"})
	gitfixture.Commit(t, repo, "config", nil)
	gitfixture.Stage(t, repo, map[string]string{"docs/decisions/0001-bad.md": "---\nstatus: [unterminated\n---\n# X\n"})
	gitfixture.Commit(t, repo, "bad adr", nil)
	p := openStaged(t, dir)

	findings, err := p.auditTransitions(testContext(t), base, "HEAD")
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
	}
	var warned bool
	for _, f := range findings {
		if f.Severity == severity.Warn && strings.Contains(f.Detail, "could not load the current-state universes") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("findings = %#v; want a load warning", findings)
	}
}

func TestAuditTransitionsRequiresComposedRepository(t *testing.T) {
	t.Parallel()
	p := &Project{Root: t.TempDir()}
	if _, err := p.auditTransitions(testContext(t), "base", "HEAD"); err == nil {
		t.Fatal("project without a composed repository accepted an audit range")
	}
}

// TestAuditTransitionsCollectError propagates an unresolvable range.
func TestAuditTransitionsCollectError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: [code-reviewer]\n")
	p := openStaged(t, dir)
	if _, err := p.auditTransitions(testContext(t), "does-not-exist", "HEAD"); err == nil {
		t.Fatal("expected an unresolvable-range error")
	}
}

// TestCheckStagedIgnoresWorkingTree proves the staged check reads the index and
// HEAD, never the working tree: a garbage working-tree topic part that would fail
// to parse leaves the staged result clean.
func TestCheckStagedIgnoresWorkingTree(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())
	// Corrupt the topic part on disk only; the index and HEAD keep the valid one.
	testsupport.WriteFile(t, filepath.Join(dir, ".awf/topics/parts/alpha/one/current-state.md"), "garbage, no Claims section\n")

	report, err := p.CheckStaged(testContext(t))
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
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	b0 := gitfixture.Commit(t, repo, "mainline", nil)
	// Branch work: remove the claim.
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	f1 := gitfixture.Commit(t, repo, "remove claim", nil)
	// Merge f1 into mainline: first parent b0 (claim present), tree = f1 (claim removed).
	merge := gitfixture.Merge(t, repo, "merge", b0, f1)
	p := openStaged(t, dir)

	findings, err := p.auditTransitions(testContext(t), b0, merge)
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
	}
	var mergeReported bool
	for _, f := range findings {
		if f.Subject == "merge" && f.Severity == severity.Error && strings.Contains(f.Detail, "was removed with no ADR remove operation") {
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
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "cutover", nil)
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	gitfixture.Commit(t, repo, "drop claim", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	findings, _, err := p.Audit(testContext(t), base, "HEAD")
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

// TestIncrementalADRLifecyclePublicPairs exercises incremental lifecycle history
// through the project staged checker and range audit rather than private
// currentstate helpers.
func TestIncrementalADRLifecyclePublicPairs(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = lockJSON(t, &manifest.Lock{AWFVersion: "0.20.0", SchemaVersion: 15, Files: map[string]manifest.Entry{}})
	v1Ops := "- add `alpha/one:v1`"
	files["docs/decisions/0002-v1-direct.md"] = strings.Replace(publicV2ADR(t, "0002", "V1 direct", "Proposed", v1Ops, ""), adr.V2FormatMarker, adr.V1FormatMarker, 1)
	gitfixture.Stage(t, repo, files)
	base := gitfixture.Commit(t, repo, "chore(config): adopt authority", nil)
	p := openStaged(t, dir)

	checkAndCommit := func(subject string, changes map[string]string) {
		t.Helper()
		gitfixture.Stage(t, repo, changes)
		report, err := p.CheckStaged(testContext(t))
		if err != nil {
			t.Fatalf("%s staged check: %v", subject, err)
		}
		if got := report.Findings(); len(got) != 0 {
			t.Fatalf("%s findings = %v", subject, got)
		}
		gitfixture.Commit(t, repo, subject, nil)
	}

	// Perform a real V1 Proposed -> Implemented operation transaction through
	// CheckStaged before exercising any V2 shape.
	v1Done := strings.Replace(publicV2ADR(t, "0002", "V1 direct", "Implemented", v1Ops,
		"- 2026-07-21: Implemented; content-sha256: %s"), adr.V2FormatMarker, adr.V1FormatMarker, 1)
	checkAndCommit("feat(invariants): apply v1 direct state", map[string]string{
		"docs/decisions/0002-v1-direct.md":             v1Done,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1"),
	})

	ops3 := "- add `alpha/one:a`\n- add `alpha/one:b`\n- add `alpha/one:c`"
	first3 := publicV2ADR(t, "0003", "Incremental", "Implementing", ops3,
		"- 2026-07-21: Implementing; content-sha256: %s\n- 2026-07-21: Applied; operations: add `alpha/one:a`")
	checkAndCommit("feat(invariants): apply first batch", map[string]string{
		"docs/decisions/0003-incremental.md":           first3,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a"),
	})
	corpus, err := adr.LoadCorpus(filepath.Join(dir, "docs", "decisions"))
	if err != nil {
		t.Fatal(err)
	}
	if index := adr.RenderIndexMD(corpus); !strings.Contains(index, "ADR-0003") || !strings.Contains(index, "Implementing") || strings.Index(index, "ADR-0003") > strings.Index(index, "## History") {
		t.Fatalf("Implementing ADR not in flight:\n%s", index)
	}

	// A direct batch from another ADR interleaves with the in-flight ADR.
	ops4 := "- add `alpha/one:x`"
	direct4 := publicV2ADR(t, "0004", "Interleave", "Implemented", ops4,
		"- 2026-07-21: Implemented; content-sha256: %s")
	checkAndCommit("feat(invariants): interleave direct batch", map[string]string{
		"docs/decisions/0004-interleave.md":            direct4,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "x"),
	})

	middle3 := publicV2ADR(t, "0003", "Incremental", "Implementing", ops3,
		"- 2026-07-21: Implementing; content-sha256: %s\n- 2026-07-21: Applied; operations: add `alpha/one:a`\n- 2026-07-22: Applied; operations: add `alpha/one:b`")
	checkAndCommit("feat(invariants): apply middle batch", map[string]string{
		"docs/decisions/0003-incremental.md":           middle3,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "x"),
	})

	final3 := publicV2ADR(t, "0003", "Incremental", "Implemented", ops3,
		"- 2026-07-21: Implementing; content-sha256: %s\n- 2026-07-21: Applied; operations: add `alpha/one:a`\n- 2026-07-22: Applied; operations: add `alpha/one:b`\n- 2026-07-23: Applied; operations: add `alpha/one:c`\n- 2026-07-23: Implemented; content-sha256: %s")
	checkAndCommit("feat(invariants): apply final batch", map[string]string{
		"docs/decisions/0003-incremental.md":           final3,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "x"),
	})

	ops5 := "- add `alpha/one:y`\n- add `alpha/one:z`"
	partial5 := publicV2ADR(t, "0005", "Partial", "Implementing", ops5,
		"- 2026-07-24: Implementing; content-sha256: %s\n- 2026-07-24: Applied; operations: add `alpha/one:y`")
	checkAndCommit("feat(invariants): apply partial batch", map[string]string{
		"docs/decisions/0005-partial.md":               partial5,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "x", "y"),
	})
	abandoned5 := publicV2ADR(t, "0005", "Partial", "Abandoned", ops5,
		"- 2026-07-24: Implementing; content-sha256: %s\n- 2026-07-24: Applied; operations: add `alpha/one:y`\n- 2026-07-25: Abandoned; content-sha256: %s; rationale: stop before z")
	checkAndCommit("docs(adr): abandon partial execution", map[string]string{"docs/decisions/0005-partial.md": abandoned5})

	// Reverse deletion plus its inverse mutation is still forbidden.
	gitfixture.Stage(t, repo, map[string]string{
		"docs/decisions/0005-partial.md":               publicV2ADR(t, "0005", "Partial", "Proposed", ops5, ""),
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "x"),
	})
	reversed, err := p.CheckStaged(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(reversed.Findings(), "\n"), "history") {
		t.Fatalf("reverse pair findings = %v", reversed.Findings())
	}
	gitfixture.Stage(t, repo, map[string]string{
		"docs/decisions/0005-partial.md":               abandoned5,
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "x", "y"),
	})

	// Commit a bad pair without checking, then a repairing endpoint. Static truth
	// at the endpoint cannot rescue the first-parent range violation.
	ops6 := "- add `alpha/one:q`"
	direct6 := publicV2ADR(t, "0006", "Bad intermediate", "Implemented", ops6,
		"- 2026-07-26: Implemented; content-sha256: %s")
	gitfixture.Stage(t, repo, map[string]string{"docs/decisions/0006-bad-intermediate.md": direct6})
	gitfixture.Commit(t, repo, "feat(invariants): record bad intermediate", nil)
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("v1", "a", "b", "c", "q", "x", "y")})
	head := gitfixture.Commit(t, repo, "fix(invariants): repair endpoint", nil)
	findings, _, err := p.Audit(testContext(t), base, head)
	if err != nil {
		t.Fatal(err)
	}
	var transitionFindings int
	for _, finding := range findings {
		if finding.Rule == currentStateTransitionRule {
			transitionFindings++
		}
	}
	if transitionFindings < 2 {
		t.Fatalf("range transition findings = %d, all = %#v", transitionFindings, findings)
	}
}

func publicV2ADR(t *testing.T, number, title, status, operations, tail string) string {
	t.Helper()
	build := func(history string) string {
		return "---\nformat: current-state-v2\nstatus: " + status + "\ndate: 2026-07-21\n---\n# ADR-" + number + ": " + title + "\n\n" +
			"## Context\n\nContext.\n\n## Decision\n\n1. Apply state.\n\n## State changes\n\n" + operations + "\n\n" +
			"## Consequences\n\nConsequence.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-07-21: Proposed" + history + "\n"
	}
	proposed := strings.Replace(build(""), "status: "+status, "status: Proposed", 1)
	record, err := adr.ParseV2(number+"-fixture.md", []byte(proposed))
	if err != nil {
		t.Fatal(err)
	}
	digest := adr.ContentDigest(record.Sections)
	if tail == "" {
		return proposed
	}
	formatted := strings.ReplaceAll(tail, "%s", digest)
	return build("\n" + formatted)
}

func publicTopicClaims(slugs ...string) string {
	var b strings.Builder
	b.WriteString("Intro.\n\n## Claims\n")
	slugs = append([]string{"r"}, slugs...)
	for _, slug := range slugs {
		owner := "0003"
		switch slug {
		case "r":
			owner = "0001"
		case "v1":
			owner = "0002"
		case "x":
			owner = "0004"
		case "y":
			owner = "0005"
		case "q":
			owner = "0006"
		}
		prose := "Rule " + slug + "."
		if slug == "r" {
			prose = "Rule prose."
		}
		fmt.Fprintf(&b, "\n### `rule: %s`\n\n%s\nOrigin: ADR-%s\n", slug, prose, owner)
	}
	return b.String()
}

// The V3 sealing edge admits exactly the schema migration's write: the computed
// corpus cutoff into an authority that carried none, with every other permanent
// value unchanged (ADR-0202 item 1).
// The integration re-seal is the second admitted edge: a cutoff sealed inside an
// unintegrated branch was computed against a corpus the integration changes, so
// it is re-derived against the staged tree. The generation must advance, which
// is what keeps an ordinary commit from moving a published cutoff, and the new
// value must be the merged corpus's own next identity rather than any number the
// author likes.
// The inherited-cutoff edge: a branch forked before the sealing generation
// merges an integration branch already past it, so the transition crosses
// generation 29 in one step and the cutoff arrives from the other parent,
// already computed against a corpus neither of these trees holds.

func TestCheckStagedRejectsInitializedVersionMutation(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	gitfixture.Stage(t, repo, map[string]string{
		".awf/awf.lock": lockJSON(t, &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14, InitializedWithVersion: "0.18.0", Files: map[string]manifest.Entry{}}),
	})
	p := openStaged(t, repo.Root())
	if _, err := p.CheckStaged(testContext(t)); err == nil || !strings.Contains(err.Error(), "initializedWithVersion") {
		t.Fatalf("error = %v, want initializedWithVersion refusal", err)
	}
}

func TestValidateLockTransition(t *testing.T) {
	empty, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateLockTransition(empty, nil, &manifest.Lock{}); err != nil {
		t.Fatal(err)
	}
	withConfig, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/config.yaml", Bytes: []byte("x")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateLockTransition(withConfig, nil, &manifest.Lock{}); err == nil {
		t.Fatal("accepted pretracking")
	}
	if err := validateLockTransition(empty, &manifest.Lock{InitializedWithVersion: "1.0.0"}, &manifest.Lock{InitializedWithVersion: "2.0.0"}); err == nil {
		t.Fatal("accepted mutation")
	}
}
