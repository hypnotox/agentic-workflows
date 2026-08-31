package checkop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
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
	if !hasCheckResults(check.AggregateAdvisoryResult()) && !hasCheckResults(check.TrackingInformationResult()) && (len(check.Warnings) > 0 || len(check.Information) > 0 || len(check.TrackingInformation) > 0) {
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
			OrdinaryAdvisories:  repositorycheck.Slot{Result: result(check.Warnings, check.Information, "test-advisory")},
			TrackingInformation: repositorycheck.Slot{Result: result(nil, check.TrackingInformation, "test-tracking")},
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

func TestRepoCheckRoutesAggregateOwnerResults(t *testing.T) {
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
	report, err := repositorycheck.Compose(repositorycheck.Inputs{
		OrdinaryAdvisories:  repositorycheck.Slot{Result: ordinary},
		TrackingInformation: repositorycheck.Slot{Result: tracking},
	})
	if err != nil {
		t.Fatal(err)
	}
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
	for _, want := range []checkresult.Property{"ordinary-property"} {
		if !slices.Contains(properties, want) {
			t.Fatalf("presented properties = %v, missing %q", properties, want)
		}
	}
	for _, want := range []string{"ordinary-information-kind", "tracking-kind"} {
		if !slices.Contains(informationKinds, want) {
			t.Fatalf("presented information kinds = %v, missing %q", informationKinds, want)
		}
	}
	for _, want := range []string{"ordinary warning", "ordinary information", "tracking information"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("typed aggregate output missing %q: %q", want, stdout.String())
		}
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
		OrdinaryAdvisories:  repositorycheck.Slot{Result: warning},
		TrackingInformation: repositorycheck.Slot{Result: information},
	})
	if err != nil {
		t.Fatal(err)
	}
	for target := 1; target <= 4; target++ {
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
	return report
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
