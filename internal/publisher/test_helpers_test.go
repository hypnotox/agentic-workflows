package publisher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type ProjectState = project.ProjectState

var Version = project.Version

func KnownTargets() []string                          { return projectstate.KnownTargets() }
func resolveTargets(names []string) ([]Target, error) { return projectstate.ResolveTargets(names) }

var testConfigs sync.Map
var targetOverrides sync.Map

const defaultFixtureBranch = "master"
const pitfallsCfg = "prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\n"
const debuggingVars = `vars:
  debuggingDoc: ""
  gateCmd: ""
  gateCmdFull: ""
  workflowDoc: ""
`
const sampleYAML = `prefix: example
profile: full
integrationBranch: main
vars:
  testCmd: go test ./...
  gateCmd: make gate
  gateCmdFull: make gate full
`

type snapshotTreeReader struct{ tree *snapshot.Tree }

func (r snapshotTreeReader) ReadFile(path string) ([]byte, bool, error) {
	file, ok := r.tree.Lookup(filepath.ToSlash(path))
	if !ok || !file.Scannable() {
		return nil, false, nil
	}
	return slices.Clone(file.Bytes), true, nil
}
func (r snapshotTreeReader) Paths(prefix string) ([]string, error) {
	var out []string
	for _, file := range r.tree.List() {
		if file.Scannable() && strings.HasPrefix(file.Path, prefix) {
			out = append(out, file.Path)
		}
	}
	return out, nil
}
func csRepo(t *testing.T, cfg string, files map[string]string) *ProjectState {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, repo.Root(), cfg)
	if _, ok := files["docs/decisions/0001-first.md"]; !ok {
		files["docs/decisions/0001-first.md"] = testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First"), testsupport.WithBody("## Context\nx\n## Consequences\nc\n"))
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(repo.Root(), rel), body)
	}
	state, err := Open(testContext(t), repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func mustDeriveTopics(t *testing.T, state *ProjectState) topic.Corpus {
	t.Helper()
	_, _, topics, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	return topics
}
func queryTopicProject(state *ProjectState, ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	repo, _, err := awfgit.OpenContaining(state.Root())
	if err != nil {
		return topic.QueryResult{}, err
	}
	return project.QueryTopic(state.Root(), repo, ctx, selector, opts)
}
func repoRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}
func withLayoutDefaults(data map[string]any) {
	if _, ok := data["skills"]; !ok {
		data["skills"] = map[string]bool{}
	}
	layout, _ := data["layout"].(map[string]any)
	if layout == nil {
		layout = map[string]any{}
		data["layout"] = layout
	}
	if _, ok := layout["docs"]; !ok {
		layout["docs"] = map[string]any{"debugging": "docs/debugging.md", "pitfalls": "docs/pitfalls.md", "roadmap": "docs/roadmap.md"}
	}
	for key, value := range map[string]string{"workflowRef": "docs/workflow.md", "domainsDir": "docs/domains", "maintainableCodeDesign": "docs/maintainable-code-design.md"} {
		if _, ok := layout[key]; !ok {
			layout[key] = value
		}
	}
}
func assertNoLeaks(t *testing.T, out string) {
	t.Helper()
	for _, leak := range []string{"<!-- awf:section", "<!-- awf:end", "<no value>", "{{", "}}"} {
		if strings.Contains(out, leak) {
			t.Errorf("rendered output contains %q", leak)
		}
	}
}
func renderSkillGolden(t *testing.T, skill string, data map[string]any) string {
	return renderGolden(t, "skills/"+skill+"/SKILL.md.tmpl", data)
}
func renderAgentGolden(t *testing.T, name string, data map[string]any) string {
	body := renderGolden(t, "agents/"+name+".md.tmpl", data)
	description, err := render.Execute(catalog.Standard.Agents[name].Description, data, nil, "agent description")
	if err != nil {
		t.Fatal(err)
	}
	out, err := encodeMarkdownAgent(agent{Name: catalog.Standard.Agents[name].Name, Description: description, Body: body})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func pitfallSource(title, extra, body string) string {
	return "---\ntitle: " + title + "\n" + extra + "---\n" + body
}
func lockFile(root string) string   { return filepath.Join(root, ".awf", "awf.lock") }
func configPath(root string) string { return filepath.Join(root, ".awf", "config.yaml") }
func writeADR(t *testing.T, root, name, body string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, "docs", "decisions", name), body)
}
func testTargets(state *ProjectState) []Target {
	if value, ok := targetOverrides.Load(state); ok {
		return append([]Target(nil), value.([]Target)...)
	}
	return state.Targets()
}
func lowerWithTargets(state *projectstate.ProjectState, targets []Target) *projectstate.ProjectState {
	return projectstate.NewDerivedWithFacts(state.Root(), state.Roots(), state.Nested(), state.Facts(), state.Catalog(), state.CompleteCatalog(), targets)
}
func lowerForConfig(state *projectstate.ProjectState, cfg *config.Config) *projectstate.ProjectState {
	facts, err := config.NewFacts(cfg)
	if err != nil {
		panic(err)
	}
	return projectstate.NewDerivedWithFacts(state.Root(), state.Roots(), state.Nested(), facts, state.Catalog(), state.CompleteCatalog(), state.Targets())
}
func (p renderInputs) residentRoots() resident.Roots { return p.state.Roots() }

