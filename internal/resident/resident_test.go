package resident

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func uninstall(t *testing.T, root string, preserve func(string) bool) (UninstallReport, error) {
	t.Helper()
	lease, err := filesystem.AcquireProjectLease(testsupport.Context(t), root, root)
	if err != nil {
		t.Fatal(err)
	}
	report, uninstallErr := UninstallLeased(testsupport.Context(t), root, preserve, lease)
	if err := lease.Release(); err != nil {
		t.Fatalf("release uninstall lease: %v", err)
	}
	return report, uninstallErr
}

func saveLock(t *testing.T, root string, files map[string]manifest.Entry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{Files: files}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
}

// TestUninstallWriterCensus prevents a second, unleased host-path mutation
// entrypoint from returning after this conversion. It scans production syntax
// and commits negative examples so the proof itself cannot quietly become a
// no-op.
func TestUninstallWriterCensus(t *testing.T) {
	source, err := os.ReadFile("resident.go")
	if err != nil {
		t.Fatal(err)
	}
	findings := uninstallWriterFindings(t, source)
	if len(findings) != 0 {
		t.Fatalf("uninstall writer census:\n%s", strings.Join(findings, "\n"))
	}
	negative := []byte(`package resident
import "os"
func Uninstall() { _ = os.Remove("unconfined") }
`)
	if findings := uninstallWriterFindings(t, negative); len(findings) != 3 {
		t.Fatalf("negative census findings = %v, want unleased export, raw remove, and missing coverage check", findings)
	}
}

func uninstallWriterFindings(t *testing.T, source []byte) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "resident.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var findings []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			if node.Name.Name == "Uninstall" {
				findings = append(findings, "exported unleased uninstall")
			}
		case *ast.SelectorExpr:
			if ident, ok := node.X.(*ast.Ident); ok && ident.Name == "os" && node.Sel.Name == "Remove" {
				findings = append(findings, "raw os.Remove")
			}
		}
		return true
	})
	if !strings.Contains(string(source), "lease.CoversProject(root, residentRoot)") {
		findings = append(findings, "leased entrypoint does not verify project coverage")
	}
	return findings
}

func TestUninstallRequiresCoveringLease(t *testing.T) {
	root := t.TempDir()
	saveLock(t, root, map[string]manifest.Entry{"generated": {}})
	if _, err := UninstallLeased(testsupport.Context(t), root, nil, nil); err == nil || !strings.Contains(err.Error(), "requires a live project lease") {
		t.Fatalf("unleased uninstall = %v", err)
	}
	if _, err := os.Stat(config.LockPath(root)); err != nil {
		t.Fatalf("unleased uninstall changed lock: %v", err)
	}
}

func TestUninstallReportOwnsExactPresentation(t *testing.T) {
	report := UninstallReport{Removed: 1, RemovedGenerated: []string{"docs/local.md"}, Backups: []Backup{{Path: "docs/local.md", Bak: "docs/local.md.awf-bak"}}, RemovedEmptyDirs: []string{"docs"}, LockRemoved: true}
	doc, err := report.Document()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := presentation.Render(&out, doc); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"removed generated docs/local.md", "backed up docs/local.md to docs/local.md.awf-bak", "removed empty directory docs", "removed lock .awf/awf.lock"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestUninstallReportsEveryCommittedEffect(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "docs/runbooks/local.md"), "operator bytes\n")
	testsupport.WriteFile(t, filepath.Join(root, "generated/a.md"), "generated\n")
	saveLock(t, root, map[string]manifest.Entry{"docs/runbooks/local.md": {TemplateID: "local"}, "generated/a.md": {}})
	report, err := uninstall(t, root, func(s string) bool { return s == "local" })
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"docs/runbooks/local.md", "generated/a.md"}; !slices.Equal(report.RemovedGenerated, want) {
		t.Fatalf("removed generated = %v, want %v", report.RemovedGenerated, want)
	}
	if want := []Backup{{Path: "docs/runbooks/local.md", Bak: "docs/runbooks/local.md.awf-bak"}}; !slices.Equal(report.Backups, want) {
		t.Fatalf("backups = %#v, want %#v", report.Backups, want)
	}
	if want := []string{"generated"}; !slices.Equal(report.RemovedEmptyDirs, want) {
		t.Fatalf("removed empty dirs = %v, want %v", report.RemovedEmptyDirs, want)
	}
	if !report.LockRemoved {
		t.Fatal("lock removal not reported")
	}
}

