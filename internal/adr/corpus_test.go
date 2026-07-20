package adr_test

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"golang.org/x/tools/go/packages"
)

var (
	productionPackagesOnce sync.Once
	productionPackages     []*packages.Package
	productionPackagesErr  error
)

func loadProductionPackages(t *testing.T) []*packages.Package {
	t.Helper()
	productionPackagesOnce.Do(func() {
		productionPackages, productionPackagesErr = packages.Load(&packages.Config{
			Dir:  filepath.Join("..", ".."),
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		}, "./...")
		if productionPackagesErr == nil {
			seen := map[string]bool{}
			for _, pkg := range productionPackages {
				if len(pkg.Errors) != 0 {
					productionPackagesErr = pkg.Errors[0]
					break
				}
				seen[pkg.PkgPath] = true
			}
			for _, path := range []string{"github.com/hypnotox/agentic-workflows/changelog", "github.com/hypnotox/agentic-workflows/templates"} {
				if productionPackagesErr == nil && !seen[path] {
					productionPackagesErr = fmt.Errorf("production package scan omitted %s", path)
				}
			}
		}
	})
	if productionPackagesErr != nil {
		t.Fatal(productionPackagesErr)
	}
	return productionPackages
}

func sourcePath(pos token.Position) string {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return filepath.ToSlash(pos.Filename)
	}
	rel, err := filepath.Rel(root, pos.Filename)
	if err != nil {
		return filepath.ToSlash(pos.Filename)
	}
	return filepath.ToSlash(rel)
}

func objectIs(obj types.Object, pkgPath, name string) bool {
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == pkgPath && obj.Name() == name
}

func structFieldIs(obj types.Object, pkgPath, typeName, fieldName string) bool {
	field, ok := obj.(*types.Var)
	if !ok || !field.IsField() || !objectIs(field, pkgPath, fieldName) {
		return false
	}
	typeObj, ok := field.Pkg().Scope().Lookup(typeName).(*types.TypeName)
	if !ok {
		return false
	}
	named, ok := typeObj.Type().(*types.Named)
	if !ok {
		return false
	}
	strct, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := range strct.NumFields() {
		if strct.Field(i) == field {
			return true
		}
	}
	return false
}

const adrPackagePath = "github.com/hypnotox/agentic-workflows/internal/adr"

type callOwner struct {
	path string
	name string
}

type functionRegion struct {
	start token.Pos
	end   token.Pos
	name  string
}

func expressionObject(info *types.Info, expr ast.Expr) types.Object {
	switch expr := expr.(type) {
	case *ast.Ident:
		if obj := info.Uses[expr]; obj != nil {
			return obj
		}
		return info.Defs[expr]
	case *ast.SelectorExpr:
		if selection := info.Selections[expr]; selection != nil {
			return selection.Obj()
		}
		return info.Uses[expr.Sel]
	case *ast.ParenExpr:
		return expressionObject(info, expr.X)
	default:
		return nil
	}
}

func assignedObject(info *types.Info, expr ast.Expr) types.Object {
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return nil
	}
	if obj := info.Defs[ident]; obj != nil {
		return obj
	}
	return info.Uses[ident]
}

// aliasesOf computes a bounded, flow-insensitive intra-file alias closure. Go
// objects are function-scoped, so local aliases cannot leak into another
// function; package-level aliases and aliases captured by closures still work.
func aliasesOf(file *ast.File, info *types.Info, target func(types.Object) bool) map[types.Object]bool {
	aliases := map[types.Object]bool{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(file, func(n ast.Node) bool {
			add := func(lhs ast.Expr, rhs ast.Expr) {
				obj := assignedObject(info, lhs)
				rhsObj := expressionObject(info, rhs)
				if obj != nil && !aliases[obj] && (target(rhsObj) || aliases[rhsObj]) {
					aliases[obj] = true
					changed = true
				}
			}
			switch n := n.(type) {
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) {
					for i := range n.Lhs {
						add(n.Lhs[i], n.Rhs[i])
					}
				}
			case *ast.ValueSpec:
				if len(n.Names) == len(n.Values) {
					for i := range n.Names {
						add(n.Names[i], n.Values[i])
					}
				}
			}
			return true
		})
	}
	return aliases
}

