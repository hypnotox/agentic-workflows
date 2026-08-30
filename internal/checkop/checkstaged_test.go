package checkop

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
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
		lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{"prior": {}}}
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

	lock := &manifest.Lock{AWFVersion: "v0.0.0", SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{"prior": {}}}
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

func TestCollectCheckStagedRoutesOrdinaryCurrentStateOwnerResults(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	ordinaryWarning := checkresult.Finding{
		Rank: severity.Warn, Property: "current-state-coverage",
		Evidence: checkresult.Evidence{Kind: "current-state", Detail: "ordinary coverage warning"},
	}
	ordinaryResult, err := checkresult.New([]checkresult.Finding{ordinaryWarning}, []checkresult.Information{{
		Evidence: checkresult.Evidence{Kind: "current-state", Detail: "ordinary provisional information"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	dependencies := productionCheckStagedDependencies()
	dependencies.stateRoot = func(context.Context, string) (project.CurrentStateReport, error) {
		return project.CurrentStateReport{CurrentResult: ordinaryResult, OwnerResult: ordinaryResult}, nil
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
	if !seen["current-state-coverage"] || informationSeen == false {
		t.Fatalf("staged typed route lost owner results: properties=%v information=%v", seen, informationSeen)
	}
	for _, want := range []string{"ordinary coverage warning", "ordinary provisional information"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("staged typed output missing %q: %q", want, stdout.String())
		}
	}
}
