package filesystem

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func openFixture(t *testing.T) (*Handle, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "file"), []byte("contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.Close(); err != nil {
			t.Error(err)
		}
	})
	return h, root
}

// invariant: tooling/filesystem-access:root-confined-paths (TestHandleConfinesPaths)
func TestHandleConfinesPaths(t *testing.T) {
	h, root := openFixture(t)
	if _, err := h.Read("."); err == nil {
		t.Fatal("Read dot succeeded")
	}
	if _, err := h.Read("dir/file"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", "/x", "..", "a//b"} {
		if _, err := h.Read(path); err == nil {
			t.Fatalf("Read(%q) succeeded", path)
		}
	}
	if err := os.Symlink("dir", filepath.Join(root, "inside")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	seen := map[string]bool{}
	modes := map[string]fs.FileMode{}
	if err := h.Walk(".", func(path string, info fs.FileInfo) (bool, error) {
		seen[path] = true
		modes[path] = info.Mode()
		if filepath.IsAbs(path) || strings.Contains(path, "\\") {
			t.Fatalf("walk path is not slash-relative: %q", path)
		}
		return path != "dir", nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"inside", "escape"} {
		if !seen[path] {
			t.Fatalf("missing symlink %s", path)
		}
		if modes[path]&fs.ModeSymlink == 0 {
			t.Fatalf("walk metadata for %s = %v, want symlink", path, modes[path])
		}
	}
	if seen["dir/file"] || seen["inside/file"] || seen["escape/outside"] {
		t.Fatal("walk descent/links escaped policy")
	}
	sentinel := errors.New("callback")
	for _, subtree := range []string{"inside", "escape"} {
		calls := 0
		if err := h.Walk(subtree, func(path string, info fs.FileInfo) (bool, error) {
			calls++
			if path != subtree || info.Mode()&fs.ModeSymlink == 0 {
				t.Fatalf("Walk(%q) = %q %v", subtree, path, info.Mode())
			}
			return true, nil
		}); err != nil {
			t.Fatalf("Walk(%q): %v", subtree, err)
		}
		if calls != 1 {
			t.Fatalf("Walk(%q) callbacks = %d, want 1", subtree, calls)
		}
		if err := h.Walk(subtree, func(string, fs.FileInfo) (bool, error) { return true, sentinel }); !errors.Is(err, sentinel) {
			t.Fatalf("Walk(%q) callback identity: %v", subtree, err)
		}
	}
	if _, err := h.Read("inside/file"); err != nil {
		t.Fatalf("inside symlink: %v", err)
	}
	if _, err := h.Read("escape/outside"); err == nil {
		t.Fatal("escaping symlink read succeeded")
	}
	rootLink := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(root, rootLink); err != nil {
		t.Skipf("root symlink unsupported: %v", err)
	}
	linkedHandle, err := Open(rootLink)
	if err != nil {
		t.Fatalf("open symlink root: %v", err)
	}
	t.Cleanup(func() {
		if err := linkedHandle.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := linkedHandle.Read("dir/file"); err != nil {
		t.Fatalf("symlink-root descendant: %v", err)
	}
	if _, err := linkedHandle.Read("escape/outside"); err == nil {
		t.Fatal("symlink-root escaping descendant succeeded")
	}
	if err := h.Walk(".", func(string, fs.FileInfo) (bool, error) { return false, sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("callback identity: %v", err)
	}
	if err := h.Walk("missing", func(string, fs.FileInfo) (bool, error) { return true, nil }); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("root operation identity: %v", err)
	}
}

// invariant: tooling/filesystem-access:root-confined-paths (TestHandleOperations)
func TestHandleOperations(t *testing.T) {
	h, root := openFixture(t)
	if err := h.MkdirAll("created/child", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := h.Publish("created/child/artifact", []byte("complete"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.Chmod("created/child/artifact", 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.Chmod("missing", 0o600); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing chmod error = %v", err)
	}
	contents, mode, err := h.ReadWithMode("created/child/artifact")
	if err != nil || string(contents) != "complete" || mode != 0o600 {
		t.Fatalf("read with mode = %q, %v, %v", contents, mode, err)
	}
	if _, _, err := h.ReadWithMode("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing read with mode error = %v", err)
	}
	if err := h.Publish("created/child/artifact", []byte("loser"), 0o600); !errors.Is(err, os.ErrExist) {
		t.Fatalf("publish collision = %v", err)
	}
	if raw, err := os.ReadFile(filepath.Join(root, "created/child/artifact")); err != nil || string(raw) != "complete" {
		t.Fatalf("published bytes = %q, %v", raw, err)
	}
	if err := h.Replace("created/child/artifact", []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, readErr := os.ReadFile(filepath.Join(root, "created/child/artifact"))
	replacedInfo, statErr := os.Stat(filepath.Join(root, "created/child/artifact"))
	if readErr != nil || statErr != nil || string(raw) != "replacement" || replacedInfo.Mode().Perm() != 0o600 {
		t.Fatalf("replacement = %q, %v, %v", raw, replacedInfo, errors.Join(readErr, statErr))
	}
	if err := h.Rename("created/child/artifact", "created/child/renamed"); err != nil {
		t.Fatal(err)
	}
	outsideRename := t.TempDir()
	outsideDestination := filepath.Join(outsideRename, "destination")
	if err := os.WriteFile(outsideDestination, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "created/child/source"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRename, filepath.Join(root, "created/child/escape-destination")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := h.Rename("created/child/source", "created/child/escape-destination/destination"); err == nil {
		t.Fatal("rename to escaping symlink ancestor succeeded")
	}
	if got, err := os.ReadFile(filepath.Join(root, "created/child/source")); err != nil || string(got) != "source" {
		t.Fatalf("source after refused rename = %q, %v", got, err)
	}
	if got, err := os.ReadFile(outsideDestination); err != nil || string(got) != "outside" {
		t.Fatalf("outside destination after refused rename = %q, %v", got, err)
	}
	if err := h.Remove("created/child/renamed"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "created/child/renamed")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removed path remains: %v", err)
	}
	if err := h.MkdirAll("created/removable/child", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := h.RemoveAll("created/removable"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "created/removable")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removed tree remains: %v", err)
	}
	if err := h.Remove("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("remove error identity = %v", err)
	}
	if err := h.Replace("missing-parent/artifact", nil, 0o644); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("replace error identity = %v", err)
	}
	outside := t.TempDir()
	outsidePath := filepath.Join(outside, "victim")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	outsideInfo, err := os.Stat(outsidePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Replace("escape/victim", []byte("replacement"), 0o644); err == nil {
		t.Fatal("replace through escaping symlink succeeded")
	}
	if err := h.Chmod("escape/victim", 0o600); err == nil {
		t.Fatal("chmod through escaping symlink succeeded")
	}
	if _, _, err := h.ReadWithMode("escape/victim"); err == nil {
		t.Fatal("read-with-mode through escaping symlink succeeded")
	}
	if err := h.Remove("escape/victim"); err == nil {
		t.Fatal("remove through escaping symlink succeeded")
	}
	if err := h.RemoveAll("escape/victim"); err == nil {
		t.Fatal("remove-all through escaping symlink succeeded")
	}
	if err := h.Rename("escape/victim", "created/escaped"); err == nil {
		t.Fatal("rename through escaping symlink succeeded")
	}
	afterOutsideInfo, statErr := os.Stat(outsidePath)
	if raw, readErr := os.ReadFile(outsidePath); readErr != nil || statErr != nil || string(raw) != "outside" || afterOutsideInfo.Mode().Perm() != outsideInfo.Mode().Perm() {
		t.Fatalf("escaping operations changed outside target = %q, %v, %v", raw, afterOutsideInfo, errors.Join(readErr, statErr))
	}
	if err := os.Symlink("dir/file", filepath.Join(root, "replace-link")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := h.Replace("replace-link", []byte("link replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkBytes, linkReadErr := os.ReadFile(filepath.Join(root, "replace-link"))
	targetBytes, targetReadErr := os.ReadFile(filepath.Join(root, "dir/file"))
	linkInfo, linkStatErr := os.Lstat(filepath.Join(root, "replace-link"))
	if linkReadErr != nil || targetReadErr != nil || linkStatErr != nil || string(linkBytes) != "link replacement" || string(targetBytes) != "contents" || linkInfo.Mode()&fs.ModeSymlink != 0 || linkInfo.Mode().Perm() != 0o600 {
		t.Fatalf("final symlink replacement = link %q target %q info %v errors %v", linkBytes, targetBytes, linkInfo, errors.Join(linkReadErr, targetReadErr, linkStatErr))
	}
	failedDestination := filepath.Join(root, "replace-failure")
	if err := os.Mkdir(failedDestination, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(failedDestination, "marker"), []byte("preserved"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := h.Replace("replace-failure", []byte("replacement"), 0o600); err == nil {
		t.Fatal("replacement over nonempty directory succeeded")
	}
	marker, markerReadErr := os.ReadFile(filepath.Join(failedDestination, "marker"))
	failedInfo, failedStatErr := os.Stat(failedDestination)
	if markerReadErr != nil || failedStatErr != nil || string(marker) != "preserved" || failedInfo.Mode().Perm() != 0o750 {
		t.Fatalf("failed replacement changed destination = %q, %v, %v", marker, failedInfo, errors.Join(markerReadErr, failedStatErr))
	}
	for _, operation := range []func() error{
		func() error { return h.MkdirAll("..", 0o755) },
		func() error { return h.Publish("../artifact", nil, 0o644) },
		func() error { return h.Replace("../artifact", nil, 0o644) },
		func() error { return h.Replace("dir/file/child", nil, 0o644) },
		func() error { return h.Remove("../artifact") },
		func() error { return h.RemoveAll("../artifact") },
		func() error { return h.Rename("dir/file", "../artifact") },
		func() error { return h.Rename("../artifact", "dir/file") },
		func() error { return h.Chmod("../artifact", 0o644) },
		func() error { _, _, err := h.ReadWithMode("../artifact"); return err },
		func() error { return h.MkdirAll("dir/file/child", 0o755) },
	} {
		if err := operation(); err == nil {
			t.Fatal("invalid root-confined mutation succeeded")
		}
	}
	if _, err := h.Info("dir/file"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("dir/file", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	link, err := h.LinkInfo("link")
	if err != nil || link.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("link info: %v %v", link, err)
	}
	info, err := h.Info("link")
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("info: %v %v", info, err)
	}
	// The descent flag is ignored for a non-directory entry.
	seenFile := false
	if err := h.Walk("dir/file", func(string, fs.FileInfo) (bool, error) { seenFile = true; return false, nil }); err != nil || !seenFile {
		t.Fatalf("nondirectory walk: %v seen=%t", err, seenFile)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Read("dir/file"); err == nil {
		t.Fatal("use after close succeeded")
	}
}

func TestHandleBackupCopiesConfinedSource(t *testing.T) {
	h, root := openFixture(t)
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("source bytes"), 0o640); err != nil {
		t.Fatal(err)
	}
	backup, err := h.Backup("source")
	if err != nil {
		t.Fatal(err)
	}
	if backup != "source.awf-bak" {
		t.Fatalf("backup path = %q", backup)
	}
	contents, err := os.ReadFile(filepath.Join(root, backup))
	if err != nil || string(contents) != "source bytes" {
		t.Fatalf("backup contents = %q, %v", contents, err)
	}
	if _, err := h.Backup("../escaping"); err == nil {
		t.Fatal("backup accepted escaping source")
	}
}

func TestBackupPropagatesSourceReadError(t *testing.T) {
	failure := errors.New("source read failed")
	_, err := Backup("source", func(string) ([]byte, fs.FileMode, error) {
		return nil, 0, failure
	}, func(string, []byte, fs.FileMode) error {
		t.Fatal("publish called after source read failure")
		return nil
	})
	if !errors.Is(err, failure) {
		t.Fatalf("backup error = %v, want %v", err, failure)
	}
}

func TestBackupPreservesModeAndSelectsAvailablePath(t *testing.T) {
	h, root := openFixture(t)
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("source bytes"), 0o640); err != nil {
		t.Fatal(err)
	}
	backup, err := Backup("source", h.ReadWithMode, h.Publish)
	if err != nil {
		t.Fatal(err)
	}
	if backup != "source.awf-bak" {
		t.Fatalf("backup path = %q, want source.awf-bak", backup)
	}
	contents, err := os.ReadFile(filepath.Join(root, backup))
	info, statErr := os.Stat(filepath.Join(root, backup))
	if err != nil || statErr != nil || string(contents) != "source bytes" || info.Mode().Perm() != 0o640 {
		t.Fatalf("backup = %q, %v, %v", contents, info, errors.Join(err, statErr))
	}
}

func TestBackupRetriesOccupiedSuffix(t *testing.T) {
	h, root := openFixture(t)
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("source bytes"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source.awf-bak"), []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup, err := Backup("source", h.ReadWithMode, h.Publish)
	if err != nil {
		t.Fatal(err)
	}
	if backup != "source.awf-bak.1" {
		t.Fatalf("backup path = %q, want source.awf-bak.1", backup)
	}
	occupied, occupiedErr := os.ReadFile(filepath.Join(root, "source.awf-bak"))
	contents, readErr := os.ReadFile(filepath.Join(root, backup))
	if occupiedErr != nil || readErr != nil || string(occupied) != "occupied" || string(contents) != "source bytes" {
		t.Fatalf("backup collision result = occupied %q backup %q errors %v", occupied, contents, errors.Join(occupiedErr, readErr))
	}
}

func TestBackupPropagatesNonCollisionPublicationFailure(t *testing.T) {
	failure := errors.New("publication failed")
	calls := 0
	_, err := Backup("source", func(string) ([]byte, fs.FileMode, error) {
		return []byte("source bytes"), 0o640, nil
	}, func(string, []byte, fs.FileMode) error {
		calls++
		return failure
	})
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("backup error = %v, publication calls = %d", err, calls)
	}
}

func TestReadWithModeUsesOneOpenedFileForBytesAndMode(t *testing.T) {
	var source []byte
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(rel string, body []byte) {
		if rel == "internal/filesystem/handle.go" {
			source = append([]byte(nil), body...)
		}
	})
	file, err := parser.ParseFile(token.NewFileSet(), "internal/filesystem/handle.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "ReadWithMode" {
			function = candidate
			break
		}
	}
	if function == nil {
		t.Fatal("ReadWithMode declaration missing")
	}
	selector := func(expression ast.Expr, object, member string) bool {
		selected, ok := expression.(*ast.SelectorExpr)
		if !ok || selected.Sel.Name != member {
			return false
		}
		if identifier, ok := selected.X.(*ast.Ident); ok {
			return identifier.Name == object
		}
		root, ok := selected.X.(*ast.SelectorExpr)
		identifier, identifierOK := root.X.(*ast.Ident)
		return ok && identifierOK && identifier.Name+"."+root.Sel.Name == object
	}
	opens, reads, stats := 0, 0, 0
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch {
		case selector(call.Fun, "h.root", "Open"):
			opens++
		case selector(call.Fun, "h.root", "ReadFile") || selector(call.Fun, "h.root", "Stat"):
			t.Fatalf("ReadWithMode split bytes and mode across root calls: %s", call.Fun)
		case selector(call.Fun, "io", "ReadAll") && len(call.Args) == 1:
			if identifier, ok := call.Args[0].(*ast.Ident); ok && identifier.Name == "file" {
				reads++
			}
		case selector(call.Fun, "file", "Stat"):
			stats++
		}
		return true
	})
	if opens != 1 || reads != 1 || stats != 1 {
		t.Fatalf("ReadWithMode operations = opens %d reads %d stats %d; want one opened file supplying bytes and mode", opens, reads, stats)
	}
}

// invariant: tooling/filesystem-access:root-confined-paths (TestExpectedIdentityReplacementAndRemovalRefuseStaleEntries)
func TestExpectedIdentityReplacementAndRemovalRefuseStaleEntries(t *testing.T) {
	root := t.TempDir()
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := os.WriteFile(filepath.Join(root, "artifact"), []byte("observed"), 0o644); err != nil {
		t.Fatal(err)
	}
	expected, err := h.ExpectedIdentity("artifact")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "artifact")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifact"), []byte("winner"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.ReplaceExpected("artifact", expected, []byte("replacement"), 0o644); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("stale replacement = %v, want identity change", err)
	}
	if err := h.RemoveExpected("artifact", expected); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("stale removal = %v, want identity change", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "artifact")); err != nil || string(got) != "winner" {
		t.Fatalf("concurrent winner = %q, %v", got, err)
	}
	if err := h.ReplaceExpected("absent", nil, []byte("created"), 0o640); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "absent")); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("exclusive creation = %v, %v", info, err)
	}
	if err := h.ReplaceExpected("absent", nil, []byte("clobber"), 0o644); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("absent-identity clobber = %v, want identity change", err)
	}
}

