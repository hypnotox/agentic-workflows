package checkop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/prosegate"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

type repoCheckCounters struct {
	loads, opens, reports, states, indexes int
}

func repoCheckTestDependencies(t *testing.T, cfg *config.Config, p *project.Session, check project.CheckReport, state currentstatecoord.CurrentStateReport, tree *snapshot.Tree, counts *repoCheckCounters) repoCheckDependencies {
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
		loadSession: func(_ context.Context, _ string, got *config.Config) (*project.Session, error) {
			counts.opens++
			if got != cfg {
				t.Fatalf("loadSession config = %p, want prepared config %p", got, cfg)
			}
			return p, nil
		},
		checkReport: func(_ context.Context, got *project.Session) (project.CheckReport, error) {
			counts.reports++
			if got != p {
				t.Fatalf("checkReport session = %p, want prepared session %p", got, p)
			}
			return check, nil
		},
		currentState: func(_ context.Context, got *project.Session) (currentstatecoord.CurrentStateReport, error) {
			counts.states++
			if got != p {
				t.Fatalf("currentState session = %p, want prepared session %p", got, p)
			}
			return state, nil
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
	deps := repoCheckTestDependencies(t, &config.Config{}, &project.Session{}, report, currentstatecoord.CurrentStateReport{}, nil, &repoCheckCounters{})
	collection, err := collectRepoCheckSelection(context.Background(), t.TempDir(), []repositoryLane{repositoryDrift}, true, true, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	var properties []checkresult.Property
	var informationKinds []string
	for _, reported := range collection.results {
		for _, finding := range reported.result.Findings() {
			properties = append(properties, finding.Property)
		}
		for _, item := range reported.result.Information() {
			informationKinds = append(informationKinds, item.Evidence.Kind)
		}
	}
	if !slices.Contains(properties, "ordinary-property") {
		t.Fatalf("reported properties = %v", properties)
	}
	for _, want := range []string{"ordinary-information-kind", "tracking-kind"} {
		if !slices.Contains(informationKinds, want) {
			t.Fatalf("reported information kinds = %v, missing %q", informationKinds, want)
		}
	}
	var stdout bytes.Buffer
	if err := renderCheckCollection(&stdout, collection); err != nil {
		t.Fatal(err)
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
	state := currentstatecoord.CurrentStateReport{CurrentResult: result, OwnerResult: result}
	deps := repoCheckTestDependencies(t, &config.Config{}, &project.Session{}, project.CheckReport{}, state, nil, &repoCheckCounters{})
	collection, err := collectRepoCheckSelection(context.Background(), t.TempDir(), []repositoryLane{repositoryState}, false, false, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(collection.results) != 1 || collection.results[0].check != "current-state" {
		t.Fatalf("current-state results = %#v", collection.results)
	}
	var stdout bytes.Buffer
	if err := renderCheckCollection(&stdout, collection); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "coverage warning") || !strings.Contains(stdout.String(), "provisional information") {
		t.Fatalf("working current-state output = %q", stdout.String())
	}
}

func TestRepoCheckTypedPreparationOrderAndCancellation(t *testing.T) {
	cfg := &config.Config{}
	session := &project.Session{}
	tree := mustSnapshotTree(t)
	counts := &repoCheckCounters{}
	deps := repoCheckTestDependencies(t, cfg, session, project.CheckReport{}, currentstatecoord.CurrentStateReport{}, tree, counts)
	var events []string
	loadConfig := deps.loadConfig
	deps.loadConfig = func(root string) (*config.Config, error) {
		events = append(events, "config")
		return loadConfig(root)
	}
	loadSession := deps.loadSession
	deps.loadSession = func(ctx context.Context, root string, got *config.Config) (*project.Session, error) {
		events = append(events, "session")
		return loadSession(ctx, root, got)
	}
	checkReport := deps.checkReport
	deps.checkReport = func(ctx context.Context, got *project.Session) (project.CheckReport, error) {
		events = append(events, "report")
		return checkReport(ctx, got)
	}
	currentState := deps.currentState
	deps.currentState = func(ctx context.Context, got *project.Session) (currentstatecoord.CurrentStateReport, error) {
		events = append(events, "state")
		return currentState(ctx, got)
	}
	indexTree := deps.indexTree
	deps.indexTree = func(ctx context.Context, root string) (*snapshot.Tree, error) {
		events = append(events, "index")
		return indexTree(ctx, root)
	}
	collection, err := collectRepoCheckSelection(context.Background(), t.TempDir(), orderedRepositoryLanes(), true, true, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"config", "session", "report", "state", "index"}; !slices.Equal(events, want) {
		t.Fatalf("preparation order = %v, want %v", events, want)
	}
	if counts.loads != 1 || counts.opens != 1 || counts.reports != 1 || counts.states != 1 || counts.indexes != 1 {
		t.Fatalf("preparation counts = %#v", counts)
	}
	var labels []string
	for _, result := range collection.results {
		labels = append(labels, result.check)
	}
	if want := []string{"advisory", "advisory", "advisory", "drift", "current-state", "prose", "memory"}; !slices.Equal(labels, want) {
		t.Fatalf("result order = %v, want %v", labels, want)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled, err := collectRepoCheckSelection(ctx, t.TempDir(), orderedRepositoryLanes(), true, true, nil, repoCheckTestDependencies(t, cfg, session, project.CheckReport{}, currentstatecoord.CurrentStateReport{}, tree, &repoCheckCounters{}))
	if !errors.Is(err, context.Canceled) || len(canceled.results) != 0 {
		t.Fatalf("pre-canceled collection = %#v, error = %v", canceled, err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	canceling := repoCheckTestDependencies(t, cfg, session, project.CheckReport{}, currentstatecoord.CurrentStateReport{}, tree, &repoCheckCounters{})
	indexTree = canceling.indexTree
	canceling.indexTree = func(ctx context.Context, root string) (*snapshot.Tree, error) {
		prepared, err := indexTree(ctx, root)
		cancel()
		return prepared, err
	}
	canceled, err = collectRepoCheckSelection(ctx, t.TempDir(), []repositoryLane{repositoryProse}, false, false, nil, canceling)
	if !errors.Is(err, context.Canceled) || len(canceled.results) != 0 {
		t.Fatalf("post-preparation cancellation = %#v, error = %v", canceled, err)
	}
}

func TestRepoCheckPreparationFailureProducesNoPartialResult(t *testing.T) {
	failure := errors.New("state preparation failed")
	deps := repoCheckTestDependencies(t, &config.Config{}, &project.Session{}, project.CheckReport{}, currentstatecoord.CurrentStateReport{}, mustSnapshotTree(t), &repoCheckCounters{})
	indexRan := false
	deps.currentState = func(context.Context, *project.Session) (currentstatecoord.CurrentStateReport, error) {
		return currentstatecoord.CurrentStateReport{}, failure
	}
	deps.indexTree = func(context.Context, string) (*snapshot.Tree, error) {
		indexRan = true
		return mustSnapshotTree(t), nil
	}
	collection, err := collectRepoCheckSelection(context.Background(), t.TempDir(), orderedRepositoryLanes(), true, true, nil, deps)
	if !errors.Is(err, failure) || len(collection.results) != 0 || indexRan {
		t.Fatalf("failed preparation collection = %#v, error = %v, indexRan = %t", collection, err, indexRan)
	}
}

func TestRepoCheckIndexPreparationPreservesScannerErrorText(t *testing.T) {
	failure := errors.New("index unavailable")
	deps := repoCheckTestDependencies(t, &config.Config{}, &project.Session{}, project.CheckReport{}, currentstatecoord.CurrentStateReport{}, mustSnapshotTree(t), &repoCheckCounters{})
	deps.indexTree = func(context.Context, string) (*snapshot.Tree, error) {
		return nil, &repoIndexPreparationError{err: fmt.Errorf("cannot read staged files: %w", failure)}
	}
	_, err := collectRepoCheckSelection(context.Background(), t.TempDir(), []repositoryLane{repositoryProse}, false, false, nil, deps)
	if !errors.Is(err, failure) || err.Error() != "check repo prose: cannot read staged files: index unavailable" {
		t.Fatalf("scanner preparation error = %v", err)
	}
}

func TestRepoCheckAggregateContinuesProducedFailuresInLaneOrder(t *testing.T) {
	drift := checkTestResult(t, []checkresult.Finding{checkFinding(severity.Error, "correctness", "missing", "AGENTS.md", "missing")}, nil)
	report := project.CheckReport{DirectResult: drift}
	stateResult := checkTestResult(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "current-state", Detail: "state completed"}}})
	state := currentstatecoord.CurrentStateReport{CurrentResult: stateResult, OwnerResult: stateResult}
	deps := repoCheckTestDependencies(t, &config.Config{}, &project.Session{}, report, state, nil, &repoCheckCounters{})
	collection, err := collectRepoCheckSelection(context.Background(), t.TempDir(), []repositoryLane{repositoryState, repositoryDrift}, true, false, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(collection.failures) != 1 {
		t.Fatalf("produced failures = %v", collection.failures)
	}
	var labels []string
	for _, result := range collection.results {
		labels = append(labels, result.check)
	}
	if want := []string{"advisory", "drift", "current-state"}; !slices.Equal(labels, want) {
		t.Fatalf("continued result order = %v, want %v", labels, want)
	}
}

func currentStateReportForTest(t *testing.T, report currentstatecoord.CurrentStateReport) currentstatecoord.CurrentStateReport {
	t.Helper()
	return report
}

// invariant: rendering/sync-and-drift:agent-guide-size-advisory (TestAggregateCheckAgentGuideSizeWarning)
func TestAggregateCheckAgentGuideSizeWarning(t *testing.T) {
	cfg := &config.Config{}
	p := &project.Session{}
	advisory := "AGENTS.md is 12289 bytes, allowed 12288 bytes; see docs/agents-md-standard.md"
	runAggregate := func(t *testing.T, notes []string) string {
		t.Helper()
		counts := &repoCheckCounters{}
		deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Warnings: notes, Notes: notes}, currentstatecoord.CurrentStateReport{}, nil, counts)
		var out bytes.Buffer
		if err := runRepoCheckSelection(context.Background(), t.TempDir(), &out, []repositoryLane{repositoryDrift}, true, true, deps); err != nil {
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
	deps := repoCheckTestDependencies(t, cfg, p, project.CheckReport{Warnings: []string{"ordinary-advisory", advisory}, Notes: []string{"ordinary-advisory", advisory}}, currentstatecoord.CurrentStateReport{}, nil, counts)
	var direct bytes.Buffer
	if err := runRepoCheckSelection(context.Background(), t.TempDir(), &direct, []repositoryLane{repositoryDrift}, false, false, deps); err != nil {
		t.Fatalf("direct drift error: %v", err)
	}
	if got := direct.String(); got != completedCheckReport {
		t.Fatalf("direct drift output = %q, want no advisory", got)
	}
}
