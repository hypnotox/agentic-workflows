package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// debuggingVars seeds every var the debugging skill template references so it
// renders without a <no value> token.
const debuggingVars = `vars:
  debuggingDoc: ""
  gateCmd: ""
  gateCmdFull: ""
  workflowDoc: ""
`

// syncAndReadDebugging syncs the project and returns the rendered debugging skill
// (the target the convention-part tests drive).
func syncAndReadDebugging(t *testing.T, root string) string {
	t.Helper()
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	rel := ".claude/skills/example-debugging/SKILL.md"
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// syncAndReadAgents syncs the project and returns the rendered AGENTS.md.
func syncAndReadAgents(t *testing.T, root string) string {
	t.Helper()
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	return string(b)
}

// invariant: rendering/sync-and-drift:agent-guide-size-advisory (TestCheckReportAgentGuideSizeAdvisoryManagedOnly)
func TestCheckReportAgentGuideSizeAdvisoryManagedOnly(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\n", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range op.writeFiles() {
		found = found || file.Path == "AGENTS.md"
	}
	if !found {
		t.Fatal("agents guide is absent from the output plan")
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	report, err := p.CheckReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, note := range report.Notes {
		if strings.Contains(note, "AGENTS.md") && strings.Contains(note, "12288") {
			t.Fatalf("local agents document size note = %q", note)
		}
	}
}

// invariant: rendering/guide-and-doc-templates:agentsdoc-parts (TestAgentsDocDocumentMapPartRetainsLocalDocs)
// invariant: rendering/doc-outputs:local-doc-output-complete (TestAgentsDocDocumentMapPartRetainsLocalDocs)
func TestAgentsDocDocumentMapPartRetainsLocalDocs(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/checks\n    title: Checks\n    description: Check local docs.\n", map[string]string{
		"parts/agents-doc/document-map.md": "## Document map\n\nCustom catalog map content.\n",
	})
	got := syncAndReadAgents(t, root)
	if !strings.Contains(got, "Custom catalog map content.") {
		t.Fatalf("document-map part did not replace catalog content:\n%s", got)
	}
	if strings.Contains(got, "**ADR index:**") {
		t.Errorf("catalog body remained after replacement:\n%s", got)
	}
	line := "- **Checks:** [docs/runbooks/checks.md](docs/runbooks/checks.md), Check local docs."
	if count := strings.Count(got, line); count != 1 {
		t.Errorf("local document row count = %d, want 1:\n%s", count, got)
	}
}

// invariant: rendering/doc-outputs:local-doc-output-complete (TestLocalDocGuideSize)
func TestLocalDocGuideSize(t *testing.T) {
	var entries strings.Builder
	for i := range 100 {
		fmt.Fprintf(&entries, "  - name: runbooks/doc-%03d\n    title: Local document %03d\n    description: %s\n", i, i, strings.Repeat("x", 100))
	}
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n"+entries.String())
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	// Reopening makes the second sync read the in-place local outputs back.
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	report, err := p.CheckReport(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Drift) != 0 {
		t.Fatalf("guide-size advisory must remain non-failing: %#v", report.Drift)
	}
	if !strings.Contains(strings.Join(report.Notes, "\n"), "12288") {
		t.Fatalf("expected unchanged 12288-byte advisory: %#v", report.Notes)
	}
}

// invariant: rendering/guide-and-doc-templates:agentsdoc-parts (TestAgentsDocPartsOverride)
func TestAgentsDocPartsOverride(t *testing.T) {
	cfg := "prefix: example\nintegrationBranch: main\n"

	// Absent → the generic, adopter-neutral default renders publication-safe with
	// empty invariants/docMap.
	def := syncAndReadAgents(t, scaffold(t, cfg))
	if strings.Contains(def, "<no value>") {
		t.Errorf("default agents-doc must be publication-safe:\n%s", def)
	}
	if !strings.Contains(def, "is a software project") {
		t.Errorf("expected the generic identity default:\n%s", def)
	}

	// A convention part overrides the identity section body.
	got := syncAndReadAgents(t, scaffoldFiles(t, cfg, map[string]string{
		"parts/agents-doc/identity.md": "## Identity\n\nExample is a widget.\n",
	}))
	if !strings.Contains(got, "Example is a widget.") {
		t.Errorf("convention part should override the identity section:\n%s", got)
	}
	if strings.Contains(got, "is a software project") {
		t.Errorf("the part should replace the generic default:\n%s", got)
	}
}

