package project

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const projectImportPath = "github.com/hypnotox/agentic-workflows/internal/project"

var allowedProjectFacadeMethods = []string{
	"AdvisoryNotes",
	"Audit",
	"CheckCommitAuthorization",
	"CheckCurrentState",
	"CheckReport",
	"CommitPolicyPresentation",
	"ConfigReferenceModel",
	"ContextState",
	"InitCollisions",
	"InitializeReport",
	"ListDocument",
	"NewADR",
	"NewPitfall",
	"NewPlan",
	"NumberPendingADRs",
	"OutputPlan",
	"PlannedOutputs",
	"PreflightLocalDoc",
	"QueryTopic",
	"ReadPlan",
	"RenderResidentMarker",
	"SyncReport",
	"VerifyCommitPolicy",
}
var allowedProjectFacadeConsumers = []string{
	"cmd/awf/adr.go:func runADR",
	"cmd/awf/audit.go:func runAudit",
	"cmd/awf/checkrepo.go:func collectCheckRepoWithPlanNotes",
	"cmd/awf/checkrepo.go:func collectRepoCheckSelectionWithPlanNotes",
	"cmd/awf/checkrepo.go:func productionRepoCheckDependencies",
	"cmd/awf/checkrepo.go:func repoCheckSystem",
	"cmd/awf/checkrepo.go:func runCheckDrift",
	"cmd/awf/checkrepo.go:func runCheckState",
	"cmd/awf/checkrepo.go:func runRepoCheckSelection",
	"cmd/awf/checkrepo.go:func runRepoCheckSelectionWithPlanNotes",
	"cmd/awf/checkrepo.go:type repoCheckDependencies",
	"cmd/awf/checkrepo.go:type repoCheckInputs",
	"cmd/awf/commitgate.go:func defaultCommitGateDependencies",
	"cmd/awf/commitgate.go:func openCommitGateProjectFromDisk",
	"cmd/awf/commitgate.go:func runCommitGate",
	"cmd/awf/commitgate.go:func runCommitGateWithDependencies",
	"cmd/awf/commitgate.go:type commitGateDependencies",
	"cmd/awf/config.go:func runConfig",
	"cmd/awf/context.go:func runContext",
	"cmd/awf/context.go:func runUncovered",
	"cmd/awf/effort.go:func openEffortComposition",
	"cmd/awf/init.go:func probeCollisions",
	"cmd/awf/init.go:func renderInitOutcome",
	"cmd/awf/init.go:func runInitWithProjectLoader",
	"cmd/awf/list_add.go:func openDomainProject",
	"cmd/awf/list_add.go:func productionDomainDependencies",
	"cmd/awf/list_add.go:func runList",
	"cmd/awf/list_add.go:func runNewDomain",
	"cmd/awf/list_add.go:func runNewDomainWith",
	"cmd/awf/list_add.go:func runRemoveDomain",
	"cmd/awf/list_add.go:func runRemoveDomainWith",
	"cmd/awf/list_add.go:func scaffoldDomainCurrentState",
	"cmd/awf/list_add.go:type domainDependencies",
	"cmd/awf/memorygate.go:func runMemoryGate",
	"cmd/awf/new.go:func newADR",
	"cmd/awf/new.go:func newDoc",
	"cmd/awf/new.go:func newDocWith",
	"cmd/awf/new.go:func newPitfall",
	"cmd/awf/new.go:func newPlan",
	"cmd/awf/new.go:func newTopic",
	"cmd/awf/new.go:func productionLocalDocDependencies",
	"cmd/awf/new.go:type localDocDependencies",
	"cmd/awf/prosegate.go:func runProseGate",
	"cmd/awf/read.go:func runReadPlan",
	"cmd/awf/sync.go:func runSyncPrinting",
	"cmd/awf/sync.go:func syncMutation",
	"cmd/awf/topic.go:func runTopic",
	"cmd/awf/upgrade_presentation.go:func productionUpgradeSyncDependencies",
	"cmd/awf/upgrade_presentation.go:func upgradeSyncMutation",
	"cmd/awf/upgrade_presentation.go:func upgradeSyncMutationWith",
	"cmd/awf/upgrade_presentation.go:type upgradeSyncDependencies",
}

