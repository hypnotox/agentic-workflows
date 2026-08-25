package upgrade

import (
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
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/fsfixture"
)

// TestTreeDigestCharacterization freezes the output captured by the pre-conversion
// digest implementation. The literal universe and digest below were produced by:
// go run ./cmd/baselinedigest (a disposable copy of b00d1274's digest logic)
func TestTreeDigestCharacterization(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		".awf/config.yaml":                              "prefix: fixture\ndomains:\n  - alpha\ncurrentState:\n  sources:\n    - globs:\n        - internal/**\n      marker: //\n",
		".awf/current-state-migration.yaml":             "version: 1\ninvariantApprovals: []\n",
		".awf/domains/alpha.yaml":                       "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/core.yaml":          "title: Core\nsummary: Core.\npaths:\n  - internal/**\n",
		".awf/topics/parts/alpha/core/current-state.md": "Intro.\n",
		"docs/decisions/0001-first.md":                  "---\nformat: current-state-v3\nstatus: Implemented\n---\n# ADR-0001: First\n",
		"internal/marker.go":                            "// marker\npackage internal\n",
		"ordinary.txt":                                  "ordinary\n",
		"nested/.awf/config.yaml":                       "prefix: nested\ndomains:\n  - nested\n",
		"nested/internal/ignored.go":                    "package ignored\n",
	}
	for path, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, path), body)
		if err := os.Chmod(filepath.Join(root, path), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wantPaths := []string{
		".awf/config.yaml", ".awf/current-state-migration.yaml", ".awf/domains/alpha.yaml",
		".awf/topics/metadata/alpha/core.yaml", ".awf/topics/parts/alpha/core/current-state.md",
		"docs/decisions/0001-first.md", "internal/marker.go",
	}
	tree := openDigestTree(t, root)
	universe := map[string]bool{".awf/config.yaml": true, approvalPath: true}
	for _, sub := range []string{".awf/domains", ".awf/topics"} {
		if err := collectUnder(tree, sub, universe); err != nil {
			t.Fatal(err)
		}
	}
	if err := collectADRs(tree, "docs/decisions", universe); err != nil {
		t.Fatal(err)
	}
	if err := collectMarkerSources(tree, configSource(), universe); err != nil {
		t.Fatal(err)
	}
	gotPaths := make([]string, 0, len(universe))
	for path := range universe {
		gotPaths = append(gotPaths, path)
	}
	slices.Sort(gotPaths)
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("universe = %#v, want %#v", gotPaths, wantPaths)
	}
	got, err := treeDigest(root, tree)
	if err != nil {
		t.Fatal(err)
	}
	const wantDigest = "sha256:ec2b28599282f9ae5e0a92d28527d228bac7870303375518e718b126f5231940"
	if got != wantDigest {
		t.Fatalf("digest = %s, want %s", got, wantDigest)
	}
}

func configSource() []config.CurrentStateSource {
	return []config.CurrentStateSource{{Globs: []string{"internal/**"}}}
}

func openDigestTree(t *testing.T, root string) *filesystem.Handle {
	t.Helper()
	tree, err := filesystem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tree.Close(); err != nil {
			t.Error(err)
		}
	})
	return tree
}

func TestCollectMarkerSourcesPrunesNamedDirectories(t *testing.T) {
	for _, path := range []string{
		".git/ignored.go", "vendor/ignored.go", "node_modules/ignored.go",
		"nested/.git/ignored.go", "nested/vendor/ignored.go", "nested/node_modules/ignored.go",
	} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(path)), "package ignored\n")
			universe := map[string]bool{}
			if err := collectMarkerSources(openDigestTree(t, root), configSource(), universe); err != nil {
				t.Fatal(err)
			}
			if universe[path] {
				t.Fatalf("pruned source %q was selected", path)
			}
		})
	}
}

func TestCollectMarkerSourcesPropagatesNestedGitProbeFailure(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "nested", "x.go"), "package nested\n")
	sentinel := errors.New("git probe")
	tree, err := fsfixture.Open(root, fsfixture.Fault{Operation: fsfixture.OperationLinkInfo, Path: "nested/.git", Err: sentinel})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tree.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := collectMarkerSources(tree, configSource(), map[string]bool{}); !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
}

func TestTreeDigestRejectsInvalidConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), "not: [yaml")
	if _, err := treeDigest(root, openDigestTree(t, root)); err == nil {
		t.Fatal("invalid config accepted")
	}
}

