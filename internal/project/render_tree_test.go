package project

import (
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
	p, err := Open(root)
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
	p, err := Open(root)
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

// invariant: agentsdoc-parts
func TestAgentsDocPartsOverride(t *testing.T) {
	cfg := "prefix: example\nskills: []\nagents: []\n"

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

func writeFileAt(t *testing.T, root, rel, body string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// invariant: parts-convention
func TestConventionPartPrecedence(t *testing.T) {
	cfg := "prefix: example\n" + debuggingVars + "skills: [debugging]\nagents: []\n"
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

// invariant: sidecar-optional
func TestSidecarAbsentRendersDefault(t *testing.T) {
	cfg := "prefix: example\n" + debuggingVars + "skills: [debugging]\nagents: []\n"
	root := scaffold(t, cfg) // no sidecar, no parts
	out := syncAndReadDebugging(t, root)
	if strings.Contains(out, "<no value>") {
		t.Errorf("absent sidecar must render the template default with no <no value>:\n%s", out)
	}
	if !strings.Contains(out, "Enumerate observable surfaces") {
		t.Errorf("expected the template default body:\n%s", out)
	}
}

// invariant: local-frontmatter
func TestLocalFrontmatterChecked(t *testing.T) {
	cfg := "prefix: example\nskills: [my-local]\nagents: []\n"
	root := scaffoldFiles(t, cfg, map[string]string{"skills/my-local.yaml": "local: true\n"})
	out := ".claude/skills/example-my-local/SKILL.md"

	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// (a) A declared local target with no on-disk file is a sync error.
	if err := p.Sync(); err == nil {
		t.Error("expected sync error: local skill file absent")
	}
	// (b) Present but with empty name/description fails identically to a rendered target.
	writeFileAt(t, root, out, "---\nname: \"\"\ndescription: \"\"\n---\nbody\n")
	if err := p.Sync(); err == nil {
		t.Error("expected sync error: local skill has empty frontmatter")
	}
	// (c) Valid frontmatter → sync succeeds and check reports no frontmatter drift.
	writeFileAt(t, root, out, "---\nname: my-local\ndescription: a local skill\n---\nbody\n")
	if err := p.Sync(); err != nil {
		t.Fatalf("sync should succeed with valid local frontmatter: %v", err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if d.Kind == "invalid-frontmatter" {
			t.Errorf("unexpected invalid-frontmatter drift: %#v", d)
		}
	}
}