func testContext(t *testing.T) context.Context { return testsupport.Context(t) }
func withTestProfile(source string) string {
	if strings.Contains(source, "profile:") {
		return source
	}
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "prefix:") {
			lines = slices.Insert(lines, i+1, "profile: full")
			break
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
func withTestGateCmd(source string) string {
	if strings.Contains(source, "gateCmd:") {
		return source
	}
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "vars:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "vars:"))
		if rest == "" || rest == "{}" {
			lines[i] = "vars:"
			lines = slices.Insert(lines, i+1, "  gateCmd: test-gate")
		}
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.TrimSuffix(source, "\n") + "\nvars:\n  gateCmd: test-gate\n"
}
func gitScaffold(t *testing.T, branch string) string {
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	if branch != defaultFixtureBranch {
		gitfixture.NativeBranch(t, repo, branch)
		gitfixture.NativeCheckout(t, repo, branch)
	}
	testsupport.WriteAwfConfig(t, root, strings.Replace(sampleYAML, "integrationBranch: main", "integrationBranch: "+branch, 1))
	return root
}
func scaffold(t *testing.T, source string) string { return scaffoldFiles(t, source, nil) }
func scaffoldFiles(t *testing.T, source string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, withTestGateCmd(withTestProfile(source)))
	for path, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", path), body)
	}
	return root
}
func Open(ctx context.Context, root string) (*ProjectState, error) {
	state, err := project.Open(ctx, root)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return nil, err
	}
	testConfigs.Store(state, cfg)
	return state, nil
}
func testConfig(state *ProjectState) *config.Config {
	if value, ok := testConfigs.Load(state); ok {
		return value.(*config.Config)
	}
	cfg := state.Config()
	testConfigs.Store(state, cfg)
	return cfg
}
func testState(cfg *config.Config) *projectstate.ProjectState {
	return projectstate.NewDerived("", resident.NewRoots("", ""), false, catalog.Standard, catalog.Standard, nil)
}
func testStateAt(root string) *projectstate.ProjectState {
	return projectstate.NewDerived(root, resident.NewRoots(root, root), false, catalog.Standard, catalog.Standard, nil)
}
func renderInputsAt(root string) renderInputs {
	cfg := &config.Config{}
	state := projectstate.NewDerived(root, resident.NewRoots(root, root), false, catalog.Standard, catalog.Standard, nil)
	return newRenderInputs(state, cfg, NewFilesystemReader(root), project.Version)
}
func testRenderInputs(cfg *config.Config, roots resident.Roots, selected, complete *catalog.Catalog, targets []Target) renderInputs {
	state := projectstate.NewDerived("", roots, false, selected, complete, targets)
	return newRenderInputs(state, cfg, NewFilesystemReader(""), project.Version)
}
func renderInputsWithTargets(state *ProjectState, targets []Target) renderInputs {
	base := state.OutputState()
	lower := projectstate.NewDerived(base.Root(), base.Roots(), base.Nested(), base.Catalog(), base.CompleteCatalog(), targets)
	return newRenderInputs(lower, testConfig(state), NewFilesystemReader(state.Root()), project.Version)
}
func setTestTargets(state *ProjectState, targets []Target) *ProjectState {
	targetOverrides.Store(state, targets)
	return state
}
func testPublisher(inputs renderInputs) *Publisher {
	return New(lowerForConfig(inputs.state, inputs.cfg), inputs.cfg, inputs.read, inputs.version)
}