// invariant: rendering/guide-and-doc-templates:maintainable-code-design-guide (TestMaintainableCodeDesignPartOverride)
func TestMaintainableCodeDesignPartOverride(t *testing.T) {
	const uniqueBody = "The local decision posture owns this change."
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"parts/maintainable-code-design/decision-posture.md": uniqueBody + "\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(root, "docs/maintainable-code-design.md"))
	if err != nil {
		t.Fatalf("read guide: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, uniqueBody) {
		t.Errorf("decision-posture override missing:\n%s", got)
	}
	if strings.Contains(got, "Start with the requested behavior and the surrounding model") {
		t.Errorf("default decision-posture body was not replaced:\n%s", got)
	}
	for _, heading := range []string{"## SOLID, DRY, and YAGNI", "## Semantic modeling", "## Boundaries and dependency direction", "## Illustrative pattern toolbox", "## Preparatory refactoring", "## Failure modes"} {
		if !strings.Contains(got, heading) {
			t.Errorf("override removed unrelated heading %q:\n%s", heading, got)
		}
	}
}

func TestConventionPartPrecedence(t *testing.T) {
	cfg := "prefix: example\nintegrationBranch: main\n" + debuggingVars + ""
	const part = "skills/parts/debugging/debugging-surfaces.md"

	// (1) A convention part present replaces the section body.
	root := scaffoldFiles(t, cfg, map[string]string{part: "CONVENTION PART BODY\n"})
	out := syncAndReadDebugging(t, root)
	if !strings.Contains(out, "CONVENTION PART BODY") {
		t.Errorf("convention part not rendered:\n%s", out)
	}
	if strings.Contains(out, "Enumerate observable surfaces") {
		t.Errorf("template default should be replaced by the convention part:\n%s", out)
	}

	// (2) A sidecar drop beats the convention part.
	root = scaffoldFiles(t, cfg, map[string]string{
		part:                    "CONVENTION PART BODY\n",
		"skills/debugging.yaml": "sections:\n  debugging-surfaces:\n    drop: true\n",
	})
	out = syncAndReadDebugging(t, root)
	if strings.Contains(out, "CONVENTION PART BODY") {
		t.Errorf("drop should beat the convention part:\n%s", out)
	}
}

// invariant: rendering/render-engine:sidecar-optional (TestSidecarAbsentRendersDefault)
func TestSidecarAbsentRendersDefault(t *testing.T) {
	cfg := "prefix: example\nintegrationBranch: main\n" + debuggingVars + ""
	root := scaffold(t, cfg) // no sidecar, no parts
	out := syncAndReadDebugging(t, root)
	if strings.Contains(out, "<no value>") {
		t.Errorf("absent sidecar must render the template default with no <no value>:\n%s", out)
	}
	if !strings.Contains(out, "Enumerate observable surfaces") {
		t.Errorf("expected the template default body:\n%s", out)
	}
}

// A local skill must exist with valid frontmatter at EVERY enabled target's path
// (ADR-0037): one present, the other absent, is a fail at the missing target.
func TestTopicPartUsesRawPublicationSafeAssembly(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "contracts", "Contracts", "paths: [\"internal/**\"]\n")
	p, _ := Open(testContext(t), root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(root, "docs/topics/rendering/contracts.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "{{ .value }}") || strings.Contains(string(out), "awf:comment") || strings.Contains(string(out), "<no value>") {
		t.Fatalf("topic output:\n%s", out)
	}
}