type failingHandle struct {
	uninstallHandle
	fail    string
	failure error
}

func (h failingHandle) RemoveExpected(path string, info fs.FileInfo) error {
	if path == h.fail {
		return h.failure
	}
	return h.uninstallHandle.RemoveExpected(path, info)
}

func TestUninstallPartialEffectsFaultMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, fail  string
		files       map[string]manifest.Entry
		wantRemoved []string
		wantBackups int
	}{
		{"after generated deletion", "z.md", map[string]manifest.Entry{"a.md": {}, "z.md": {}}, []string{"a.md"}, 0},
		{"after backup", "z.md", map[string]manifest.Entry{"a.md": {TemplateID: "local"}, "z.md": {}}, []string{"a.md"}, 1},
		{"after directory cleanup", ".awf/awf.lock", map[string]manifest.Entry{"a/b.md": {}}, []string{"a/b.md"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for path := range tc.files {
				testsupport.WriteFile(t, filepath.Join(root, path), "x\n")
			}
			saveLock(t, root, tc.files)
			failure := errors.New("injected failure")
			ops := uninstallOps{open: func(path string) (uninstallHandle, error) {
				h, e := productionUninstallOpen(path)
				if e != nil {
					return nil, e
				}
				return failingHandle{h, tc.fail, failure}, nil
			}, inspectRoots: func(uninstallHandle) ([]string, error) { return nil, nil }}
			report, err := uninstallWith(context.Background(), root, func(s string) bool { return s == "local" }, ops)
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
			var partial *PartialUninstallError
			if !errors.As(err, &partial) {
				t.Fatalf("error is not partial: %T", err)
			}
			if !slices.Equal(report.RemovedGenerated, tc.wantRemoved) || len(report.Backups) != tc.wantBackups {
				t.Fatalf("reported effects %#v", report)
			}
			for _, path := range tc.wantRemoved {
				if _, e := os.Lstat(filepath.Join(root, path)); !errors.Is(e, fs.ErrNotExist) {
					t.Errorf("reported removed %s remains: %v", path, e)
				}
			}
		})
	}
}

type closeFailingHandle struct {
	uninstallHandle
	failure error
}

func (h closeFailingHandle) Close() error { return h.failure }

// A close fault after the lock primitive commits exercises the terminal
// partial-outcome boundary: the lock effect remains visible to recovery rather
// than disappearing behind the later fault.
func TestUninstallReportsLockRemovalBeforeTerminalFault(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "generated.md"), "generated\n")
	saveLock(t, root, map[string]manifest.Entry{"generated.md": {}})
	failure := errors.New("close after lock")
	report, err := uninstallWith(context.Background(), root, nil, uninstallOps{
		open: func(path string) (uninstallHandle, error) {
			handle, err := productionUninstallOpen(path)
			if err != nil {
				return nil, err
			}
			return closeFailingHandle{uninstallHandle: handle, failure: failure}, nil
		},
		inspectRoots: func(uninstallHandle) ([]string, error) { return nil, nil },
	})
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	var partial *PartialUninstallError
	if !errors.As(err, &partial) || !report.LockRemoved || !slices.Equal(report.RemovedGenerated, []string{"generated.md"}) {
		t.Fatalf("terminal partial = %#v, %v", report, err)
	}
	if _, statErr := os.Lstat(config.LockPath(root)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("reported lock removal did not commit: %v", statErr)
	}
	document, documentErr := partial.Document()
	if documentErr != nil {
		t.Fatal(documentErr)
	}
	var output strings.Builder
	if renderErr := presentation.Render(&output, document); renderErr != nil || !strings.Contains(output.String(), "removed lock .awf/awf.lock") {
		t.Fatalf("partial document = %q, %v", output.String(), renderErr)
	}
}