func expressionMatches(info *types.Info, aliases map[types.Object]bool, target func(types.Object) bool, expr ast.Expr) bool {
	obj := expressionObject(info, expr)
	return target(obj) || aliases[obj]
}

func functionRegions(file *ast.File, fset *token.FileSet) []functionRegion {
	var regions []functionRegion
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			regions = append(regions, functionRegion{start: n.Pos(), end: n.End(), name: n.Name.Name})
		case *ast.FuncLit:
			pos := fset.Position(n.Pos())
			regions = append(regions, functionRegion{start: n.Pos(), end: n.End(), name: "<func literal at line " + strconv.Itoa(pos.Line) + ">"})
		}
		return true
	})
	return regions
}

func enclosingFunction(regions []functionRegion, pos token.Pos) string {
	name := "<package scope>"
	var width token.Pos
	for _, region := range regions {
		if region.start <= pos && pos < region.end && (width == 0 || region.end-region.start < width) {
			name = region.name
			width = region.end - region.start
		}
	}
	return name
}

func isParseDir(obj types.Object) bool {
	return objectIs(obj, adrPackagePath, "ParseDir")
}

func isCorpusRaw(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok || !objectIs(fn, adrPackagePath, "Raw") {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() != nil && namedTypeIs(sig.Recv().Type(), adrPackagePath, "Corpus")
}

func namedTypeIs(typ types.Type, pkgPath, name string) bool {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	return ok && objectIs(named.Obj(), pkgPath, name)
}

// parseDirCallFindings is the one type-aware ParseDir caller detector used by
// both the production scan and mutation overlays. Calls are attributed to the
// innermost enclosing function, and local or package-level aliases count.
func parseDirCallFindings(pkgs []*packages.Package) map[callOwner][]string {
	calls := map[callOwner][]string{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			path := sourcePath(pkg.Fset.Position(file.Pos()))
			aliases := aliasesOf(file, pkg.TypesInfo, isParseDir)
			regions := functionRegions(file, pkg.Fset)
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !expressionMatches(pkg.TypesInfo, aliases, isParseDir, call.Fun) {
					return true
				}
				pos := pkg.Fset.Position(call.Pos())
				owner := callOwner{path: path, name: enclosingFunction(regions, call.Pos())}
				calls[owner] = append(calls[owner], fromLine(path, pos.Line-1, "ParseDir call"))
				return true
			})
		}
	}
	return calls
}

// sectionReadFindings is the type-aware detector used for both the production
// corpus scan and its mutation fixtures. Selection.Obj is the field declaration
// itself, so promoted selections through an embedded ADR are covered too.
func sectionReadFindings(pkgs []*packages.Package) []string {
	var bad []string
	for _, pkg := range pkgs {
		if pkg.PkgPath == adrPackagePath {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				selection := pkg.TypesInfo.Selections[sel]
				if selection != nil && structFieldIs(selection.Obj(), adrPackagePath, "ADR", "Sections") {
					pos := pkg.Fset.Position(sel.Pos())
					bad = append(bad, fromLine(sourcePath(pos), pos.Line-1, "ADR.Sections"))
				}
				return true
			})
		}
	}
	return bad
}

// rawAccessFindings is likewise the one type-aware detector for Corpus.Raw and
// os.ReadFile values derived from ADR.Path, shared by production and mutations.
func rawAccessFindings(pkgs []*packages.Package) (map[string][]string, []string) {
	raw := map[string][]string{}
	var pathReads []string
	isADRPath := func(obj types.Object) bool { return structFieldIs(obj, adrPackagePath, "ADR", "Path") }
	isReadFile := func(obj types.Object) bool { return objectIs(obj, "os", "ReadFile") }
	for _, pkg := range pkgs {
		if pkg.PkgPath == adrPackagePath {
			continue
		}
		for _, file := range pkg.Syntax {
			path := sourcePath(pkg.Fset.Position(file.Pos()))
			rawAliases := aliasesOf(file, pkg.TypesInfo, isCorpusRaw)
			pathAliases := aliasesOf(file, pkg.TypesInfo, isADRPath)
			readFileAliases := aliasesOf(file, pkg.TypesInfo, isReadFile)
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if expressionMatches(pkg.TypesInfo, rawAliases, isCorpusRaw, call.Fun) {
					pos := pkg.Fset.Position(call.Pos())
					raw[path] = append(raw[path], fromLine(path, pos.Line-1, "Corpus.Raw call"))
				}
				if len(call.Args) != 0 && expressionMatches(pkg.TypesInfo, readFileAliases, isReadFile, call.Fun) &&
					expressionMatches(pkg.TypesInfo, pathAliases, isADRPath, call.Args[0]) {
					pos := pkg.Fset.Position(call.Pos())
					pathReads = append(pathReads, fromLine(path, pos.Line-1, "os.ReadFile(ADR.Path-derived value)"))
				}
				return true
			})
		}
	}
	return raw, pathReads
}

