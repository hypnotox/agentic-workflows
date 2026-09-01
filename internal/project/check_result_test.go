package project

import (
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

func projectTestResult(t *testing.T, findings []checkresult.Finding, information []checkresult.Information) checkresult.Result {
	t.Helper()
	result, err := checkresult.New(findings, information)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProducerResultsPreserveRankPropertyAndEvidence(t *testing.T) {
	producer := projectTestResult(t, []checkresult.Finding{
		{Rank: severity.Error, Property: propertyReproducibility, Evidence: checkresult.Evidence{Kind: "missing", Path: "AGENTS.md", Detail: "file absent"}},
		{Rank: severity.Error, Property: propertyCorrectness, Evidence: checkresult.Evidence{Kind: "dead-reference", Path: "docs/example.md", Detail: "missing.md"}},
	}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "unused-var", Path: ".awf/config.yaml", Detail: "var is unused"}}})
	advisories := projectTestResult(t, []checkresult.Finding{{Rank: severity.Warn, Property: propertyHeuristic, Evidence: checkresult.Evidence{Kind: "advisory", Detail: "heuristic warning"}}}, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "advisory", Detail: "optional cleanup"}}})
	tracking := projectTestResult(t, nil, []checkresult.Information{{Evidence: checkresult.Evidence{Kind: "tracking", Detail: "tracking unavailable"}}})

	report, err := repositorycheck.Compose(repositorycheck.Inputs{
		ProducerResults:     []repositorycheck.Slot{{Result: producer, IncludeInformationInDirect: true}},
		OrdinaryAdvisories:  repositorycheck.Slot{Result: advisories},
		TrackingInformation: repositorycheck.Slot{Result: tracking},
	})
	if err != nil {
		t.Fatal(err)
	}
	findings := report.Result.Findings()
	wantProperties := []checkresult.Property{propertyReproducibility, propertyCorrectness, propertyHeuristic}
	for i, want := range wantProperties {
		if findings[i].Property != want {
			t.Errorf("finding %d property = %q, want %q", i, findings[i].Property, want)
		}
	}
	var information []string
	for _, item := range report.Result.Information() {
		information = append(information, item.Evidence.Detail)
	}
	if want := []string{"var is unused", "optional cleanup", "tracking unavailable"}; !slices.Equal(information, want) {
		t.Fatalf("information = %v, want %v", information, want)
	}
}

func TestOwnerResultBoundaryRefusesIncompleteEvidence(t *testing.T) {
	if _, err := checkresult.New([]checkresult.Finding{{Rank: severity.Error, Property: propertyCorrectness, Evidence: checkresult.Evidence{Kind: "missing", Path: "AGENTS.md"}}}, nil); err == nil {
		t.Fatal("result boundary accepted a finding without evidence detail")
	}
}

// invariant: rendering/doc-outputs:stub-notes-path-keyed (TestStubNotesKeyByOutputPath)
func TestStubNotesKeyByOutputPath(t *testing.T) {
	files := []RenderedFile{
		{Path: ".claude/skills/example/SKILL.md", TemplateID: "skills/example/SKILL.md.tmpl", stubDefaults: []string{"body"}},
		{Path: ".pi/skills/example/SKILL.md", TemplateID: "skills/example/SKILL.md.tmpl", stubDefaults: []string{"body"}},
	}
	got := stubNotes(files)
	if len(got) != 2 || !strings.Contains(got[0], files[0].Path) || !strings.Contains(got[1], files[1].Path) {
		t.Fatalf("stub notes = %v, want one path-keyed note per rendered output", got)
	}
}
