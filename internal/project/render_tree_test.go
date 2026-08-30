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
	if err := syncProject(p); err != nil {
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
	if err := syncProject(p); err != nil {
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
	op, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range planWriteFiles(op) {
		found = found || file.Path == "AGENTS.md"
	}
	if !found {
		t.Fatal("agents guide is absent from the output plan")
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	report, err := checkReportProject(p, testContext(t))
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
	const cfg = "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/checks\n    title: Checks\n    description: Check local docs.\n"
	const row = "- **Checks:** [docs/runbooks/checks.md](docs/runbooks/checks.md), Check local docs."
	for _, tc := range []struct {
		name, sidecar, part, want, absent string
	}{
		{"default", "", "", "## Document map\n\n- **Decision records:", ""},
		{"headingless part", "", "Custom catalog map content.\n", "## Document map\nCustom catalog map content.\n\n" + row, "**Decision records:"},
		{"heading-bearing part", "", "## Document map\n\nCustom catalog map content.\n", "## Document map\n## Document map\n\nCustom catalog map content.\n\n" + row, "**Decision records:"},
		{"dropped section", "sections:\n  document-map:\n    drop: true\n", "", "## Document map", "**Decision records:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{}
			if tc.sidecar != "" {
				files["agents-doc.yaml"] = tc.sidecar
			}
			if tc.part != "" {
				files["parts/agents-doc/document-map.md"] = tc.part
			}
			got := syncAndReadAgents(t, scaffoldFiles(t, cfg, files))
			if !strings.Contains(got, tc.want) {
				t.Fatalf("document-map ordering missing %q:\n%s", tc.want, got)
			}
			if tc.name == "dropped section" && strings.Index(got, tc.want) > strings.Index(got, row) {
				t.Fatalf("fallback heading does not precede local rows:\n%s", got)
			}
			if tc.absent != "" && strings.Contains(got, tc.absent) {
				t.Fatalf("unexpected catalog content %q:\n%s", tc.absent, got)
			}
			if strings.Contains(got, "<no value>") {
				t.Fatalf("publication-unsafe guide:\n%s", got)
			}
		})
	}
	for _, tc := range []struct{ name, cfg, sidecar string }{
		{"omitted local docs", "prefix: example\nintegrationBranch: main\n", ""},
		{"empty local docs", "prefix: example\nintegrationBranch: main\nlocalDocs: []\n", ""},
		{"dropped empty local docs", "prefix: example\nintegrationBranch: main\n", "sections:\n  document-map:\n    drop: true\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{}
			if tc.sidecar != "" {
				files["agents-doc.yaml"] = tc.sidecar
			}
			got := syncAndReadAgents(t, scaffoldFiles(t, tc.cfg, files))
			if strings.Contains(got, "## Document map") && tc.sidecar != "" {
				t.Fatalf("dropped empty map retained structure:\n%s", got)
			}
			if strings.Contains(got, row) || strings.Contains(got, "<no value>") {
				t.Fatalf("empty local docs changed guide publication:\n%s", got)
			}
		})
	}
}

// invariant: rendering/doc-outputs:local-doc-output-complete (TestAgentsDocDefaultEmptyLocalDocsByteInertia)
func TestAgentsDocDefaultEmptyLocalDocsByteInertia(t *testing.T) {
	const suffix = "<!-- awf:edit document-map: default; create .awf/parts/agents-doc/document-map.md to override -->\n## Document map\n\n- **Decision records:**"
	for _, cfg := range []string{
		"prefix: example\nintegrationBranch: main\n",
		"prefix: example\nintegrationBranch: main\nlocalDocs: []\n",
	} {
		got := syncAndReadAgents(t, scaffoldFiles(t, cfg, nil))
		if !strings.Contains(got, suffix) {
			t.Fatalf("default document-map boundary changed:\n%s", got)
		}
		if strings.Contains(got, "<no value>") {
			t.Fatalf("default empty guide is not publication-safe:\n%s", got)
		}
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
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	// Reopening makes the second sync read the in-place local outputs back.
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	report, err := checkReportProject(p, testContext(t))
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
	if err := syncProject(p); err != nil {
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

func TestTopicPartUsesRawPublicationSafeAssembly(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root)
	p, _ := Open(testContext(t), root)
	if err := syncProject(p); err != nil {
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
