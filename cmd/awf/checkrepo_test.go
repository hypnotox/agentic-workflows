package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

type repoCheckCounters struct {
	loads, opens, reports, states, indexes int
}

func repoCheckTestDependencies(t *testing.T, cfg *config.Config, p *project.Project, check project.CheckReport, state project.CurrentStateReport, tree *snapshot.Tree, counts *repoCheckCounters) repoCheckDependencies {
	t.Helper()
	return repoCheckDependencies{
		loadConfig: func(string) (*config.Config, error) {
			counts.loads++
			return cfg, nil
		},
		openProject: func(_ context.Context, _ string, got *config.Config) (*project.Project, error) {
			counts.opens++
			if got != cfg {
				t.Fatalf("openProject config = %p, want prepared config %p", got, cfg)
			}
			return p, nil
		},
		checkReport: func(_ context.Context, got *project.Project) (project.CheckReport, error) {
			counts.reports++
			if got != p {
				t.Fatalf("checkReport project = %p, want prepared project %p", got, p)
			}
			return check, nil
		},
		currentState: func(_ context.Context, got *project.Project) (project.CurrentStateReport, error) {
			counts.states++
			if got != p {
				t.Fatalf("currentState project = %p, want prepared project %p", got, p)
			}
			return state, nil
		},
		driftCategories:        project.DriftCategories,
		currentStateCategories: project.CurrentStateCategories,
		indexTree: func(context.Context, string) (*snapshot.Tree, error) {
			counts.indexes++
			return tree, nil
		},
	}
}

func TestRepoCheckCategoryFailuresPropagate(t *testing.T) {
	cfg := &config.Config{}
	p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
	for _, tc := range []struct {
		name string
		step execution.StepID
		set  func(*repoCheckDependencies, error)
	}{
		{"drift", repoStepDrift, func(deps *repoCheckDependencies, failure error) {
			deps.driftCategories = func([]manifest.Drift, bool) ([]presentation.ReportCategory, error) { return nil, failure }
		}},
		{"state", repoStepState, func(deps *repoCheckDependencies, failure error) {
			deps.currentStateCategories = func(project.CurrentStateReport, bool) ([]presentation.ReportCategory, error) { return nil, failure }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			counts := &repoCheckCounters{}
			deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{}, project.CurrentStateReport{}, nil, counts)
			failure := errors.New(tc.name + " category failure")
			tc.set(&deps, failure)
			if err := runRepoCheckSelection(context.Background(), t.TempDir(), io.Discard, []execution.StepID{tc.step}, execution.StopOnFailure, false, deps); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want %v", err, failure)
			}
		})
	}
}

