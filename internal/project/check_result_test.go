package project

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func projectTestResult(t *testing.T, findings []checkresult.Finding, information []checkresult.Information) checkresult.Result {
	t.Helper()
	result, err := checkresult.New(findings, information)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProducerResultsNameRankAndProtectedProperty(t *testing.T) {
	producer := projectTestResult(t, []checkresult.Finding{
		{Rank: severity.Error, Property: propertyReproducibility, Evidence: checkresult.Evidence{Kind: "missing", Path: "AGENTS.md", Detail: "file absent"}},
		{Rank: severity.Error, Property: propertyAuthority, Evidence: checkresult.Evidence{Kind: "plan-adr-link", Path: "docs/plans/example.md", Detail: "ADR not found"}},
		{Rank: severity.Error, Property: propertyCorrectness, Evidence: checkresult.Evidence{Kind: "dead-reference", Path: "docs/example.md", Detail: "missing.md"}},
	}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "unused-var", Path: ".awf/config.yaml", Detail: "var is unused"}}})
	advisories := projectTestResult(t, []checkresult.Finding{{Rank: severity.Warn, Property: propertyHeuristic, Evidence: checkresult.Evidence{Kind: "advisory", Detail: "heuristic warning"}}}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "advisory", Detail: "optional cleanup"}}})
	tracking := projectTestResult(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "tracking", Detail: "tracking unavailable"}}})
	planWarnings := projectTestResult(t, []checkresult.Finding{{Rank: severity.Warn, Property: propertyPlanDetail, Evidence: checkresult.Evidence{Kind: "plan-advisory", Detail: "plan warning"}}}, nil)

	report, err := repositorycheck.Compose(repositorycheck.Inputs{
		ProducerResults:      []repositorycheck.Slot{{Result: producer, IncludeInformationInDrift: true}},
		OrdinaryAdvisories:   repositorycheck.Slot{Result: advisories},
		TrackingInformation:  repositorycheck.Slot{Result: tracking},
		DeferredPlanWarnings: repositorycheck.Slot{Result: planWarnings},
	})
	if err != nil {
		t.Fatal(err)
	}
	findings := report.Result.Findings()
	wantProperties := []checkresult.Property{propertyReproducibility, propertyAuthority, propertyCorrectness, propertyHeuristic, propertyPlanDetail}
	for i, want := range wantProperties {
		if findings[i].Property != want {
			t.Errorf("finding %d property = %q, want %q", i, findings[i].Property, want)
		}
	}
	if got := report.Warnings; !slices.Equal(got, []string{"heuristic warning"}) {
		t.Fatalf("Warnings = %v", got)
	}
	if got := report.PlanWarnings; !slices.Equal(got, []string{"plan warning"}) {
		t.Fatalf("PlanWarnings = %v", got)
	}
	if got := report.Information; !slices.Equal(got, []string{"optional cleanup"}) {
		t.Fatalf("Information = %v", got)
	}
	if got := report.TrackingInformation; !slices.Equal(got, []string{"tracking unavailable"}) {
		t.Fatalf("TrackingInformation = %v", got)
	}
}

func TestKnownDynamicPlanDiagnosticCategoriesAreClosed(t *testing.T) {
	for _, category := range []string{"field", "frontmatter", "numbering", "path", "paths", "phase-close", "projection", "relationship", "structure"} {
		if !knownDynamicPlanDiagnosticCategory(category) {
			t.Errorf("known category %q was refused", category)
		}
	}
	if knownDynamicPlanDiagnosticCategory("future-category") {
		t.Fatal("unknown dynamic plan category was accepted")
	}
}

func TestOwnerResultBoundaryRefusesIncompleteEvidence(t *testing.T) {
	if _, err := checkresult.New([]checkresult.Finding{{Rank: severity.Error, Property: propertyCorrectness, Evidence: checkresult.Evidence{Kind: "missing", Path: "AGENTS.md"}}}, nil); err == nil {
		t.Fatal("result boundary accepted a finding without evidence detail")
	}
}

// The construction-identity claim is backed by TestPublishingConsumerPlanIdentity;
// this behavior fixture separately pins the complete generated tracking set.
// invariant: rendering/sync-and-drift:generated-artifacts-tracked (TestCheckReportBuildsOneOutputPlan)
func TestCheckReportBuildsOneOutputPlan(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	root := fixture.Root()
	testsupport.WriteAwfConfig(t, root, withTestGateCmd("prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\ndomains: [config]\n"))
	testsupport.WriteFile(t, filepath.Join(root, ".awf/parts/config-reference/intro.md"), "<!-- awf:stub -->\nConfig intro.\n<!-- awf:section bogus -->\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	reportValue, err := checkReportProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	expectedTrackingPaths := func(p *ProjectState) []string {
		t.Helper()
		corpus, pitfalls, topics, effective, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
		if err != nil {
			t.Fatal(err)
		}
		op, err := outputPlanWithPitfalls(renderInputsForTest(p), corpus, pitfalls, topics, effective)
		if err != nil {
			t.Fatal(err)
		}
		required := map[string]bool{config.DirName + "/awf.lock": true}
		for _, output := range planWriteFiles(op) {
			if p.nested() && resident.IsResidentPath(output.Path) {
				continue
			}
			required[output.Path] = true
		}
		paths := make([]string, 0, len(required))
		for path := range required {
			paths = append(paths, path)
		}
		slices.Sort(paths)
		return paths
	}
	reportTrackingPaths := func(report CheckReport) []string {
		var paths []string
		for _, finding := range report.Drift {
			if finding.Kind == "untracked" {
				paths = append(paths, finding.Path)
			}
		}
		slices.Sort(paths)
		return paths
	}
	if got, want := reportTrackingPaths(reportValue), expectedTrackingPaths(p); !slices.Equal(got, want) {
		t.Errorf("top-level CheckReport tracking paths differ from every OutputPlan write plus lock:\n got %q\nwant %q", got, want)
	}
	directNotes, err := advisoryNotesProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for name, notes := range map[string][]string{"CheckReport": reportValue.Notes, "AdvisoryNotes": directNotes} {
		joined := strings.Join(notes, "\n")
		for _, want := range []string{
			"docs/domains/config.md has unauthored stub content",
			"docs/config-reference.md has unauthored stub content: stub-marked parts: intro",
		} {
			if got := strings.Count(joined, want); got != 2 {
				t.Errorf("%s notes contain planned write node %q %d times, want compatibility multiplicity 2:\n%s", name, want, got, joined)
			}
		}
		marker := "part .awf/parts/config-reference/intro.md contains a marker-shaped line"
		if got := strings.Count(joined, marker); got != 1 {
			t.Errorf("%s marker note multiplicity = %d, want deduplicated 1:\n%s", name, got, joined)
		}
	}

	nestedFixture := gitfixture.InitRepo(t)
	nestedRoot := filepath.Join(nestedFixture.Root(), "nested")
	testsupport.WriteAwfConfig(t, nestedRoot, withTestGateCmd("prefix: example\nprofile: full\nintegrationBranch: main\nvars: {}\n"))
	nestedProject, err := Open(testContext(t), nestedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !nestedProject.nested() {
		t.Fatal("nested project did not preserve its containing-repository prefix")
	}
	if err := syncProject(nestedProject); err != nil {
		t.Fatal(err)
	}
	nestedReport, err := checkReportProject(nestedProject, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := reportTrackingPaths(nestedReport), expectedTrackingPaths(nestedProject); !slices.Equal(got, want) {
		t.Errorf("nested CheckReport tracking paths differ from every non-resident OutputPlan write plus lock:\n got %q\nwant %q", got, want)
	}
}
