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

// A sync's lock prune that removes the co-owned runner output (an outgoing
// lock entry whose template id is runner/x.tmpl) backs the file up through the
// standard backup path - never clobbering a prior backup - instead of deleting
// it, and still reports the path as pruned.
// invariant: rendering/companion-scripts:runner-prune-backup (TestPruneBacksUpCoOwnedRunner)
func TestPruneBacksUpCoOwnedRunner(t *testing.T) {
	for _, tc := range []struct {
		name     string
		staleBak bool // a pre-existing x.awf-bak occupies the plain suffix
		wantBak  string
	}{
		{"plain suffix", false, "x.awf-bak"},
		{"collision-suffixed", true, "x.awf-bak.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := syncProject(p); err != nil {
				t.Fatal(err)
			}
			// Rewrite the lock so the rendered wrapper entry presents as the
			// legacy co-owned runner at path x (the ADR-0101 shape the prune
			// backup exists for), and move the file with it.
			lock, err := manifest.Load(lockFile(root))
			if err != nil {
				t.Fatal(err)
			}
			entry := lock.Files["awf"]
			entry.TemplateID = coOwnedRunnerTID
			lock.Files["x"] = entry
			delete(lock.Files, "awf")
			if err := lock.Save(lockFile(root)); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(root, "awf"), filepath.Join(root, "x")); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(root, "x"))
			if err != nil {
				t.Fatal(err)
			}
			const stale = "stale prior backup\n"
			if tc.staleBak {
				if err := os.WriteFile(filepath.Join(root, "x.awf-bak"), []byte(stale), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			disabled := "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {gateCmd: test-gate}\n"
			if err := os.WriteFile(configPath(root), []byte(disabled), 0o644); err != nil {
				t.Fatal(err)
			}
			p2, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			backups, _, pruned, err := syncReportProject(p2)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "x")); !os.IsNotExist(err) {
				t.Errorf("pruned runner must be gone from its path, stat err = %v", err)
			}
			bak, err := os.ReadFile(filepath.Join(root, tc.wantBak))
			if err != nil {
				t.Fatalf("runner backup missing: %v", err)
			}
			if string(bak) != string(before) {
				t.Errorf("backup content differs from the pruned runner:\n%s", bak)
			}
			if !slices.Contains(pruned, "x") {
				t.Errorf("runner must still be reported pruned: %v", pruned)
			}
			if !slices.Contains(backups, Backup{Path: "x", Bak: tc.wantBak}) {
				t.Errorf("runner backup must be reported alongside other backups: %v", backups)
			}
			if tc.staleBak {
				prior, err := os.ReadFile(filepath.Join(root, "x.awf-bak"))
				if err != nil || string(prior) != stale {
					t.Errorf("prior backup clobbered: %q, err = %v", prior, err)
				}
			}
		})
	}
}

func TestPruneRemovesManagedRunnerSymlinkWithoutTargetAccess(t *testing.T) {
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
	entry.TemplateID = coOwnedRunnerTID
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
		t.Fatalf("managed runner symlink backups = %v", backups)
	}
	if !slices.Contains(pruned, "x") {
		t.Fatalf("managed runner symlink pruned = %v", pruned)
	}
	if _, err := os.Lstat(filepath.Join(root, "x")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed runner symlink remains: %v", err)
	}
}

// invariant: rendering/companion-scripts:runner-prune-backup (TestRunnerPrunePropagatesBackupFailure)
func TestRunnerPrunePropagatesBackupFailure(t *testing.T) {
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
	entry.TemplateID = coOwnedRunnerTID
	lock.Files["x"] = entry
	delete(lock.Files, "awf")
	if err := lock.Save(lockFile(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "awf")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = syncReportProject(p)
	if err == nil || !strings.Contains(err.Error(), "back up pruned runner x") || !strings.Contains(err.Error(), "read backup source") {
		t.Fatalf("runner prune backup error = %v", err)
	}
	var pathError *os.PathError
	if !errors.As(err, &pathError) {
		t.Fatalf("runner prune backup error identity = %T, want *os.PathError", err)
	}
	if info, statErr := os.Stat(filepath.Join(root, "x")); statErr != nil || !info.IsDir() {
		t.Fatalf("runner source changed after backup refusal: info=%v error=%v", info, statErr)
	}
	preserved, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := preserved.Files["x"]; !ok {
		t.Fatal("runner lock entry was removed after backup refusal")
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
