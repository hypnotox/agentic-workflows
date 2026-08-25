package project

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// CheckStagedDrift preserves legacy test fixtures while production consumers
// use the owner-classified semantic result directly.
func CheckStagedDrift(prep *ContextPreparation, plan outputplan.Plan) ([]manifest.Drift, error) {
	result, err := CheckStagedDriftResult(prep, plan)
	if err != nil {
		return nil, err
	}
	return stagedCompatibilityDrift(result), nil
}

func stagedCompatibilityDrift(result checkresult.Result) []manifest.Drift {
	findings := result.Findings()
	drift := make([]manifest.Drift, len(findings))
	for i, finding := range findings {
		drift[i] = manifest.Drift{Path: finding.Evidence.Path, Kind: finding.Evidence.Kind, Detail: finding.Evidence.Detail}
	}
	return drift
}

// TestCheckStagedRefusesHistoricalWorkflowTelemetry proves staged validation is
// live-source validation, not an audit forward-decoding path.
func TestCheckStagedRefusesHistoricalWorkflowTelemetry(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, map[string]string{
		".awf/config.yaml": "prefix: example\nprofile: full\nintegrationBranch: main\nworkflowTelemetry:\n  retention: {}\n",
		".awf/awf.lock":    `{"awfVersion":"0.20.0","schemaVersion":19,"files":{}}`,
	})
	gitfixture.Commit(t, repo, "generation 19", nil)
	gitfixture.Stage(t, repo, map[string]string{
		".awf/config.yaml": "prefix: example\nprofile: full\nintegrationBranch: main\n",
		".awf/awf.lock":    `{"awfVersion":"0.20.0","schemaVersion":20,"files":{}}`,
	})
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); err == nil || !strings.Contains(err.Error(), "live floor 46") {
		t.Fatalf("CheckStaged historical source error = %v", err)
	}
}

func TestCheckStagedRefusesHistoricalMalformedOrDuplicateConfigAndCurrentInvalidBlock(t *testing.T) {
	t.Parallel()
	currentLock := fmt.Sprintf(`{"schemaVersion":%d,"files":{}}`, migrate.Current())
	for _, tc := range []struct {
		name, headConfig, headLock, stagedConfig, stagedLock string
	}{
		{"historical malformed", "prefix: [\nworkflowTelemetry: {}\n", `{"schemaVersion":19,"files":{}}`, "prefix: example\nprofile: full\nintegrationBranch: main\n", `{"schemaVersion":20,"files":{}}`},
		{"historical duplicate", "prefix: example\nprofile: full\nintegrationBranch: main\nworkflowTelemetry: {}\nworkflowTelemetry: {}\n", `{"schemaVersion":19,"files":{}}`, "prefix: example\nprofile: full\nintegrationBranch: main\n", `{"schemaVersion":20,"files":{}}`},
		{"current invalid", "prefix: example\nprofile: full\nintegrationBranch: main\n", currentLock, "prefix: example\nprofile: full\nintegrationBranch: main\nunknown: true\n", currentLock},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gitfixture.InitRepo(t)
			dir := repo.Root()
			gitfixture.Stage(t, repo, map[string]string{
				".awf/config.yaml": tc.headConfig,
				".awf/awf.lock":    tc.headLock,
			})
			gitfixture.Commit(t, repo, "head", nil)
			gitfixture.Stage(t, repo, map[string]string{
				".awf/config.yaml": tc.stagedConfig,
				".awf/awf.lock":    tc.stagedLock,
			})
			p := testStateAt(dir)
			if _, err := checkStagedProject(p, testContext(t)); err == nil {
				t.Fatal("CheckStaged accepted invalid config")
			}
		})
	}
}
