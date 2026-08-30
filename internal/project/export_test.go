package project

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

var testConfigs sync.Map

const crefYAML = "prefix: example\nintegrationBranch: main\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\n"
const domainCfg = "prefix: example\nintegrationBranch: main\ndomains: [rendering]\n"

func syncedProject(t *testing.T, configYAML string, files map[string]string) (string, *ProjectState) {
	t.Helper()
	root := scaffoldFiles(t, configYAML, files)
	state, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(state); err != nil {
		t.Fatal(err)
	}
	return root, state
}

func explorationFixtureConfig(string) string {
	return "prefix: example\nintegrationBranch: main\n"
}
func explorationRenderedByPath(t *testing.T, configYAML string) map[string]string {
	t.Helper()
	state, err := Open(testContext(t), scaffold(t, configYAML))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(state)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, file := range files {
		out[file.Path] = file.Content
	}
	return out
}
func renderPiExtensionFile(t *testing.T, name string) string {
	t.Helper()
	out := explorationRenderedByPath(t, explorationFixtureConfig("pi"))[".pi/extensions/"+name]
	if out == "" {
		t.Fatalf("missing Pi extension %s", name)
	}
	return out
}
func assertV3ADRTemplatePublicationSafe(t *testing.T) {
	t.Helper()
	state, err := Open(testContext(t), scaffold(t, sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Path == "docs/decisions/template.md" {
			if !strings.Contains(file.Content, "format: current-state-v4") || !strings.Contains(file.Content, "- YYYY-MM-DD: Proposed") || strings.Contains(file.Content, "<no value>") {
				t.Fatalf("V4 lifecycle template is not publication-safe:\n%s", file.Content)
			}
			return
		}
	}
	t.Fatal("missing ADR template")
}
func writeADR(t *testing.T, root, name, body string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", name), body)
}
func writeProjectTopic(t *testing.T, root string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata/rendering", "contracts.yaml"), "title: Contracts\nsummary: Current Contracts contracts.\npaths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering", "contracts", "current-state.md"), "<!-- awf:comment author note -->\nAuthored raw {{ .value }}.\n\n## Claims\n\n### `rule: stable`\nStable behavior.\n")
}
func topicProject(t *testing.T) string {
	t.Helper()
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: ./x gate\ndomains: [rendering]\n", map[string]string{"domains/rendering.yaml": "paths: [\"internal/**\"]\n"})
	writeADR(t, root, "0001-topic.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Topic"), testsupport.WithBody("## Decision\n\n1. Topic.\n")))
	return root
}
func hasDrift(drift []manifest.Drift, path, kind string) bool {
	for _, item := range drift {
		if item.Path == path && item.Kind == kind {
			return true
		}
	}
	return false
}

func difference(a, b []string) []string {
	set := map[string]bool{}
	for _, value := range b {
		set[value] = true
	}
	var out []string
	for _, value := range a {
		if !set[value] {
			out = append(out, value)
		}
	}
	return out
}

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

func testPlan(state *ProjectState) (outputplan.Plan, error) {
	return testPublisher(operationInputs(state, testConfig(state))).Plan()
}