func TestCreateDirectoryReturnsPublishedIdentityAndRefusesExistingDestination(t *testing.T) {
	container := t.TempDir()
	root := filepath.Join(container, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	created, err := h.CreateDirectory("owned", 0o750)
	if err != nil {
		t.Fatal(err)
	}
	owned := filepath.Join(root, "owned")
	if info, err := os.Lstat(owned); err != nil || !created.SameFile(info) || info.Mode().Perm() != 0o750 {
		t.Fatalf("created identity = %v, path identity = %v, error %v", created, info, err)
	}
	if _, err := h.CreateDirectory("owned", 0o700); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("existing directory creation = %v, want exists", err)
	}
	if _, err := h.CreateDirectory("../escape", 0o700); err == nil {
		t.Fatal("invalid directory creation succeeded")
	}
	if _, err := h.CreateDirectory("missing/child", 0o700); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing-parent directory creation = %v, want not exist", err)
	}
	if err := publishDirectoryNoReplace(h.root, "temporary", "other/destination"); err == nil {
		t.Fatal("different-parent directory publication succeeded")
	}
	outside := filepath.Join(container, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape-parent")); err == nil {
		if err := publishDirectoryNoReplace(h.root, "escape-parent/temporary", "escape-parent/destination"); err == nil {
			t.Fatal("directory publication through escaping parent succeeded")
		}
	}
	if _, err := exchangeExpected(h.root, "temporary", "other/destination", created, false, false); err == nil {
		t.Fatal("different-parent expected mutation succeeded")
	}
	relocated := filepath.Join(container, "relocated")
	if err := os.Rename(owned, relocated); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(owned, 0o700); err != nil {
		t.Fatal(err)
	}
	relocatedBefore, err := os.Lstat(relocated)
	if err != nil || !created.SameFile(relocatedBefore) {
		t.Fatalf("returned identity does not name created directory after relocation: %v, %v", relocatedBefore, err)
	}
	if err := h.RemoveExpected("owned", created); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("replacement removal = %v, want identity change", err)
	}
	if info, err := os.Lstat(relocated); err != nil || !os.SameFile(relocatedBefore, info) {
		t.Fatalf("relocated created directory changed: %v, %v", info, err)
	}
	if info, err := os.Lstat(owned); err != nil || os.SameFile(created, info) {
		t.Fatalf("replacement directory was claimed or removed: %v, %v", info, err)
	}
}