func loadMutationPackage(t *testing.T, rel, pattern, body string) []*packages.Package {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, filepath.FromSlash(rel))
	pkgs, err := packages.Load(&packages.Config{
		Dir:     root,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Overlay: map[string][]byte{filename: []byte(body)},
	}, pattern)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
	}
	return pkgs
}

// TestCorpusParsedOnce enforces ADR-0130 item 1: one parse per invocation.
// adr.ParseDir has no production caller outside internal/adr - every consumer
// enters through Corpus construction - and inside internal/adr only that seam
// and NextNumber call it. NextNumber is the enumerated exception: it runs on
// the awf new adr path, which holds no corpus.
// invariant: adr-system/adr-lifecycle:corpus-parsed-once
func parseDirProblems(callers map[callOwner][]string) []string {
	want := map[callOwner]bool{
		{path: "internal/adr/corpus.go", name: "LoadCorpus"}: true,
		{path: "internal/adr/adr.go", name: "NextNumber"}:    true,
	}
	var problems []string
	for owner, positions := range callers {
		if !want[owner] {
			problems = append(problems, owner.path+":"+owner.name+" calls ParseDir outside LoadCorpus or NextNumber: "+strings.Join(positions, ", "))
		}
	}
	for owner := range want {
		if len(callers[owner]) != 1 {
			problems = append(problems, owner.path+":"+owner.name+" must call ParseDir exactly once; found "+strconv.Itoa(len(callers[owner])))
		}
	}
	return problems
}

func replaceMutationSource(t *testing.T, rel, old, replacement string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(testsupport.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), old) != 1 {
		t.Fatalf("mutation target %q occurs %d times in %s, want 1", old, strings.Count(string(body), old), rel)
	}
	return strings.Replace(string(body), old, replacement, 1)
}

