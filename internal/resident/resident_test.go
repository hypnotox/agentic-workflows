package resident

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// A lock entry escaping the repo root (corrupted or malicious lock) must be
// skipped: the out-of-tree target survives and the empty-dir ancestor walk
// terminates instead of looping forever below the root.
func TestUninstallSkipsEscapingLockPaths(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "..", "victim.txt")
	testsupport.WriteFile(t, victim, "keep me\n")
	const inTree = ".claude/skills/x/SKILL.md"
	testsupport.WriteFile(t, filepath.Join(root, inTree), "x\n")
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		"../victim.txt": {},
		inTree:          {},
	}}
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	report, err := Uninstall(testsupport.Context(t), root)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if report.Removed != 1 {
		t.Errorf("removed = %d, want 1 (the in-tree file only)", report.Removed)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("escaping lock entry deleted the out-of-tree file: %v", err)
	}
	// invariant: rendering/sync-and-drift:uninstall-removes-lock-entries
	if _, err := os.Stat(filepath.Join(root, inTree)); !os.IsNotExist(err) {
		t.Errorf("in-tree lock entry not removed (err = %v)", err)
	}
}

func TestInspectRootsTreatsAnyDirectChildAsData(t *testing.T) {
	root := t.TempDir()
	efforts := filepath.Join(root, config.DirName, "efforts")
	if err := os.MkdirAll(efforts, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(efforts, "unreadable-entry")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	preserved, err := InspectRoots(root)
	if err != nil || !slices.Contains(preserved, "efforts") {
		t.Fatalf("preserved=%v err=%v", preserved, err)
	}
}

// The table is the single home of the root set, so handing it out must not hand
// out the ability to edit it.
func TestTableHandsOutACopy(t *testing.T) {
	first := Table()
	if len(first) == 0 {
		t.Fatal("resident table is empty")
	}
	original := first[0].Name
	first[0].Name = "tampered"
	if Table()[0].Name != original {
		t.Fatal("mutating a handed-out table row changed the declaration")
	}
	if !slices.Equal(RootNames(), []string{"efforts", "worktrees"}) {
		t.Fatalf("RootNames() = %v", RootNames())
	}
}

// The path predicate is closed to the owned roots: a root itself and anything
// below it is resident, a near-miss sibling is not.
func TestIsResidentPathAndKind(t *testing.T) {
	for _, path := range []string{".awf/efforts", ".awf/efforts/slug/memory.md", ".awf/worktrees"} {
		if !IsResidentPath(path) {
			t.Errorf("IsResidentPath(%q) = false", path)
		}
	}
	for _, path := range []string{".awf/effort/other", ".awf/config.yaml", "internal/owned.go"} {
		if IsResidentPath(path) {
			t.Errorf("IsResidentPath(%q) = true", path)
		}
	}
	if !IsResidentKind("efforts") || !IsResidentKind("worktrees") {
		t.Error("an owned root name is not recognised as a resident render kind")
	}
	if IsResidentKind("hooks") || IsResidentKind("") {
		t.Error("a non-resident render kind was recognised as resident")
	}
}

// ResolveOutput sends resident paths to the primary control root and everything
// else to the invoking checkout.
func TestRootsResolveOutput(t *testing.T) {
	r := NewRoots(filepath.FromSlash("/tracked"), filepath.FromSlash("/primary"))
	if r.Tracked != filepath.FromSlash("/tracked") || r.Resident != filepath.FromSlash("/primary") {
		t.Fatalf("NewRoots did not carry its anchors: %#v", r)
	}
	if got, want := r.ResolveOutput(".awf/efforts/.gitignore"),
		filepath.Join(filepath.FromSlash("/primary"), filepath.FromSlash(".awf/efforts/.gitignore")); got != want {
		t.Errorf("resident output = %q, want %q", got, want)
	}
	if got, want := r.ResolveOutput("AGENTS.md"),
		filepath.Join(filepath.FromSlash("/tracked"), "AGENTS.md"); got != want {
		t.Errorf("tracked output = %q, want %q", got, want)
	}
}

// PreserveRemoval protects a preserved root and its descendants, and nothing
// else: an unpreserved root stays removable.
func TestPreserveRemoval(t *testing.T) {
	preserved := []string{"efforts"}
	for _, path := range []string{".awf/efforts", ".awf/efforts/slug/memory.md"} {
		if !PreserveRemoval(path, preserved) {
			t.Errorf("PreserveRemoval(%q) = false", path)
		}
	}
	for _, path := range []string{".awf/worktrees/slug", ".awf/effortsx", "AGENTS.md"} {
		if PreserveRemoval(path, preserved) {
			t.Errorf("PreserveRemoval(%q) = true", path)
		}
	}
}

// A planned path that exists on disk and is absent from the lock is a
// collision; an awf-managed path that exists is not.
func TestCollisionsAtIgnoresManagedPaths(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "AGENTS.md"), "x\n")
	testsupport.WriteFile(t, filepath.Join(root, "README.md"), "x\n")
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"README.md": {}}}
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	got, err := CollisionsAt(root, []string{"AGENTS.md", "README.md", "absent.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"AGENTS.md"}) {
		t.Fatalf("collisions = %v, want [AGENTS.md]", got)
	}
}