func TestUninstallRefusesParentSwapWithoutOutsideMutation(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "safe/generated.md"), "inside\n")
	testsupport.WriteFile(t, filepath.Join(outside, "generated.md"), "outside\n")
	saveLock(t, root, map[string]manifest.Entry{"safe/generated.md": {}})
	ops := uninstallOps{open: productionUninstallOpen, inspectRoots: func(uninstallHandle) ([]string, error) {
		if err := os.Rename(filepath.Join(root, "safe"), filepath.Join(root, "safe-old")); err != nil {
			return nil, err
		}
		return nil, os.Symlink(outside, filepath.Join(root, "safe"))
	}}
	report, err := uninstallWith(context.Background(), root, nil, ops)
	if err == nil {
		t.Fatal("uninstall accepted swapped parent")
	}
	if _, e := os.Stat(filepath.Join(outside, "generated.md")); e != nil {
		t.Fatalf("outside sentinel changed: %v", e)
	}
	if len(report.RemovedGenerated) != 0 {
		t.Fatalf("swapped path reported removed: %#v", report)
	}
}

func TestInspectRootsReportsMissingResidentRoot(t *testing.T) {
	if _, err := InspectRoots(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing resident root inspection succeeded")
	}
}

func TestUninstallKeepsPreservationInspectionAndMutationOnOpenedRoot(t *testing.T) {
	container := t.TempDir()
	root := filepath.Join(container, "project")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(config.DirName, "efforts", ".gitignore")
	testsupport.WriteFile(t, filepath.Join(root, ignore), "original ignore\n")
	testsupport.WriteFile(t, filepath.Join(root, config.DirName, "efforts", "owned-data"), "preserve\n")
	saveLock(t, root, map[string]manifest.Entry{filepath.ToSlash(ignore): {}})
	relocated := filepath.Join(container, "opened-root")
	ops := uninstallOps{open: productionUninstallOpen, inspectRoots: func(handle uninstallHandle) ([]string, error) {
		if err := os.Rename(root, relocated); err != nil {
			return nil, err
		}
		if err := os.Mkdir(root, 0o755); err != nil {
			return nil, err
		}
		testsupport.WriteFile(t, filepath.Join(root, "replacement-sentinel"), "keep\n")
		return inspectRootsConfined(handle)
	}}
	report, err := uninstallWith(context.Background(), root, nil, ops)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.PreservedRoots, []string{"efforts"}) {
		t.Fatalf("preserved roots = %v", report.PreservedRoots)
	}
	if got, err := os.ReadFile(filepath.Join(relocated, ignore)); err != nil || string(got) != "original ignore\n" {
		t.Fatalf("opened-root preserved file = %q, %v", got, err)
	}
	if _, err := os.Lstat(config.LockPath(relocated)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("opened-root lock was not removed: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "replacement-sentinel")); err != nil || string(got) != "keep\n" {
		t.Fatalf("replacement root changed: %q, %v", got, err)
	}
}

func TestUninstallRefusesCorruptLockBeforeMutation(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "generated"), "keep\n")
	testsupport.WriteFile(t, config.LockPath(root), `{"files":{}}`)
	if _, err := uninstall(t, root, nil); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt uninstall = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "generated")); err != nil {
		t.Fatal(err)
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

func TestUninstallReportPartialDocumentPreservesCommittedFactsAndRecovery(t *testing.T) {
	document, err := (UninstallReport{Removed: 2, Backups: []Backup{{Path: "docs/local.md", Bak: "docs/local.md.awf-bak.1"}}}).PartialDocument()
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	got := rendered.String()
	if !strings.Contains(got, "status: uninstall partially committed") || !strings.Contains(got, "generated files removed: 2") || !strings.Contains(got, "backed up docs/local.md to docs/local.md.awf-bak.1") || !strings.Contains(got, "retry awf uninstall") {
		t.Fatalf("partial uninstall document = %q", got)
	}
}

func TestUninstallPartialErrorRetainsReportCauseAndRecovery(t *testing.T) {
	cause := errors.New("remove failed")
	partial := &PartialUninstallError{Report: UninstallReport{Removed: 1, PreservedRoots: []string{"efforts"}}, Cause: cause}
	if !errors.Is(partial, cause) {
		t.Fatal("partial uninstall lost cause identity")
	}
	document, err := partial.Document()
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "uninstall partially committed") || !strings.Contains(rendered.String(), "preserved resident data under .awf/efforts") || !strings.Contains(rendered.String(), "inspect the reported cause, then retry awf uninstall") {
		t.Fatalf("document = %q", rendered.String())
	}
}
