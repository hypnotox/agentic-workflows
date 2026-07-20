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
	gitfixture.Stage(t, repo, dir, committed)
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

func TestCheckStagedCommandUsesIndexLockForGateAndAheadNote(t *testing.T) {
	lockText := func(version string, generation int) string {
		t.Helper()
		lock := &manifest.Lock{AWFVersion: version, SchemaVersion: generation, Files: map[string]manifest.Entry{}}
		b, err := lock.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	configText := "prefix: example\nskills: [tdd]\nagents: []\n"

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
		if code := run([]string{"awf", "check", "--staged"}, &out, &errOut); code != 0 {
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
		if code := run([]string{"awf", "check", "--staged"}, &out, &errOut); code != 1 {
			t.Fatalf("staged ahead-schema exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "schema generation") || !strings.Contains(errOut.String(), strconv.Itoa(migrate.Current()+1)) {
			t.Fatalf("staged schema diagnostic = %q", errOut.String())
		}
	})
}

func TestCheckStagedCommandUsesStagedProjectStateWhenWorkingConfigIsAbsent(t *testing.T) {
	lockText := func(attested bool) string {
		t.Helper()
		lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
		if attested {
			lock.BridgeAttestation = &manifest.BridgeAttestation{Version: 1, PreparedHead: "head", TreeDigest: "sha256:x", ADRFormatV1From: 2}
		}
		b, err := lock.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	configText := "prefix: example\nskills: [tdd]\nagents: []\n"

	t.Run("missing repository refuses", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var out, errOut bytes.Buffer
		if code := run([]string{"awf", "check", "--staged"}, &out, &errOut); code != 1 {
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
		if code := run([]string{"awf", "check", "--staged"}, &out, &errOut); code != 0 {
			t.Fatalf("staged check exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(out.String(), "awf check --staged: clean") {
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
		if code := run([]string{"awf", "check", "--staged"}, &out, &errOut); code != 1 {
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
		if code := run([]string{"awf", "check", "--staged"}, &out, &errOut); code != 1 {
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
		if code := run([]string{"awf", "check", "--staged"}, &out, &errOut); code != 1 {
			t.Fatalf("journaled staged check exit = %d, stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "upgrade journal is present") {
			t.Fatalf("journal diagnostic = %q", errOut.String())
		}
	})
}

// TestRunCheckStagedError covers the error return of the staged route: the index
// carries no config, so CheckStaged fails.
func TestRepositoryPreCommitRejectsSliceMissingNestedHelper(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, map[string]string{"README.md": "staged\n"})
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

func TestRepositoryPreCommitInvokesNestedStagedHelperForInvalidTransition(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}, ADRFormatV1From: 2}
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
		prefix + ".awf/config.yaml":                             "prefix: sundial\nskills: []\nagents: []\ndomains: [alpha]\n",
		prefix + ".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		prefix + ".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/**\n",
		prefix + ".awf/topics/parts/alpha/one/current-state.md": "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-0001\n",
		prefix + "docs/decisions/0001-first.md":                 testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First")),
	}
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "nested head", nil)
	gitfixture.Stage(t, repo, dir, map[string]string{
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
	cmd.Env = append(os.Environ(), "AWF_HOOK_WRAPPER="+wrapper, "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
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

func TestHookCommandHelper(_ *testing.T) {
	if os.Getenv("AWF_HOOK_COMMAND_HELPER") == "" {
		return
	}
	var err error
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "check" || os.Args[len(os.Args)-1] != "--staged" {
		err = fmt.Errorf("unexpected helper arguments: %v", os.Args)
	} else if err = gateStaged("."); err == nil {
		err = runCheck(".", true, os.Stdout)
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
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nskills: [tdd]\nagents: []\n")
	lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	lockBytes, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	gitfixture.Stage(t, repo, dir, map[string]string{
		".awf/awf.lock": string(lockBytes),
		"internal/x.go": "package x\n",
	})
	if err := runCheck(dir, true, io.Discard); err == nil {
		t.Fatal("expected the staged check to fail with no staged config")
	}
}
