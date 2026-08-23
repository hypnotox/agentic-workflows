package checkop

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
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/prosegate"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

type repoCheckCounters struct {
	loads, opens, reports, states, indexes int
}

func repoCheckTestDependencies(t *testing.T, cfg *config.Config, p *project.ProjectState, check project.CheckReport, state project.CurrentStateReport, tree *snapshot.Tree, counts *repoCheckCounters) repoCheckDependencies {
	t.Helper()
	state = currentStateReportForTest(t, state)
	if !hasCheckResults(check.AggregateAdvisoryResult()) && !hasCheckResults(check.TrackingInformationResult()) && !hasCheckResults(check.DeferredPlanWarningResult()) && (len(check.Warnings) > 0 || len(check.Information) > 0 || len(check.TrackingInformation) > 0 || len(check.PlanWarnings) > 0) {
		result := func(warnings []string, informationNotes []string, property checkresult.Property) checkresult.Result {
			findings := make([]checkresult.Finding, 0, len(warnings))
			for _, warning := range warnings {
				findings = append(findings, checkresult.Finding{Rank: severity.Warn, Property: property, Evidence: checkresult.Evidence{Kind: "test-compatibility", Detail: warning}})
			}
			information := make([]checkresult.Information, 0, len(informationNotes))
			for _, note := range informationNotes {
				information = append(information, checkresult.Information{Evidence: checkresult.Evidence{Kind: "test-compatibility", Detail: note}})
			}
			out, err := checkresult.New(findings, information)
			if err != nil {
				t.Fatal(err)
			}
			return out
		}
		typed, err := repositorycheck.Compose(repositorycheck.Inputs{
			OrdinaryAdvisories:   repositorycheck.Slot{Result: result(check.Warnings, check.Information, "test-advisory")},
			TrackingInformation:  repositorycheck.Slot{Result: result(nil, check.TrackingInformation, "test-tracking")},
			DeferredPlanWarnings: repositorycheck.Slot{Result: result(check.PlanWarnings, nil, "test-plan")},
		})
		if err != nil {
			t.Fatal(err)
		}
		typed.DirectResult = check.DirectResult
		typed.Drift = check.Drift
		check = typed
	}
	if len(check.DirectResult.Findings()) == 0 && len(check.DirectResult.Information()) == 0 && len(check.Drift) > 0 {
		var findings []checkresult.Finding
		var information []checkresult.Information
		for _, drift := range check.Drift {
			evidence := checkresult.Evidence{Kind: drift.Kind, Path: drift.Path, Detail: drift.Detail}
			if drift.Kind == "unused-var" || drift.Kind == "unused-data" {
				information = append(information, checkresult.Information{Evidence: evidence})
			} else {
				findings = append(findings, checkresult.Finding{Rank: severity.Error, Property: "test-compatibility", Evidence: evidence})
			}
		}
		var err error
		check.DirectResult, err = checkresult.New(findings, information)
		if err != nil {
			t.Fatal(err)
		}
	}
	return repoCheckDependencies{
		loadConfig: func(string) (*config.Config, error) {
			counts.loads++
			return cfg, nil
		},
		openProject: func(_ context.Context, _ string, got *config.Config) (*project.ProjectState, *awfgit.Repo, error) {
			counts.opens++
			if got != cfg {
				t.Fatalf("openProject config = %p, want prepared config %p", got, cfg)
			}
			return p, nil, nil
		},
		checkReport: func(_ context.Context, got *project.ProjectState, gotConfig *config.Config, _ *awfgit.Repo) (project.CheckReport, error) {
			counts.reports++
			if gotConfig != cfg {
				t.Fatalf("checkReport config = %p, want %p", gotConfig, cfg)
			}
			if got != p {
				t.Fatalf("checkReport project = %p, want prepared project %p", got, p)
			}
			return check, nil
		},
		currentState: func(_ context.Context, _ string, _ *awfgit.Repo) (project.CurrentStateReport, error) {
			counts.states++
			return state, nil
		},
		present: func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
			if evidence {
				return repositorycheck.PresentEvidence(result, check)
			}
			return repositorycheck.Present(result, check)
		},
		indexTree: func(context.Context, string) (*snapshot.Tree, error) {
			counts.indexes++
			return tree, nil
		},
	}
}

