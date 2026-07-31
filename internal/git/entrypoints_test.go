package git

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// suite names the contract suite that pins one entrypoint's semantics. A suite
// may live outside this package: ProjectResidentRoot composes two seam steps for
// exactly one consumer and is pinned where that consumer is wired.
type suite struct {
	Package string
	Test    string
}

// entrypointSuites maps every exported entrypoint of the seam to the suite that
// pins it. The table is checked in both directions against the package source,
// so it cannot drift: a new entrypoint fails here until it is registered, a
// registration naming a test that does not exist fails, and an entry for an
// entrypoint that no longer exists fails. That is what makes
// pinned-entrypoint-semantics a claim about reality rather than about intent.
//
// A suite is the test that asserts what the entrypoint ANSWERS. The
// cancellation, error-identity, and isolation suites cut across every
// entrypoint and deliberately do not appear here; registering one of those
// would satisfy the table without anything pinning the entrypoint's semantics.
var entrypointSuites = map[string]suite{
	// Object reads.
	"Root":         {"internal/git", "TestWorkingPaths"},
	"WorkingPaths": {"internal/git", "TestWorkingPaths"},
	"IndexBlobs":   {"internal/git", "TestIndexBlobs"},
	"CommitBlobs":  {"internal/git", "TestObjectReadContracts"},
	"RangeBlobs":   {"internal/git", "TestObjectReadContracts"},
	"FileText":     {"internal/git", "TestRangeCommitsLinearRangeCarriesChangesAndText"},
	"HeadExists":   {"internal/git", "TestHeadExists"},
	"HeadHash":     {"internal/git", "TestHeadHash"},

	// Working-tree truth.
	"ChangeCounts": {"internal/git", "TestChangeCountsSeparatesEveryDirtyTreeState"},

	// Revision, branch, and control-file reads.
	"ResolveCommit":   {"internal/git", "TestRevisionAndBranchReadsReportRepositoryState"},
	"CurrentBranch":   {"internal/git", "TestRevisionAndBranchReadsReportRepositoryState"},
	"GitPath":         {"internal/git", "TestRevisionAndBranchReadsReportRepositoryState"},
	"Branches":        {"internal/git", "TestBranchesReportsEveryLocalBranchShortName"},
	"BranchExists":    {"internal/git", "TestWorktreeRegistrationRoundTrip"},
	"ValidateRefName": {"internal/git", "TestValidateRefNameAcceptsSlugsAndRejectsMalformedNames"},
	"Ancestor":        {"internal/git", "TestAncestorAnswersTheFullTruthTable"},

	// Worktree and branch lifecycle.
	"WorktreeAdd":    {"internal/git", "TestWorktreeRegistrationRoundTrip"},
	"WorktreeRemove": {"internal/git", "TestWorktreeRegistrationRoundTrip"},
	"WorktreeList":   {"internal/git", "TestWorktreeRegistrationRoundTrip"},
	"WorktreePrune":  {"internal/git", "TestWorktreePruneRetiresAnAbandonedRegistration"},
	"BranchDelete":   {"internal/git", "TestWorktreeRegistrationRoundTrip"},

	// Integration.
	"MergeBase":        {"internal/git", "TestRangeNativeReadOperations"},
	"MergeFastForward": {"internal/git", "TestMergeEntrypointsAdvanceAndStageWithoutCommitting"},
	"MergeNoCommit":    {"internal/git", "TestMergeEntrypointsAdvanceAndStageWithoutCommitting"},

	// Commit-range walking.
	"RangeCommits":      {"internal/git", "TestRangeCommitsLinearRangeCarriesChangesAndText"},
	"RangeChangedPaths": {"internal/git", "TestRangeNativeReadOperations"},
	"RangeDiffText":     {"internal/git", "TestRangeNativeReadOperations"},
	"ChangedPaths":      {"internal/git", "TestChangedPathsStaged"},

	// Free entrypoints: each precedes an opened repository or does without one.
	"Open":                      {"internal/git", "TestOpenRepo"},
	"OpenContaining":            {"internal/git", "TestOpenContainingStopsAtMalformedCandidateAndHidesBackendErrors"},
	"ResolveControlRoots":       {"internal/git", "TestControlRootsAgreeWithRegisteredTopology"},
	"ListWorktreeRegistrations": {"internal/git", "TestListWorktreeRegistrationsReportsEveryRegisteredCheckout"},
	"MergeInProgress":           {"internal/git", "TestMergeInProgressPrimaryCheckout"},
	"ParseRange":                {"internal/git", "TestParseRangeTable"},
	"ProjectResidentRoot":       {"cmd/awf", "TestResolveProjectResidentRoot"},
}

// moduleRoot resolves the module root from this package's directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil { // coverage-ignore: Abs fails only when the working directory cannot be resolved, which would fail the test binary first
		t.Fatal(err)
	}
	return root
}

