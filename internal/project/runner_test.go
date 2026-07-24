package project

import (
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
func runnerFile(t *testing.T, configYAML string) *RenderedFile {
	t.Helper()
	root := scaffold(t, configYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.RenderAll()
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

// With the singleton enabled, exactly one wrapper `awf` renders at the repo
// root; absent or disabled, none does.
// invariant: rendering/companion-scripts:runner-singleton-toggle
func TestRunnerToggle(t *testing.T) {
	if runnerFile(t, "prefix: example\nrunner:\n  enabled: true\n") == nil {
		t.Error("expected the wrapper awf to render when enabled")
	}
	for _, cfg := range []string{
		"prefix: example\n",
		"prefix: example\nrunner:\n  enabled: false\n",
	} {
		if rf := runnerFile(t, cfg); rf != nil {
			t.Errorf("expected no runner for config %q, got %q", cfg, rf.Path)
		}
	}
}

// The rendered wrapper is a pure forwarder: no per-verb dispatch, no in-place
// region, exactly one exec form per resolution branch, every one forwarding
// all arguments verbatim.
// invariant: rendering/companion-scripts:runner-pure-forwarder
func TestRunnerPureForwarder(t *testing.T) {
	rf := runnerFile(t, "prefix: example\nrunner:\n  enabled: true\n")
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

// With vars.awfInvokeCmd set the wrapper execs exactly that command; with it
// unset it probes the bootstrap pin first and falls back to the PATH awf.
// invariant: rendering/companion-scripts:runner-resolution-pinned-first
func TestRunnerResolutionPinnedFirst(t *testing.T) {
	configured := runnerFile(t, "prefix: example\nvars:\n  awfInvokeCmd: go run ./cmd/awf\nrunner:\n  enabled: true\n")
	if configured == nil {
		t.Fatal("wrapper did not render with awfInvokeCmd set")
	}
	if !strings.Contains(configured.Content, "\nexec go run ./cmd/awf \"$@\"\n") {
		t.Errorf("configured wrapper must exec the awfInvokeCmd verbatim:\n%s", configured.Content)
	}
	for _, absent := range []string{".awf/bootstrap.sh", "exec awf \"$@\""} {
		if strings.Contains(configured.Content, absent) {
			t.Errorf("configured wrapper must not carry the default resolution %q:\n%s", absent, configured.Content)
		}
	}

	fallback := runnerFile(t, "prefix: example\nrunner:\n  enabled: true\n")
	if fallback == nil {
		t.Fatal("wrapper did not render with awfInvokeCmd unset")
	}
	c := fallback.Content
	probe := strings.Index(c, `if [ -f .awf/bootstrap.sh ] && pinned="$(bash .awf/bootstrap.sh)"; then`)
	pinnedExec := strings.Index(c, "\texec \"$pinned\" \"$@\"")
	pathExec := strings.Index(c, "\nexec awf \"$@\"\n")
	if probe < 0 || pinnedExec < 0 || pathExec < 0 || probe >= pinnedExec || pinnedExec >= pathExec {
		t.Errorf("default wrapper must probe the bootstrap pin, exec it, then fall back to PATH awf:\n%s", c)
	}
}

// The wrapper renders leak-free (no unresolved token, no stray section/marker
// residue) - the publication-safety contract every awf template meets.
// invariant: rendering/companion-scripts:runner-render-publication-safe
func TestRunnerPublicationSafe(t *testing.T) {
	rf := runnerFile(t, "prefix: example\nrunner:\n  enabled: true\n")
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
// invariant: rendering/companion-scripts:runner-prune-backup
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
			root := scaffold(t, "prefix: example\nrunner:\n  enabled: true\n")
			p, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Sync(); err != nil {
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
			disabled := "prefix: example\nrunner:\n  enabled: false\n"
			if err := os.WriteFile(configPath(root), []byte(disabled), 0o644); err != nil {
				t.Fatal(err)
			}
			p2, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			backups, _, pruned, err := p2.SyncReport()
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

// The runner is a dedicated config-tree render block, not a catalog DocEntry, so it
// stays out of SingletonKinds() - the unified-doc-model completeness set is
// unchanged by the runner's existence.
// invariant: rendering/singletons-and-payloads:singleton-kinds-complete
func TestRunnerNotASingletonKind(t *testing.T) {
	if slices.Contains(catalog.SingletonKinds(), "runner") {
		t.Error("the runner must not be a catalog SingletonKind (it is a dedicated render block)")
	}
}

// A convention part authored for the wrapper's awf-owned section (as its
// `create ... to override` pointer invites) is claimed by the closed-tree sweep, so
// override renders and `awf check` does not flag `.awf/runner` as unclaimed.
func TestRunnerPartOverrideClaimed(t *testing.T) {
	root := scaffold(t, "prefix: example\nrunner:\n  enabled: true\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, ".awf/runner/parts/runner-body.md")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("exec custom-awf \"$@\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	wrapper, err := os.ReadFile(filepath.Join(root, "awf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wrapper), "custom-awf") {
		t.Errorf("runner-body part override not applied:\n%s", wrapper)
	}
	drift, err := p.Check()
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
	root := scaffold(t, "prefix: example\nrunner:\n  enabled: true\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(root, ".awf/runner/parts/runner-body.md")
	if err := os.MkdirAll(part, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderAll(); err == nil {
		t.Fatal("part read error accepted")
	}
}