func TestScanProsePropagatesScannerFailure(t *testing.T) {
	want := errors.New("scanner unavailable")
	_, err := scanProse(&config.Config{}, mustSnapshotTree(t), proseDependencies{
		scan: func([]prosegate.File, []prosegate.Exemption) ([]prosegate.Finding, []string, error) {
			return nil, nil, want
		},
	})
	if !errors.Is(err, want) {
		t.Fatalf("scanProse error = %v, want %v", err, want)
	}
}

func mustSnapshotTree(t *testing.T) *snapshot.Tree {
	t.Helper()
	tree, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestRepoCheckRoutesAggregateOwnerResultsWithoutCompatibilitySlices(t *testing.T) {
	ordinary, err := checkresult.New([]checkresult.Finding{{
		Rank: severity.Warn, Property: "ordinary-property",
		Evidence: checkresult.Evidence{Kind: "ordinary-kind", Detail: "ordinary warning"},
	}}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "ordinary-information-kind", Detail: "ordinary information"}}})
	if err != nil {
		t.Fatal(err)
	}
	tracking, err := checkresult.New(nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "tracking-kind", Detail: "tracking information"}}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := checkresult.New([]checkresult.Finding{{
		Rank: severity.Warn, Property: "plan-property",
		Evidence: checkresult.Evidence{Kind: "plan-kind", Detail: "plan warning"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := repositorycheck.Compose(repositorycheck.Inputs{
		OrdinaryAdvisories:   repositorycheck.Slot{Result: ordinary},
		TrackingInformation:  repositorycheck.Slot{Result: tracking},
		DeferredPlanWarnings: repositorycheck.Slot{Result: plan},
	})
	if err != nil {
		t.Fatal(err)
	}
	report.Warnings = []string{"mutated compatibility warning"}
	report.Information = []string{"mutated compatibility information"}
	report.TrackingInformation = []string{"mutated compatibility tracking"}
	report.PlanWarnings = []string{"mutated compatibility plan warning"}

	counts := &repoCheckCounters{}
	deps := repoCheckTestDependencies(t, &config.Config{}, &project.ProjectState{}, report, project.CurrentStateReport{}, nil, counts)
	present := deps.present
	var properties []checkresult.Property
	var informationKinds []string
	deps.present = func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
		for _, finding := range result.Findings() {
			properties = append(properties, finding.Property)
		}
		for _, item := range result.Information() {
			informationKinds = append(informationKinds, item.Evidence.Kind)
		}
		return present(result, check, evidence)
	}
	var stdout bytes.Buffer
	if err := runRepoCheckSelection(context.Background(), t.TempDir(), &stdout, []execution.StepID{repoStepDrift}, execution.ContinueOnFailure, true, deps); err != nil {
		t.Fatal(err)
	}
	for _, want := range []checkresult.Property{"ordinary-property", "plan-property"} {
		if !slices.Contains(properties, want) {
			t.Fatalf("presented properties = %v, missing %q", properties, want)
		}
	}
	for _, want := range []string{"ordinary-information-kind", "tracking-kind"} {
		if !slices.Contains(informationKinds, want) {
			t.Fatalf("presented information kinds = %v, missing %q", informationKinds, want)
		}
	}
	for _, want := range []string{"ordinary warning", "plan warning", "ordinary information", "tracking information"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("typed aggregate output missing %q: %q", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "mutated compatibility") {
		t.Fatalf("compatibility projection changed aggregate routing: %q", stdout.String())
	}
}

func TestRepoCheckRoutesWorkingCurrentStateOwnerResult(t *testing.T) {
	result, err := checkresult.New([]checkresult.Finding{{
		Rank: severity.Warn, Property: "current-state-coverage",
		Evidence: checkresult.Evidence{Kind: "current-state", Detail: "coverage warning"},
	}}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "current-state", Detail: "provisional information"}}})
	if err != nil {
		t.Fatal(err)
	}
	state := project.CurrentStateReport{
		CurrentResult: result,
		OwnerResult:   result,
		PlanNotes:     []string{"mutated compatibility warning"},
		Provisional:   []currentstate.Introduction{{Identity: "mutated-compatibility-information"}},
	}
	counts := &repoCheckCounters{}
	deps := repoCheckTestDependencies(t, &config.Config{}, &project.ProjectState{}, project.CheckReport{}, state, nil, counts)
	present := deps.present
	var properties []checkresult.Property
	var informationKinds []string
	deps.present = func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
		for _, finding := range result.Findings() {
			properties = append(properties, finding.Property)
		}
		for _, item := range result.Information() {
			informationKinds = append(informationKinds, item.Evidence.Kind)
		}
		return present(result, check, evidence)
	}
	var stdout bytes.Buffer
	if err := runRepoCheckSelection(context.Background(), t.TempDir(), &stdout, []execution.StepID{repoStepState}, execution.StopOnFailure, false, deps); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(properties, "current-state-coverage") || !slices.Contains(informationKinds, "current-state") {
		t.Fatalf("working current-state typed route lost owner results: properties=%v information=%v", properties, informationKinds)
	}
	if !strings.Contains(stdout.String(), "coverage warning") || !strings.Contains(stdout.String(), "provisional information") {
		t.Fatalf("working current-state output = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "mutated compatibility") || strings.Contains(stdout.String(), "mutated-compatibility-information") {
		t.Fatalf("compatibility projection changed current-state routing: %q", stdout.String())
	}
}