// TestVerifyUsesInjectedFilesystem proves public wiring and the injected behavioral boundary.
// invariant: code-design/dependency-composition:upgrade-attestation-filesystem-wiring (TestVerifyUsesInjectedFilesystem)
func TestVerifyUsesInjectedFilesystem(t *testing.T) {
	dir, head, digest := sealedRepo(t)
	sentinel := errors.New("read fault")
	tree, err := fsfixture.Open(dir, fsfixture.Fault{Operation: fsfixture.OperationRead, Path: ".awf/config.yaml", Err: sentinel})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tree.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := verifyWithFilesystem(testContext(t), dir, sealedAtt(head, digest), tree); !errors.Is(err, sentinel) {
		t.Fatalf("fault identity: %v", err)
	}

	if finding := verifyFilesystemConstruction(upgradeProductionSources(t)); finding != "" {
		t.Fatalf("live construction proof failed: %s", finding)
	}
	good := `package upgrade
import treepkg "github.com/hypnotox/agentic-workflows/internal/filesystem"
type attestationTree interface{ Close() error }
func Verify(ctx, root, att any) error { tree, err := treepkg.Open(root); if err != nil { return err }; defer tree.Close(); return verifyWithFilesystem(ctx, root, att, tree) }
func verifyWithFilesystem(ctx, root, att any, tree attestationTree) error { return treeDigest(root, tree) }
func treeDigest(root string, tree attestationTree) error { collectUnder(tree); collectADRs(tree); return collectMarkerSources(tree) }
func collectUnder(tree attestationTree) {}
func collectADRs(tree attestationTree) {}
func collectMarkerSources(tree attestationTree) error { return nil }`
	for _, tc := range []struct {
		name    string
		sources map[string]string
		want    string
	}{
		{"package explicit default", map[string]string{"wiring.go": strings.Replace(good, "type attestationTree interface{ Close() error }", "type attestationTree interface{ Close() error }\nvar defaultTree attestationTree", 1)}, "package dependency var explicit"},
		{"package concrete default", map[string]string{"wiring.go": strings.Replace(good, "type attestationTree interface{ Close() error }", "type attestationTree interface{ Close() error }\nvar defaultTree *treepkg.Handle", 1)}, "package dependency var concrete"},
		{"third-file aliased mutable default", map[string]string{"wiring.go": good, "default.go": `package upgrade; import treepkg "github.com/hypnotox/agentic-workflows/internal/filesystem"; var defaultConstructor = treepkg.Open`}, "package dependency var inferred"},
		{"third-file aliased constructor", map[string]string{"wiring.go": good, "hidden.go": `package upgrade; import treepkg "github.com/hypnotox/agentic-workflows/internal/filesystem"; func hidden(root string) { treepkg.Open(root) }`}, "filesystem.Open outside Verify"},
		{"third-file dot-import constructor", map[string]string{"wiring.go": good, "hidden.go": `package upgrade; import . "github.com/hypnotox/agentic-workflows/internal/filesystem"; func hidden(root string) { Open(root) }`}, "filesystem.Open outside Verify"},
		{"missing deferred Close", map[string]string{"wiring.go": strings.Replace(good, "defer tree.Close(); ", "", 1)}, "constructor result is not deferred through Close"},
		{"missing helper parameter", map[string]string{"wiring.go": strings.Replace(good, "func collectADRs(tree attestationTree)", "func collectADRs()", 1)}, "collectADRs does not receive tree"},
		{"rebound constructor", map[string]string{"wiring.go": strings.Replace(good, "defer tree.Close()", "tree = nil; defer tree.Close()", 1)}, "constructor result rebound"},
		{"helper rebinding", map[string]string{"wiring.go": strings.Replace(good, "func collectUnder(tree attestationTree) {}", "func collectUnder(tree attestationTree) { tree = nil }", 1)}, "collectUnder rebinds tree"},
	} {
		if finding := verifyFilesystemConstruction(tc.sources); !strings.Contains(finding, tc.want) {
			t.Fatalf("%s finding = %q, want %q", tc.name, finding, tc.want)
		}
	}
}

func upgradeProductionSources(t *testing.T) map[string]string {
	t.Helper()
	sources := map[string]string{}
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(rel string, body []byte) {
		if strings.HasPrefix(rel, "internal/upgrade/") && rel != "internal/upgrade/journal.go" {
			// The generic journal independently owns a root-confined transaction
			// handle; this census is specific to attestation verification wiring.
			sources[rel] = string(body)
		}
	})
	return sources
}

