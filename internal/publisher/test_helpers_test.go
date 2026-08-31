package publisher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/glossarycheck"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type Session = project.Session

type derivedSession struct {
	root    string
	roots   resident.Roots
	nested  bool
	cfg     *config.Config
	reader  ProjectTreeReader
	catalog *catalog.Catalog
	targets []Target
}

func (s *derivedSession) Root() string          { return s.root }
func (s *derivedSession) Roots() resident.Roots { return s.roots }
func (s *derivedSession) Nested() bool          { return s.nested }
func (s *derivedSession) Config() *config.Config {
	if s.cfg == nil {
		return nil
	}
	facts, err := config.NewFacts(s.cfg)
	if err != nil {
		panic(err)
	}
	return s.cfg.OperationTree().Bind(facts)
}
func (s *derivedSession) Reader() ProjectTreeReader { return s.reader }
func (s *derivedSession) Catalog() *catalog.Catalog { return catalog.NewView(s.catalog).Catalog() }
func (s *derivedSession) Targets() []Target         { return append([]Target(nil), s.targets...) }

func deriveSession(base ProjectSession, cfg *config.Config, reader ProjectTreeReader, selected *catalog.Catalog, targets []Target) *derivedSession {
	if cfg == nil {
		cfg = base.Config()
	}
	if reader == nil {
		reader = base.Reader()
	}
	if selected == nil {
		selected = base.Catalog()
	}
	if targets == nil {
		targets = base.Targets()
	}
	return &derivedSession{root: base.Root(), roots: base.Roots(), cfg: cfg, reader: reader, catalog: selected, targets: append([]Target(nil), targets...)}
}

func newRenderInputs(state ProjectSession, cfg *config.Config, read ProjectTreeReader, version string) renderInputs {
	return renderInputsFromSession(deriveSession(state, cfg, read, nil, nil), version)
}

func newPublisher(state ProjectSession, cfg *config.Config, read ProjectTreeReader, version string) *Publisher {
	if state == nil {
		return New(nil, version)
	}
	derived := deriveSession(state, state.Config(), state.Reader(), state.Catalog(), state.Targets())
	derived.cfg = cfg
	derived.reader = read
	return New(derived, version)
}

var Version = project.Version

func KnownTargets() []string                          { return artifactregistry.KnownTargets() }
func resolveTargets(names []string) ([]Target, error) { return artifactregistry.ResolveTargets(names) }

var testConfigs sync.Map
var targetOverrides sync.Map

var (
	initializedSampleSeedOnce sync.Once
	initializedSampleSeed     testsupport.TreeSeed
	initializedSampleSeedErr  error
)

