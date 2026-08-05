package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/prosegate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const checkYAML = `prefix: example
integrationBranch: main
vars: {testCmd: go test ./..., gateCmd: make gate}
skills: [tdd]
agents: []
`

type mutatingWriter struct {
	out     io.Writer
	trigger string
	mutate  func()
	done    bool
}

func (w *mutatingWriter) Write(p []byte) (int, error) {
	n, err := w.out.Write(p)
	if !w.done && strings.Contains(string(p), w.trigger) {
		w.done = true
		w.mutate()
	}
	return n, err
}

// invariant: tooling/cli:check-universe-groups (TestRunCheckCleanThenDirty)
// invariant: adr-system/plan-artifacts:plan-v2-assignment-advisories (TestRunCheckCleanThenDirty)
func TestRunCheckPropagatesOperationalGitAndStagedDriftFailures(t *testing.T) {
	ctx := testContext(t)
	root := syncedGitProject(t, checkYAML)

	gitFailure := errors.New("git lookup failed")
	dependencies := productionCheckDependencies()
	dependencies.openContaining = func(string) (*awfgit.Repo, string, error) { return nil, "", gitFailure }
	if err := runCheckWith(ctx, root, io.Discard, dependencies); !errors.Is(err, gitFailure) {
		t.Fatalf("git failure = %v, want %v", err, gitFailure)
	}

	stagedFailure := errors.New("staged collection failed")
	dependencies = productionCheckDependencies()
	dependencies.collectStaged = func(context.Context, string, planNoteSink) (checkCollection, error) {
		return checkCollection{}, stagedFailure
	}
	if err := runCheckWith(ctx, root, io.Discard, dependencies); !errors.Is(err, stagedFailure) {
		t.Fatalf("staged collection failure = %v, want %v", err, stagedFailure)
	}
	driftFailure := errors.New("staged drift failed")
	stagedDependencies := productionCheckStagedDependencies()
	stagedDependencies.driftRoot = func(context.Context, string) ([]manifest.Drift, error) { return nil, driftFailure }
	dependencies = productionCheckDependencies()
	dependencies.collectStaged = func(ctx context.Context, root string, notes planNoteSink) (checkCollection, error) {
		return collectCheckStagedWith(ctx, root, notes, stagedDependencies)
	}
	if err := runCheckWith(ctx, root, io.Discard, dependencies); !errors.Is(err, driftFailure) {
		t.Fatalf("staged drift failure = %v, want %v", err, driftFailure)
	}
}