func verifyFilesystemConstruction(sources map[string]string) string {
	decls := map[string]*ast.FuncDecl{}
	aliases := map[string]map[string]bool{}
	constructors := 0
	constructorOutsideVerify := false
	for path, src := range sources {
		f, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			return path + ": parse: " + err.Error()
		}
		filesystemNames := filesystemImportNames(f)
		for _, decl := range f.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				for _, spec := range gen.Specs {
					if value, ok := spec.(*ast.ValueSpec); ok {
						if category := dependencyVarCategory(value, filesystemNames); category != "" {
							return "package dependency var " + category
						}
					}
				}
			}
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			decls[fn.Name.Name] = fn
			aliases[fn.Name.Name] = filesystemNames
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && isFilesystemOpen(call, filesystemNames) {
					constructors++
					if fn.Name.Name != "Verify" {
						constructorOutsideVerify = true
					}
				}
				return true
			})
		}
	}
	verify := decls["Verify"]
	if constructorOutsideVerify {
		return "filesystem.Open outside Verify"
	}
	if verify == nil || constructors != 1 {
		return "expected Verify's single filesystem.Open"
	}
	for _, name := range []string{"verifyWithFilesystem", "treeDigest", "collectUnder", "collectADRs", "collectMarkerSources"} {
		if !receivesTree(decls[name]) {
			return name + " does not receive tree"
		}
		if rebinding := treeRebinding(decls[name]); rebinding != "" {
			return name + " rebinds tree"
		}
	}
	tree, finding := verifyTreeBinding(verify, aliases["Verify"])
	if finding != "" {
		return finding
	}
	if !callsWithTree(verify, "verifyWithFilesystem", tree) {
		return "Verify does not forward tree to verifyWithFilesystem"
	}
	if !callsWithTree(decls["verifyWithFilesystem"], "treeDigest", "tree") {
		return "verifyWithFilesystem does not forward tree to treeDigest"
	}
	for _, name := range []string{"collectUnder", "collectADRs", "collectMarkerSources"} {
		if !callsWithTree(decls["treeDigest"], name, "tree") {
			return "treeDigest does not forward tree to " + name
		}
	}
	return ""
}

func receivesTree(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if name.Name == "tree" && isIdent(field.Type, "attestationTree") {
				return true
			}
		}
	}
	return false
}

func treeRebinding(fn *ast.FuncDecl) string {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, left := range assign.Lhs {
			if isIdent(left, "tree") {
				found = true
			}
		}
		return true
	})
	if found {
		return "tree"
	}
	return ""
}

func verifyTreeBinding(verify *ast.FuncDecl, filesystemNames map[string]bool) (string, string) {
	var tree string
	deferred := false
	var finding string
	ast.Inspect(verify.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for i, left := range n.Lhs {
				if !isIdent(left, "tree") {
					continue
				}
				if len(n.Rhs) == 1 && i == 0 {
					if call, ok := n.Rhs[0].(*ast.CallExpr); ok && isFilesystemOpen(call, filesystemNames) && n.Tok == token.DEFINE {
						tree = "tree"
						continue
					}
				}
				finding = "constructor result rebound or not directly bound"
			}
		case *ast.DeferStmt:
			if isMethodCall(n.Call, "tree", "Close") {
				deferred = true
			}
		}
		return true
	})
	if tree == "" {
		return "", "Verify does not bind filesystem.Open directly"
	}
	if finding != "" {
		return "", finding
	}
	if !deferred {
		return "", "constructor result is not deferred through Close"
	}
	return tree, ""
}

func callsWithTree(fn *ast.FuncDecl, name, tree string) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if ok && called(call, name) {
			for _, arg := range call.Args {
				if isIdent(arg, tree) {
					found = true
				}
			}
		}
		return true
	})
	return found
}

func filesystemImportNames(f *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, im := range f.Imports {
		if strings.Trim(im.Path.Value, `"`) != "github.com/hypnotox/agentic-workflows/internal/filesystem" {
			continue
		}
		name := "filesystem"
		if im.Name != nil {
			name = im.Name.Name
		}
		names[name] = true
	}
	return names
}

func dependencyVarCategory(value *ast.ValueSpec, filesystemNames map[string]bool) string {
	if isIdent(value.Type, "attestationTree") {
		return "explicit"
	}
	if referencesFilesystem(value.Type, filesystemNames) {
		return "concrete"
	}
	for _, expression := range value.Values {
		if referencesFilesystem(expression, filesystemNames) {
			return "inferred"
		}
	}
	return ""
}