// invariant: tooling/cli:repo-check-capability-plan (TestRepoCheckCapabilityPlan)
func TestRepoCheckCapabilityPlan(t *testing.T) {
	t.Run("aggregate prepares each capability once and preserves successful output order", func(t *testing.T) {
		cfg := &config.Config{ProseGate: &config.ProseGateConfig{}, MemoryCite: &config.MemoryCiteConfig{}}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Notes: []string{"project-advisory-sentinel"}, PlanNotes: []string{"working-plan-note-sentinel"}}, project.CurrentStateReport{}, tree, counts)
		var out bytes.Buffer
		err = runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepMemory, repoStepProse, repoStepState, repoStepDrift}, execution.ContinueOnFailure, true, deps)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := *counts, (repoCheckCounters{loads: 1, opens: 1, reports: 1, states: 1, indexes: 1}); got != want {
			t.Fatalf("capability counts = %+v, want %+v", got, want)
		}
		want := "status: warnings\n\nsummary:\n  findings: 0 errors, 2 warnings\n\nfindings:\n  warnings:\n    advisory | project-advisory-sentinel\n    advisory | working-plan-note-sentinel\n"
		if got := out.String(); got != want {
			t.Fatalf("output = %q, want %q", got, want)
		}
	})

	t.Run("aggregate keeps universes distinct and continues after action errors", func(t *testing.T) {
		cfg := &config.Config{ProseGate: &config.ProseGateConfig{}, MemoryCite: &config.MemoryCiteConfig{}}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree([]snapshot.File{
			{Path: "prose-index-sentinel.txt", Bytes: []byte("bad \u2014")},
			{Path: "docs/decisions/memory-index-sentinel.md", Bytes: []byte(".awf/efforts/example/memory.md")},
		})
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		check := project.CheckReport{
			Notes: []string{"working-advisory-sentinel"},
			Drift: []manifest.Drift{{Kind: "changed", Path: "working-drift-sentinel", Detail: "working bytes"}},
		}
		state := project.CurrentStateReport{Static: []currentstate.Finding{{Message: "current-state-sentinel"}}}
		deps := repoCheckTestDependencies(t, cfg, p, check, state, tree, counts)
		selected := []execution.StepID{repoStepDrift, repoStepState, repoStepProse, repoStepMemory}
		collection, err := collectRepoCheckSelectionWithPlanNotes(context.Background(), t.TempDir(), selected, execution.ContinueOnFailure, true, nil, planNoteSink{}, deps)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		err = renderCheckCollection(&out, collection)
		if err == nil {
			t.Fatal("aggregate error = nil, want first drift action error")
		}
		if got, want := err.Error(), `execute step "drift": check repo drift failed`; got != want {
			t.Fatalf("aggregate error = %q, want first failure only %q", got, want)
		}
		if len(collection.failures) < 2 {
			t.Fatalf("collected failures = %d, want multiple identities", len(collection.failures))
		}
		if !errors.Is(err, collection.failures[0]) {
			t.Fatalf("aggregate error %v does not retain first failure identity %v", err, collection.failures[0])
		}
		for _, later := range collection.failures[1:] {
			if errors.Is(err, later) {
				t.Fatalf("aggregate error %v retained later failure identity %v", err, later)
			}
		}
		if got, want := *counts, (repoCheckCounters{loads: 1, opens: 1, reports: 1, states: 1, indexes: 1}); got != want {
			t.Fatalf("capability counts = %+v, want %+v", got, want)
		}
		const want = "status: failed\n\nsummary:\n  findings: 4 errors, 1 warnings\n\nfindings:\n  errors:\n    drift | changed: working-drift-sentinel: working bytes\n    current-state | current-state-sentinel\n    prose | prose-index-sentinel.txt: em-dash (U+2014) appears 1 time(s); use plain punctuation\n    memory | docs/decisions/memory-index-sentinel.md: 1 effort-owned memory citation(s) on line(s) 1; name the .awf/efforts/ directory, use an angle-bracket slug placeholder, or remove the ephemeral file citation\n  warnings:\n    advisory | working-advisory-sentinel\n"
		if got := out.String(); got != want {
			t.Fatalf("output = %q, want %q", got, want)
		}
	})

	t.Run("later preparation failure emits no action or advisory output", func(t *testing.T) {
		cfg := &config.Config{ProseGate: &config.ProseGateConfig{}}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Notes: []string{"must-not-print"}}, project.CurrentStateReport{}, tree, counts)
		failure := errors.New("current-state preparation failed")
		deps.currentState = func(context.Context, *project.Project) (project.CurrentStateReport, error) {
			counts.states++
			return project.CurrentStateReport{}, failure
		}
		var out bytes.Buffer
		err = runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepDrift, repoStepState, repoStepProse}, execution.ContinueOnFailure, true, deps)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v, want %v", err, failure)
		}
		if out.Len() != 0 {
			t.Fatalf("preparation failure output = %q, want empty", out.String())
		}
		if got, want := *counts, (repoCheckCounters{loads: 1, opens: 1, reports: 1, states: 1}); got != want {
			t.Fatalf("capability counts = %+v, want %+v", got, want)
		}
	})

	t.Run("direct selections acquire no unrelated capability", func(t *testing.T) {
		cases := []struct {
			name     string
			cfg      *config.Config
			step     execution.StepID
			want     repoCheckCounters
			wantText string
		}{
			{name: "drift", cfg: &config.Config{}, step: repoStepDrift, want: repoCheckCounters{loads: 1, opens: 1, reports: 1}, wantText: completedCheckReport},
			{name: "state", cfg: &config.Config{}, step: repoStepState, want: repoCheckCounters{loads: 1, opens: 1, states: 1}, wantText: completedCheckReport},
			{name: "prose enabled", cfg: &config.Config{ProseGate: &config.ProseGateConfig{}}, step: repoStepProse, want: repoCheckCounters{loads: 1, indexes: 1}, wantText: completedCheckReport},
			{name: "memory unconditional", cfg: &config.Config{}, step: repoStepMemory, want: repoCheckCounters{loads: 1, indexes: 1}, wantText: completedCheckReport},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := &project.Project{Root: "working-project-sentinel", Cfg: tc.cfg}
				tree, err := snapshot.NewTree(nil)
				if err != nil {
					t.Fatal(err)
				}
				counts := &repoCheckCounters{}
				deps := repoCheckTestDependencies(t, tc.cfg, p, project.CheckReport{Notes: []string{"aggregate-only"}}, project.CurrentStateReport{}, tree, counts)
				var out bytes.Buffer
				if err := runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{tc.step}, execution.StopOnFailure, false, deps); err != nil {
					t.Fatal(err)
				}
				if *counts != tc.want {
					t.Fatalf("capability counts = %+v, want %+v", *counts, tc.want)
				}
				if got := out.String(); got != tc.wantText {
					t.Fatalf("output = %q, want %q", got, tc.wantText)
				}
			})
		}
	})

	t.Run("disabled aggregate scanners prepare no index", func(t *testing.T) {
		cfg := &config.Config{}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{}, project.CurrentStateReport{}, tree, counts)
		var out bytes.Buffer
		if err := runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepDrift, repoStepState, repoStepProse, repoStepMemory}, execution.ContinueOnFailure, true, deps); err != nil {
			t.Fatal(err)
		}
		if got, want := *counts, (repoCheckCounters{loads: 1, opens: 1, reports: 1, states: 1, indexes: 1}); got != want {
			t.Fatalf("capability counts = %+v, want %+v", got, want)
		}
		if !strings.Contains(out.String(), "findings: 0 errors, 0 warnings") {
			t.Fatalf("unconditional aggregate output = %q", out.String())
		}
	})

	t.Run("index preparation retains scanner error prefixes", func(t *testing.T) {
		cause := errors.New("index unavailable")
		cases := []struct {
			name     string
			cfg      *config.Config
			selected []execution.StepID
			prefix   string
		}{
			{"direct prose", &config.Config{ProseGate: &config.ProseGateConfig{}}, []execution.StepID{repoStepProse}, "check repo prose: cannot read staged files"},
			{"direct memory", &config.Config{MemoryCite: &config.MemoryCiteConfig{}}, []execution.StepID{repoStepMemory}, "check repo memory: cannot read staged files"},
			{"aggregate scanners", &config.Config{ProseGate: &config.ProseGateConfig{}, MemoryCite: &config.MemoryCiteConfig{}}, []execution.StepID{repoStepMemory, repoStepProse}, "check repo prose: cannot read staged files"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := &project.Project{Root: "working-project-sentinel", Cfg: tc.cfg}
				tree, err := snapshot.NewTree(nil)
				if err != nil {
					t.Fatal(err)
				}
				counts := &repoCheckCounters{}
				deps := repoCheckTestDependencies(t, tc.cfg, p, project.CheckReport{}, project.CurrentStateReport{}, tree, counts)
				deps.indexTree = func(context.Context, string) (*snapshot.Tree, error) {
					counts.indexes++
					return nil, &repoIndexPreparationError{err: fmt.Errorf("cannot read staged files: %w", cause)}
				}
				err = runRepoCheckSelection(context.Background(), t.TempDir(), io.Discard, tc.selected, execution.StopOnFailure, false, deps)
				if !errors.Is(err, cause) || !strings.Contains(err.Error(), tc.prefix) {
					t.Fatalf("error = %v, want prefix %q and wrapped cause", err, tc.prefix)
				}
			})
		}
	})

	t.Run("execution cancellation remains separate from outcomes", func(t *testing.T) {
		cfg := &config.Config{}
		p := &project.Project{Root: "working-project-sentinel", Cfg: cfg}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{}, project.CurrentStateReport{}, tree, counts)
		ctx, cancel := context.WithCancel(context.Background())
		deps.checkReport = func(context.Context, *project.Project) (project.CheckReport, error) {
			cancel()
			return project.CheckReport{}, nil
		}
		err = runRepoCheckSelection(ctx, t.TempDir(), io.Discard, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, deps)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context cancellation", err)
		}
	})

	assertRepoCheckProductionWiring(t)
}

