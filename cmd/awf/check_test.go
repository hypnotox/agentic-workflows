package main

import (
	"bytes"
	"fmt"
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
func TestRunCheckCleanThenDirty(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := syncedGitProject(t, checkYAML+"proseGate:\n  enabled: true\nmemoryCite:\n  enabled: true\n")
	var clean bytes.Buffer
	if err := runCheck(ctx, root, &clean); err != nil {
		t.Errorf("expected clean check, got %v", err)
	}
	for _, want := range []string{"awf check repo drift: clean", "awf check repo state: clean", "check repo prose: clean", "check repo memory: clean", "awf check staged: clean"} {
		if !strings.Contains(clean.String(), want) {
			t.Errorf("bare check omitted %q from its universe aggregates:\n%s", want, clean.String())
		}
	}
	if strings.Contains(clean.String(), "check staged commit") {
		t.Errorf("bare check must not aggregate staged commit:\n%s", clean.String())
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
		if !strings.Contains(out.String(), "staged check universe unavailable outside a git repository") {
			t.Fatalf("missing staged-unavailable note:\n%s", out.String())
		}
	})
	t.Run("repository becomes malformed", func(t *testing.T) {
		root := syncedGitProject(t, checkYAML)
		w := &mutatingWriter{out: io.Discard, trigger: "awf check repo state: clean", mutate: func() {
			if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core\nbroken = = =\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}}
		if err := runCheck(testContext(t), root, w); err == nil {
			t.Fatal("bare check accepted malformed git metadata before the staged universe")
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

// TestRunCheckCurrentStateError covers the CheckCurrentState error path in
// runCheck, distinct from a coverage finding: a drift-clean but non-git project
// fails the working-tree read inside CheckCurrentState after Check() succeeds.
func TestRunCheckCurrentStateError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	if err := runCheck(ctx, root, io.Discard); err == nil {
		t.Fatal("expected a working-tree error from CheckCurrentState outside a git repository")
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
	root := syncedGitProject(t, checkYAML)
	repinLockVersion(t, root, "0.3.0")
	var out bytes.Buffer
	if err := runCheck(ctx, root, &out); err != nil {
		t.Fatalf("expected clean check, got %v", err)
	}
	if !strings.Contains(out.String(), "awf check repo drift: clean") || !strings.Contains(out.String(), "awf check staged: clean") {
		t.Errorf("expected both universe clean outputs, got %q", out.String())
	}
	if !strings.Contains(out.String(), "is ahead of this project (rendered by 0.3.0)") {
		t.Errorf("expected ahead notice, got %q", out.String())
	}

	root2 := syncedGitProject(t, checkYAML)
	repinLockVersion(t, root2, project.Version) // equal to the binary -> no notice
	var out2 bytes.Buffer
	if err := runCheck(ctx, root2, &out2); err != nil {
		t.Fatalf("expected clean check, got %v", err)
	}
	if strings.Contains(out2.String(), "is ahead") {
		t.Errorf("did not expect ahead notice for equal version, got %q", out2.String())
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
	if !strings.Contains(err.Error(), "current-state issue") {
		t.Errorf("expected a current-state issue error, got: %v", err)
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
	if !strings.Contains(out.String(), "note: ") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected a fan-out warn note, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "awf check repo state: clean") || !strings.Contains(out.String(), "awf check staged: clean") {
		t.Errorf("expected universe clean statuses alongside the note, got: %q", out.String())
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
	if err == nil || !strings.Contains(err.Error(), "current-state issue") {
		t.Fatalf("expected a staged current-state issue error, got %v", err)
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
	if !strings.Contains(out.String(), "note: ") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected a fan-out warn note, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "awf check staged: clean") {
		t.Errorf("expected the clean staged status, got: %q", out.String())
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
		if !strings.Contains(out.String(), "awf check staged: clean") {
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
	for _, required := range []string{"check_slice \"$tmp\" \"the repository\"", "check_slice \"$tmp/examples/sundial\"", "bash \"$staged_helper\"", "rm -rf -- \"$tmp\"", "trap - EXIT", "exec bash .awf/hooks/pre-commit.sh"} {
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

func TestRepositoryPreCommitRejectsSliceMissingNestedHelper(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, map[string]string{"README.md": "staged\n"})
	hook, err := filepath.Abs(filepath.Join("..", "..", ".githooks", "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", hook)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("pre-commit accepted a staged slice missing its nested helper: %s", out)
	}
	if !strings.Contains(string(out), "staged slice is missing .githooks/check-nested-staged") {
		t.Fatalf("missing-helper diagnostic = %q", out)
	}
}

func TestRepositoryPreCommitRemovesSliceBeforePayload(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	helperPath, err := filepath.Abs(filepath.Join("..", "..", ".githooks", "check-nested-staged"))
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "payload-ran")
	gitfixture.StageFile(t, repo, ".githooks/check-nested-staged", string(helper), 0o755)
	gitfixture.Stage(t, repo, map[string]string{
		"examples/sundial/.keep":   "\n",
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

func TestRepositoryPreCommitInvokesNestedStagedHelperForInvalidTransition(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	lockBytes, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	prefix := "examples/sundial/"
	helperPath, err := filepath.Abs(filepath.Join("..", "..", ".githooks", "check-nested-staged"))
	if err != nil {
		t.Fatal(err)
	}
	helperBody, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".githooks/check-nested-staged":                         string(helperBody),
		prefix + ".awf/awf.lock":                                string(lockBytes),
		prefix + ".awf/config.yaml":                             "prefix: sundial\nintegrationBranch: main\nskills: []\nagents: []\ndomains: [alpha]\n",
		prefix + ".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		prefix + ".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/**\n",
		prefix + ".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-0001\n",
		prefix + "docs/decisions/0001-first.md":                 testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First")),
	}
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "nested head", nil)
	gitfixture.Stage(t, repo, map[string]string{
		prefix + ".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n",
	})

	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	wrapper := filepath.Join(tools, "awf-helper")
	wrapperBody := "#!/bin/sh\nif [ \"$#\" -eq 1 ] && [ \"$1\" = check ]; then exit 0; fi\nAWF_HOOK_COMMAND_HELPER=1 exec \"" + testBinary + "\" -test.run=TestHookCommandHelper -- \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(wrapperBody), 0o755); err != nil {
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
	cmd := exec.Command("bash", hook)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "AWF_HOOK_WRAPPER="+wrapper, "AWF_PREP_BRIDGE=/removed", "AWF_PREP_BRIDGE_SHA256=removed", "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("pre-commit accepted an unmatched nested claim removal: %s", out)
	}
	text := string(out)
	if !strings.Contains(text, "was removed with no ADR remove operation") ||
		!strings.Contains(text, "pre-commit: the staged slice fails examples/sundial's HEAD-to-index current-state transition check") {
		t.Fatalf("pre-commit nested staged diagnostic = %q", text)
	}
}

func TestHookCommandHelper(t *testing.T) {
	ctx := testContext(t)
	if os.Getenv("AWF_HOOK_COMMAND_HELPER") == "" {
		return
	}
	var err error
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "check" || os.Args[len(os.Args)-1] != "staged" {
		err = fmt.Errorf("unexpected helper arguments: %v", os.Args)
	} else if err = gateStaged(ctx, "."); err == nil {
		err = runCheckStaged(ctx, ".", os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "awf:", err)
		os.Exit(1)
	}
	os.Exit(0)
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