func TestCorpusParsedOnce(t *testing.T) {
	pkgs := loadProductionPackages(t)
	callers := parseDirCallFindings(pkgs)
	inspectedADRFiles := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			path := sourcePath(pkg.Fset.Position(file.Pos()))
			if strings.HasPrefix(path, "internal/adr/") {
				inspectedADRFiles[path] = true
			}
		}
	}
	testsupport.WalkRepoFiles(t, testsupport.RepoRoot(t), func(rel string) bool {
		return strings.HasPrefix(rel, "internal/adr/") && strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
	}, func(rel string, _ []byte) {
		if !inspectedADRFiles[rel] {
			t.Errorf("production ADR source %s was not inspected", rel)
		}
	})
	if problems := parseDirProblems(callers); len(problems) != 0 {
		t.Errorf("ParseDir call set differs from the single LoadCorpus and NextNumber calls:\n\t%s", strings.Join(problems, "\n\t"))
	}

	aliasMutation := loadMutationPackage(t, "internal/currentstate/corpus_mutation_fixture.go", "./internal/currentstate", `package currentstate

import "github.com/hypnotox/agentic-workflows/internal/adr"

func mutationParseDir() {
	parse := adr.ParseDir
	_, _ = parse("docs/decisions")
}
`)
	aliasOwner := callOwner{path: "internal/currentstate/corpus_mutation_fixture.go", name: "mutationParseDir"}
	if got := parseDirCallFindings(aliasMutation)[aliasOwner]; len(got) != 1 {
		t.Fatalf("aliased ParseDir invocation escaped the production detector: %#v", got)
	}

	const parseCall = "\tadrs, err := ParseDir(dir)\n"
	withoutNext := replaceMutationSource(t, "internal/adr/adr.go", parseCall, "\tvar adrs []ADR\n\tvar err error\n")
	withoutNextPkgs := loadMutationPackage(t, "internal/adr/adr.go", "./internal/adr", withoutNext)
	if problems := parseDirProblems(parseDirCallFindings(withoutNextPkgs)); len(problems) != 1 ||
		!strings.Contains(strings.Join(problems, "\n"), "NextNumber must call ParseDir exactly once") {
		// LoadCorpus is in corpus.go and remains visible, so only the missing
		// call should be reported.
		t.Fatalf("removed NextNumber call escaped cardinality proof: %#v", problems)
	}

	extraFunction := replaceMutationSource(t, "internal/adr/adr.go", "\nfunc NextNumber(dir string)", `
func mutationOtherParseDir(dir string) {
	_, _ = ParseDir(dir)
}

func NextNumber(dir string)`)
	extraFunctionPkgs := loadMutationPackage(t, "internal/adr/adr.go", "./internal/adr", extraFunction)
	if problems := parseDirProblems(parseDirCallFindings(extraFunctionPkgs)); len(problems) != 1 ||
		!strings.Contains(strings.Join(problems, "\n"), "adr.go:mutationOtherParseDir calls ParseDir outside") {
		t.Fatalf("additional adr.go function call escaped enclosing-function proof: %#v", problems)
	}

	duplicateNext := replaceMutationSource(t, "internal/adr/adr.go", parseCall, parseCall+"\t_, _ = ParseDir(dir)\n")
	duplicateNextPkgs := loadMutationPackage(t, "internal/adr/adr.go", "./internal/adr", duplicateNext)
	if problems := parseDirProblems(parseDirCallFindings(duplicateNextPkgs)); len(problems) != 1 ||
		!strings.Contains(strings.Join(problems, "\n"), "NextNumber must call ParseDir exactly once; found 2") {
		t.Fatalf("duplicate NextNumber ParseDir call escaped cardinality proof: %#v", problems)
	}
}

// TestCorpusOwnsFieldReads enforces ADR-0130's corpus-owns-field-reads: the
// section questions ADR.Sections answers are asked of the view, not re-derived
// from the field.
// invariant: adr-system/adr-lifecycle:corpus-owns-field-reads
func TestCorpusOwnsFieldReads(t *testing.T) {
	bad := sectionReadFindings(loadProductionPackages(t))
	if len(bad) != 0 {
		t.Errorf("ADR.Sections is read outside internal/adr; ask the view instead (ADR-0130 item 2):\n\t%s", strings.Join(bad, "\n\t"))
	}
	mutation := loadMutationPackage(t, "internal/currentstate/corpus_mutation_fixture.go", "./internal/currentstate", `package currentstate

import "github.com/hypnotox/agentic-workflows/internal/adr"

type mutationEmbeddedADR struct {
	adr.ADR
}

func mutationSections(rec adr.ADR, embedded mutationEmbeddedADR) {
	x := rec.Sections
	_ = x
	for range rec.Sections {
	}
	_ = embedded.Sections
}
`)
	if findings := sectionReadFindings(mutation); len(findings) != 3 {
		t.Fatalf("Sections direct/embedded mutation fixtures: detected %d, want 3: %#v", len(findings), findings)
	}
}

func rawAccessProblems(raw map[string][]string, allowed map[string]bool) []string {
	var problems []string
	for path, positions := range raw {
		if !allowed[path] {
			problems = append(problems, path+" accesses Corpus.Raw outside the enumerated migration seams: "+strings.Join(positions, ", "))
		}
	}
	for path := range allowed {
		positions := raw[path]
		if len(positions) != 1 {
			problems = append(problems, path+" must contain exactly one Corpus.Raw call; found "+strconv.Itoa(len(positions))+": "+strings.Join(positions, ", "))
		}
	}
	return problems
}

