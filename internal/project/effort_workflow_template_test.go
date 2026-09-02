package project

import (
	"strings"
	"testing"
)

func TestEffortWorkflowTemplateCreationCommand(t *testing.T) {
	out := renderGolden(t, "skills/effort-workflow/SKILL.md.tmpl", map[string]any{
		"prefix": "example",
	})
	const command = "Create with `./awf effort new --slug <slug> \"<title>\"`."
	if !strings.Contains(out, command) {
		t.Errorf("expected exact effort creation command %q:\n%s", command, out)
	}
	if strings.Contains(out, "ordinary `./awf effort` creation command") {
		t.Errorf("effort workflow retains ambiguous creation guidance:\n%s", out)
	}
}