func TestRetireExpectedRemovesOnlyTheObservedNonemptyDirectory(t *testing.T) {
	root := t.TempDir()
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := os.Mkdir(filepath.Join(root, "retired"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "retired", "payload"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := h.ExpectedIdentity("retired")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.RetireExpected("retired", expected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "retired")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("retired directory remains: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "retired"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "retired", "successor"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.RetireExpected("retired", expected); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("successor retirement = %v, want identity change", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "retired", "successor")); err != nil || string(got) != "keep" {
		t.Fatalf("successor changed: bytes=%q error=%v", got, err)
	}
}

func TestExpectedIdentityCannotAuthorizeAfterReleaseOrConsumption(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "artifact"), []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	released, err := h.ExpectedIdentity("artifact")
	if err != nil {
		t.Fatal(err)
	}
	if err := released.Release(); err != nil {
		t.Fatal(err)
	}
	if err := h.ReplaceExpected("artifact", released, []byte("released"), 0o600); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("released identity replacement = %v, want identity change", err)
	}

	consumed, err := h.ExpectedIdentity("artifact")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.ReplaceExpected("artifact", consumed, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.ReplaceExpected("artifact", consumed, []byte("reused"), 0o600); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("consumed identity replacement = %v, want identity change", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "artifact")); err != nil || string(got) != "after" {
		t.Fatalf("artifact = %q, %v", got, err)
	}
}