func TestRepoCheckAggregateTypedPresentationFailuresPropagate(t *testing.T) {
	warning, err := checkresult.New([]checkresult.Finding{{
		Rank: severity.Warn, Property: "test-warning",
		Evidence: checkresult.Evidence{Kind: "warning", Detail: "warning"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	information, err := checkresult.New(nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "information", Detail: "information"}}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := repositorycheck.Compose(repositorycheck.Inputs{
		OrdinaryAdvisories:   repositorycheck.Slot{Result: warning},
		TrackingInformation:  repositorycheck.Slot{Result: information},
		DeferredPlanWarnings: repositorycheck.Slot{Result: warning},
	})
	if err != nil {
		t.Fatal(err)
	}
	for target := 1; target <= 5; target++ {
		t.Run(fmt.Sprintf("projection-%d", target), func(t *testing.T) {
			failure := fmt.Errorf("projection %d failed", target)
			deps := repoCheckTestDependencies(t, &config.Config{}, &project.ProjectState{}, report, project.CurrentStateReport{}, nil, &repoCheckCounters{})
			present := deps.present
			calls := 0
			deps.present = func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
				calls++
				if calls == target {
					return repositorycheck.Presentation{}, failure
				}
				return present(result, check, evidence)
			}
			err := runRepoCheckSelectionWithPlanNotes(context.Background(), t.TempDir(), io.Discard, []execution.StepID{repoStepDrift}, execution.ContinueOnFailure, true, []string{"leading information"}, planNoteSink{}, deps)
			if !errors.Is(err, failure) {
				t.Fatalf("presentation failure = %v, want %v", err, failure)
			}
		})
	}
}

func TestPresentCurrentStateReportPropagatesPlanPartitionFailure(t *testing.T) {
	planError, err := checkresult.New([]checkresult.Finding{{
		Rank: severity.Error, Property: "plan-validity",
		Evidence: checkresult.Evidence{Kind: "plan", Detail: "broken"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("plan partition presentation failed")
	calls := 0
	present := func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
		calls++
		if calls == 2 {
			return repositorycheck.Presentation{}, failure
		}
		return repositorycheck.Present(result, check)
	}
	_, err = presentCurrentStateReport(project.CurrentStateReport{PlanArtifactResult: planError, OwnerResult: planError}, "current-state", planNoteSink{}, present)
	if !errors.Is(err, failure) {
		t.Fatalf("plan partition failure = %v, want %v", err, failure)
	}
}

func TestRepoCheckCategoryFailuresPropagate(t *testing.T) {
	cfg := &config.Config{}
	p := &project.ProjectState{}
	for _, tc := range []struct {
		name string
		step execution.StepID
		set  func(*repoCheckDependencies, error)
	}{
		{"drift", repoStepDrift, func(deps *repoCheckDependencies, failure error) {
			deps.present = func(checkresult.Result, string, bool) (repositorycheck.Presentation, error) {
				return repositorycheck.Presentation{}, failure
			}
		}},
		{"state", repoStepState, func(deps *repoCheckDependencies, failure error) {
			deps.present = func(checkresult.Result, string, bool) (repositorycheck.Presentation, error) {
				return repositorycheck.Presentation{}, failure
			}
		}},
		{"prose", repoStepProse, func(deps *repoCheckDependencies, failure error) {
			deps.present = func(checkresult.Result, string, bool) (repositorycheck.Presentation, error) {
				return repositorycheck.Presentation{}, failure
			}
		}},
		{"memory", repoStepMemory, func(deps *repoCheckDependencies, failure error) {
			deps.present = func(checkresult.Result, string, bool) (repositorycheck.Presentation, error) {
				return repositorycheck.Presentation{}, failure
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			counts := &repoCheckCounters{}
			tree, err := snapshot.NewTree(nil)
			if err != nil {
				t.Fatal(err)
			}
			deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{}, project.CurrentStateReport{}, tree, counts)
			failure := errors.New(tc.name + " category failure")
			tc.set(&deps, failure)
			if err := runRepoCheckSelection(context.Background(), t.TempDir(), io.Discard, []execution.StepID{tc.step}, execution.StopOnFailure, false, deps); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want %v", err, failure)
			}
		})
	}
}

func currentStateReportForTest(t *testing.T, report project.CurrentStateReport) project.CurrentStateReport {
	t.Helper()
	if len(report.Static) == 0 {
		return report
	}
	findings := make([]checkresult.Finding, 0, len(report.Static))
	for _, finding := range report.Static {
		findings = append(findings, checkresult.Finding{Rank: severity.Error, Property: "current-state-authority", Evidence: checkresult.Evidence{Kind: "current-state", Detail: finding.Message}})
	}
	result, err := checkresult.New(findings, nil)
	if err != nil {
		t.Fatal(err)
	}
	report.CurrentResult = result
	report.OwnerResult = result
	return report
}

// invariant: rendering/sync-and-drift:generated-artifacts-tracked (TestRepoCheckCapabilityPlan)
// invariant: tooling/cli:repo-check-capability-plan (TestRepoCheckCapabilityPlan)
func TestRepoCheckCapabilityPlan(t *testing.T) {
	t.Run("aggregate prepares each capability once and preserves successful output membership", func(t *testing.T) {
		cfg := &config.Config{ProseGate: &config.ProseGateConfig{}, MemoryCite: &config.MemoryCiteConfig{}}
		p := &project.ProjectState{}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Warnings: []string{"project-advisory-sentinel"}, PlanWarnings: []string{"working-plan-note-sentinel"}, Notes: []string{"project-advisory-sentinel"}, PlanNotes: []string{"working-plan-note-sentinel"}}, project.CurrentStateReport{}, tree, counts)
		var out bytes.Buffer
		err = runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepMemory, repoStepProse, repoStepState, repoStepDrift}, execution.ContinueOnFailure, true, deps)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := *counts, (repoCheckCounters{loads: 1, opens: 1, reports: 1, states: 1, indexes: 1}); got != want {
			t.Fatalf("capability counts = %+v, want %+v", got, want)
		}
		got := out.String()
		const prefix = "status: warnings\n\nsummary:\n  findings: 0 errors, 2 warnings, 0 information\n\nfindings:\n  warnings:\n"
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("output header = %q, want prefix %q", got, prefix)
		}
		for _, line := range []string{"    advisory | project-advisory-sentinel\n", "    advisory | working-plan-note-sentinel\n"} {
			if strings.Count(got, line) != 1 {
				t.Fatalf("output contains %q %d times, want once: %q", line, strings.Count(got, line), got)
			}
		}
	})

	t.Run("aggregate keeps universes distinct and continues after action errors", func(t *testing.T) {
		cfg := &config.Config{ProseGate: &config.ProseGateConfig{}, MemoryCite: &config.MemoryCiteConfig{}}
		p := &project.ProjectState{}
		tree, err := snapshot.NewTree([]snapshot.File{
			{Path: "prose-index-sentinel.txt", Bytes: []byte("bad \u2013")},
			{Path: "docs/decisions/memory-index-sentinel.md", Bytes: []byte(".awf/efforts/example/memory.md")},
		})
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		check := project.CheckReport{
			Warnings: []string{"working-advisory-sentinel"},
			Notes:    []string{"working-advisory-sentinel"},
			Drift:    []manifest.Drift{{Kind: "changed", Path: "working-drift-sentinel", Detail: "working bytes"}},
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
		got := out.String()
		const prefix = "status: failed\n\nsummary:\n  findings: 3 errors, 2 warnings, 0 information\n\nfindings:\n  errors:\n    drift | changed: working-drift-sentinel: working bytes\n    current-state | current-state-sentinel\n    memory | docs/decisions/memory-index-sentinel.md: 1 effort-owned memory citation(s) on line(s) 1; name the .awf/efforts/ directory, use an angle-bracket slug placeholder, or remove the ephemeral file citation\n  warnings:\n"
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("output = %q, want protected prefix %q", got, prefix)
		}
		for _, line := range []string{"    advisory | working-advisory-sentinel\n", "    prose | prose-index-sentinel.txt: en-dash (U+2013) appears 1 time(s); en dashes are not permitted\n"} {
			if strings.Count(got, line) != 1 {
				t.Fatalf("output contains %q %d times, want once: %q", line, strings.Count(got, line), got)
			}
		}
	})

	t.Run("later preparation failure emits no action or advisory output", func(t *testing.T) {
		cfg := &config.Config{ProseGate: &config.ProseGateConfig{}}
		p := &project.ProjectState{}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Warnings: []string{"must-not-print"}, Notes: []string{"must-not-print"}}, project.CurrentStateReport{}, tree, counts)
		failure := errors.New("current-state preparation failed")
		deps.currentState = func(context.Context, string, *awfgit.Repo) (project.CurrentStateReport, error) {
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
				p := &project.ProjectState{}
				tree, err := snapshot.NewTree(nil)
				if err != nil {
					t.Fatal(err)
				}
				counts := &repoCheckCounters{}
				deps := repoCheckTestDependencies(t, tc.cfg, p, project.CheckReport{Warnings: []string{"aggregate-only"}, Notes: []string{"aggregate-only"}}, project.CurrentStateReport{}, tree, counts)
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

	t.Run("tracking notes appear directly and compose with aggregate advisories", func(t *testing.T) {
		cfg := &config.Config{}
		p := &project.ProjectState{}
		report := project.CheckReport{Warnings: []string{"aggregate-only"}, TrackingInformation: []string{"tracking unavailable"}, Notes: []string{"aggregate-only"}, TrackingNotes: []string{"tracking unavailable"}}

		deps := repoCheckTestDependencies(t, cfg, p, report, project.CurrentStateReport{}, nil, &repoCheckCounters{})
		var direct bytes.Buffer
		if err := runRepoCheckSelection(context.Background(), t.TempDir(), &direct, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, deps); err != nil {
			t.Fatalf("direct tracking advisory: %v", err)
		}
		if got, want := direct.String(), "status: completed\n\nsummary:\n  findings: 0 errors, 0 warnings, 1 information\n\nfindings:\n  information:\n    advisory | tracking unavailable\n"; got != want {
			t.Fatalf("direct tracking advisory = %q, want %q", got, want)
		}

		deps = repoCheckTestDependencies(t, cfg, p, report, project.CurrentStateReport{}, nil, &repoCheckCounters{})
		var aggregate bytes.Buffer
		if err := runRepoCheckSelection(context.Background(), t.TempDir(), &aggregate, []execution.StepID{repoStepDrift}, execution.ContinueOnFailure, true, deps); err != nil {
			t.Fatalf("aggregate tracking advisory: %v", err)
		}
		if got, want := aggregate.String(), "status: warnings\n\nsummary:\n  findings: 0 errors, 1 warnings, 1 information\n\nfindings:\n  warnings:\n    advisory | aggregate-only\n  information:\n    advisory | tracking unavailable\n"; got != want {
			t.Fatalf("aggregate tracking advisory = %q, want %q", got, want)
		}
	})

	t.Run("disabled aggregate scanners prepare no index", func(t *testing.T) {
		cfg := &config.Config{}
		p := &project.ProjectState{}
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
				p := &project.ProjectState{}
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
		p := &project.ProjectState{}
		tree, err := snapshot.NewTree(nil)
		if err != nil {
			t.Fatal(err)
		}
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{}, project.CurrentStateReport{}, tree, counts)
		ctx, cancel := context.WithCancel(context.Background())
		deps.checkReport = func(context.Context, *project.ProjectState, *config.Config, *awfgit.Repo) (project.CheckReport, error) {
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
		{"checkrepo.go", "productionRepoCheckDependencies", []string{"project.NewLoader(", "project.NewLoaderWithoutRepository(", "project.BuildCheckReport("}},
		{"operation.go", "Run", []string{"case Repository:", "collectCheckRepoWithPlanNotes"}},
		{"operation.go", "Run", []string{"case RepositoryDrift:", "case RepositoryState:", "case RepositoryProse:", "case RepositoryMemory:"}},
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
			case "Run":
				callee = ""
			}
			if callee != "" && strings.Count(body, callee) != 1 {
				t.Fatalf("%s %s must call %s exactly once:\n%s", tc.file, tc.function, callee, body)
			}
		})
	}
}

