package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const checkYAML = `prefix: example
profile: full
integrationBranch: main
vars: {testCmd: go test ./..., gateCmd: make gate}
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

func TestRunCheckCleanThenDirty(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := syncedGitProject(t, checkYAML+"")
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
	if proposedFirst.String() != proposedSecond.String() || strings.Count(proposedFirst.String(), proposedNote) != 1 {
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
	// Implemented source. The plan-note sink retains the staged advisory once;
	// a working edit can neither remove nor duplicate it.
	gitfixture.Stage(t, gitfixture.At(implementedRoot), map[string]string{planPath: validPlan})
	testsupport.WriteFile(t, filepath.Join(implementedRoot, planPath), strings.Replace(validPlan, "status: Proposed", "status: Implemented", 1))
	var stagedProposed bytes.Buffer
	if err := runCheck(ctx, implementedRoot, &stagedProposed); err != nil {
		t.Fatalf("staged Proposed plan advisory must stay green: %v\n%s", err, stagedProposed.String())
	}
	if got := strings.Count(stagedProposed.String(), proposedNote); got != 1 {
		t.Fatalf("staged Proposed advisory note count = %d, want 1: %q", got, stagedProposed.String())
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
		root := syncedGitProject(t, checkYAML+"")
		repo := gitfixture.At(root)
		gitfixture.Stage(t, repo, map[string]string{"bad.txt": "banned \u2013 punctuation\n"})
		var out bytes.Buffer
		if err := runCheckRepo(ctx, root, &out); err != nil {
			t.Fatalf("aggregate prose warning failed: %v", err)
		}
		if !strings.Contains(out.String(), "warnings:") || !strings.Contains(out.String(), "bad.txt") {
			t.Fatalf("aggregate did not surface prose warning: %q", out.String())
		}
	})
	t.Run("memory", func(t *testing.T) {
		root := syncedGitProject(t, checkYAML+"")
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
	if err := runCheck(ctx, root, &out); err == nil || !strings.Contains(err.Error(), "cannot read staged files") {
		t.Fatalf("bare check outside git = %v, want unconditional scanner refusal", err)
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
	if !strings.Contains(out.String(), "status: completed") || !strings.Contains(out.String(), "information:") {
		t.Errorf("expected informational ahead output, got %q", out.String())
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
	return "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: make gate}\ndomains: [alpha]\n" +
		"currentState:\n"
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
	if strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("fixed fan-out budget should leave two topics clean, got: %q", out.String())
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

// invariant: rendering/sync-and-drift:generated-artifacts-tracked (TestCheckStagedDriftRenderedOutput)
// invariant: rendering/sync-and-drift:staged-drift-rendered-output (TestCheckStagedDriftRenderedOutput)
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

	t.Run("absent staged lock refuses partial authority before drift dispatch", func(t *testing.T) {
		for _, args := range [][]string{{"awf", "check", "staged", "drift"}, {"awf", "check", "staged"}} {
			root, repo := setup(t)
			gitfixture.StageRemoval(t, repo, ".awf/awf.lock", "AGENTS.md")
			var out, errOut bytes.Buffer
			if code := runAt(t, root, args, &out, &errOut); code != 1 {
				t.Fatalf("%v exit = %d, want 1; stdout=%q stderr=%q", args, code, out.String(), errOut.String())
			}
			if out.Len() != 0 || !strings.Contains(errOut.String(), "partial authority") {
				t.Fatalf("%v absent-lock refusal streams stdout=%q stderr=%q", args, out.String(), errOut.String())
			}
		}
	})

	t.Run("direct state still refuses an absent staged lock", func(t *testing.T) {
		root, repo := setup(t)
		gitfixture.StageRemoval(t, repo, ".awf/awf.lock")
		var out, errOut bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check", "staged", "state"}, &out, &errOut); code != 1 {
			t.Fatalf("staged state exit = %d, want 1; stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if out.Len() != 0 || !strings.Contains(errOut.String(), "partial authority") {
			t.Fatalf("staged state absent-lock streams stdout=%q stderr=%q", out.String(), errOut.String())
		}
	})

	t.Run("ignored working replacement cannot mask staged deletion", func(t *testing.T) {
		root, repo := setup(t)
		gitfixture.StageRemoval(t, repo, "AGENTS.md")
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("ignored working replacement\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		exclude := filepath.Join(root, ".git", "info", "exclude")
		if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(exclude, []byte("AGENTS.md\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check", "staged", "drift"}, &out, &errOut); code != 1 {
			t.Fatalf("ignored replacement exit = %d, want 1; stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if errOut.Len() != 0 || !strings.Contains(out.String(), "drift | untracked: AGENTS.md") {
			t.Fatalf("ignored replacement report streams stdout=%q stderr=%q", out.String(), errOut.String())
		}
	})

	t.Run("tracked output remains valid after matching an ignore rule", func(t *testing.T) {
		root, _ := setup(t)
		exclude := filepath.Join(root, ".git", "info", "exclude")
		if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(exclude, []byte("AGENTS.md\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check", "staged", "drift"}, &out, &errOut); code != 0 {
			t.Fatalf("tracked ignored output exit = %d, want 0; stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if out.String() != completedCheckReport || errOut.Len() != 0 {
			t.Fatalf("tracked ignored output streams stdout=%q stderr=%q", out.String(), errOut.String())
		}
	})

	t.Run("nested adopter excludes resident outputs from staged membership", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
		root := filepath.Join(repo.Root(), "nested")
		testsupport.WriteAwfConfig(t, root, minimalYAML)
		if err := initializeProject(testContext(t), root, io.Discard); err != nil {
			t.Fatalf("initialize nested project: %v", err)
		}
		gitfixture.AddAll(t, repo)
		gitfixture.Commit(t, repo, "rendered nested project", nil)
		var out, errOut bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check", "staged", "drift"}, &out, &errOut); code != 0 {
			t.Fatalf("nested staged drift exit = %d, want 0; stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if out.String() != completedCheckReport || errOut.Len() != 0 {
			t.Fatalf("nested staged drift streams stdout=%q stderr=%q", out.String(), errOut.String())
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
	root := syncedGitProjectFiles(t, coverageYAML(), fanoutFiles())
	var out bytes.Buffer
	if err := runCheckStaged(ctx, root, &out); err != nil {
		t.Fatalf("a warn-ranked finding must not fail the staged check, got: %v", err)
	}
	if !strings.Contains(out.String(), "findings: 0 errors, 0 warnings") {
		t.Errorf("fixed fan-out budget should leave two topics clean, got: %q", out.String())
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
	configText := "prefix: example\nintegrationBranch: main\n"

	t.Run("working lock cannot fail staged gate or suppress staged ahead note", func(t *testing.T) {
		root := syncedGitProject(t, configText)
		lockPath := filepath.Join(root, ".awf", "awf.lock")
		lock, err := manifest.Load(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		lock.AWFVersion = project.Version
		if err := lock.Save(lockPath); err != nil {
			t.Fatal(err)
		}
		gitfixture.Add(t, gitfixture.At(root), ".awf/awf.lock")
		// Diverge only the working lock: both its schema and release version would
		// refuse the command if either gate consulted it.
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "awf.lock"), lockText("99.0.0", migrate.Current()+1))
		t.Chdir(root)
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 0 {
			t.Fatalf("staged check exit = %d, stderr=%q", code, errOut.String())
		}
		if strings.Contains(out.String(), "99.0.0") {
			t.Fatalf("staged output consulted working lock: %q", out.String())
		}
	})

	t.Run("staged schema ahead fails despite current working lock", func(t *testing.T) {
		root := syncedGitProject(t, configText)
		lockPath := filepath.Join(root, ".awf", "awf.lock")
		lock, err := manifest.Load(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		lock.SchemaVersion = migrate.Current() + 1
		if err := lock.Save(lockPath); err != nil {
			t.Fatal(err)
		}
		gitfixture.Add(t, gitfixture.At(root), ".awf/awf.lock")
		// Restore a current working lock without changing the index.
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "awf.lock"), lockText(project.Version, migrate.Current()))
		t.Chdir(root)
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "staged"}, &out, &errOut); code != 1 {
			t.Fatalf("staged ahead-schema exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "ahead of live schema") || !strings.Contains(errOut.String(), strconv.Itoa(migrate.Current()+1)) {
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
	configText := "prefix: example\nintegrationBranch: main\n"

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
		root := syncedGitProject(t, configText)
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
		root := syncedGitProject(t, configText)
		gitfixture.Stage(t, gitfixture.At(root), map[string]string{".awf/awf.lock": "{not json"})
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
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nintegrationBranch: main\n")
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

func TestRunCheckErrorPaths(t *testing.T) {
	ctx := testContext(t)
	t.Run("stale-schema", func(t *testing.T) {
		root := t.TempDir()
		claude := filepath.Join(root, ".claude")
		if err := os.MkdirAll(claude, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// check is Gated: the driver refuses a stale schema before the handler.
		var out, errb bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check"}, &out, &errb); code != 1 {
			t.Errorf("expected the driver to refuse check on stale schema, got %d", code)
		}
	})
	t.Run("check-error-malformed-adr", func(t *testing.T) {
		// A malformed ADR makes Project.CheckReport (INDEX.md generation) error.
		root := scaffoldProject(t)
		adrDir := filepath.Join(root, "docs", "decisions")
		if err := os.MkdirAll(adrDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte("---\n: : bad yaml : :\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runCheck(ctx, root, io.Discard); err == nil {
			t.Error("expected check error on a malformed ADR")
		}
	})
}