func TestExpectedIdentityReplacementAndRemovalCommitFilesAndEmptyDirectories(t *testing.T) {
	root := t.TempDir()
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	artifact := filepath.Join(root, "artifact")
	if err := os.WriteFile(artifact, []byte("observed"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := h.ExpectedIdentity("artifact")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.ReplaceExpected("artifact", expected, []byte("replacement"), 0o640); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(artifact); err != nil || string(got) != "replacement" {
		t.Fatalf("replacement = %q, %v", got, err)
	}
	if info, err := os.Stat(artifact); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("replacement mode = %v, %v", info, err)
	}
	expected, err = h.ExpectedIdentity("artifact")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.RemoveExpected("artifact", expected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(artifact); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed file remains: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	expected, err = h.ExpectedIdentity("empty")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.RemoveExpected("empty", expected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "empty")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed directory remains: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected-identity mutation left residue: %v", entries)
	}
}

func TestExpectedMutationRootAnchorRefusesRelocatedParent(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("native expected mutation is unavailable")
	}
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "replace", true: "remove"}[remove], func(t *testing.T) {
			container := t.TempDir()
			rootPath := filepath.Join(container, "root")
			outside := filepath.Join(container, "outside")
			if err := os.MkdirAll(filepath.Join(rootPath, "parent"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			h, err := Open(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			if err := os.WriteFile(filepath.Join(rootPath, "parent", "destination"), []byte("observed"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(rootPath, "parent", "temporary"), []byte("replacement"), 0o640); err != nil {
				t.Fatal(err)
			}
			expected, err := h.ExpectedIdentity("parent/destination")
			if err != nil {
				t.Fatal(err)
			}
			relocated := filepath.Join(outside, "parent")
			if err := os.Rename(filepath.Join(rootPath, "parent"), relocated); err != nil {
				t.Fatal(err)
			}
			consumed, err := exchangeExpected(h.root, "parent/temporary", "parent/destination", expected, remove, false)
			if err == nil || consumed {
				t.Fatalf("relocated-parent commit = consumed %v, error %v; want uncommitted refusal", consumed, err)
			}
			if got, err := os.ReadFile(filepath.Join(relocated, "destination")); err != nil || string(got) != "observed" {
				t.Fatalf("relocated destination = %q, %v", got, err)
			}
			if got, err := os.ReadFile(filepath.Join(relocated, "temporary")); err != nil || string(got) != "replacement" {
				t.Fatalf("relocated temporary = %q, %v", got, err)
			}
		})
	}
}

func TestExpectedMutationRefusesDisappearedDestination(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("covers the Linux exchange syscall refusal")
	}
	root := t.TempDir()
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := os.WriteFile(filepath.Join(root, "destination"), []byte("observed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "temporary"), []byte("replacement"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := h.ExpectedIdentity("destination")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "destination")); err != nil {
		t.Fatal(err)
	}
	consumed, err := exchangeExpected(h.root, "temporary", "destination", expected, false, false)
	if err == nil || consumed {
		t.Fatalf("disappeared-destination commit = consumed %v, error %v; want uncommitted refusal", consumed, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "temporary")); err != nil || string(got) != "replacement" {
		t.Fatalf("temporary after refusal = %q, %v", got, err)
	}
}

func TestExpectedIdentityRemovalPreservesNonemptyDirectory(t *testing.T) {
	root := t.TempDir()
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := os.MkdirAll(filepath.Join(root, "owned"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "owned", "child"), []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := h.ExpectedIdentity("owned")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.RemoveExpected("owned", expected); !errors.Is(err, ErrDirectoryNotEmpty) {
		t.Fatalf("nonempty directory removal error = %v, want typed identity", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "owned", "child")); err != nil || string(got) != "preserve" {
		t.Fatalf("nonempty directory child = %q, %v", got, err)
	}
}

func TestHandleIdentityAndRetirementInputRefusals(t *testing.T) {
	h, root := openFixture(t)
	if _, err := h.RootMatches(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing root identity comparison succeeded")
	}
	fileRoot := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(fileRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := h.RootMatches(fileRoot); err == nil {
		t.Fatal("file root identity comparison succeeded")
	}
	if err := h.RetireExpected("../outside", nil); err == nil {
		t.Fatal("invalid retirement path succeeded")
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	regular, err := h.ExpectedIdentity("file")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.RetireExpected("file", regular); !errors.Is(err, ErrIdentityChanged) {
		t.Fatalf("regular-file retirement = %v, want identity change", err)
	}
	if _, err := h.ReadDir("../outside"); err == nil {
		t.Fatal("invalid read-directory path succeeded")
	}
}

func TestOpenFailureAndWalkErrors(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("open missing succeeded")
	}
	h, _ := openFixture(t)
	if err := h.Walk("", func(string, fs.FileInfo) (bool, error) { return true, nil }); err == nil {
		t.Fatal("invalid walk succeeded")
	}
	for _, operation := range []func(string) error{func(path string) error { _, err := h.Info(path); return err }, func(path string) error { _, err := h.LinkInfo(path); return err }} {
		if err := operation(".."); err == nil {
			t.Fatal("invalid metadata path succeeded")
		}
		if err := operation("missing"); err == nil {
			t.Fatal("missing metadata path succeeded")
		}
	}
}

// TestRootConfinedFilesystemSingleHome is intentionally structural: production root handles have one home.
// invariant: tooling/filesystem-access:single-production-handle (TestRootConfinedFilesystemSingleHome)
// invariant: tooling/filesystem-access:root-scoped-project-mutation-leases (TestRootConfinedFilesystemSingleHome)
func TestRootConfinedFilesystemSingleHome(t *testing.T) {
	var consumers []string
	filesystemSources := map[string]string{}
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(rel string, body []byte) {
		if strings.HasPrefix(rel, "internal/filesystem/") {
			filesystemSources[rel] = string(body)
		}
		if finding := rootSourceFinding(rel, string(body)); finding != "" {
			t.Fatalf("production root ownership: %s", finding)
		}
		if finding := filesystemConsumerFinding(rel, string(body)); finding != "" {
			t.Fatalf("production filesystem consumer: %s", finding)
		} else if rel != "internal/filesystem/handle.go" && strings.Contains(string(body), "internal/filesystem") {
			consumers = append(consumers, rel)
		}
		if finding := advisoryLeaseOwnerFinding(rel, string(body)); finding != "" {
			t.Fatalf("production advisory-lease ownership: %s", finding)
		}
	})
	if finding := filesystemPackageFinding(filesystemSources); finding != "" {
		t.Fatalf("filesystem package shape: %s", finding)
	}
	if len(consumers) == 0 {
		t.Fatal("no outside-package production constructor/capability flow imports filesystem")
	}
	for _, tc := range []struct {
		name, path, src, want string
	}{
		{"canonical handle", "internal/filesystem/handle.go", "package filesystem\nimport \"os\"\ntype Handle struct { root *os.Root }\nfunc Open(x string) { os.OpenRoot(x) }", ""},
		{"outside root constructor", "internal/other/other.go", "package other\nimport root \"os\"\nfunc Open(x string) { root.OpenRoot(x) }", "outside filesystem concrete root use"},
		{"outside root storage", "internal/other/other.go", "package other\nimport root \"os\"\ntype x struct { r *root.Root }", "outside filesystem concrete root use"},
		{"aliased root constructor", "internal/other/other.go", "package other\nimport \"os\"\nvar openRoot = os.OpenRoot\nfunc Open(x string) { openRoot(x) }", "outside filesystem concrete root use"},
		{"aliased root storage", "internal/other/other.go", "package other\nimport \"os\"\ntype rootAlias = os.Root\ntype x struct { r *rootAlias }", "outside filesystem concrete root use"},
		{"second handle", "internal/filesystem/handle.go", "package filesystem\ntype Other struct{}\ntype Handle struct{}", "exported concrete Other"},
		{"provider interface", "internal/filesystem/handle.go", "package filesystem\ntype Handle struct{}\ntype Filesystem interface{ Read(string) }", "interface Filesystem"},
		{"unexported interface", "internal/filesystem/handle.go", "package filesystem\ntype Handle struct{}\ntype filesystem interface{ Read(string) }", "interface filesystem"},
		{"returned constructor", "internal/upgrade/upgrade.go", "package upgrade\nimport \"github.com/hypnotox/agentic-workflows/internal/filesystem\"\nfunc Open(x string) (*filesystem.Handle, error) { return filesystem.Open(x) }", ""},
		{"compile-only reference", "internal/upgrade/upgrade.go", "package upgrade\nimport \"github.com/hypnotox/agentic-workflows/internal/filesystem\"\nvar _ = filesystem.Open", "filesystem import without constructor/capability flow"},
		{"arbitrary selector", "internal/upgrade/upgrade.go", "package upgrade\nimport \"github.com/hypnotox/agentic-workflows/internal/filesystem\"\nvar _ = filesystem.Handle", "filesystem import without constructor/capability flow"},
		{"supported entry policy", "internal/upgrade/upgrade.go", "package upgrade\nimport \"github.com/hypnotox/agentic-workflows/internal/filesystem\"\nfunc supported(entry fs.DirEntry) { filesystem.SupportedTreeEntry(entry) }", ""},
		{"second flock owner", "internal/upgrade/upgrade.go", "package upgrade\nimport \"github.com/gofrs/flock\"\nvar _ = flock.New", "outside filesystem imports advisory-lock implementation"},
	} {
		got := rootSourceFinding(tc.path, tc.src)
		if got == "" {
			got = filesystemConsumerFinding(tc.path, tc.src)
		}
		if got == "" {
			got = advisoryLeaseOwnerFinding(tc.path, tc.src)
		}
		if got == "" && strings.HasPrefix(tc.path, "internal/filesystem/") {
			got = filesystemPackageFinding(map[string]string{tc.path: tc.src})
		}
		if !strings.Contains(got, tc.want) {
			t.Fatalf("%s finding = %q, want category %q", tc.name, got, tc.want)
		}
	}
	for _, tc := range []struct {
		name string
		src  map[string]string
		want string
	}{
		{"cross-file extra concrete", map[string]string{"internal/filesystem/handle.go": "package filesystem\ntype Handle struct{}", "internal/filesystem/extra.go": "package filesystem\ntype Extra struct{}"}, "exported concrete Extra"},
		{"cross-file interface", map[string]string{"internal/filesystem/handle.go": "package filesystem\ntype Handle struct{}", "internal/filesystem/contract.go": "package filesystem\ntype Contract interface{}"}, "interface Contract"},
	} {
		if got := filesystemPackageFinding(tc.src); !strings.Contains(got, tc.want) {
			t.Fatalf("%s finding = %q, want category %q", tc.name, got, tc.want)
		}
	}
}

func advisoryLeaseOwnerFinding(rel, src string) string {
	f, err := parser.ParseFile(token.NewFileSet(), rel, src, parser.ImportsOnly)
	if err != nil {
		return rel + ": parse: " + err.Error()
	}
	for _, im := range f.Imports {
		if strings.Trim(im.Path.Value, `"`) == "github.com/gofrs/flock" && rel != "internal/filesystem/lease.go" {
			return rel + ": outside filesystem imports advisory-lock implementation"
		}
	}
	return ""
}

func rootSourceFinding(rel, src string) string {
	f, err := parser.ParseFile(token.NewFileSet(), rel, src, 0)
	if err != nil {
		return rel + ": parse: " + err.Error()
	}
	osNames := osImportNames(f)
	openRootAliases, rootAliases := rootAliases(f, osNames)
	rootUses := len(openRootAliases) != 0 || len(rootAliases) != 0
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if isOpenRoot(n.Fun, osNames, openRootAliases) {
				rootUses = true
			}
		case *ast.StarExpr:
			if isRootType(n.X, osNames, rootAliases) {
				rootUses = true
			}
		}
		return true
	})
	if rel == "internal/testsupport/fsfixture/fsfixture.go" || strings.HasPrefix(rel, "internal/filesystem/") {
		return ""
	}
	if rootUses {
		return rel + ": outside filesystem concrete root use"
	}
	return ""
}