func renderInputsForTest(state *ProjectState) renderInputs {
	lower := state.OutputState()
	if value, ok := targetOverrides.Load(state); ok {
		lower = projectstate.NewDerived(lower.Root(), lower.Roots(), lower.Nested(), lower.Catalog(), lower.CompleteCatalog(), value.([]Target))
	}
	return newRenderInputs(lower, testConfig(state), NewFilesystemReader(state.Root()), project.Version)
}
func declaredSections(p renderInputs, kind, name string) []string {
	if d, ok := descriptorByPlural(kind); ok && d.sections != nil {
		sections, _ := d.sections(projectCatalog(p), name)
		return sections
	}
	return nil
}
func mustDeriveSkills(t *testing.T, state *ProjectState) map[string]bool {
	t.Helper()
	out, err := effectiveSkills(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func newADRProject(state *ProjectState, ctx context.Context, title string) (string, error) {
	repo, err := awfgit.Open(state.Root())
	if err != nil {
		return "", err
	}
	return project.NewADR(state.Root(), testConfig(state), repo, ctx, title)
}
func projectOperationSemantics(prepared Preparation) project.OperationSemantics {
	return project.OperationSemantics{ADRs: prepared.ADRs(), Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(), EffectiveSkills: prepared.EffectiveSkills(), Plans: prepared.Plans(), PlansError: prepared.PlansError()}
}
func checkReportProject(state *ProjectState, ctx context.Context) (project.CheckReport, error) {
	cfg := testConfig(state)
	prepared, err := New(lowerForConfig(state.OutputState(), cfg), cfg, NewFilesystemReader(state.Root()), project.Version).Prepare()
	if err != nil {
		return project.CheckReport{}, err
	}
	return project.BuildCheckReport(state, cfg, nil, ctx, prepared.Plan(), projectOperationSemantics(prepared))
}
func checkProject(state *ProjectState, _ ...context.Context) ([]manifest.Drift, error) {
	cfg := testConfig(state)
	prepared, err := New(lowerForConfig(state.OutputState(), cfg), cfg, NewFilesystemReader(state.Root()), project.Version).Prepare()
	if err != nil {
		return nil, err
	}
	report, err := project.BuildCheckReport(state, cfg, nil, context.Background(), prepared.Plan(), projectOperationSemantics(prepared))
	return report.Drift, err
}
func advisoryNotesProject(state *ProjectState) ([]string, error) {
	cfg := testConfig(state)
	prepared, err := New(lowerForConfig(state.OutputState(), cfg), cfg, NewFilesystemReader(state.Root()), project.Version).Prepare()
	if err != nil {
		return nil, err
	}
	return project.AdvisoryNotes(state, cfg, prepared.Plan(), projectOperationSemantics(prepared))
}
func initializeReportProject(state *ProjectState, seed InitAuthority) ([]Backup, []Change, []string, error) {
	cfg := testConfig(state)
	result, err := New(lowerForConfig(state.OutputState(), cfg), cfg, NewFilesystemReader(state.Root()), project.Version).Initialize(seed)
	return result.Backups(), result.Changes(), result.Pruned(), err
}
func syncReportProject(state *ProjectState) ([]Backup, []Change, []string, error) {
	cfg := testConfig(state)
	result, err := New(lowerForConfig(state.OutputState(), cfg), cfg, NewFilesystemReader(state.Root()), project.Version).Sync()
	return result.Backups(), result.Changes(), result.Pruned(), err
}
func contextStateProject(state *ProjectState, ctx context.Context) (project.ContextState, error) {
	prep, err := project.PrepareContextState(state, nil, ctx)
	if err != nil {
		return project.ContextState{}, err
	}
	plan, err := New(prep.State, prep.Config, prep.Reader, project.Version).Plan()
	if err != nil {
		return project.ContextState{}, err
	}
	return project.CompleteContextState(prep, plan), nil
}
func StagedContextState(ctx context.Context, root string) (project.ContextState, error) {
	prep, err := project.PrepareStagedContextState(ctx, root)
	if err != nil {
		return project.ContextState{}, err
	}
	plan, err := New(prep.State, prep.Config, prep.Reader, project.Version).Plan()
	if err != nil {
		return project.ContextState{}, err
	}
	return project.CompleteStagedContextState(prep, plan), nil
}
func plannedOutputsProject(state *ProjectState) ([]string, error) {
	plan, err := New(lowerForConfig(state.OutputState(), testConfig(state)), testConfig(state), NewFilesystemReader(state.Root()), project.Version).Plan()
	if err != nil {
		return nil, err
	}
	return plan.Paths(), nil
}
func configReferenceProject(state *ProjectState) (ConfigReference, error) {
	return configReferenceModel(renderInputsForTest(state))
}
func outputPlanProject(state *ProjectState) (*OutputPlan, error) {
	return outputPlan(renderInputsForTest(state))
}
func renderResidentMarkerProject(state *ProjectState, name string) (RenderedFile, error) {
	plan, err := outputPlan(renderInputsForTest(state))
	if err != nil {
		return RenderedFile{}, err
	}
	want := config.DirName + "/" + name + "/.gitignore"
	for _, node := range plan.Nodes {
		if node.Path == want && node.file != nil {
			return *node.file, nil
		}
	}
	return RenderedFile{}, fmt.Errorf("resident marker %s is absent from test plan", want)
}
func renderAll(state *ProjectState) ([]RenderedFile, error) {
	plan, err := outputPlan(renderInputsForTest(state))
	if err != nil {
		return nil, err
	}
	return plan.writeFiles(), nil
}
func syncProject(state *ProjectState) error {
	pub := testPublisher(renderInputsForTest(state))
	_, found, err := manifest.LoadOptional(config.LockPath(state.Root()))
	if err != nil {
		return err
	}
	if found {
		_, err = pub.Sync()
	} else {
		_, err = pub.Initialize(InitAuthority{InitializedWithVersion: project.Version})
	}
	return err
}
func renderInputsWithCatalog(state *ProjectState, selected *catalog.Catalog) renderInputs {
	base := state.OutputState()
	lower := projectstate.NewDerived(base.Root(), base.Roots(), base.Nested(), selected, base.CompleteCatalog(), base.Targets())
	return newRenderInputs(lower, testConfig(state), NewFilesystemReader(state.Root()), project.Version)
}
