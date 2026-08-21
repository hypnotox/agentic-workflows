package project

import (
	"context"
	"errors"
	"sync"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

var testConfigs sync.Map

func testConfig(state *ProjectState) *config.Config {
	if cached, ok := testConfigs.Load(state); ok {
		return cached.(*config.Config)
	}
	cfg := state.Config()
	if state.Root() != "" {
		loaded, err := config.Load(config.RootDir(state.Root()))
		if err == nil {
			cfg = loaded
		}
	}
	actual, _ := testConfigs.LoadOrStore(state, cfg)
	return actual.(*config.Config)
}

func testState(cfg *config.Config) *ProjectState {
	state, err := newProjectState("", resident.NewRoots("", ""), false, cfg, catalog.Standard, catalog.Standard, nil)
	if err != nil {
		panic(err)
	}
	testConfigs.Store(state, cfg)
	return state
}

func testStateAt(root string) *ProjectState {
	state := testState(&config.Config{})
	out, err := newProjectState(root, resident.NewRoots(root, root), false, testConfig(state), catalog.Standard, catalog.Standard, state.Targets())
	if err != nil {
		panic(err)
	}
	testConfigs.Store(out, testConfig(state))
	return out
}
func testStateWith(state *ProjectState, root string, roots resident.Roots, nested bool, selected, complete *catalog.Catalog, targets []Target) *ProjectState {
	out, err := newProjectState(root, roots, nested, testConfig(state), selected, complete, targets)
	if err != nil {
		panic(err)
	}
	testConfigs.Store(out, testConfig(state))
	return out
}

func testRepo(state *ProjectState) *awfgit.Repo {
	repo, _, err := awfgit.OpenContaining(state.Root())
	if err != nil && !errors.Is(err, awfgit.ErrNotARepository) {
		return nil
	}
	return repo
}

func syncProject(state *ProjectState) error {
	cfg := testConfig(state)
	_, found, err := manifest.LoadOptional(lockPath(state.Root()))
	if err != nil {
		return err
	}
	if !found {
		_, _, _, err = InitializeReport(state, cfg, InitAuthority{InitializedWithVersion: Version})
		return err
	}
	_, _, _, err = SyncReport(state, cfg)
	return err
}

func renderAll(state *ProjectState) ([]RenderedFile, error) {
	op, err := outputPlan(operationInputs(state, testConfig(state)))
	if err != nil {
		return nil, err
	}
	return op.writeFiles(), nil
}

func checkStagedProject(state *ProjectState, ctx context.Context) (CurrentStateReport, error) {
	return checkStaged(state.Root(), testRepo(state), ctx)
}
func checkStagedDriftProject(state *ProjectState, ctx context.Context) ([]manifest.Drift, error) {
	return checkStagedDrift(newRenderInputs(state, testConfig(state), nil), testRepo(state), ctx)
}
func outputPlanProject(state *ProjectState) (*OutputPlan, error) {
	return outputPlan(operationInputs(state, testConfig(state)))
}
func syncReportProject(state *ProjectState) ([]Backup, []Change, []string, error) {
	return SyncReport(state, testConfig(state))
}
func initializeReportProject(state *ProjectState, seed InitAuthority) ([]Backup, []Change, []string, error) {
	return InitializeReport(state, testConfig(state), seed)
}
func checkReportProject(state *ProjectState, ctx context.Context) (CheckReport, error) {
	return BuildCheckReport(state, testConfig(state), testRepo(state), ctx)
}
func configReferenceProject(state *ProjectState) (ConfigReference, error) {
	return BuildConfigReference(state, testConfig(state))
}
func initCollisionsProject(state *ProjectState) ([]string, error) {
	return InitCollisions(state, testConfig(state))
}
func plannedOutputsProject(state *ProjectState) ([]string, error) {
	return PlannedOutputs(state, testConfig(state))
}
func advisoryNotesProject(state *ProjectState) ([]string, error) {
	return AdvisoryNotes(state, testConfig(state))
}
func contextStateProject(state *ProjectState, ctx context.Context) (ContextState, error) {
	return BuildContextState(state, testRepo(state), ctx)
}
func checkCurrentStateProject(state *ProjectState, ctx context.Context) (CurrentStateReport, error) {
	return CheckCurrentState(state.Root(), testRepo(state), ctx)
}
func numberPendingADRsProject(state *ProjectState, slugs []string) (NumberingReport, error) {
	return NumberPendingADRs(state, testConfig(state), slugs)
}
func renderResidentMarkerProject(state *ProjectState, name string) (RenderedFile, error) {
	return RenderResidentMarker(state, testConfig(state), name)
}
func newADRProject(state *ProjectState, ctx context.Context, title string) (string, error) {
	return NewADR(state.Root(), testConfig(state), testRepo(state), ctx, title)
}
func newPitfallProject(state *ProjectState, title string) (presentation.Document, error) {
	return NewPitfall(state.Root(), title)
}
func auditProject(state *ProjectState, ctx context.Context, base, head string) ([]audit.Finding, int, error) {
	return Audit(state.Root(), testConfig(state), ctx, base, head)
}
func checkCommitAuthorizationProject(state *ProjectState, ctx context.Context, msg commitmsg.Message) (CommitAuthorizationResult, error) {
	return CheckCommitAuthorization(state.Root(), testRepo(state), ctx, msg)
}
func readPlanProject(state *ProjectState, name, selector string) ([]byte, error) {
	return ReadPlan(state.Root(), name, selector)
}
func queryTopicProject(state *ProjectState, ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	return QueryTopic(state.Root(), testRepo(state), ctx, selector, opts)
}

func testTargets(state *ProjectState) []Target { return state.Targets() }
func setTestTargets(state *ProjectState, targets []Target) *ProjectState {
	return testStateWith(state, state.Root(), state.roots(), state.nested(), state.catalog(), state.completeCatalog(), targets)
}
func setTestRoots(state *ProjectState, roots resident.Roots) *ProjectState {
	return testStateWith(state, state.Root(), roots, state.nested(), state.catalog(), state.completeCatalog(), state.Targets())
}
func renderInputsForTest(state *ProjectState) renderInputs {
	return newRenderInputs(state, testConfig(state), filesystemProjectReader{root: state.Root()})
}