func osImportNames(f *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, im := range f.Imports {
		if strings.Trim(im.Path.Value, `"`) != "os" {
			continue
		}
		name := "os"
		if im.Name != nil {
			name = im.Name.Name
		}
		names[name] = true
	}
	return names
}

func rootAliases(f *ast.File, osNames map[string]bool) (map[string]bool, map[string]bool) {
	openRootAliases, rootAliases := map[string]bool{}, map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ValueSpec:
			for i, value := range n.Values {
				if i < len(n.Names) && isOpenRoot(value, osNames, openRootAliases) {
					openRootAliases[n.Names[i].Name] = true
				}
			}
		case *ast.AssignStmt:
			for i, value := range n.Rhs {
				if i >= len(n.Lhs) || !isOpenRoot(value, osNames, openRootAliases) {
					continue
				}
				if name, ok := n.Lhs[i].(*ast.Ident); ok {
					openRootAliases[name.Name] = true
				}
			}
		case *ast.TypeSpec:
			if n.Assign.IsValid() && isRootType(n.Type, osNames, rootAliases) {
				rootAliases[n.Name.Name] = true
			}
		}
		return true
	})
	return openRootAliases, rootAliases
}

func isOpenRoot(expr ast.Expr, osNames, aliases map[string]bool) bool {
	if id, ok := expr.(*ast.Ident); ok && aliases[id.Name] {
		return true
	}
	s, ok := expr.(*ast.SelectorExpr)
	return ok && s.Sel.Name == "OpenRoot" && importedOS(s.X, osNames)
}