func assertRepoCheckProductionWiring(t *testing.T) {
	t.Helper()
	cases := []struct {
		file     string
		function string
		contains []string
	}{
		{"checkrepo.go", "productionRepoCheckDependencies", []string{"project.NewLoader(", "project.NewLoaderWithoutRepository(", "p.CheckReport("}},
		{"checkrepo.go", "runCheckRepo", []string{"runCheckRepoWithPlanNotes", "planNoteSink{}"}},
		{"checkrepo.go", "runCheckRepoWithPlanNotes", []string{"collectCheckRepoWithPlanNotes", "renderCheckCollection"}},
		{"checkrepo.go", "collectCheckRepoWithPlanNotes", []string{"repoStepDrift", "repoStepState", "repoStepProse", "repoStepMemory", "execution.ContinueOnFailure", "true", "productionRepoCheckDependencies()"}},
		{"checkrepo.go", "runCheckDrift", []string{"repoStepDrift", "execution.StopOnFailure", "false", "productionRepoCheckDependencies()"}},
		{"checkrepo.go", "runCheckState", []string{"repoStepState", "execution.StopOnFailure", "false", "productionRepoCheckDependencies()"}},
		{"prosegate.go", "runProseGate", []string{"repoStepProse", "execution.StopOnFailure", "false", "productionRepoCheckDependencies()"}},
		{"memorygate.go", "runMemoryGate", []string{"repoStepMemory", "execution.StopOnFailure", "false", "productionRepoCheckDependencies()"}},
	}
	for _, tc := range cases {
		t.Run("wiring/"+tc.function, func(t *testing.T) {
			body := formattedFunctionBody(t, tc.file, tc.function)
			for _, fragment := range tc.contains {
				if !strings.Contains(body, fragment) {
					t.Fatalf("%s %s body does not contain %q:\n%s", tc.file, tc.function, fragment, body)
				}
			}
			callee := "runRepoCheckSelection("
			switch tc.function {
			case "productionRepoCheckDependencies":
				callee = ""
			case "runCheckRepo":
				callee = "runCheckRepoWithPlanNotes("
			case "runCheckRepoWithPlanNotes":
				callee = "collectCheckRepoWithPlanNotes("
			case "collectCheckRepoWithPlanNotes":
				callee = "collectRepoCheckSelectionWithPlanNotes("
			}
			if callee != "" && strings.Count(body, callee) != 1 {
				t.Fatalf("%s %s must call %s exactly once:\n%s", tc.file, tc.function, callee, body)
			}
		})
	}

	aggregate := formattedFunctionBody(t, "checkrepo.go", "collectCheckRepoWithPlanNotes")
	versionOutput := strings.Index(aggregate, "fmt.Sprintf")
	executionCall := strings.Index(aggregate, "collectRepoCheckSelectionWithPlanNotes")
	if versionOutput < 0 || executionCall < 0 || versionOutput >= executionCall {
		t.Fatalf("aggregate version note preparation must precede execution:\n%s", aggregate)
	}
}

