package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/referencecheck"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

var testConfigs sync.Map

const crefYAML = "prefix: example\nintegrationBranch: main\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\n"
const domainCfg = "prefix: example\nintegrationBranch: main\ndomains: [rendering]\n"
const csYAML = "prefix: example\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n"
const csRuleTopic = "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\n"

func loadTestSession(ctx context.Context, root string) (*Session, error) {
	repo, _, err := awfgit.OpenContaining(root)
	if err != nil {
		if !errors.Is(err, awfgit.ErrNotARepository) {
			return nil, err
		}
		return NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot).Load(ctx, root)
	}
	return NewLoader(config.Load, catalog.Standard, awfgit.ProjectResidentRoot, repo).Load(ctx, root)
}

func csRepo(t *testing.T, cfg string, files map[string]string) *Session {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, repo.Root(), cfg)
	if err := (&manifest.Lock{AWFVersion: Version, SchemaVersion: 50, Files: map[string]manifest.Entry{}}).Save(lockPath(repo.Root())); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(repo.Root(), rel), body)
	}
	state, err := loadTestSession(testContext(t), repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func syncedProject(t *testing.T, configYAML string, files map[string]string) (string, *Session) {
	t.Helper()
	root := scaffoldFiles(t, configYAML, files)
	state, err := loadTestSession(testContext(t), root)
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
	state, err := loadTestSession(testContext(t), scaffold(t, configYAML))
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
	state, err := loadTestSession(testContext(t), scaffold(t, sampleYAML))
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

func testConfig(state *Session) *config.Config {
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

func testState(cfg *config.Config) *Session {
	state, err := newSession("", resident.NewRoots("", ""), false, cfg, catalog.Standard, nil, nil, publisher.NewFilesystemReader(""))
	if err != nil {
		panic(err)
	}
	testConfigs.Store(state, cfg)
	return state
}

func testStateAt(root string) *Session {
	state := testState(&config.Config{})
	out, err := newSession(root, resident.NewRoots(root, root), false, testConfig(state), catalog.Standard, state.Targets(), testRepo(state), publisher.NewFilesystemReader(root))
	if err != nil {
		panic(err)
	}
	testConfigs.Store(out, testConfig(state))
	return out
}
func testStateWith(state *Session, root string, roots resident.Roots, nested bool, selected *catalog.Catalog, targets []Target) *Session {
	out, err := newSession(root, roots, nested, testConfig(state), selected, targets, testRepo(state), publisher.NewFilesystemReader(root))
	if err != nil {
		panic(err)
	}
	testConfigs.Store(out, testConfig(state))
	return out
}

func testRepo(state *Session) *awfgit.Repo {
	repo, _, err := awfgit.OpenContaining(state.Root())
	if err != nil && !errors.Is(err, awfgit.ErrNotARepository) {
		return nil
	}
	return repo
}

func testPlan(state *Session) (outputplan.Plan, error) {
	return testPublisher(operationInputs(state, testConfig(state))).Plan()
}

func syncProject(state *Session) error {
	pub := testPublisher(operationInputs(state, testConfig(state)))
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
func renderAll(state *Session) ([]RenderedFile, error) {
	plan, err := testPlan(state)
	if err != nil {
		return nil, err
	}
	return planWriteFiles(&plan), nil
}
func outputPlanProject(state *Session) (*OutputPlan, error) {
	return outputPlan(operationInputs(state, testConfig(state)))
}
func projectResult(result publisher.Result, err error) ([]Backup, []Change, []string, error) {
	return result.Backups(), result.Changes(), result.Pruned(), err
}
func syncReportProject(state *Session) ([]Backup, []Change, []string, error) {
	return projectResult(testPublisher(operationInputs(state, testConfig(state))).SyncLeased(context.Background(), nil))
}
func initializeReportProject(state *Session, seed InitAuthority) ([]Backup, []Change, []string, error) {
	return projectResult(testPublisher(operationInputs(state, testConfig(state))).Initialize(seed))
}
func checkReportProject(state *Session, ctx context.Context) (CheckReport, error) {
	operation := testPublisher(operationInputs(state, testConfig(state)))
	plan, err := operation.Plan()
	if err != nil {
		return CheckReport{}, err
	}
	pitfalls, err := operation.Pitfalls()
	if err != nil {
		return CheckReport{}, err
	}
	skills, err := operation.EffectiveSkills()
	if err != nil {
		return CheckReport{}, err
	}
	generated, err := operation.GeneratedOutput()
	if err != nil {
		return CheckReport{}, err
	}
	glossary, err := operation.Glossary()
	if err != nil {
		return CheckReport{}, err
	}
	return BuildCheckReport(state, testConfig(state), testRepo(state), ctx, plan, pitfalls, skills, generated, glossary)
}
func configReferenceProject(state *Session) (publisher.ConfigReference, error) {
	return testPublisher(operationInputs(state, testConfig(state))).BuildConfigReference()
}
func initCollisionsProject(state *Session) ([]string, error) {
	return testPublisher(operationInputs(state, testConfig(state))).InitCollisions()
}
func plannedOutputsProject(state *Session) ([]string, error) {
	plan, err := testPlan(state)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(plan.Outputs()))
	for _, output := range plan.Outputs() {
		paths = append(paths, output.Path())
	}
	return paths, nil
}
func advisoryNotesProject(state *Session) ([]string, error) {
	operation := testPublisher(operationInputs(state, testConfig(state)))
	plan, err := operation.Plan()
	if err != nil {
		return nil, err
	}
	glossary, err := operation.Glossary()
	if err != nil {
		return nil, err
	}
	return AdvisoryNotes(state, testConfig(state), plan, glossary)
}

func newPitfallProject(state *Session, title string) (presentation.Document, error) {
	return NewPitfall(state.Root(), title)
}
func auditProject(state *Session, ctx context.Context, base, head string) ([]audit.Finding, int, error) {
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
func setTestRoots(state *Session, roots resident.Roots) *Session {
	return testStateWith(state, state.Root(), roots, state.Nested(), state.catalog(), state.Targets())
}
func renderInputsForTest(state *Session) renderInputs {
	return newRenderInputs(state, testConfig(state), filesystemProjectReader{root: state.Root()})
}

func testOutputPlan(rendered map[string]RenderedFile) outputplan.Plan {
	nodes := make([]outputplan.Node, 0, len(rendered))
	for _, file := range rendered {
		output := outputplan.NewOutput(outputplan.OutputSpec{Path: file.Path, Content: file.Content, TemplateID: file.TemplateID, TemplateHash: file.TemplateHash, ConfigHash: file.ConfigHash, Policy: file.Policy, Assembled: file.assembled, Kind: file.kind, Artifact: file.artifact, PartVarRefs: file.partVarRefs})
		nodes = append(nodes, outputplan.NewNode(outputplan.NodeSpec{Path: file.Path, Output: &output}))
	}
	return outputplan.New(nodes)
}

func testReferenceDrift(result checkresult.Result) []manifest.Drift {
	out := make([]manifest.Drift, 0, len(result.Findings()))
	for _, finding := range result.Findings() {
		out = append(out, manifest.Drift{Path: finding.Evidence.Path, Kind: finding.Evidence.Kind, Detail: finding.Evidence.Detail})
	}
	return out
}

func checkDeadRefs(p renderInputs, files []RenderedFile) []manifest.Drift {
	rendered := make(map[string]RenderedFile, len(files))
	for _, file := range files {
		rendered[file.Path] = file
	}
	result, err := referencecheck.Check(testOutputPlan(rendered), p.cfg.Prefix, nil, nil, func(path string) bool { _, err := os.Stat(filepath.Join(p.root(), path)); return err == nil })
	if err != nil {
		panic(err)
	}
	return testReferenceDrift(result)
}

func checkDeadSkillRefs(p renderInputs, files []RenderedFile, effective map[string]bool) []manifest.Drift {
	rendered, known := make(map[string]RenderedFile, len(files)), map[string]bool{}
	for _, file := range files {
		rendered[file.Path] = file
	}
	for name := range p.catalog().Skills {
		known[name] = true
	}
	result, err := referencecheck.Check(testOutputPlan(rendered), p.cfg.Prefix, effective, known, func(string) bool { return true })
	if err != nil {
		panic(err)
	}
	return testReferenceDrift(result)
}

func checkLockedDrift(roots resident.Roots, lock *manifest.Lock, rendered map[string]RenderedFile, tracking []manifest.Drift) []manifest.Drift {
	findings := make([]checkresult.Finding, 0, len(tracking))
	for _, item := range tracking {
		if item.Kind == "untracked" {
			detail := item.Detail
			if detail == "" {
				detail = "untracked"
			}
			findings = append(findings, checkresult.Finding{Rank: severity.Error, Property: propertyReproducibility, Evidence: checkresult.Evidence{Kind: item.Kind, Path: item.Path, Detail: detail}})
		}
	}
	tracked, _ := checkresult.New(findings, nil)
	result, err := generatedcheck.Locked(false, lock, testOutputPlan(rendered), func(path string) ([]byte, error) {
		return os.ReadFile(roots.ResolveOutput(path))
	}, tracked)
	if err != nil {
		panic(err)
	}
	drift := make([]manifest.Drift, 0, len(result.Findings()))
	for _, finding := range result.Findings() {
		drift = append(drift, manifest.Drift{Path: finding.Evidence.Path, Kind: finding.Evidence.Kind, Detail: finding.Evidence.Detail})
	}
	return drift
}