// parseDir parses one package directory, including or excluding its test files.
// The caller supplies the file set so a caller that needs to print a
// declaration back to source shares the positions it was parsed with.
func parseDir(t *testing.T, fset *token.FileSet, dir string, testFiles bool) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil { // coverage-ignore: both parsed directories are checked-in package directories
		t.Fatal(err)
	}
	files := []*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") != testFiles {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil { // coverage-ignore: every file in a compiling package parses
			t.Fatal(err)
		}
		files = append(files, file)
	}
	return files
}

// seamEntrypoints enumerates the seam's exported surface from its source: every
// exported method on *Repo, and every exported package-level function. Deriving
// it rather than listing it is what makes the table complete by construction.
func seamEntrypoints(t *testing.T) []string {
	t.Helper()
	names := []string{}
	for _, file := range parseDir(t, token.NewFileSet(), ".", false) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() {
				continue
			}
			if fn.Recv == nil {
				names = append(names, fn.Name.Name)
				continue
			}
			if receiverTypeName(fn.Recv) == "Repo" {
				names = append(names, fn.Name.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// receiverTypeName returns the bare type name a method is declared on.
func receiverTypeName(recv *ast.FieldList) string {
	if len(recv.List) == 0 { // coverage-ignore: the parser rejects a method with an empty receiver list
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	ident, ok := expr.(*ast.Ident)
	if !ok { // coverage-ignore: no generic receiver exists in this package
		return ""
	}
	return ident.Name
}

// testFunctionBodies maps each test function in a package to its source text,
// so a caller can ask both whether a named test exists and what it references.
func testFunctionBodies(t *testing.T, pkgDir string) map[string]string {
	t.Helper()
	bodies := map[string]string{}
	fset := token.NewFileSet()
	for _, file := range parseDir(t, fset, pkgDir, true) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			var buf bytes.Buffer
			// The BODY only, never the whole declaration: printing the FuncDecl
			// emits the function's own name, and a suite named after its
			// entrypoint would then satisfy a reference check by existing. That
			// was true of 17 of the registrations below before this narrowed.
			if err := printer.Fprint(&buf, fset, fn.Body); err != nil { // coverage-ignore: printing a parsed block to an in-memory buffer has no failure a test can provoke
				t.Fatal(err)
			}
			bodies[fn.Name.Name] = buf.String()
		}
	}
	return bodies
}

// TestEveryEntrypointHasAContractSuite is the completeness half of
// pinned-entrypoint-semantics: the contract suites prove behaviour, and this
// proves every entrypoint has one. An entrypoint added without a suite fails
// here, which is the only mechanism that keeps "every entrypoint is pinned"
// true as the seam grows.
func TestEveryEntrypointHasAContractSuite(t *testing.T) {
	t.Parallel()
	entrypoints := seamEntrypoints(t)
	if len(entrypoints) == 0 {
		t.Fatal("enumerated no entrypoints, so the table proves nothing")
	}
	declared := map[string]bool{}
	for _, name := range entrypoints {
		declared[name] = true
		if _, ok := entrypointSuites[name]; !ok {
			t.Errorf("entrypoint %q has no registered contract suite; pin its semantics and register the suite", name)
		}
	}
	for name := range entrypointSuites {
		if !declared[name] {
			t.Errorf("registered entrypoint %q no longer exists in the seam; remove its entry", name)
		}
	}
}

// TestEveryRegisteredSuiteExercisesItsEntrypoint closes the other direction: a
// registration is only worth anything if the suite it names is real AND
// actually reaches the entrypoint. Without the second half the table is
// satisfied by a typo or by a suite that merely sits nearby, which is not
// hypothetical - "Root" was registered against a suite that called a
// same-named method on the fixture type and never touched the seam's, and the
// table passed.
//
// The reference check is syntactic, so a same-named method on another type
// inside the suite still satisfies it. That residual is why the registration
// comment demands a suite asserting what the entrypoint ANSWERS: this test
// catches the stale and the mistyped, not a reviewer's judgement.
func TestEveryRegisteredSuiteExercisesItsEntrypoint(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	byPackage := map[string]map[string]string{}
	for entrypoint, s := range entrypointSuites {
		bodies, ok := byPackage[s.Package]
		if !ok {
			bodies = testFunctionBodies(t, filepath.Join(root, filepath.FromSlash(s.Package)))
			byPackage[s.Package] = bodies
		}
		body, exists := bodies[s.Test]
		if !exists {
			t.Errorf("entrypoint %q registers %s.%s, which does not exist", entrypoint, s.Package, s.Test)
			continue
		}
		if !strings.Contains(body, entrypoint) {
			t.Errorf("entrypoint %q registers %s.%s, whose body never names it; register the suite that exercises the entrypoint", entrypoint, s.Package, s.Test)
		}
	}
}