func syncProject(state *ProjectState) error {
	pub := publisher.New(state.OutputState(), testConfig(state), publisher.NewFilesystemReader(state.Root()), Version)
	_, found, err := manifest.LoadOptional(lockPath(state.Root()))
	if err != nil {
		return err
	}
	if !found {
		_, err = pub.Initialize(publisher.InitAuthority{InitializedWithVersion: Version})
	} else {
		_, err = pub.SyncLeased(context.Background(), nil)
	}
	return err
}
func renderAll(state *ProjectState) ([]RenderedFile, error) {
	plan, err := testPlan(state)
	if err != nil {
		return nil, err
	}
	return planWriteFiles(&plan), nil
}
func checkStagedProject(state *ProjectState, ctx context.Context) (CurrentStateReport, error) {
	return currentstatecoord.CheckStaged(state.Root(), testRepo(state), ctx)
}
func checkStagedDriftProject(state *ProjectState, ctx context.Context) ([]manifest.Drift, error) {
	prep, err := PrepareStagedOutputState(ctx, state.Root())
	if err != nil {
		return nil, err
	}
	plan, err := publisher.New(prep.State, prep.Config, prep.Reader, Version).Plan()
	if err != nil {
		return nil, err
	}
	return CheckStagedDrift(prep, plan)
}
func outputPlanProject(state *ProjectState) (*OutputPlan, error) {
	return outputPlan(operationInputs(state, testConfig(state)))
}
func projectResult(result publisher.Result, err error) ([]Backup, []Change, []string, error) {
	return result.Backups(), result.Changes(), result.Pruned(), err
}
func syncReportProject(state *ProjectState) ([]Backup, []Change, []string, error) {
	return projectResult(publisher.New(state.OutputState(), testConfig(state), publisher.NewFilesystemReader(state.Root()), Version).SyncLeased(context.Background(), nil))
}
func initializeReportProject(state *ProjectState, seed InitAuthority) ([]Backup, []Change, []string, error) {
	return projectResult(publisher.New(state.OutputState(), testConfig(state), publisher.NewFilesystemReader(state.Root()), Version).Initialize(seed))
}
func checkReportProject(state *ProjectState, ctx context.Context) (CheckReport, error) {
	prepared, err := testPublisher(operationInputs(state, testConfig(state))).Prepare()
	if err != nil {
		return CheckReport{}, err
	}
	semantics := OperationSemantics{ADRs: prepared.ADRs(), Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(), EffectiveSkills: prepared.EffectiveSkills(), GeneratedOutput: prepared.GeneratedOutput(), Glossary: prepared.Glossary()}
	return BuildCheckReport(state, testConfig(state), testRepo(state), ctx, prepared.Plan(), semantics)
}
func configReferenceProject(state *ProjectState) (publisher.ConfigReference, error) {
	return testPublisher(operationInputs(state, testConfig(state))).BuildConfigReference()
}
func initCollisionsProject(state *ProjectState) ([]string, error) {
	return publisher.New(state.OutputState(), testConfig(state), publisher.NewFilesystemReader(state.Root()), Version).InitCollisions()
}
func plannedOutputsProject(state *ProjectState) ([]string, error) {
	plan, err := testPlan(state)
	if err != nil {
		return nil, err
	}
	return plan.Paths(), nil
}
func advisoryNotesProject(state *ProjectState) ([]string, error) {
	prepared, err := testPublisher(operationInputs(state, testConfig(state))).Prepare()
	if err != nil {
		return nil, err
	}
	semantics := OperationSemantics{ADRs: prepared.ADRs(), Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(), EffectiveSkills: prepared.EffectiveSkills(), GeneratedOutput: prepared.GeneratedOutput(), Glossary: prepared.Glossary()}
	return AdvisoryNotes(state, testConfig(state), prepared.Plan(), semantics)
}

func checkCurrentStateProject(state *ProjectState, ctx context.Context) (CurrentStateReport, error) {
	return currentstatecoord.CheckWorking(state.Root(), testRepo(state), ctx)
}
func newADRProject(state *ProjectState, ctx context.Context, title string) (result string, returnErr error) {
	lease, err := filesystem.AcquireTrackedLease(ctx, state.Root())
	if err != nil {
		return "", err
	}
	defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
	files, err := filesystem.Open(state.Root())
	if err != nil {
		return "", err
	}
	defer func() { returnErr = errors.Join(returnErr, files.Close()) }()
	return NewADRLeased(state.Root(), testConfig(state), testRepo(state), ctx, title, lease, files)
}
func newPitfallProject(state *ProjectState, title string) (presentation.Document, error) {
	return NewPitfall(state.Root(), title)
}
func auditProject(state *ProjectState, ctx context.Context, base, head string) ([]audit.Finding, int, error) {
	generated := map[string]bool{}
	lock, _, err := manifest.LoadOptional(config.LockPath(state.Root()))
	if err != nil {
		return nil, 0, err
	}
	if lock != nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	return audit.Run(ctx, state.Root(), base, head, audit.Inputs{
		Settings:       audit.Resolve(config.AuditScopes(testConfig(state).Audit)),
		GeneratedPaths: generated,
		DocsDir:        config.DocsDir,
	})
}
func setTestRoots(state *ProjectState, roots resident.Roots) *ProjectState {
	return testStateWith(state, state.Root(), roots, state.nested(), state.catalog(), state.completeCatalog(), state.Targets())
}
func renderInputsForTest(state *ProjectState) renderInputs {
	return newRenderInputs(state, testConfig(state), filesystemProjectReader{root: state.Root()})
}

func checkLockedDrift(roots resident.Roots, lock *manifest.Lock, rendered map[string]RenderedFile, tracking []manifest.Drift) []manifest.Drift {
	findings := checkLockedFiles(roots, lock, rendered, tracking)
	drift := make([]manifest.Drift, 0, len(findings))
	for _, finding := range findings {
		drift = append(drift, finding.Drift)
	}
	return drift
}