func TestRunCheckCleanThenDirty(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := syncedGitProject(t, checkYAML+"proseGate:\n  enabled: true\nmemoryCite:\n  enabled: true\n")
	var clean bytes.Buffer
	if err := runCheck(ctx, root, &clean); err != nil {
		t.Errorf("expected clean check, got %v", err)
	}
	if !strings.HasPrefix(clean.String(), "status: ") || !strings.Contains(clean.String(), "summary:\n  findings:") {
		t.Errorf("bare check did not render one structured aggregate:\n%s", clean.String())
	}
	if strings.Contains(clean.String(), "check staged commit") {
		t.Errorf("bare check must not aggregate staged commit:\n%s", clean.String())
	}

	planPath := "docs/plans/2026-08-03-check-v2.md"
	validPlan := readCommandV2Plan
	artifactRoot := syncedGitProjectFiles(t, checkYAML, map[string]string{
		planPath:                    validPlan,
		"docs/decisions/fixture.md": readCommandV4ADRFor(t, "fixture", "Fixture decisions", "`decision: first` First exact Decision block."),
		"docs/decisions/context.md": readCommandV4ADRFor(t, "context", "Context decisions", "`decision: second` Second exact Decision block."),
		"docs/decisions/third.md":   readCommandV4ADRFor(t, "third", "Third decisions", "`decision: third` Third unselected Decision block."),
	})
	var proposedFirst, proposedSecond bytes.Buffer
	if err := runCheck(ctx, artifactRoot, &proposedFirst); err != nil {
		t.Fatalf("Proposed plan advisories must stay green: %v\n%s", err, proposedFirst.String())
	}
	if err := runCheck(ctx, artifactRoot, &proposedSecond); err != nil {
		t.Fatalf("repeat Proposed plan check: %v\n%s", err, proposedSecond.String())
	}
	proposedNote := "advisory | 2026-08-03-check-v2.md Decision third:third has no Applying assignment\n"
	if proposedFirst.String() != proposedSecond.String() || strings.Count(proposedFirst.String(), proposedNote) != 2 {
		t.Fatalf("Proposed plan assignment advisories must deterministically join the repo universe without failing; first=%q second=%q", proposedFirst.String(), proposedSecond.String())
	}

	brokenPlan := strings.Replace(validPlan, "fixture:first", "missing:first", 1)
	testsupport.WriteFile(t, filepath.Join(artifactRoot, planPath), brokenPlan)
	var workingBroken bytes.Buffer
	if err := runCheck(ctx, artifactRoot, &workingBroken); err == nil || !strings.Contains(workingBroken.String(), planPath) || !strings.Contains(workingBroken.String(), "ADR not found") {
		t.Fatalf("broken working plan reference must block the repo aggregate: err=%v output=%q", err, workingBroken.String())
	}

	repo := gitfixture.At(artifactRoot)
	gitfixture.Stage(t, repo, map[string]string{planPath: brokenPlan})
	// Deliberately restore only the working tree. The staged aggregate must use
	// the index rather than borrowing this unstaged repair.
	testsupport.WriteFile(t, filepath.Join(artifactRoot, planPath), validPlan)
	var stagedBroken bytes.Buffer
	if err := runCheck(ctx, artifactRoot, &stagedBroken); err == nil || !strings.Contains(stagedBroken.String(), planPath) || !strings.Contains(stagedBroken.String(), "ADR not found") {
		t.Fatalf("staged broken plan reference must block aggregate despite working repair: err=%v output=%q", err, stagedBroken.String())
	}

	implementedRoot := syncedGitProjectFiles(t, checkYAML, map[string]string{
		planPath:                    strings.Replace(validPlan, "status: Proposed", "status: Implemented", 1),
		"docs/decisions/fixture.md": readCommandV4ADRFor(t, "fixture", "Fixture decisions", "`decision: first` First exact Decision block."),
		"docs/decisions/context.md": readCommandV4ADRFor(t, "context", "Context decisions", "`decision: second` Second exact Decision block."),
		"docs/decisions/third.md":   readCommandV4ADRFor(t, "third", "Third decisions", "`decision: third` Third unselected Decision block."),
	})
	var implemented bytes.Buffer
	if err := runCheck(ctx, implementedRoot, &implemented); err != nil {
		t.Fatalf("Implemented plan check: %v\n%s", err, implemented.String())
	}
	if strings.Contains(implemented.String(), "2026-08-03-check-v2.md Decision") || strings.Contains(implemented.String(), "no outcome assignment") {
		t.Fatalf("Implemented plan must be silent in plan assignment advisories: %q", implemented.String())
	}

	// The index now carries the Proposed source while working bytes restore the
	// Implemented source. Both source-ordered universes retain their evidence;
	// a working edit can neither remove nor add the staged note.
	gitfixture.Stage(t, gitfixture.At(implementedRoot), map[string]string{planPath: validPlan})
	testsupport.WriteFile(t, filepath.Join(implementedRoot, planPath), strings.Replace(validPlan, "status: Proposed", "status: Implemented", 1))
	var stagedProposed bytes.Buffer
	if err := runCheck(ctx, implementedRoot, &stagedProposed); err != nil {
		t.Fatalf("staged Proposed plan advisory must stay green: %v\n%s", err, stagedProposed.String())
	}
	if got := strings.Count(stagedProposed.String(), proposedNote); got != 2 {
		t.Fatalf("staged Proposed advisory note count = %d, want 2: %q", got, stagedProposed.String())
	}

	// Hand-edit the rendered skill.
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	if err := os.WriteFile(skill, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(ctx, root, io.Discard); err == nil {
		t.Errorf("expected drift error after hand-edit")
	}
}

func TestProseCheckFindingsPropagatesScannerFailure(t *testing.T) {
	failure := errors.New("scan failed")
	dependencies := productionProseDependencies()
	dependencies.scan = func([]prosegate.File, []prosegate.Exemption) ([]prosegate.Finding, []string, error) {
		return nil, nil, failure
	}
	tree, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proseCheckFindingsWith(&config.Config{ProseGate: &config.ProseGateConfig{Enabled: true}}, tree, dependencies); !errors.Is(err, failure) {
		t.Fatalf("scanner failure = %v, want %v", err, failure)
	}
}

func TestRunCheckRepoScannerErrors(t *testing.T) {
	ctx := testContext(t)
	t.Run("lock", func(t *testing.T) {
		root := syncedGitProject(t, checkYAML)
		if err := os.WriteFile(filepath.Join(root, ".awf", "awf.lock"), []byte("{bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runCheckRepo(ctx, root, io.Discard); err == nil {
			t.Fatal("repo aggregate accepted a corrupt working lock")
		}
	})
	t.Run("prose", func(t *testing.T) {
		root := syncedGitProject(t, checkYAML+"proseGate:\n  enabled: true\n")
		repo := gitfixture.At(root)
		gitfixture.Stage(t, repo, map[string]string{"bad.txt": "banned \u2014 punctuation\n"})
		if err := runCheckRepo(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "plain punctuation") {
			t.Fatalf("aggregate did not surface prose failure: %v", err)
		}
	})
	t.Run("memory", func(t *testing.T) {
		root := syncedGitProject(t, checkYAML+"memoryCite:\n  enabled: true\n")
		repo := gitfixture.At(root)
		gitfixture.Stage(t, repo, map[string]string{"docs/plans/citation.txt": cite() + "\n"})
		if err := runCheckRepo(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "memoryCite.exemptions") {
			t.Fatalf("aggregate did not surface memory failure: %v", err)
		}
	})
}

func TestRunCheckStagedLoadError(t *testing.T) {
	if err := runCheckStaged(testContext(t), t.TempDir(), io.Discard); err == nil {
		t.Fatal("direct staged aggregate accepted a non-repository")
	}
}

// TestRunCheckNoLock covers p.Check's error propagating out of runCheck: on a
// never-synced tree AdvisoryNotes renders in memory and stays green, so the
// failure surfaces at the Check() call (the lock is loaded only there), before
// the working-Tree read, so this fixture needs no git repository.
func TestRunCheckReportsStagedUniverseAvailability(t *testing.T) {
	t.Run("repository disappears", func(t *testing.T) {
		root := syncedGitProject(t, checkYAML)
		var out bytes.Buffer
		w := &mutatingWriter{out: &out, trigger: "awf check repo state: clean", mutate: func() {
			if err := os.Rename(filepath.Join(root, ".git"), filepath.Join(root, ".git-away")); err != nil {
				t.Fatal(err)
			}
		}}
		if err := runCheck(testContext(t), root, w); err != nil {
			t.Fatalf("bare check did not degrade after repository became unavailable: %v", err)
		}
		if strings.Contains(out.String(), "staged check universe unavailable outside a git repository") {
			t.Fatalf("collection must finish before its one atomic render, got:\n%s", out.String())
		}
	})
	t.Run("repository becomes malformed", func(t *testing.T) {
		root := syncedGitProject(t, checkYAML)
		w := &mutatingWriter{out: io.Discard, trigger: "awf check repo state: clean", mutate: func() {
			if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core\nbroken = = =\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}}
		if err := runCheck(testContext(t), root, w); err != nil {
			t.Fatalf("the atomic post-collection mutation must not affect the completed report: %v", err)
		}
	})
}

func TestRunCheckInvalidGitMetadata(t *testing.T) {
	root := syncedGitProject(t, checkYAML)
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core\nbroken = = =\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(testContext(t), root, io.Discard); err == nil {
		t.Fatal("expected malformed git metadata to refuse bare check")
	}
}

func TestRunCheckNoLock(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	if err := runCheck(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "no lock") {
		t.Fatalf("expected the no-lock error, got %v", err)
	}
}

// TestRunCheckOutsideGitDegrades covers the stable non-git form: the repo
// universe uses the filesystem snapshot and the staged universe is reported
// unavailable.
// invariant: tooling/cli:check-universe-groups (TestRunCheckOutsideGitDegrades)
func TestRunCheckOutsideGitDegrades(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	var out bytes.Buffer
	if err := runCheck(ctx, root, &out); err != nil {
		t.Fatalf("bare check outside git: %v", err)
	}
	if !strings.Contains(out.String(), "status: warnings") || !strings.Contains(out.String(), "staged check universe unavailable outside a git repository") {
		t.Fatalf("outside-git output omitted repo execution or staged disclosure:\n%s", out.String())
	}
}

// repinLockVersion rewrites the synced project's lock awfVersion in place (schema
// unchanged) so the ahead/equal version comparison can be exercised.
func repinLockVersion(t *testing.T, root, version string) {
	t.Helper()
	lockPath := filepath.Join(root, ".awf", "awf.lock")
	l, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	l.AWFVersion = version
	l.InitializedWithVersion = ""
	if err := l.Save(lockPath); err != nil {
		t.Fatal(err)
	}
}

// TestRunCheckAheadNotice covers the ahead-skew notice in runCheck: a synced
// project whose lock awfVersion is behind the binary prints a non-failing notice;
// an equal version prints none.
func TestRunCheckAheadNotice(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := syncedGitProject(t)
	repinLockVersion(t, root, "0.3.0")
	var out bytes.Buffer
	if err := runCheck(ctx, root, &out); err != nil {
		t.Fatalf("expected clean check, got %v", err)
	}
	if !strings.Contains(out.String(), "status: warnings") || !strings.Contains(out.String(), "summary:") {
		t.Errorf("expected both universe clean outputs, got %q", out.String())
	}
	if !strings.Contains(out.String(), "is ahead of this project (rendered by 0.3.0)") {
		t.Errorf("expected ahead notice, got %q", out.String())
	}

	root2 := syncedGitProject(t)
	repinLockVersion(t, root2, project.Version) // equal to the binary -> no notice
	var out2 bytes.Buffer
	if err := runCheck(ctx, root2, &out2); err != nil {
		t.Fatalf("expected clean check, got %v", err)
	}
	if strings.Contains(out2.String(), "is ahead") {
		t.Errorf("did not expect ahead notice for equal version, got %q", out2.String())
	}

	root3 := syncedGitProject(t)
	repinLockVersion(t, root3, "0.3.0")
	testsupport.WriteFile(t, filepath.Join(root3, ".awf", "config.yaml"), "prefix: [invalid\n")
	var failed bytes.Buffer
	if err := runCheckRepo(ctx, root3, &failed); err == nil {
		t.Fatal("ahead repository check with invalid config succeeded")
	}
	if failed.Len() != 0 {
		t.Fatalf("preparation failure emitted ahead notice before readiness: %q", failed.String())
	}
}

// coverageYAML owns internal/** with the fan-out budget the warn fixtures need.
// The currentState block stays non-empty because those fixtures need the budget,
// and a bare "currentState:" key is a hard parse error. It is no longer what
// switches coverage on: ADR-0192 made coverage and fan-out evaluate whether or
// not the config declares the block.
func coverageYAML() string {
	return "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\ndomains: [alpha]\n" +
		"currentState:\n  maxTopicsPerPath: 1\n"
}

// coverageFiles owns internal/** but declares no scoped topic, so internal/bar.go
// is a coverage gap surfaced by CheckCurrentState through runCheck. It matches no
// scoped topic, so its fan-out count is 0 and the shared budget of 1 is never
// exceeded: the only finding is the error-ranked Uncovered one.
func coverageFiles() map[string]string {
	return map[string]string{
		".awf/domains/alpha.yaml": "paths:\n  - internal/**\n",
		"internal/bar.go":         "package internalx\n",
	}
}

// fanoutFiles gives internal/bar.go two path-scoped claim-bearing topics, so the
// path is covered but its topic count exceeds coverageYAML's budget of 1. Fan-out
// is the one warn-ranked coverage class that survives ADR-0183, so the note-channel
// fixtures ride it rather than a suppressed coverage severity. Both parts carry a
// rule: claim, not an invariant: claim - an invariant would additionally demand a
// Backing: line and a proof marker in a testGlobs file the fixture cannot supply.
func fanoutFiles() map[string]string {
	const part = "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-0001\n"
	return map[string]string{
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/**\n",
		".awf/topics/parts/alpha/one/current-state.md": part,
		".awf/topics/metadata/alpha/two.yaml":          "title: Two\nsummary: T.\npaths:\n  - internal/**\n",
		".awf/topics/parts/alpha/two/current-state.md": part,
		"docs/decisions/0001-one.md":                   testsupport.ADR("Implemented", testsupport.WithTitle("0001: One")),
		"internal/bar.go":                              "package internalx\n",
	}
}

// TestRunCheckSurfacesCurrentStateFinding covers the CheckCurrentState error path
// in runCheck: a drift-clean project whose owned path has no scoped topic yields
// an error-severity coverage finding, which must fail runCheck.
// invariant: tooling/cli:invariants-in-check (TestRunCheckSurfacesCurrentStateFinding)
func TestRunCheckSurfacesCurrentStateFinding(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := syncedGitProjectFiles(t, coverageYAML(), coverageFiles())
	var out bytes.Buffer
	err := runCheck(ctx, root, &out)
	if err == nil {
		t.Fatal("expected runCheck to fail on the current-state coverage finding")
	}
	if !strings.Contains(err.Error(), "check repo state failed") {
		t.Errorf("expected a collected current-state error, got: %v", err)
	}
	if !strings.Contains(out.String(), "current-state") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected the finding line, got: %q", out.String())
	}
}

// TestRunCheckCurrentStateWarnNote covers the note: channel in runCheck: a
// warn-ranked fan-out finding prints a note without failing the check.
func TestRunCheckCurrentStateWarnNote(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := syncedGitProjectFiles(t, coverageYAML(), fanoutFiles())
	var out bytes.Buffer
	if err := runCheck(ctx, root, &out); err != nil {
		t.Fatalf("a warn-ranked finding must not fail runCheck, got: %v", err)
	}
	if !strings.Contains(out.String(), "warnings:") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected a structured fan-out warning, got: %q", out.String())
	}
	if strings.Contains(out.String(), "note:") {
		t.Errorf("ordinary check report must not contain legacy notes, got: %q", out.String())
	}
}

// stagedCheckProject builds a git repo whose HEAD holds the given committed files
// and whose index additionally holds stageOnly, so `awf check staged` sees a
// HEAD-to-index delta. The config lives in commit, so Open resolves it.
func stagedCheckProject(t *testing.T, commit, stageOnly map[string]string) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	committed := map[string]string{}
	for path, body := range commit {
		committed[path] = body
	}
	if _, ok := committed[".awf/awf.lock"]; !ok {
		lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
		b, err := lock.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		committed[".awf/awf.lock"] = string(b)
	}
	gitfixture.Stage(t, repo, committed)
	gitfixture.Commit(t, repo, "head", nil)
	if len(stageOnly) > 0 {
		gitfixture.Stage(t, repo, stageOnly)
	}
	return dir
}

func TestCheckStagedDriftRenderedOutput(t *testing.T) {
	setup := func(t *testing.T) (string, gitfixture.Fixture) {
		t.Helper()
		root := scaffoldProject(t)
		repo := gitfixture.At(root)
		gitfixture.AddAll(t, repo)
		gitfixture.Commit(t, repo, "rendered baseline", nil)
		return root, repo
	}

	t.Run("config without rendered output reports stale", func(t *testing.T) {
		root, repo := setup(t)
		gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: make gate full", 1)})
		var out, errOut bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check", "staged"}, &out, &errOut); code != 1 {
			t.Fatalf("staged drift exit = %d, want 1; stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(out.String(), "stale") || errOut.Len() != 0 {
			t.Fatalf("staged drift report streams stdout=%q stderr=%q", out.String(), errOut.String())
		}
	})

	t.Run("fully staged render is clean", func(t *testing.T) {
		root, repo := setup(t)
		testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: make gate full", 1))
		if err := runSync(testContext(t), root, io.Discard); err != nil {
			t.Fatalf("render changed config: %v", err)
		}
		gitfixture.AddAll(t, repo)
		var out, errOut bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check", "staged", "drift"}, &out, &errOut); code != 0 {
			t.Fatalf("staged drift exit = %d, want 0; stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if out.String() != completedCheckReport {
			t.Fatalf("clean staged drift output = %q", out.String())
		}
	})
}

// invariant: tooling/cli:check-universe-groups (TestRunCheckRunsStagedAfterRepoFailure)
func TestRunCheckRunsStagedAfterRepoFailure(t *testing.T) {
	root := stagedCheckProject(t,
		map[string]string{".awf/config.yaml": coverageYAML(), ".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"},
		map[string]string{"internal/bar.go": "package internalx\n"})
	var out bytes.Buffer
	if err := runCheck(testContext(t), root, &out); err == nil {
		t.Fatal("heterogeneous bare-check failures returned nil")
	}
	if !strings.Contains(out.String(), "unsynced") || strings.Count(out.String(), "current-state") < 2 {
		t.Fatalf("bare check did not report repo failure and continue into staged findings:\n%s", out.String())
	}
}

// TestRunCheckStagedSurfacesFinding covers the staged route of runCheck: an
// error-severity index coverage finding prints the finding line and fails.
func TestRunCheckStagedSurfacesFinding(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := stagedCheckProject(t,
		map[string]string{".awf/config.yaml": coverageYAML(), ".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"},
		map[string]string{"internal/bar.go": "package internalx\n"})
	var out bytes.Buffer
	err := runCheckStaged(ctx, root, &out)
	if err == nil || !strings.Contains(err.Error(), "check staged state failed") {
		t.Fatalf("expected a collected staged current-state error, got %v", err)
	}
	if !strings.Contains(out.String(), "current-state") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected the finding line, got: %q", out.String())
	}
}

// TestRunCheckStagedWarnNote covers the staged note channel and clean status: a
// warn-ranked index fan-out finding prints a note without failing.
func TestRunCheckStagedWarnNote(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	staged := fanoutFiles()
	work := map[string]string{"internal/bar.go": staged["internal/bar.go"]}
	delete(staged, "internal/bar.go")
	staged[".awf/config.yaml"] = coverageYAML()
	root := stagedCheckProject(t, staged, work)
	var out bytes.Buffer
	if err := runCheckStaged(ctx, root, &out); err != nil {
		t.Fatalf("a warn-ranked finding must not fail the staged check, got: %v", err)
	}
	if !strings.Contains(out.String(), "warnings:") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected a structured fan-out warning, got: %q", out.String())
	}
	if strings.Contains(out.String(), "note:") {
		t.Errorf("staged report must not contain legacy notes, got: %q", out.String())
	}
}

func TestCheckStagedCommandUsesIndexLockForGateAndAheadNote(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	lockText := func(version string, generation int) string {
		t.Helper()
		lock := &manifest.Lock{AWFVersion: version, SchemaVersion: generation, Files: map[string]manifest.Entry{}}
		b, err := lock.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	configText := "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n"

	t.Run("working lock cannot fail staged gate or suppress staged ahead note", func(t *testing.T) {
		root := stagedCheckProject(t, map[string]string{
			".awf/config.yaml": configText,
			".awf/awf.lock":    lockText("0.3.0", migrate.Current()),
		}, nil)
		// Diverge only the working lock: both its schema and release version would
		// refuse the command if either gate consulted it.
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "awf.lock"), lockText("99.0.0", migrate.Current()+1))
		t.Chdir(root)
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 0 {
			t.Fatalf("staged check exit = %d, stderr=%q", code, errOut.String())
		}
		if !strings.Contains(out.String(), "rendered by 0.3.0") {
			t.Fatalf("ahead note did not use staged lock: %q", out.String())
		}
	})

	t.Run("staged schema ahead fails despite current working lock", func(t *testing.T) {
		root := stagedCheckProject(t, map[string]string{
			".awf/config.yaml": configText,
			".awf/awf.lock":    lockText(project.Version, migrate.Current()),
		}, map[string]string{".awf/awf.lock": lockText(project.Version, migrate.Current()+1)})
		// Restore a current working lock without changing the index.
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "awf.lock"), lockText(project.Version, migrate.Current()))
		t.Chdir(root)
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 1 {
			t.Fatalf("staged ahead-schema exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "schema generation") || !strings.Contains(errOut.String(), strconv.Itoa(migrate.Current()+1)) {
			t.Fatalf("staged schema diagnostic = %q", errOut.String())
		}
	})
}

func TestCheckStagedCommandUsesStagedProjectStateWhenWorkingConfigIsAbsent(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	lockText := func(attested bool) string {
		t.Helper()
		lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
		if attested {
			lock.BridgeAttestation = &manifest.BridgeAttestation{Version: 1, PreparedHead: "head", TreeDigest: "sha256:x", ADRFormatV1From: 2, LegacyADRGaps: []int{}}
		}
		b, err := lock.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	configText := "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n"

	t.Run("missing repository refuses", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 1 {
			t.Fatalf("non-repository staged check exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "repository") {
			t.Fatalf("non-repository diagnostic = %q", errOut.String())
		}
	})

	t.Run("valid staged project runs", func(t *testing.T) {
		root := stagedCheckProject(t, map[string]string{
			".awf/config.yaml": configText,
			".awf/awf.lock":    lockText(false),
		}, nil)
		if err := os.Remove(filepath.Join(root, ".awf", "config.yaml")); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 0 {
			t.Fatalf("staged check exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if out.String() != completedCheckReport {
			t.Fatalf("staged check output = %q", out.String())
		}
	})

	t.Run("malformed staged lock refuses", func(t *testing.T) {
		root := stagedCheckProject(t, map[string]string{
			".awf/config.yaml": configText,
			".awf/awf.lock":    "{not json",
		}, nil)
		if err := os.Remove(filepath.Join(root, ".awf", "config.yaml")); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 1 {
			t.Fatalf("malformed-lock staged check exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "parse staged lock") {
			t.Fatalf("malformed-lock diagnostic = %q", errOut.String())
		}
	})

	t.Run("staged attestation still refuses", func(t *testing.T) {
		root := stagedCheckProject(t, map[string]string{
			".awf/config.yaml": configText,
			".awf/awf.lock":    lockText(true),
		}, nil)
		if err := os.Remove(filepath.Join(root, ".awf", "config.yaml")); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 1 {
			t.Fatalf("attested staged check exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "committed current-state attestation") {
			t.Fatalf("attestation diagnostic = %q", errOut.String())
		}
	})

	t.Run("staged journal still refuses", func(t *testing.T) {
		root := stagedCheckProject(t, map[string]string{
			".awf/config.yaml":                   configText,
			".awf/awf.lock":                      lockText(false),
			".awf/current-state-upgrade.journal": `{"version":1,"phase":"prepared","finalLockSHA256":"sha256:x","operations":[{"path":".awf/awf.lock","prior":{"present":false,"mode":0,"content":null},"replacement":{"present":false,"mode":0,"content":null}}]}`,
		}, nil)
		if err := os.Remove(filepath.Join(root, ".awf", "config.yaml")); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 1 {
			t.Fatalf("journaled staged check exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "upgrade journal is present") {
			t.Fatalf("journal diagnostic = %q", errOut.String())
		}
	})
}

// TestRunCheckStagedError covers the error return of the staged route: the index
// carries no config, so CheckStaged fails.
func TestRepositoryPreCommitHasOnlyPermanentPath(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	hook, err := os.ReadFile(filepath.Join("..", "..", ".githooks", "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(hook)
	if strings.Contains(body, "AWF_PREP_BRIDGE") || strings.Contains(body, "prep=") {
		t.Fatal("pre-commit still carries preparation-only bridge behavior")
	}
	last := -1
	for _, required := range []string{"check_slice \"$tmp\" \"the repository\"", "rm -rf -- \"$tmp\"", "trap - EXIT", "exec bash .awf/hooks/pre-commit.sh"} {
		index := strings.Index(body, required)
		if index == -1 {
			t.Errorf("pre-commit missing permanent step %q", required)
			continue
		}
		if index <= last {
			t.Errorf("pre-commit step %q appears before its required predecessor", required)
		}
		last = index
	}
}

func TestRepositoryPreCommitRemovesSliceBeforePayload(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	marker := filepath.Join(t.TempDir(), "payload-ran")
	gitfixture.Stage(t, repo, map[string]string{
		".awf/hooks/pre-commit.sh": "#!/bin/sh\nif [ -n \"$(find \"$TMPDIR\" -mindepth 1 -maxdepth 1 -print -quit)\" ]; then\n  echo \"payload inherited staged slice\" >&2\n  exit 1\nfi\ntouch \"$AWF_PAYLOAD_MARKER\"\n",
	})

	tools := t.TempDir()
	wrapper := filepath.Join(tools, "awf-wrapper")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nif [ \"$1\" = check ]; then exit 0; fi\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeGo := filepath.Join(tools, "go")
	fakeGoBody := `#!/bin/sh
out=
while [ "$#" -gt 0 ]; do
	if [ "$1" = -o ]; then out="$2"; shift 2; continue; fi
	shift
done
if [ -z "$out" ]; then exit 0; fi
cp "$AWF_HOOK_WRAPPER" "$out"
chmod +x "$out"
`
	if err := os.WriteFile(fakeGo, []byte(fakeGoBody), 0o755); err != nil {
		t.Fatal(err)
	}
	hook, err := filepath.Abs(filepath.Join("..", "..", ".githooks", "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	tmpRoot := t.TempDir()
	cmd := exec.Command("bash", hook)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "AWF_HOOK_WRAPPER="+wrapper, "AWF_PAYLOAD_MARKER="+marker, "TMPDIR="+tmpRoot, "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pre-commit failed: %v: %s", err, out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("payload marker: %v", err)
	}
	entries, err := os.ReadDir(tmpRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("TMPDIR entries after payload handoff = %v", entries)
	}
}

// TestRunCheckStagedError covers the error return of the staged route: the index
// carries no config, so CheckStaged fails.
func TestRunCheckStagedError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n")
	lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	lockBytes, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	gitfixture.Stage(t, repo, map[string]string{
		".awf/awf.lock": string(lockBytes),
		"internal/x.go": "package x\n",
	})
	if err := runCheckStaged(ctx, dir, io.Discard); err == nil {
		t.Fatal("expected the staged check to fail with no staged config")
	}
}

func TestRunCheckStagedContinuesAfterStatePresentationFailure(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML}, nil)
	stateFailure := errors.New("state category mapping failed")
	driftFailure := errors.New("staged drift failed")
	dependencies := productionCheckStagedDependencies()
	dependencies.currentStateCategories = func(project.CurrentStateReport, bool) ([]presentation.ReportCategory, error) {
		return nil, stateFailure
	}
	driftRan := false
	dependencies.driftRoot = func(context.Context, string) ([]manifest.Drift, error) {
		driftRan = true
		return nil, driftFailure
	}
	var stdout bytes.Buffer
	collection, err := collectCheckStagedWith(testContext(t), root, planNoteSink{}, dependencies)
	if err != nil {
		t.Fatalf("collection error = %v, want operational failures retained in the collection", err)
	}
	err = renderCheckCollection(&stdout, collection)
	if !driftRan {
		t.Fatal("staged drift did not run after state presentation failure")
	}
	if !errors.Is(err, stateFailure) || !errors.Is(err, driftFailure) {
		t.Fatalf("operational error = %v, want joined state and drift failures", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want suppressed partial report", stdout.String())
	}
}

func TestCollectCheckStagedPropagatesDriftCategoryFailure(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML}, nil)
	failure := errors.New("drift category mapping failed")
	dependencies := productionCheckStagedDependencies()
	dependencies.driftCategories = func([]manifest.Drift, bool) ([]presentation.ReportCategory, error) { return nil, failure }
	if _, err := collectCheckStagedSelectionWith(testContext(t), root, planNoteSink{}, false, true, dependencies); !errors.Is(err, failure) {
		t.Fatalf("category mapping failure = %v, want %v", err, failure)
	}
}
