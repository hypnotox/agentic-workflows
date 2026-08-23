package checkop

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const checkYAML = `prefix: example
profile: full
integrationBranch: main
vars: {testCmd: go test ./..., gateCmd: make gate}
`

func stagedCheckProject(t *testing.T, commit map[string]string) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	committed := make(map[string]string, len(commit)+1)
	for path, body := range commit {
		committed[path] = body
	}
	if _, ok := committed[".awf/awf.lock"]; !ok {
		lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
		body, err := lock.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		committed[".awf/awf.lock"] = string(body)
	}
	gitfixture.Stage(t, repo, committed)
	gitfixture.Commit(t, repo, "head", nil)
	return repo.Root()
}

func TestCollectCheckStagedRetainsMissingLockFailureAndReportsVersionSkew(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	repo := gitfixture.At(root)
	gitfixture.StageRemoval(t, repo, ".awf/awf.lock")
	collection, err := collectCheckStagedSelectionWith(context.Background(), root, planNoteSink{}, true, false, productionCheckStagedDependencies())
	if err != nil {
		t.Fatal(err)
	}
	if len(collection.operational) != 1 || !errors.Is(collection.operational[0], errNoStagedLock) {
		t.Fatalf("missing staged lock operational errors = %v", collection.operational)
	}

	lock := &manifest.Lock{AWFVersion: "v0.0.0", SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	body, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": string(body)})
	collection, err = collectCheckStagedSelectionWith(context.Background(), root, planNoteSink{}, false, false, productionCheckStagedDependencies())
	if err != nil {
		t.Fatal(err)
	}
	if len(collection.information) != 1 || !strings.Contains(collection.information[0], "ahead of this project") {
		t.Fatalf("version skew information = %v", collection.information)
	}
}

func TestRunCheckStagedContinuesAfterStatePresentationFailure(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	stateFailure := errors.New("state category mapping failed")
	driftFailure := errors.New("staged drift failed")
	dependencies := productionCheckStagedDependencies()
	dependencies.present = func(checkresult.Result, string, bool) (repositorycheck.Presentation, error) {
		return repositorycheck.Presentation{}, stateFailure
	}
	driftRan := false
	dependencies.driftRoot = func(context.Context, string) (checkresult.Result, error) {
		driftRan = true
		return checkresult.Result{}, driftFailure
	}
	var stdout bytes.Buffer
	collection, err := collectCheckStagedWith(context.Background(), root, planNoteSink{}, dependencies)
	if err != nil {
		t.Fatalf("collection error = %v, want operational failures retained in the collection", err)
	}
	err = renderCheckCollection(&stdout, collection)
	if !driftRan {
		t.Fatal("staged drift did not run after state presentation failure")
	}
	if !errors.Is(err, stateFailure) || !errors.Is(err, driftFailure) {
		t.Fatalf("operational error = %v, want joined state and drift failures", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want suppressed partial report", stdout.String())
	}
}

func TestCollectCheckStagedContinuesAfterPlanWarningPresentationFailure(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	planResult, err := checkresult.New([]checkresult.Finding{{
		Rank: severity.Warn, Property: "plan-detail-quality",
		Evidence: checkresult.Evidence{Kind: "plan-assignment", Detail: "staged-plan-note-sentinel"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	planFailure := errors.New("plan warning presentation failed")
	dependencies := productionCheckStagedDependencies()
	dependencies.stateRoot = func(context.Context, string) (project.CurrentStateReport, error) {
		return project.CurrentStateReport{PlanResult: planResult, PlanArtifactResult: planResult, OwnerResult: planResult}, nil
	}
	present := dependencies.present
	dependencies.present = func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
		if check == "advisory" {
			return repositorycheck.Presentation{}, planFailure
		}
		return present(result, check, evidence)
	}
	driftRan := false
	dependencies.driftRoot = func(context.Context, string) (checkresult.Result, error) {
		driftRan = true
		return checkresult.New(nil, nil)
	}
	collection, err := collectCheckStagedWith(context.Background(), root, planNoteSink{}, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if !driftRan {
		t.Fatal("staged drift did not run after plan warning presentation failure")
	}
	if len(collection.operational) != 1 || !errors.Is(collection.operational[0], planFailure) {
		t.Fatalf("operational errors = %v, want plan warning presentation failure", collection.operational)
	}
	var stdout bytes.Buffer
	if err := renderCheckCollection(&stdout, collection); !errors.Is(err, planFailure) {
		t.Fatalf("render error = %v, want plan warning presentation failure", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want suppressed partial report", stdout.String())
	}
}

func TestCollectCheckStagedRoutesOrdinaryCurrentStateOwnerResults(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	ordinaryWarning := checkresult.Finding{
		Rank: severity.Warn, Property: "current-state-coverage",
		Evidence: checkresult.Evidence{Kind: "current-state", Detail: "ordinary coverage warning"},
	}
	planWarning := checkresult.Finding{
		Rank: severity.Warn, Property: "plan-detail-quality",
		Evidence: checkresult.Evidence{Kind: "plan-assignment", Detail: "plan warning"},
	}
	planResult, err := checkresult.New([]checkresult.Finding{planWarning}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ordinaryResult, err := checkresult.New([]checkresult.Finding{ordinaryWarning}, []checkresult.Information{{
		Evidence: checkresult.Evidence{Kind: "current-state", Detail: "ordinary provisional information"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ownerResult, err := checkresult.New([]checkresult.Finding{ordinaryWarning, planWarning}, ordinaryResult.Information())
	if err != nil {
		t.Fatal(err)
	}
	dependencies := productionCheckStagedDependencies()
	dependencies.stateRoot = func(context.Context, string) (project.CurrentStateReport, error) {
		return project.CurrentStateReport{
			CurrentResult:      ordinaryResult,
			PlanArtifactResult: planResult,
			OwnerResult:        ownerResult,
			PlanResult:         planResult,
			PlanNotes:          []string{"mutated compatibility plan warning"},
			Provisional:        []currentstate.Introduction{{Identity: "mutated-compatibility-information"}},
		}, nil
	}
	present := dependencies.present
	seen := map[checkresult.Property]bool{}
	informationSeen := false
	dependencies.present = func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
		for _, finding := range result.Findings() {
			seen[finding.Property] = true
		}
		for _, item := range result.Information() {
			if item.Evidence.Kind == "current-state" {
				informationSeen = true
			}
		}
		return present(result, check, evidence)
	}
	collection, err := collectCheckStagedSelectionWith(context.Background(), root, planNoteSink{}, true, false, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := renderCheckCollection(&stdout, collection); err != nil {
		t.Fatal(err)
	}
	if !seen["current-state-coverage"] || !seen["plan-detail-quality"] || !informationSeen {
		t.Fatalf("staged typed route lost owner results: properties=%v information=%v", seen, informationSeen)
	}
	for _, want := range []string{"ordinary coverage warning", "plan warning", "ordinary provisional information"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("staged typed output missing %q: %q", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "mutated compatibility") || strings.Contains(stdout.String(), "mutated-compatibility-information") {
		t.Fatalf("compatibility projection changed staged routing: %q", stdout.String())
	}
}

func TestCollectCheckStagedRoutesTypedPlanWarningsOnce(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	planWarning := checkresult.Finding{
		Rank: severity.Warn, Property: "plan-detail-quality",
		Evidence: checkresult.Evidence{Kind: "plan-assignment", Detail: "staged-plan-note-sentinel"},
	}
	planResult, err := checkresult.New([]checkresult.Finding{planWarning, planWarning}, nil)
	if err != nil {
		t.Fatal(err)
	}
	dependencies := productionCheckStagedDependencies()
	dependencies.stateRoot = func(context.Context, string) (project.CurrentStateReport, error) {
		return project.CurrentStateReport{PlanResult: planResult, PlanArtifactResult: planResult, OwnerResult: planResult, PlanNotes: []string{"staged-plan-note-sentinel"}}, nil
	}
	present := dependencies.present
	typedWarningSeen := false
	dependencies.present = func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
		if check == "advisory" {
			findings := result.Findings()
			if len(findings) != 1 || findings[0].Rank != severity.Warn || findings[0].Property != "plan-detail-quality" {
				t.Fatalf("staged plan presentation result = %#v", findings)
			}
			typedWarningSeen = true
		}
		return present(result, check, evidence)
	}
	collection, err := collectCheckStagedSelectionWith(context.Background(), root, planNoteSink{}, true, false, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := renderCheckCollection(&stdout, collection); err != nil {
		t.Fatal(err)
	}
	if !typedWarningSeen {
		t.Fatal("staged plan warning did not reach presentation as an owner-classified result")
	}
	want := "status: warnings\n\nsummary:\n  findings: 0 errors, 1 warnings, 0 information\n\nfindings:\n  warnings:\n    advisory | staged-plan-note-sentinel\n"
	if got := stdout.String(); got != want {
		t.Fatalf("typed staged plan warning output = %q, want %q", got, want)
	}

	collection, err = collectCheckStagedSelectionWith(context.Background(), root, planNoteSink{"staged-plan-note-sentinel": {}}, true, false, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := renderCheckCollection(&stdout, collection); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "staged-plan-note-sentinel") {
		t.Fatalf("shared plan warning was not deduplicated: %q", stdout.String())
	}

	dependencies.stateRoot = func(context.Context, string) (project.CurrentStateReport, error) {
		return project.CurrentStateReport{PlanNotes: []string{"legacy-plan-note-must-not-route"}}, nil
	}
	collection, err = collectCheckStagedSelectionWith(context.Background(), root, planNoteSink{}, true, false, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := renderCheckCollection(&stdout, collection); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "legacy-plan-note-must-not-route") {
		t.Fatalf("legacy PlanNotes routed staged warning policy: %q", stdout.String())
	}
}

func TestCollectCheckStagedRetainsStateFailureWhenDriftCategoryMappingFails(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	driftFailure := errors.New("drift category mapping failed")
	dependencies := productionCheckStagedDependencies()
	dependencies.stateRoot = func(context.Context, string) (project.CurrentStateReport, error) {
		return currentStateReportForTest(t, project.CurrentStateReport{Static: []currentstate.Finding{{Message: "state-failure-sentinel"}}}), nil
	}
	dependencies.driftRoot = func(context.Context, string) (checkresult.Result, error) { return checkresult.New(nil, nil) }
	dependencies.present = func(result checkresult.Result, check string, evidence bool) (repositorycheck.Presentation, error) {
		if check == "staged drift" {
			return repositorycheck.Presentation{}, driftFailure
		}
		if evidence {
			return repositorycheck.PresentEvidence(result, check)
		}
		return repositorycheck.Present(result, check)
	}
	collection, err := collectCheckStagedSelectionWith(context.Background(), root, planNoteSink{}, true, true, dependencies)
	if err != nil {
		t.Fatalf("collection error = %v, want retained operational failures", err)
	}
	if len(collection.failures) != 1 || collection.failures[0].Error() != "check staged state failed" {
		t.Fatalf("state failures = %v, want staged state failure", collection.failures)
	}
	if len(collection.presentation.Errors) == 0 {
		t.Fatal("state errors were discarded")
	}
	if len(collection.operational) != 1 || !errors.Is(collection.operational[0], driftFailure) {
		t.Fatalf("operational failures = %v, want drift category failure", collection.operational)
	}
}