func referencesFilesystem(node ast.Node, filesystemNames map[string]bool) bool {
	if node == nil {
		return false
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		s, ok := n.(*ast.SelectorExpr)
		if ok {
			id, ok := s.X.(*ast.Ident)
			found = ok && filesystemNames[id.Name]
		}
		if id, ok := n.(*ast.Ident); ok && filesystemNames["."] && (id.Name == "Open" || id.Name == "Handle") {
			found = true
		}
		return !found
	})
	return found
}

func isFilesystemOpen(call *ast.CallExpr, filesystemNames map[string]bool) bool {
	if id, ok := call.Fun.(*ast.Ident); ok {
		return filesystemNames["."] && id.Name == "Open"
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Open" {
		return false
	}
	id, ok := selector.X.(*ast.Ident)
	return ok && filesystemNames[id.Name]
}
func isIdent(expr ast.Expr, name string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == name
}
func isMethodCall(call *ast.CallExpr, receiver, method string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == method && isIdent(selector.X, receiver)
}
func called(call *ast.CallExpr, name string) bool { return isIdent(call.Fun, name) }

func TestDigestFaults(t *testing.T) {
	root, _, _ := sealedRepo(t)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/config.yaml"), "prefix: example\ndomains:\n  - alpha\ncurrentState:\n  sources:\n    - globs:\n        - internal/**\n      marker: //\n")
	testsupport.WriteFile(t, filepath.Join(root, "nested", "x.go"), "package nested\n")
	for _, tc := range []struct {
		name    string
		fault   fsfixture.Fault
		context string
		invoke  func(*fsfixture.Handle) error
	}{
		{"config missing", fsfixture.Fault{Operation: fsfixture.OperationRead, Path: ".awf/config.yaml", Err: fs.ErrNotExist}, "not an awf project", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"config read", fsfixture.Fault{Operation: fsfixture.OperationRead, Path: ".awf/config.yaml", Err: errors.New("config")}, "read config", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"sidecar walk", fsfixture.Fault{Operation: fsfixture.OperationWalk, Path: ".awf/domains", Err: errors.New("walk")}, "", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"decision walk", fsfixture.Fault{Operation: fsfixture.OperationWalk, Path: "docs/decisions", Err: errors.New("walk")}, "", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"marker walk", fsfixture.Fault{Operation: fsfixture.OperationWalk, Path: ".", Err: errors.New("walk")}, "", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"entry info", fsfixture.Fault{Operation: fsfixture.OperationWalkInfo, Path: ".awf/domains", Err: errors.New("info")}, "", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"selected read", fsfixture.Fault{Operation: fsfixture.OperationRead, Path: ".awf/domains/alpha.yaml", Err: errors.New("read")}, "", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"selected info", fsfixture.Fault{Operation: fsfixture.OperationInfo, Path: ".awf/domains/alpha.yaml", Err: errors.New("info")}, "", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"git probe", fsfixture.Fault{Operation: fsfixture.OperationLinkInfo, Path: "nested/.git", Err: errors.New("git")}, "", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
		{"awf probe", fsfixture.Fault{Operation: fsfixture.OperationInfo, Path: "nested/.awf", Err: errors.New("awf")}, "", func(h *fsfixture.Handle) error { _, err := treeDigest(root, h); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := fsfixture.Open(root, tc.fault)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := tree.Close(); err != nil {
					t.Error(err)
				}
			})
			err = tc.invoke(tree)
			if err == nil || !errors.Is(err, tc.fault.Err) || (tc.context != "" && !strings.Contains(err.Error(), tc.context)) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestTreeDigestSelectionEdges(t *testing.T) {
	root, _, original := sealedRepo(t)
	// A selected entry missing at its initial Read is optional, even though the sidecar walk kept it in the universe.
	tree, err := fsfixture.Open(root, fsfixture.Fault{Operation: fsfixture.OperationRead, Path: ".awf/domains/alpha.yaml", Err: fs.ErrNotExist})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tree.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := treeDigest(root, tree); err != nil {
		t.Fatalf("initial selected-file absence: %v", err)
	}
	// A nonregular authored entry is not a digest record.
	if err := os.Mkdir(filepath.Join(root, ".awf", "domains", "directory.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := treeDigest(root, openDigestTree(t, root)); err != nil {
		t.Fatalf("nonregular entry: %v", err)
	}
	// Permission mode is part of the record encoding, independently of contents.
	modeRoot, _, modeDigest := sealedRepo(t)
	if err := os.Chmod(filepath.Join(modeRoot, ".awf", "domains", "alpha.yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	moved, err := treeDigest(modeRoot, openDigestTree(t, modeRoot))
	if err != nil {
		t.Fatal(err)
	}
	if moved == modeDigest || original == "" {
		t.Fatal("digest ignored mode")
	}
}