func isRootType(expr ast.Expr, osNames, aliases map[string]bool) bool {
	if id, ok := expr.(*ast.Ident); ok && aliases[id.Name] {
		return true
	}
	s, ok := expr.(*ast.SelectorExpr)
	return ok && s.Sel.Name == "Root" && importedOS(s.X, osNames)
}

func filesystemConsumerFinding(rel, src string) string {
	f, err := parser.ParseFile(token.NewFileSet(), rel, src, 0)
	if err != nil {
		return rel + ": parse: " + err.Error()
	}
	if rel == "internal/filesystem/handle.go" {
		return ""
	}
	imports := map[string]bool{}
	for _, im := range f.Imports {
		if strings.Trim(im.Path.Value, `"`) != "github.com/hypnotox/agentic-workflows/internal/filesystem" {
			continue
		}
		name := "filesystem"
		if im.Name != nil {
			name = im.Name.Name
		}
		imports[name] = true
	}
	if len(imports) == 0 {
		return ""
	}
	bound := map[string]bool{}
	capability := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			star, ok := n.Type.(*ast.StarExpr)
			if !ok {
				break
			}
			selector, ok := star.X.(*ast.SelectorExpr)
			if !ok || !importedOS(selector.X, imports) {
				break
			}
			if selector.Sel.Name == "ExpectedIdentity" {
				capability = true
				break
			}
			if selector.Sel.Name != "Handle" {
				break
			}
			for _, name := range n.Names {
				bound[name.Name] = true
			}
		case *ast.AssignStmt:
			for i, rhs := range n.Rhs {
				call, ok := rhs.(*ast.CallExpr)
				if !ok || !isFilesystemConstructor(call, imports) || i >= len(n.Lhs) {
					continue
				}
				if id, ok := n.Lhs[i].(*ast.Ident); ok {
					bound[id.Name] = true
				}
			}
		case *ast.CallExpr:
			if isFilesystemConstructor(n, imports) {
				capability = true
			}
			if s, ok := n.Fun.(*ast.SelectorExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok && bound[id.Name] {
					capability = true
				}
				if id, ok := s.X.(*ast.Ident); ok && imports[id.Name] && (s.Sel.Name == "Backup" || s.Sel.Name == "CanonicalRoot" || s.Sel.Name == "SupportedTreeEntry" || s.Sel.Name == "Acquire" || s.Sel.Name == "AcquireProject" || s.Sel.Name == "AcquireTrackedLease" || s.Sel.Name == "AcquireResidentLease" || s.Sel.Name == "AcquireProjectLease") {
					capability = true
				}
			}
			for _, arg := range n.Args {
				if id, ok := arg.(*ast.Ident); ok && bound[id.Name] {
					capability = true
				}
			}
		}
		return true
	})
	if !capability {
		return rel + ": filesystem import without constructor/capability flow"
	}
	return ""
}

