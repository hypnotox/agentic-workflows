package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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
