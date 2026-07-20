package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const checkYAML = `prefix: example
vars: {testCmd: go test ./..., gateCmd: make gate}
skills: [tdd]
agents: []
`

func TestRunCheckCleanThenDirty(t *testing.T) {
	root := syncedGitProject(t, checkYAML)
	if err := runCheck(root, false, io.Discard); err != nil {
		t.Errorf("expected clean check, got %v", err)
	}
	// Hand-edit the rendered skill.
	skill := filepath.Join(root, ".claude/skills/example-tdd/SKILL.md")
	if err := os.WriteFile(skill, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root, false, io.Discard); err == nil {
		t.Errorf("expected drift error after hand-edit")
	}
}

// TestRunCheckNoLock covers p.Check's error propagating out of runCheck: on a
// never-synced tree AdvisoryNotes renders in memory and stays green, so the
// failure surfaces at the Check() call (the lock is loaded only there), before
// the working-Tree read, so this fixture needs no git repository.
func TestRunCheckNoLock(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	if err := runCheck(root, false, io.Discard); err == nil || !strings.Contains(err.Error(), "no lock") {
		t.Fatalf("expected the no-lock error, got %v", err)
	}
}

// TestRunCheckCurrentStateError covers the CheckCurrentState error path in
// runCheck, distinct from a coverage finding: a drift-clean but non-git project
// fails the working-tree read inside CheckCurrentState after Check() succeeds.
func TestRunCheckCurrentStateError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	if err := runSync(root, io.Discard); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	if err := runCheck(root, false, io.Discard); err == nil {
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
	if err := l.Save(lockPath); err != nil {
		t.Fatal(err)
	}
}

// TestRunCheckAheadNotice covers the ahead-skew notice in runCheck: a synced
// project whose lock awfVersion is behind the binary prints a non-failing notice;
// an equal version prints none.
func TestRunCheckAheadNotice(t *testing.T) {
	root := syncedGitProject(t, checkYAML)
	repinLockVersion(t, root, "0.3.0")
	var out bytes.Buffer
	if err := runCheck(root, false, &out); err != nil {
		t.Fatalf("expected clean check, got %v", err)
	}
	if !strings.Contains(out.String(), "awf check: clean") {
		t.Errorf("expected clean output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "is ahead of this project (rendered by 0.3.0)") {
		t.Errorf("expected ahead notice, got %q", out.String())
	}

	root2 := syncedGitProject(t, checkYAML)
	repinLockVersion(t, root2, project.Version) // equal to the binary -> no notice
	var out2 bytes.Buffer
	if err := runCheck(root2, false, &out2); err != nil {
		t.Fatalf("expected clean check, got %v", err)
	}
	if strings.Contains(out2.String(), "is ahead") {
		t.Errorf("did not expect ahead notice for equal version, got %q", out2.String())
	}
}

// coverageYAML owns internal/** with a coverage severity the test parameterizes.
func coverageYAML(severity string) string {
	return "prefix: example\nskills: [tdd]\nagents: []\ndomains: [alpha]\n" +
		"currentState:\n  topicCoverage: " + severity + "\n  topicFanout: off\n"
}

// coverageFiles owns internal/** but declares no scoped topic, so internal/bar.go
// is a coverage gap surfaced by CheckCurrentState through runCheck.
func coverageFiles() map[string]string {
	return map[string]string{
		".awf/domains/alpha.yaml": "paths:\n  - internal/**\n",
		"internal/bar.go":         "package internalx\n",
	}
}

// TestRunCheckSurfacesCurrentStateFinding covers the CheckCurrentState error path
// in runCheck: a drift-clean project whose owned path has no scoped topic yields
// an error-severity coverage finding, which must fail runCheck.
// invariant: tooling/cli:invariants-in-check
func TestRunCheckSurfacesCurrentStateFinding(t *testing.T) {
	root := syncedGitProjectFiles(t, coverageYAML("error"), coverageFiles())
	var out bytes.Buffer
	err := runCheck(root, false, &out)
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
// warn-severity coverage finding prints a note without failing the check.
func TestRunCheckCurrentStateWarnNote(t *testing.T) {
	root := syncedGitProjectFiles(t, coverageYAML("warn"), coverageFiles())
	var out bytes.Buffer
	if err := runCheck(root, false, &out); err != nil {
		t.Fatalf("warn coverage must not fail runCheck, got: %v", err)
	}
	if !strings.Contains(out.String(), "note: ") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected a coverage warn note, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "awf check: clean") {
		t.Errorf("expected clean status alongside the note, got: %q", out.String())
	}
}

// stagedCheckProject builds a git repo whose HEAD holds the given committed files
// and whose index additionally holds stageOnly, so `awf check --staged` sees a
// HEAD-to-index delta. The config lives in commit, so Open resolves it.
func stagedCheckProject(t *testing.T, commit, stageOnly map[string]string) string {
	t.Helper()
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, commit)
	gitfixture.Commit(t, repo, dir, "head", nil)
	if len(stageOnly) > 0 {
		gitfixture.Stage(t, repo, dir, stageOnly)
	}
	return dir
}

// TestRunCheckStagedSurfacesFinding covers the staged route of runCheck: an
// error-severity index coverage finding prints the finding line and fails.
func TestRunCheckStagedSurfacesFinding(t *testing.T) {
	root := stagedCheckProject(t,
		map[string]string{".awf/config.yaml": coverageYAML("error"), ".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"},
		map[string]string{"internal/bar.go": "package internalx\n"})
	var out bytes.Buffer
	err := runCheck(root, true, &out)
	if err == nil || !strings.Contains(err.Error(), "current-state issue") {
		t.Fatalf("expected a staged current-state issue error, got %v", err)
	}
	if !strings.Contains(out.String(), "current-state") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected the finding line, got: %q", out.String())
	}
}

// TestRunCheckStagedWarnNote covers the staged note channel and clean status: a
// warn-severity index coverage finding prints a note without failing.
func TestRunCheckStagedWarnNote(t *testing.T) {
	root := stagedCheckProject(t,
		map[string]string{".awf/config.yaml": coverageYAML("warn"), ".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"},
		map[string]string{"internal/bar.go": "package internalx\n"})
	var out bytes.Buffer
	if err := runCheck(root, true, &out); err != nil {
		t.Fatalf("warn coverage must not fail the staged check, got: %v", err)
	}
	if !strings.Contains(out.String(), "note: ") || !strings.Contains(out.String(), "internal/bar.go") {
		t.Errorf("expected a coverage warn note, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "awf check --staged: clean") {
		t.Errorf("expected the clean staged status, got: %q", out.String())
	}
}

// TestRunCheckStagedError covers the error return of the staged route: the index
// carries no config, so CheckStaged fails.
func TestRunCheckStagedError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nskills: [tdd]\nagents: []\n")
	gitfixture.Stage(t, repo, dir, map[string]string{"internal/x.go": "package x\n"})
	if err := runCheck(dir, true, io.Discard); err == nil {
		t.Fatal("expected the staged check to fail with no staged config")
	}
}