func filesystemPackageFinding(sources map[string]string) string {
	handles := 0
	rootUses := false
	for rel, src := range sources {
		f, err := parser.ParseFile(token.NewFileSet(), rel, src, 0)
		if err != nil {
			return rel + ": parse: " + err.Error()
		}
		if f.Name.Name != "filesystem" {
			return rel + ": wrong package"
		}
		osNames := map[string]bool{}
		for _, im := range f.Imports {
			if strings.Trim(im.Path.Value, `"`) == "os" {
				name := "os"
				if im.Name != nil {
					name = im.Name.Name
				}
				osNames[name] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			s, ok := n.(*ast.SelectorExpr)
			if ok && s.Sel.Name == "OpenRoot" && importedOS(s.X, osNames) {
				rootUses = true
			}
			return true
		})
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					return rel + ": interface " + typeSpec.Name.Name
				}
				if !typeSpec.Name.IsExported() {
					continue
				}
				if typeSpec.Name.Name == "Lease" || typeSpec.Name.Name == "LeaseError" || typeSpec.Name.Name == "LeaseErrorKind" || typeSpec.Name.Name == "ExpectedIdentity" {
					continue
				}
				if typeSpec.Name.Name != "Handle" {
					return rel + ": exported concrete " + typeSpec.Name.Name
				}
				if _, ok := typeSpec.Type.(*ast.StructType); !ok {
					return rel + ": Handle is not concrete"
				}
				handles++
			}
		}
	}
	if handles != 1 || !rootUses {
		return "internal/filesystem: expected one concrete Handle and root use"
	}
	return ""
}

func isFilesystemConstructor(call *ast.CallExpr, imports map[string]bool) bool {
	s, ok := call.Fun.(*ast.SelectorExpr)
	// Handle construction and the neutral lease capability are the two allowed
	// production flows from this package; neither exports another concrete type.
	return ok && (s.Sel.Name == "Open" || s.Sel.Name == "CanonicalRoot" || s.Sel.Name == "Acquire" || s.Sel.Name == "AcquireProject" || s.Sel.Name == "AcquireTrackedLease" || s.Sel.Name == "AcquireResidentLease" || s.Sel.Name == "AcquireProjectLease") && importedOS(s.X, imports)
}

func importedOS(expr ast.Expr, names map[string]bool) bool {
	id, ok := expr.(*ast.Ident)
	return ok && names[id.Name]
}
