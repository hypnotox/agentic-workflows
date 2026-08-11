package filesystem

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
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
	if err := h.Remove("created/child/artifact"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "created/child/artifact")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removed path remains: %v", err)
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
		{"compile-only reference", "internal/upgrade/upgrade.go", "package upgrade\nimport \"github.com/hypnotox/agentic-workflows/internal/filesystem\"\nvar _ = filesystem.Open", "filesystem import without constructor/capability flow"},
		{"arbitrary selector", "internal/upgrade/upgrade.go", "package upgrade\nimport \"github.com/hypnotox/agentic-workflows/internal/filesystem\"\nvar _ = filesystem.Handle", "filesystem import without constructor/capability flow"},
	} {
		got := rootSourceFinding(tc.path, tc.src)
		if got == "" {
			got = filesystemConsumerFinding(tc.path, tc.src)
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
			if s, ok := n.Fun.(*ast.SelectorExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok && bound[id.Name] {
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
	return ok && s.Sel.Name == "Open" && importedOS(s.X, imports)
}

func importedOS(expr ast.Expr, names map[string]bool) bool {
	id, ok := expr.(*ast.Ident)
	return ok && names[id.Name]
}
