package project

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
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
	"CheckStaged",
	"CheckStagedDrift",
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
	"advisoryNotesWithState",
	"agentTID",
	"artifactConfigHash",
	"backupFileConfined",
	"buildClaimedModel",
	"catalog",
	"checkADRRelatedLinks",
	"checkDeadRefs",
	"checkDeadSkillRefs",
	"checkGeneratedTracking",
	"checkGlossary",
	"checkKindAgainstCatalog",
	"checkLockedFiles",
	"checkPendingADRs",
	"checkPitfalls",
	"checkPlans",
	"checkTagVocabulary",
	"checkWithTrackingState",
	"commitScopeSentence",
	"commitScopeTable",
	"commitScopesDisplay",
	"compatPitfallCorpus",
	"completeCatalog",
	"configReferenceData",
	"configReferenceRows",
	"consumedParts",
	"crefRel",
	"currentValueResolvers",
	"data",
	"dataKeyRowsTyped",
	"decisionsDir",
	"declaredSections",
	"deriveOperationStateWithPitfalls",
	"docOutPath",
	"docTID",
	"documentMapDocs",
	"effectiveSkills",
	"encodeAgent",
	"fullProfile",
	"generateConfigReference",
	"generateDomainDocs",
	"generateIndexMD",
	"generateLocalDocs",
	"generatePitfallLeaves",
	"generateTopicDocs",
	"gitRepo",
	"glossaryTersenessNotes",
	"headTreeAndLock",
	"indexCurrentState",
	"indexTree",
	"layout",
	"liveTemplateEncoders",
	"liveTemplateIDs",
	"loadPitfallCorpus",
	"localDocumentMapDocs",
	"lockPath",
	"newPitfallWith",
	"numberingCorpus",
	"observeRenderInputs",
	"onIntegrationBranch",
	"openSyncFilesystems",
	"outputPlanWithPitfalls",
	"partRel",
	"placeholderRegistry",
	"planCommitScopeNotes",
	"planSections",
	"projectTreeReader",
	"renderAllBase",
	"renderKind",
	"renderResidentMarker",
	"renderTarget",
	"resolvedDocs",
	"skillTID",
	"substitutePlaceholders",
	"sweepConfigTree",
	"syncReport",
	"syncReportWithPitfalls",
	"tagHealthNotes",
	"targetOutputDeclarations",
	"templateSourceRoot",
	"templateSourceRootMarker",
	"unsetVarNotes",
	"unusedDataDrift",
	"unusedVarDrift",
	"validateAgainstCatalog",
	"validateLiveTemplates",
	"validateLocalDocOutputCollisions",
	"validateTemplateSources",
	"varState",
	"workingCurrentState",
	"workingTree",
}

var allowedProjectFacadeConsumers = []string{
	"cmd/awf/adr.go:func runADR",
	"cmd/awf/audit.go:func runAudit",
	"cmd/awf/checkrepo.go:func productionRepoCheckDependencies",
	"cmd/awf/checkrepo.go:type repoCheckDependencies",
	"cmd/awf/checkrepo.go:type repoCheckInputs",
	"cmd/awf/commitgate.go:func defaultCommitGateDependencies",
	"cmd/awf/commitgate.go:func openCommitGateProjectFromDisk",
	"cmd/awf/commitgate.go:type commitGateDependencies",
	"cmd/awf/config.go:func runConfig",
	"cmd/awf/context.go:func runContext",
	"cmd/awf/context.go:func runUncovered",
	"cmd/awf/effort.go:func openEffortComposition",
	"cmd/awf/init.go:func probeCollisions",
	"cmd/awf/init.go:func renderInitOutcome",
	"cmd/awf/init.go:func runInitWithProjectLoader",
	"cmd/awf/list_add.go:func openDomainProject",
	"cmd/awf/list_add.go:func runList",
	"cmd/awf/list_add.go:func scaffoldDomainCurrentState",
	"cmd/awf/list_add.go:type domainDependencies",
	"cmd/awf/new.go:func newADR",
	"cmd/awf/new.go:func newPitfall",
	"cmd/awf/new.go:func newPlan",
	"cmd/awf/new.go:func newTopic",
	"cmd/awf/new.go:func productionLocalDocDependencies",
	"cmd/awf/new.go:type localDocDependencies",
	"cmd/awf/read.go:func runReadPlan",
	"cmd/awf/sync.go:func syncMutation",
	"cmd/awf/topic.go:func runTopic",
	"cmd/awf/upgrade_presentation.go:func productionUpgradeSyncDependencies",
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

func facadeImportNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != projectImportPath {
			continue
		}
		name := "project"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = true
	}
	return names
}

func usesProjectFacade(node ast.Node, importNames map[string]bool) bool {
	used := false
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "Open" && selector.Sel.Name != "Project") {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && importNames[ident.Name] {
			used = true
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
			imports := facadeImportNames(file)
			for _, declaration := range file.Decls {
				switch decl := declaration.(type) {
				case *ast.FuncDecl:
					if pkg.PkgPath == projectImportPath && facadeReceiverName(pkg.TypesInfo, decl.Recv) == "Project" {
						methods = append(methods, decl.Name.Name)
					}
					if pkg.PkgPath != projectImportPath && usesProjectFacade(decl, imports) {
						consumers = append(consumers, filename+":func "+decl.Name.Name)
					}
				case *ast.GenDecl:
					if pkg.PkgPath == projectImportPath {
						continue
					}
					for _, spec := range decl.Specs {
						if !usesProjectFacade(spec, imports) {
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

func TestProjectFacadeFrozenAllowlist(t *testing.T) {
	production, root := loadFacadePackages(t, nil)
	methods, consumers := projectFacadeInventory(production, root)
	if !slices.Equal(methods, allowedProjectFacadeMethods) {
		t.Errorf("Project receiver method allowlist changed:\ngot  %#v\nwant %#v", methods, allowedProjectFacadeMethods)
	}
	if !slices.Equal(consumers, allowedProjectFacadeConsumers) {
		t.Errorf("Project production consumer allowlist changed:\ngot  %#v\nwant %#v", consumers, allowedProjectFacadeConsumers)
	}

	methodFixture := filepath.Join(root, filepath.FromSlash("internal/project/facade_method_mutation_fixture.go"))
	consumerFixture := filepath.Join(root, filepath.FromSlash("cmd/awf/facade_consumer_mutation_fixture.go"))
	mutated, mutatedRoot := loadFacadePackages(t, map[string][]byte{
		methodFixture:   []byte("package project\n\nfunc (p *Project) facadeMutation() {}\n"),
		consumerFixture: []byte("package main\n\nimport (\n\t\"context\"\n\t\"github.com/hypnotox/agentic-workflows/internal/project\"\n)\n\nfunc facadeMutation(ctx context.Context) { _, _ = project.Open(ctx, \".\") }\n"),
	})
	mutatedMethods, mutatedConsumers := projectFacadeInventory(mutated, mutatedRoot)
	if !slices.Contains(mutatedMethods, "facadeMutation") {
		t.Error("an added Project receiver method escaped the facade allowlist detector")
	}
	if !slices.Contains(mutatedConsumers, "cmd/awf/facade_consumer_mutation_fixture.go:func facadeMutation") {
		t.Error("an added Project production consumer escaped the facade allowlist detector")
	}
}