func loadFacadePackages(t *testing.T, overlay map[string][]byte) ([]*packages.Package, string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{Dir: root, Mode: projectPackageMode, Overlay: overlay}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
	}
	return pkgs, root
}

func facadeReceiverName(info *types.Info, recv *ast.FieldList) string {
	if recv == nil || len(recv.List) != 1 {
		return ""
	}
	typ := info.TypeOf(recv.List[0].Type)
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != projectImportPath {
		return ""
	}
	return named.Obj().Name()
}

func containsProjectFacadeType(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil {
		return false
	}
	typ = types.Unalias(typ)
	if seen[typ] {
		return false
	}
	seen[typ] = true
	switch value := typ.(type) {
	case *types.Named:
		if obj := value.Obj(); obj.Pkg() != nil && obj.Pkg().Path() == projectImportPath && obj.Name() == "Project" {
			return true
		}
		return containsProjectFacadeType(value.Underlying(), seen)
	case *types.Pointer:
		return containsProjectFacadeType(value.Elem(), seen)
	case *types.Array:
		return containsProjectFacadeType(value.Elem(), seen)
	case *types.Slice:
		return containsProjectFacadeType(value.Elem(), seen)
	case *types.Map:
		return containsProjectFacadeType(value.Key(), seen) || containsProjectFacadeType(value.Elem(), seen)
	case *types.Chan:
		return containsProjectFacadeType(value.Elem(), seen)
	case *types.Signature:
		return containsProjectFacadeType(value.Params(), seen) || containsProjectFacadeType(value.Results(), seen)
	case *types.Tuple:
		for i := range value.Len() {
			if containsProjectFacadeType(value.At(i).Type(), seen) {
				return true
			}
		}
	case *types.Struct:
		for i := range value.NumFields() {
			if containsProjectFacadeType(value.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Interface:
		value.Complete()
		for i := range value.NumMethods() {
			if containsProjectFacadeType(value.Method(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

func usesProjectFacade(node ast.Node, info *types.Info) bool {
	used := false
	ast.Inspect(node, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			obj := info.Uses[value]
			if obj == nil {
				obj = info.Defs[value]
			}
			if obj != nil {
				if obj.Pkg() != nil && obj.Pkg().Path() == projectImportPath && (obj.Name() == "Open" || obj.Name() == "Project") {
					used = true
				}
				if containsProjectFacadeType(obj.Type(), map[types.Type]bool{}) {
					used = true
				}
			}
		case ast.Expr:
			if containsProjectFacadeType(info.TypeOf(value), map[types.Type]bool{}) {
				used = true
			}
		}
		return true
	})
	return used
}

func projectFacadeInventory(pkgs []*packages.Package, root string) ([]string, []string) {
	var methods []string
	var consumers []string
	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			filename, err := filepath.Rel(root, pkg.CompiledGoFiles[i])
			if err != nil {
				continue
			}
			filename = filepath.ToSlash(filename)
			for _, declaration := range file.Decls {
				switch decl := declaration.(type) {
				case *ast.FuncDecl:
					if pkg.PkgPath == projectImportPath && facadeReceiverName(pkg.TypesInfo, decl.Recv) == "Project" {
						methods = append(methods, decl.Name.Name)
					}
					if pkg.PkgPath != projectImportPath && usesProjectFacade(decl, pkg.TypesInfo) {
						consumers = append(consumers, filename+":func "+decl.Name.Name)
					}
				case *ast.GenDecl:
					if pkg.PkgPath == projectImportPath {
						continue
					}
					for _, spec := range decl.Specs {
						if !usesProjectFacade(spec, pkg.TypesInfo) {
							continue
						}
						switch value := spec.(type) {
						case *ast.TypeSpec:
							consumers = append(consumers, filename+":type "+value.Name.Name)
						case *ast.ValueSpec:
							for _, name := range value.Names {
								consumers = append(consumers, filename+":var "+name.Name)
							}
						}
					}
				}
			}
		}
	}
	sort.Strings(methods)
	sort.Strings(consumers)
	return methods, consumers
}

// facadeProductionViolations applies the production detector and the mutation
// fixtures alike. The compatibility facade may be constructed by Open, but no
// replacement operation may accept or return Project, revive renderProject, or
// be exported before Phase 3 gives it an outside-package owner.
func facadeProductionViolations(pkgs []*packages.Package) []string {
	var violations []string
	for _, pkg := range pkgs {
		if pkg.PkgPath != projectImportPath {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, declaration := range file.Decls {
				if generic, ok := declaration.(*ast.GenDecl); ok {
					for _, spec := range generic.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == "renderProject" {
							violations = append(violations, "renderProject type")
						}
					}
					continue
				}
				fn, ok := declaration.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if ast.IsExported(fn.Name.Name) && strings.HasSuffix(fn.Name.Name, "Operation") {
					violations = append(violations, "exported replacement "+fn.Name.Name)
				}
				if facadeReceiverName(pkg.TypesInfo, fn.Recv) == "Project" || fn.Name.Name == "Open" {
					continue
				}
				if containsProjectFacadeType(pkg.TypesInfo.Defs[fn.Name].Type(), map[types.Type]bool{}) {
					violations = append(violations, "broad Project signature "+fn.Name.Name)
				}
			}
		}
	}
	return violations
}

func TestProjectFacadeFrozenAllowlist(t *testing.T) {
	production, root := loadFacadePackages(t, nil)
	methods, consumers := projectFacadeInventory(production, root)
	if !slices.Equal(methods, allowedProjectFacadeMethods) {
		t.Errorf("Project receiver method allowlist changed:\ngot  %#v\nwant %#v", methods, allowedProjectFacadeMethods)
	}
	if !slices.Equal(consumers, allowedProjectFacadeConsumers) {
		t.Errorf("Project production consumer allowlist changed:\ngot  %#v\nwant %#v", consumers, allowedProjectFacadeConsumers)
	}
	if violations := facadeProductionViolations(production); len(violations) != 0 {
		t.Errorf("facade production violations: %v", violations)
	}

	methodFixture := filepath.Join(root, filepath.FromSlash("internal/project/facade_method_mutation_fixture.go"))
	consumerFixture := filepath.Join(root, filepath.FromSlash("cmd/awf/facade_consumer_mutation_fixture.go"))
	mutated, mutatedRoot := loadFacadePackages(t, map[string][]byte{
		methodFixture:   []byte("package project\n\ntype renderProject struct{}\nfunc (p *Project) facadeMutation() {}\nfunc broadMutation(p *Project) *Project { return p }\nfunc RenderAllOperation() {}\n"),
		consumerFixture: []byte("package main\n\nimport (\n\t\"context\"\n\tp \"github.com/hypnotox/agentic-workflows/internal/project\"\n)\n\nfunc facadeMutation(ctx context.Context) { _, _ = p.Open(ctx, \".\") }\n\nfunc facadeIndirectMutation(inputs repoCheckInputs) { _ = inputs.project.Root }\n"),
	})
	mutatedMethods, mutatedConsumers := projectFacadeInventory(mutated, mutatedRoot)
	if !slices.Contains(mutatedMethods, "facadeMutation") {
		t.Error("an added Project receiver method escaped the facade allowlist detector")
	}
	for _, want := range []string{
		"cmd/awf/facade_consumer_mutation_fixture.go:func facadeMutation",
		"cmd/awf/facade_consumer_mutation_fixture.go:func facadeIndirectMutation",
	} {
		if !slices.Contains(mutatedConsumers, want) {
			t.Errorf("added Project production consumer %s escaped the facade allowlist detector", want)
		}
	}
	for _, want := range []string{"renderProject type", "broad Project signature broadMutation", "exported replacement RenderAllOperation"} {
		if !slices.Contains(facadeProductionViolations(mutated), want) {
			t.Errorf("facade mutation %q escaped production detector", want)
		}
	}
}