const defaultFixtureBranch = "master"
const pitfallsCfg = "prefix: example\nintegrationBranch: main\nvars: {}\n"
const debuggingVars = `vars:
  debuggingDoc: ""
  gateCmd: ""
  workflowDoc: ""
`
const sampleYAML = `prefix: example
integrationBranch: main
vars:
  testCmd: go test ./...
  gateCmd: make gate
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
func csRepo(t *testing.T, cfg string, files map[string]string) *Session {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, repo.Root(), cfg)
	if err := (&manifest.Lock{AWFVersion: Version, SchemaVersion: 46, Files: map[string]manifest.Entry{}}).Save(lockFile(repo.Root())); err != nil {
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
func mustDeriveTopics(t *testing.T, state *Session) topic.Corpus {
	t.Helper()
	_, topics, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	return topics
}
func queryTopicProject(state *Session, ctx context.Context, selector string, opts topic.QueryOptions) (topic.QueryResult, error) {
	repo, _, err := awfgit.OpenContaining(state.Root())
	if err != nil {
		return topic.QueryResult{}, err
	}
	return currentstatecoord.QueryTopic(state.Root(), repo, ctx, selector, opts)
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
func testTargets(state *Session) []Target {
	if value, ok := targetOverrides.Load(state); ok {
		return append([]Target(nil), value.([]Target)...)
	}
	return state.Targets()
}
func lowerWithTargets(state ProjectSession, targets []Target) *derivedSession {
	derived := deriveSession(state, state.Config(), state.Reader(), state.Catalog(), state.Targets())
	derived.targets = append([]Target(nil), targets...)
	return derived
}
func lowerForConfig(state ProjectSession, cfg *config.Config) *derivedSession {
	return deriveSession(state, cfg, state.Reader(), state.Catalog(), state.Targets())
}
func (p renderInputs) residentRoots() resident.Roots { return p.session.Roots() }

func testContext(t *testing.T) context.Context { return testsupport.Context(t) }
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

func initializedSampleProject(t *testing.T) (string, *Session) {
	t.Helper()
	initializedSampleSeedOnce.Do(func() {
		root := scaffold(t, sampleYAML)
		state, err := loadTestSession(testContext(t), root)
		if err != nil {
			initializedSampleSeedErr = err
			return
		}
		if err := syncProject(state); err != nil {
			initializedSampleSeedErr = err
			return
		}
		initializedSampleSeed, initializedSampleSeedErr = testsupport.CaptureTree(root)
	})
	if initializedSampleSeedErr != nil {
		t.Fatalf("prepare publisher sample seed: %v", initializedSampleSeedErr)
	}
	root := filepath.Join(t.TempDir(), "project")
	if err := initializedSampleSeed.Clone(root); err != nil {
		t.Fatalf("clone publisher sample seed: %v", err)
	}
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatalf("open cloned publisher sample seed: %v", err)
	}
	return root, state
}

func scaffoldFiles(t *testing.T, source string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, withTestGateCmd(source))
	for path, body := range files {
		testsupport.WriteFile(t, filepath.Join(root, ".awf", path), body)
	}
	return root
}
func loadTestSession(ctx context.Context, root string) (*Session, error) {
	repo, _, repoErr := awfgit.OpenContaining(root)
	var state *project.Session
	var err error
	if repoErr == nil {
		state, err = project.NewLoader(config.Load, catalog.Standard, awfgit.ProjectResidentRoot, repo).Load(ctx, root)
	} else if errors.Is(repoErr, awfgit.ErrNotARepository) {
		state, err = project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot).Load(ctx, root)
	} else {
		return nil, repoErr
	}
	if err != nil {
		return nil, err
	}
	testConfigs.Store(state, state.Config())
	return state, nil
}
func testConfig(state *Session) *config.Config {
	if value, ok := testConfigs.Load(state); ok {
		return value.(*config.Config)
	}
	cfg := state.Config()
	testConfigs.Store(state, cfg)
	return cfg
}
func testState() *derivedSession {
	return &derivedSession{roots: resident.NewRoots("", ""), cfg: &config.Config{}, reader: NewFilesystemReader(""), catalog: catalog.Standard}
}
func testStateAt(root string) *derivedSession {
	return &derivedSession{root: root, roots: resident.NewRoots(root, root), cfg: &config.Config{}, reader: NewFilesystemReader(root), catalog: catalog.Standard}
}
func renderInputsAt(root string) renderInputs {
	return renderInputsFromSession(testStateAt(root), project.Version)
}
func testRenderInputs(cfg *config.Config, roots resident.Roots, selected, _ *catalog.Catalog, targets []Target) renderInputs {
	state := &derivedSession{roots: roots, cfg: cfg, reader: NewFilesystemReader(""), catalog: selected, targets: targets}
	return renderInputsFromSession(state, project.Version)
}
func renderInputsWithTargets(state *Session, targets []Target) renderInputs {
	return newRenderInputs(lowerWithTargets(state, targets), testConfig(state), NewFilesystemReader(state.Root()), project.Version)
}
func setTestTargets(state *Session, targets []Target) *Session {
	targetOverrides.Store(state, targets)
	return state
}
func testPublisher(inputs renderInputs) *Publisher {
	return newPublisher(inputs.session, inputs.cfg, inputs.read, inputs.version)
}

func renderInputsForTest(state *Session) renderInputs {
	var selected ProjectSession = state
	if value, ok := targetOverrides.Load(state); ok {
		selected = lowerWithTargets(state, value.([]Target))
	}
	return newRenderInputs(selected, testConfig(state), NewFilesystemReader(state.Root()), project.Version)
}
func declaredSections(p renderInputs, kind, name string) []string {
	if d, ok := descriptorByPlural(kind); ok && d.sections != nil {
		sections, _ := d.sections(projectCatalog(p), name)
		return sections
	}
	return nil
}
func mustDeriveSkills(t *testing.T, state *Session) map[string]bool {
	t.Helper()
	out, err := effectiveSkills(renderInputsForTest(state))
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func operationCheckInputs(operation *Publisher) (outputplan.Plan, pitfall.Corpus, map[string]bool, generatedcheck.AdditionalInput, glossarycheck.Input, error) {
	plan, err := operation.Plan()
	if err != nil {
		return outputplan.Plan{}, pitfall.Corpus{}, nil, generatedcheck.AdditionalInput{}, glossarycheck.Input{}, err
	}
	pitfalls, err := operation.Pitfalls()
	if err != nil {
		return outputplan.Plan{}, pitfall.Corpus{}, nil, generatedcheck.AdditionalInput{}, glossarycheck.Input{}, err
	}
	skills, err := operation.EffectiveSkills()
	if err != nil {
		return outputplan.Plan{}, pitfall.Corpus{}, nil, generatedcheck.AdditionalInput{}, glossarycheck.Input{}, err
	}
	generated, err := operation.GeneratedOutput()
	if err != nil {
		return outputplan.Plan{}, pitfall.Corpus{}, nil, generatedcheck.AdditionalInput{}, glossarycheck.Input{}, err
	}
	glossary, err := operation.Glossary()
	return plan, pitfalls, skills, generated, glossary, err
}
func checkReportProject(state *Session, ctx context.Context) (project.CheckReport, error) {
	cfg := testConfig(state)
	plan, pitfalls, skills, generated, glossary, err := operationCheckInputs(newPublisher(lowerForConfig(state, cfg), cfg, NewFilesystemReader(state.Root()), project.Version))
	if err != nil {
		return project.CheckReport{}, err
	}
	return project.BuildCheckReport(state, cfg, nil, ctx, plan, pitfalls, skills, generated, glossary)
}
func checkProject(state *Session, _ ...context.Context) ([]manifest.Drift, error) {
	cfg := testConfig(state)
	plan, pitfalls, skills, generated, glossary, err := operationCheckInputs(newPublisher(lowerForConfig(state, cfg), cfg, NewFilesystemReader(state.Root()), project.Version))
	if err != nil {
		return nil, err
	}
	report, err := project.BuildCheckReport(state, cfg, nil, context.Background(), plan, pitfalls, skills, generated, glossary)
	return report.Drift, err
}
func advisoryNotesProject(state *Session) ([]string, error) {
	cfg := testConfig(state)
	operation := newPublisher(lowerForConfig(state, cfg), cfg, NewFilesystemReader(state.Root()), project.Version)
	plan, err := operation.Plan()
	if err != nil {
		return nil, err
	}
	glossary, err := operation.Glossary()
	if err != nil {
		return nil, err
	}
	return project.AdvisoryNotes(state, cfg, plan, glossary)
}
func initializeReportProject(state *Session, seed InitAuthority) ([]Backup, []Change, []string, error) {
	cfg := testConfig(state)
	result, err := newPublisher(lowerForConfig(state, cfg), cfg, NewFilesystemReader(state.Root()), project.Version).Initialize(seed)
	return result.Backups(), result.Changes(), result.Pruned(), err
}
func syncReportProject(state *Session) ([]Backup, []string, error) {
	cfg := testConfig(state)
	result, err := newPublisher(lowerForConfig(state, cfg), cfg, NewFilesystemReader(state.Root()), project.Version).SyncLeased(context.Background(), nil)
	return result.Backups(), result.Pruned(), err
}
func plannedOutputsProject(state *Session) ([]string, error) {
	plan, err := newPublisher(lowerForConfig(state, testConfig(state)), testConfig(state), NewFilesystemReader(state.Root()), project.Version).Plan()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(plan.Outputs()))
	for _, output := range plan.Outputs() {
		paths = append(paths, output.Path())
	}
	return paths, nil
}
func configReferenceProject(state *Session) (ConfigReference, error) {
	return testPublisher(renderInputsForTest(state)).BuildConfigReference()
}
func outputPlanProject(state *Session) (*OutputPlan, error) {
	return outputPlan(renderInputsForTest(state))
}
func renderResidentMarkerProject(state *Session, name string) (RenderedFile, error) {
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
func renderAll(state *Session) ([]RenderedFile, error) {
	plan, err := outputPlan(renderInputsForTest(state))
	if err != nil {
		return nil, err
	}
	return plan.writeFiles(), nil
}
func syncProject(state *Session) error {
	pub := testPublisher(renderInputsForTest(state))
	_, found, err := manifest.LoadOptional(config.LockPath(state.Root()))
	if err != nil {
		return err
	}
	if found {
		_, err = pub.SyncLeased(context.Background(), nil)
	} else {
		_, err = pub.Initialize(InitAuthority{InitializedWithVersion: project.Version})
	}
	return err
}
func renderInputsWithCatalog(state *Session, selected *catalog.Catalog) renderInputs {
	lower := deriveSession(state, testConfig(state), NewFilesystemReader(state.Root()), selected, state.Targets())
	return renderInputsFromSession(lower, project.Version)
}