// TestCorpusRawAccessEnumerated enforces the closed raw-byte seams: the ordered
// schema migrations that perform offset surgery. Each goes through the view's
// named accessor rather than re-reading the file.
// invariant: adr-system/adr-lifecycle:corpus-raw-access-enumerated
func TestCorpusRawAccessEnumerated(t *testing.T) {
	raw, pathReads := rawAccessFindings(loadProductionPackages(t))
	want := map[string]bool{
		"internal/migrate/retirementtokens.go": true,
		"internal/migrate/supersessionkeys.go": true,
	}
	if problems := rawAccessProblems(raw, want); len(problems) != 0 {
		t.Errorf("Corpus.Raw call set differs from the two single-call migration seams:\n\t%s", strings.Join(problems, "\n\t"))
	}
	if len(pathReads) != 0 {
		t.Errorf("an ADR file is read directly rather than through the view's accessor:\n\t%s", strings.Join(pathReads, "\n\t"))
	}
	mutation := loadMutationPackage(t, "internal/migrate/corpus_mutation_fixture.go", "./internal/migrate", `package migrate

import (
	"os"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

func mutationRaw(c adr.Corpus, rec adr.ADR) {
	_, _ = c.Raw("0001")
	_, _ = c.Raw("0002")
	raw := c.Raw
	_, _ = raw("0003")
	_, _ = os.ReadFile(rec.Path)
	path := rec.Path
	_, _ = os.ReadFile(path)
}
`)
	mutationRaw, mutationReads := rawAccessFindings(mutation)
	mutationPath := "internal/migrate/corpus_mutation_fixture.go"
	if len(mutationRaw[mutationPath]) != 3 || len(mutationReads) != 2 {
		t.Fatalf("raw-access direct/method-value mutation fixtures: Raw=%#v ReadFile(ADR.Path)=%#v, want three typed calls and two typed reads", mutationRaw, mutationReads)
	}
	mutationAllowed := map[string]bool{
		"internal/migrate/retirementtokens.go": true,
		"internal/migrate/supersessionkeys.go": true,
		mutationPath:                           true,
	}
	if problems := rawAccessProblems(mutationRaw, mutationAllowed); len(problems) != 1 || !strings.Contains(problems[0], "exactly one") {
		t.Fatalf("extra Raw call in an allowed file escaped the cardinality check: %#v", problems)
	}
}

func fromLine(path string, i int, line string) string {
	return path + ":" + strconv.Itoa(i+1) + ": " + strings.TrimSpace(line)
}

// TestCorpusAbsentADR covers the view's absent-ADR guards. Every lookup takes a
// number that may not resolve: a token can cite an ADR that does not exist, and
// the check that reports that is a different one from the accessor. The
// accessors answer emptily rather than panicking, leaving the missing-target
// finding to its owner.
func TestCorpusAbsentADR(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-only.md"),
		testsupport.ADR("Accepted", testsupport.WithTitle("0001: Only"),
			testsupport.WithBody("## Decision\n\n1. x.\n\n## Invariants\n\n- `invariant: only-slug` - x.\n")))
	c, err := adr.LoadCorpus(dir)
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}

	if _, ok := c.ByNumber("9999"); ok {
		t.Error("ByNumber resolved an absent ADR")
	}
	if c.Has("9999") {
		t.Error("Has reported an absent ADR present")
	}
	if _, err := c.Raw("9999"); err == nil {
		t.Error("Raw on an absent ADR returned no error")
	}

	// The present ADR answers for real, so the guards above are not passing
	// vacuously over an empty corpus.
	if _, err := c.Raw("0001"); err != nil {
		t.Errorf("Raw(0001): %v", err)
	}
}

// TestAuditSharesADRParser enforces ADR-0130 item 5: internal/audit reads git
// blobs rather than the working tree, so it cannot take a Corpus - but it takes
// the bytes seam, and declares no frontmatter struct of its own. The duplication
// the ADR removed was the parser and the schema, not the loading strategy.
// invariant: adr-system/adr-lifecycle:audit-shares-adr-parser
func TestAuditSharesADRParser(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "audit", "audit.go"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, banned := range []string{`yaml:"status"`, `yaml:"domains"`, `yaml:"supersedes"`, `yaml:"related"`} {
		if strings.Contains(body, banned) {
			t.Errorf("internal/audit declares its own ADR frontmatter field %s; parse through adr.ParseBytes instead (ADR-0130 item 5)", banned)
		}
	}
	if !strings.Contains(body, "adr.ParseBytes(") {
		t.Error("internal/audit no longer calls adr.ParseBytes - has the shared seam moved?")
	}
}
