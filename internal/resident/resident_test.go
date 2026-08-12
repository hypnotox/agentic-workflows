package resident

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestUninstallReportOwnsCompletePresentation(t *testing.T) {
	document, err := (UninstallReport{Removed: 2, PreservedRoots: []string{"efforts"}}).Document()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, document); err != nil {
		t.Fatal(err)
	}
	const want = "status: uninstall completed\n\nmutation:\n  identity:\n    generated files removed: 2\n  notes:\n    the authored .awf config remains in place; delete it to fully remove\n    preserved resident data under .awf/efforts\n"
	if out.String() != want {
		t.Fatalf("report = %q, want %q", out.String(), want)
	}
}

// A lock entry escaping the repo root (corrupted or malicious lock) must be
// skipped: the out-of-tree target survives and the empty-dir ancestor walk
// terminates instead of looping forever below the root.
// invariant: rendering/sync-and-drift:uninstall-removes-lock-entries (TestUninstallSkipsEscapingLockPaths)
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
	report, err := Uninstall(testsupport.Context(t), root, nil)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if report.Removed != 1 {
		t.Errorf("removed = %d, want 1 (the in-tree file only)", report.Removed)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("escaping lock entry deleted the out-of-tree file: %v", err)
	}
	// invariant: rendering/sync-and-drift:uninstall-removes-lock-entries (TestUninstallSkipsEscapingLockPaths)
	if _, err := os.Stat(filepath.Join(root, inTree)); !os.IsNotExist(err) {
		t.Errorf("in-tree lock entry not removed (err = %v)", err)
	}
}

// Removing an already absent generated entry remains the no-op contract used
// by uninstall now that sync owns its separate root-confined removal path.
func TestRemoveGeneratedFileAbsentIsNoOp(t *testing.T) {
	removed, err := RemoveGeneratedFile(filepath.Join(t.TempDir(), "absent"))
	if err != nil || removed {
		t.Fatalf("RemoveGeneratedFile absent = %t, %v", removed, err)
	}
}

// invariant: rendering/sync-and-drift:uninstall-removes-lock-entries (TestUninstallPreservesLocalDocuments)
func TestUninstallPreservesLocalDocuments(t *testing.T) {
	root := t.TempDir()
	const local = "docs/runbooks/incident.md"
	const ordinary = "docs/ordinary.md"
	localPath := filepath.Join(root, filepath.FromSlash(local))
	testsupport.WriteFile(t, localPath, "operator body\n")
	testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(ordinary)), "generated\n")
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		local:    {TemplateID: "docs/local.md.tmpl"},
		ordinary: {},
	}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	report, err := Uninstall(testsupport.Context(t), root, func(template string) bool { return template == "docs/local.md.tmpl" })
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 2 {
		t.Fatalf("removed = %d, want 2", report.Removed)
	}
	backup, err := os.ReadFile(localPath + ".awf-bak")
	if err != nil || string(backup) != "operator body\n" {
		t.Fatalf("local backup = %q, %v", backup, err)
	}
	for _, path := range []string{localPath, filepath.Join(root, filepath.FromSlash(ordinary)), config.LockPath(root)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("%s remains after uninstall: %v", path, err)
		}
	}
}

func TestUninstallRejectsUnsafeLocalDocument(t *testing.T) {
	root := t.TempDir()
	localPath := filepath.Join(root, "docs/local.md")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	testsupport.WriteFile(t, outside, "keep\n")
	if err := os.Symlink(outside, localPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{Files: map[string]manifest.Entry{"docs/local.md": {TemplateID: "docs/local.md.tmpl"}}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(testsupport.Context(t), root, func(template string) bool { return template == "docs/local.md.tmpl" }); err == nil || !strings.Contains(err.Error(), "unsafe local document") {
		t.Fatalf("unsafe local document error = %v", err)
	}
	if _, err := os.Stat(config.LockPath(root)); err != nil {
		t.Fatalf("lock removed after unsafe refusal: %v", err)
	}
}

func TestUninstallLocalDocumentAbsentNeedsNoBackup(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{Files: map[string]manifest.Entry{"docs/missing.md": {TemplateID: "docs/local.md.tmpl"}}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(testsupport.Context(t), root, func(template string) bool { return template == "docs/local.md.tmpl" }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/missing.md.awf-bak")); !os.IsNotExist(err) {
		t.Fatalf("absent local document has backup: %v", err)
	}
}

func TestUninstallRemovalFailureKeepsLock(t *testing.T) {
	root := t.TempDir()
	const locked = "generated.md"
	path := filepath.Join(root, locked)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(path, "resident"), "keep\n")
	lock := &manifest.Lock{Files: map[string]manifest.Entry{locked: {}}}
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(testsupport.Context(t), root, nil); err == nil || !strings.Contains(err.Error(), "remove generated file") {
		t.Fatalf("uninstall error = %v", err)
	}
	if _, err := os.Stat(config.LockPath(root)); err != nil {
		t.Fatalf("lock removed after failed uninstall: %v", err)
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
	first := RootNames()
	if len(first) == 0 {
		t.Fatal("resident table is empty")
	}
	original := first[0]
	first[0] = "tampered"
	if RootNames()[0] != original {
		t.Fatal("mutating a handed-out table entry changed the declaration")
	}
	if !slices.Equal(RootNames(), []string{"efforts", "worktrees", "effort-archive"}) {
		t.Fatalf("RootNames() = %v", RootNames())
	}
}

// The path predicate is closed to the owned roots: a root itself and anything
// below it is resident, a near-miss sibling is not.
func TestIsResidentPathAndKind(t *testing.T) {
	for _, path := range []string{".awf/efforts", ".awf/efforts/slug/memory.md", ".awf/worktrees", ".awf/effort-archive", ".awf/effort-archive/opaque/file"} {
		if !IsResidentPath(path) {
			t.Errorf("IsResidentPath(%q) = false", path)
		}
	}
	for _, path := range []string{".awf/effort/other", ".awf/config.yaml", "internal/owned.go"} {
		if IsResidentPath(path) {
			t.Errorf("IsResidentPath(%q) = true", path)
		}
	}
	if !IsResidentKind("efforts") || !IsResidentKind("worktrees") || !IsResidentKind("effort-archive") {
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
