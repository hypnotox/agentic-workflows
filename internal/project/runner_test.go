package project

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// runnerFile renders a project with the given config and returns the rendered
// awf wrapper (or nil when none is produced).
func runnerFile(t *testing.T) *RenderedFile {
	t.Helper()
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	var found *RenderedFile
	for i := range out {
		if out[i].Path == "awf" {
			if found != nil {
				t.Fatalf("more than one runner rendered")
			}
			found = &out[i]
		}
	}
	return found
}

// Exactly one wrapper `awf` always renders at the repository root.
// invariant: rendering/companion-scripts:runner-wrapper-rendered (TestRunnerRendered)
func TestRunnerRendered(t *testing.T) {
	if rf := runnerFile(t); rf == nil || rf.Path != "awf" {
		t.Fatalf("runner = %#v, want one repo-root awf wrapper", rf)
	}
}

// The rendered wrapper is a pure forwarder: no per-verb dispatch, no in-place
// region, exactly one exec form per resolution branch, every one forwarding
// all arguments verbatim.
// invariant: rendering/companion-scripts:runner-pure-forwarder (TestRunnerPureForwarder)
func TestRunnerPureForwarder(t *testing.T) {
	rf := runnerFile(t)
	if rf == nil {
		t.Fatal("wrapper did not render")
	}
	if rf.RegenChecked {
		t.Error("the pure wrapper carries no in-place section, so it must not be regeneration-checked")
	}
	c := rf.Content
	if !strings.HasPrefix(c, "#!/usr/bin/env bash\n") {
		t.Errorf("wrapper must open with the bash shebang:\n%s", c)
	}
	if strings.Contains(c, "case ") || strings.Contains(c, "esac") {
		t.Errorf("wrapper must carry no per-verb case dispatch:\n%s", c)
	}
	if strings.Contains(c, "awf:edit-in-place") {
		t.Errorf("wrapper must carry no in-place-editable region:\n%s", c)
	}
	for _, line := range strings.Split(c, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "exec ") && !strings.HasSuffix(trimmed, `"$@"`) {
			t.Errorf("every exec must forward all arguments verbatim: %q", line)
		}
	}
}

// The standard wrapper probes the bootstrap pin first and falls back to the
// PATH awf. Repository-specific invocation replaces the runner-body convention
// part rather than configuring a public render var.
// invariant: rendering/companion-scripts:runner-resolution-pinned-first (TestRunnerResolutionPinnedFirst)
func TestRunnerResolutionPinnedFirst(t *testing.T) {
	fallback := runnerFile(t)
	if fallback == nil {
		t.Fatal("wrapper did not render")
	}
	c := fallback.Content
	probe := strings.Index(c, `if [ -f .awf/bootstrap.sh ] && pinned="$(bash .awf/bootstrap.sh)"; then`)
	pinnedExec := strings.Index(c, "\texec \"$pinned\" \"$@\"")
	pathExec := strings.Index(c, "\nexec awf \"$@\"\n")
	if probe < 0 || pinnedExec < 0 || pathExec < 0 || probe >= pinnedExec || pinnedExec >= pathExec {
		t.Errorf("default wrapper must probe the bootstrap pin, exec it, then fall back to PATH awf:\n%s", c)
	}
	if strings.Contains(c, "awfInvokeCmd") {
		t.Errorf("default wrapper must not retain the retired invocation var:\n%s", c)
	}
}

// The wrapper renders leak-free (no unresolved token, no stray section/marker
// residue) - the publication-safety contract every awf template meets.
// invariant: rendering/companion-scripts:runner-render-publication-safe (TestRunnerPublicationSafe)
func TestRunnerPublicationSafe(t *testing.T) {
	rf := runnerFile(t)
	if rf == nil {
		t.Fatal("wrapper did not render")
	}
	if strings.Contains(rf.Content, "<no value>") {
		t.Errorf("wrapper leaked an unresolved-value token:\n%s", rf.Content)
	}
	for _, marker := range []string{"awf:section", "awf:end"} {
		if strings.Contains(rf.Content, marker) {
			t.Errorf("wrapper leaked a structural %q marker:\n%s", marker, rf.Content)
		}
	}
}

// A retired co-owned runner identity has no special prune behavior. Like an
// ordinary managed output, it is removed without a backup and reported pruned.
func TestPruneTreatsRetiredRunnerAsOrdinaryManagedOutput(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	entry := lock.Files["awf"]
	entry.TemplateID = "runner/x.tmpl"
	lock.Files["x"] = entry
	delete(lock.Files, "awf")
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "awf"), filepath.Join(root, "x")); err != nil {
		t.Fatal(err)
	}
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	backups, _, pruned, err := syncReportProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "x")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("pruned output remains, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "x.awf-bak")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ordinary prune created a backup, stat err = %v", err)
	}
	for _, backup := range backups {
		if backup.Path == "x" {
			t.Errorf("ordinary prune reported a backup: %v", backups)
		}
	}
	if !slices.Contains(pruned, "x") {
		t.Errorf("ordinary prune was not reported: %v", pruned)
	}
}

func TestPruneRemovesManagedSymlinkWithoutTargetAccess(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	entry := lock.Files["awf"]
	lock.Files["x"] = entry
	delete(lock.Files, "awf")
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "awf")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-target", filepath.Join(root, "x")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	backups, _, pruned, err := syncReportProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("managed symlink backups = %v", backups)
	}
	if !slices.Contains(pruned, "x") {
		t.Fatalf("managed symlink pruned = %v", pruned)
	}
	if _, err := os.Lstat(filepath.Join(root, "x")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed symlink remains: %v", err)
	}
}

// The runner is a dedicated config-tree render block, not a catalog DocEntry, so it
// stays out of SingletonKinds() - the unified-doc-model completeness set is
// unchanged by the runner's existence.
// invariant: rendering/singletons-and-payloads:singleton-kinds-complete (TestRunnerNotASingletonKind)
func TestRunnerNotASingletonKind(t *testing.T) {
	if slices.Contains(catalog.SingletonKinds(), "runner") {
		t.Error("the runner must not be a catalog SingletonKind (it is a dedicated render block)")
	}
}

// A convention part authored for the wrapper's awf-owned section (as its
// `create ... to override` pointer invites) is claimed by the closed-tree sweep, so
// override renders and `./awf check` does not flag `.awf/runner` as unclaimed.
func TestRunnerPartOverrideClaimed(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, ".awf/runner/parts/runner-body.md")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("exec custom-awf \"$@\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	wrapper, err := os.ReadFile(filepath.Join(root, "awf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wrapper), "custom-awf") {
		t.Errorf("runner-body part override not applied:\n%s", wrapper)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if strings.Contains(d.Path, ".awf/runner") {
			t.Errorf("runner parts must be claimed by the sweep, got drift %v", d)
		}
	}
}

// A part path that reads as a directory surfaces as a render error rather
// than a silent default.
func TestRunnerPartReadError(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, ".awf/runner/parts/runner-body.md")
	if err := os.MkdirAll(part, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := testPlan(p); err == nil {
		t.Fatal("part read error accepted")
	} else if !strings.Contains(err.Error(), "runner-body") {
		t.Fatalf("render error = %v, want runner-body read error", err)
	}
}
