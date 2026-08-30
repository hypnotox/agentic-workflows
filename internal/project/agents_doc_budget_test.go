package project

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

const (
	directDefaultAgentsDocBudget = 8 * 1024
	selfHostedAgentsDocBudget    = 10 * 1024
)

// largestAgentsDocSections reports the largest rendered sections for test
// diagnostics only. Production rendering deliberately has no section-attribution
// model.
func largestAgentsDocSections(body string) string {
	type contribution struct {
		name  string
		bytes int
	}
	var sections []contribution
	lines := strings.SplitAfter(body, "\n")
	name := "preamble"
	bytes := 0
	appendSection := func() {
		sections = append(sections, contribution{name: name, bytes: bytes})
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			appendSection()
			name = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			bytes = 0
		}
		bytes += len(line)
	}
	appendSection()
	sort.Slice(sections, func(i, j int) bool {
		if sections[i].bytes != sections[j].bytes {
			return sections[i].bytes > sections[j].bytes
		}
		return sections[i].name < sections[j].name
	})
	parts := make([]string, 0, min(3, len(sections)))
	for _, section := range sections[:min(3, len(sections))] {
		parts = append(parts, fmt.Sprintf("%s=%d", section.name, section.bytes))
	}
	return strings.Join(parts, ", ")
}

func agentsDocBudgetDiagnostic(surface, body string, allowed int) (string, bool) {
	observed := len(body)
	return fmt.Sprintf("%s is %d bytes, allowed %d bytes; largest sections: %s", surface, observed, allowed, largestAgentsDocSections(body)), observed > allowed
}

func requireAgentsDocBudget(t *testing.T, surface, body string, allowed int) {
	t.Helper()
	diagnostic, over := agentsDocBudgetDiagnostic(surface, body, allowed)
	t.Log(diagnostic)
	if over {
		t.Error(diagnostic)
	}
}

// invariant: rendering/guide-and-doc-templates:agent-guide-size-budgets (TestDirectDefaultAgentsDocBudget)
func TestDirectDefaultAgentsDocBudget(t *testing.T) {
	body := renderGolden(t, "agents-doc/AGENTS.md.tmpl", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{},
	})
	requireAgentsDocBudget(t, "direct default AGENTS.md", body, directDefaultAgentsDocBudget)
}

// invariant: rendering/guide-and-doc-templates:agent-guide-size-budgets (TestSelfHostedAgentsDocBudget)
func TestSelfHostedAgentsDocBudget(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRootDir(t), "AGENTS.md"))
	if err != nil {
		t.Fatalf("read committed AGENTS.md: %v", err)
	}
	requireAgentsDocBudget(t, "self-hosted AGENTS.md", string(body), selfHostedAgentsDocBudget)
}

// invariant: rendering/guide-and-doc-templates:agent-guide-size-budgets (TestAgentsDocBudgetDiagnostics)
func TestAgentsDocBudgetDiagnostics(t *testing.T) {
	body := "# Guide\n\n## Small\nx\n## Largest\n1234567890\n## Middle\n12345\n"
	diagnostic, over := agentsDocBudgetDiagnostic("fixture AGENTS.md", body, len(body)-1)
	want := "fixture AGENTS.md is 58 bytes, allowed 57 bytes; largest sections: Largest=22, Middle=16, Small=11"
	if !over || diagnostic != want {
		t.Fatalf("over-budget diagnostic = %q, over = %v; want %q, true", diagnostic, over, want)
	}
	if _, over := agentsDocBudgetDiagnostic("fixture AGENTS.md", body, len(body)); over {
		t.Fatal("exact budget boundary reported an overage")
	}
}

// invariant: rendering/sync-and-drift:agent-guide-size-advisory (TestAgentGuideSizeAdvisoryBoundary)
// invariant: rendering/sync-and-drift:agent-guide-size-advisory (TestAgentGuideSizeAdvisoryBoundary)
func TestAgentGuideSizeAdvisoryBoundary(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bytes int
		want  bool
	}{
		{name: "below", bytes: 12*1024 - 1},
		{name: "boundary", bytes: 12 * 1024},
		{name: "over", bytes: 12*1024 + 1, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\naudit:\n  allowedScopes:\n    - name: awf\n", map[string]string{"parts/agents-doc/identity.md": "x"})
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			op, err := outputPlanProject(p)
			if err != nil {
				t.Fatal(err)
			}
			var actual int
			for _, file := range planWriteFiles(op) {
				if file.Path == "AGENTS.md" {
					actual = len(file.Content)
				}
			}
			testsupport.WriteFile(t, filepath.Join(root, ".awf/parts/agents-doc/identity.md"), strings.Repeat("x", tc.bytes-actual+1))
			p, err = Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			op, err = outputPlanProject(p)
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range planWriteFiles(op) {
				if file.Path == "AGENTS.md" && len(file.Content) != tc.bytes {
					t.Fatalf("expected guide bytes = %d, want %d", len(file.Content), tc.bytes)
				}
			}
			if err := syncProject(p); err != nil {
				t.Fatal(err)
			}
			report, err := checkReportProject(p, testContext(t))
			if err != nil {
				t.Fatal(err)
			}
			var notes []string
			for _, note := range report.Notes {
				if strings.Contains(note, "AGENTS.md") && strings.Contains(note, "12288") {
					notes = append(notes, note)
				}
			}
			if tc.want {
				var classified bool
				for _, finding := range report.Result.Findings() {
					classified = classified || finding.Rank == severity.Warn && finding.Property == "heuristic-quality" && strings.Contains(finding.Evidence.Detail, "12289")
				}
				if !classified {
					t.Fatalf("CheckReport omitted owner-classified guide warning: %#v", report.Result.Findings())
				}
				if len(notes) != 1 || !strings.Contains(notes[0], "12289") || !strings.Contains(notes[0], "docs/agents-md-standard.md") {
					t.Fatalf("overage note = %#v", notes)
				}
				// The size advisory remains aggregate-only and follows ordinary
				// advisories. Use the scaffold's stub-content advisory rather than
				// the retired plan scope checker to preserve that ordering contract.
				ordinaryIndex := slices.IndexFunc(report.Notes, func(note string) bool { return strings.Contains(note, "unauthored stub content") })
				sizeIndex := slices.Index(report.Notes, notes[0])
				if ordinaryIndex < 0 || sizeIndex < 0 || ordinaryIndex >= sizeIndex {
					t.Fatalf("CheckReport notes do not place ordinary advisory before size advisory: %#v", report.Notes)
				}
				direct, err := advisoryNotesProject(p)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(strings.Join(direct, "\n"), "12288") {
					t.Fatalf("AdvisoryNotes included aggregate-only size note: %#v", direct)
				}
				for _, resident := range []string{"missing", "stale"} {
					if resident == "missing" {
						if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
							t.Fatal(err)
						}
					} else {
						testsupport.WriteFile(t, filepath.Join(root, "AGENTS.md"), "stale")
					}
					residentReport, err := checkReportProject(p, testContext(t))
					if err != nil {
						t.Fatal(err)
					}
					if got := strings.Count(strings.Join(residentReport.Notes, "\n"), "12289"); got != 1 {
						t.Fatalf("%s resident size notes = %d, want 1: %#v", resident, got, residentReport.Notes)
					}
				}
				return
			}
			if len(notes) != 0 {
				t.Fatalf("notes = %#v, want none", notes)
			}
		})
	}
}