// invariant: rendering/sync-and-drift:agent-guide-size-advisory (TestAggregateCheckAgentGuideSizeWarning)
func TestAggregateCheckAgentGuideSizeWarning(t *testing.T) {
	cfg := &config.Config{}
	p := &project.ProjectState{}
	advisory := "AGENTS.md is 12289 bytes, allowed 12288 bytes; see docs/agents-md-standard.md"
	runAggregate := func(t *testing.T, notes []string) string {
		t.Helper()
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Warnings: notes, Notes: notes}, project.CurrentStateReport{}, nil, counts)
		var out bytes.Buffer
		if err := runRepoCheckSelection(context.Background(), t.TempDir(), &out, []execution.StepID{repoStepDrift}, execution.ContinueOnFailure, true, deps); err != nil {
			t.Fatalf("warning-only aggregate error: %v", err)
		}
		return out.String()
	}

	t.Run("size advisory is the only finding", func(t *testing.T) {
		want := "status: warnings\n\nsummary:\n  findings: 0 errors, 1 warnings, 0 information\n\nfindings:\n  warnings:\n    advisory | " + advisory + "\n"
		if got := runAggregate(t, []string{advisory}); got != want {
			t.Fatalf("aggregate output = %q, want %q", got, want)
		}
	})

	t.Run("production warning membership is deterministic", func(t *testing.T) {
		first := runAggregate(t, []string{"ordinary-advisory", advisory})
		second := runAggregate(t, []string{"ordinary-advisory", advisory})
		if first != second {
			t.Fatalf("aggregate output is not deterministic: first=%q second=%q", first, second)
		}
		const prefix = "status: warnings\n\nsummary:\n  findings: 0 errors, 2 warnings, 0 information\n\nfindings:\n  warnings:\n"
		if !strings.HasPrefix(first, prefix) {
			t.Fatalf("aggregate output = %q, want prefix %q", first, prefix)
		}
		for _, line := range []string{"    advisory | ordinary-advisory\n", "    advisory | " + advisory + "\n"} {
			if strings.Count(first, line) != 1 {
				t.Fatalf("aggregate output contains %q %d times, want once: %q", line, strings.Count(first, line), first)
			}
		}
	})

	counts := &repoCheckCounters{}
	deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Warnings: []string{"ordinary-advisory", advisory}, Notes: []string{"ordinary-advisory", advisory}}, project.CurrentStateReport{}, nil, counts)
	var direct bytes.Buffer
	if err := runRepoCheckSelection(context.Background(), t.TempDir(), &direct, []execution.StepID{repoStepDrift}, execution.StopOnFailure, false, deps); err != nil {
		t.Fatalf("direct drift error: %v", err)
	}
	if got := direct.String(); got != completedCheckReport {
		t.Fatalf("direct drift output = %q, want no advisory", got)
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