// invariant: rendering/sync-and-drift:agent-guide-size-advisory (TestAggregateCheckAgentGuideSizeWarning)
func TestAggregateCheckAgentGuideSizeWarning(t *testing.T) {
	cfg := &config.Config{}
	p := &project.Project{Root: "oversized-guide-project", Cfg: cfg}
	advisory := "AGENTS.md is 12289 bytes, allowed 12288 bytes; see docs/agents-md-standard.md"
	runAggregate := func(t *testing.T, notes []string) string {
		t.Helper()
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Notes: notes}, project.CurrentStateReport{}, nil, counts)
		var out bytes.Buffer
		if err := runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepDrift}, execution.ContinueOnFailure, true, deps); err != nil {
			t.Fatalf("warning-only aggregate error: %v", err)
		}
		return out.String()
	}

	t.Run("size advisory is the only finding", func(t *testing.T) {
		want := "status: warnings\n\nsummary:\n  findings: 0 errors, 1 warnings\n\nfindings:\n  warnings:\n    advisory | " + advisory + "\n"
		if got := runAggregate(t, []string{advisory}); got != want {
			t.Fatalf("aggregate output = %q, want %q", got, want)
		}
	})

	t.Run("production note order is preserved", func(t *testing.T) {
		want := "status: warnings\n\nsummary:\n  findings: 0 errors, 2 warnings\n\nfindings:\n  warnings:\n    advisory | ordinary-advisory\n    advisory | " + advisory + "\n"
		if got := runAggregate(t, []string{"ordinary-advisory", advisory}); got != want {
			t.Fatalf("aggregate output = %q, want %q", got, want)
		}
	})

	counts := &repoCheckCounters{}
	deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Notes: []string{"ordinary-advisory", advisory}}, project.CurrentStateReport{}, nil, counts)
	var direct bytes.Buffer
	if err := runRepoCheckSelection(context.Background(), t.TempDir(), &direct, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, deps); err != nil {
		t.Fatalf("direct drift error: %v", err)
	}
	if got := direct.String(); got != completedCheckReport {
		t.Fatalf("direct drift output = %q, want no advisory", got)
	}

	deps = repoCheckTestDependencies(t, cfg, p, project.CheckReport{Notes: []string{"aggregate-only"}, TrackingNotes: []string{"tracking unavailable"}}, project.CurrentStateReport{}, nil, &repoCheckCounters{})
	var tracking bytes.Buffer
	if err := runRepoCheckSelection(context.Background(), t.TempDir(), &tracking, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, deps); err != nil {
		t.Fatalf("direct tracking advisory: %v", err)
	}
	if got, want := tracking.String(), "status: warnings\n\nsummary:\n  findings: 0 errors, 1 warnings\n\nfindings:\n  warnings:\n    advisory | tracking unavailable\n"; got != want {
		t.Fatalf("direct tracking advisory = %q, want %q", got, want)
	}

	deps = repoCheckTestDependencies(t, cfg, p, project.CheckReport{Notes: []string{"aggregate-only"}, TrackingNotes: []string{"tracking unavailable"}}, project.CurrentStateReport{}, nil, &repoCheckCounters{})
	var aggregate bytes.Buffer
	if err := runRepoCheckSelection(context.Background(), t.TempDir(), &aggregate, []execution.StepID{repoStepDrift}, execution.ContinueOnFailure, true, deps); err != nil {
		t.Fatalf("aggregate tracking advisory: %v", err)
	}
	if got, want := aggregate.String(), "status: warnings\n\nsummary:\n  findings: 0 errors, 2 warnings\n\nfindings:\n  warnings:\n    advisory | tracking unavailable\n    advisory | aggregate-only\n"; got != want {
		t.Fatalf("aggregate tracking advisory = %q, want %q", got, want)
	}
}

func formattedFunctionBody(t *testing.T, path, name string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != name {
			continue
		}
		var out bytes.Buffer
		if err := format.Node(&out, fset, function.Body); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	t.Fatalf("function %s not found in %s", name, path)
	return ""
}
