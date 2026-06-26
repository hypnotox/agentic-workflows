package render

import (
	"strings"
	"testing"
)

func sampleData() map[string]any {
	return map[string]any{
		"prefix": "example",
		"vars":   map[string]any{"testCmd": "go test ./...", "gateCmd": "make gate"},
		"data": map[string]any{
			"testSurfaces": []any{
				map[string]any{"name": "Logic", "location": "internal", "kind": "Go unit"},
			},
		},
	}
}

const tmpl = "# {{ .prefix }}\n\n<!-- awf:section surfaces -->\nS:{{ range .data.testSurfaces }}{{ .name }}{{ end }}\n<!-- awf:end -->\n\nrun {{ .vars.testCmd }}\n<!-- awf:section notes -->\nNOTE\n<!-- awf:end -->\n"

func TestRenderDefault(t *testing.T) {
	out, err := Render(tmpl, nil, sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# example") || !strings.Contains(out, "S:Logic") ||
		!strings.Contains(out, "run go test ./...") || !strings.Contains(out, "NOTE") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if !strings.Contains(out, "<!-- awf:edit surfaces — default;") ||
		!strings.Contains(out, "<!-- awf:edit notes — default;") {
		t.Errorf("default edit pointers missing:\n%s", out)
	}
	if strings.Contains(out, "awf:section") || strings.Contains(out, "awf:end") {
		t.Errorf("markers leaked into output:\n%s", out)
	}
}

func TestRenderDropsSection(t *testing.T) {
	plan := map[string]SectionPlan{"notes": {Drop: true}}
	out, err := Render(tmpl, plan, sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "NOTE") {
		t.Errorf("notes section should be dropped:\n%s", out)
	}
	if !strings.Contains(out, "S:Logic") {
		t.Errorf("surfaces section should remain:\n%s", out)
	}
}

func TestRenderConventionPart(t *testing.T) {
	plan := map[string]SectionPlan{"notes": {HasPart: true, PartBody: "CUSTOM {{ .prefix }}", EditPath: ".awf/x.md"}}
	out, err := Render(tmpl, plan, sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CUSTOM example") || strings.Contains(out, "NOTE") {
		t.Errorf("convention part substitution failed:\n%s", out)
	}
	if !strings.Contains(out, "<!-- awf:edit notes — from .awf/x.md -->") {
		t.Errorf("convention part pointer missing:\n%s", out)
	}
}

func TestExecuteParseError(t *testing.T) {
	_, err := Execute("{{ .prefix", sampleData())
	if err == nil {
		t.Fatal("expected parse error from malformed template, got nil")
	}
	if !strings.Contains(err.Error(), "parse template") {
		t.Errorf("error missing parse context: %q", err.Error())
	}
}

func TestExecuteExecError(t *testing.T) {
	// .prefix is a string; ranging over it is a parse-valid but execution-time error.
	_, err := Execute("{{ range .prefix }}{{ end }}", sampleData())
	if err == nil {
		t.Fatal("expected execution error, got nil")
	}
	if !strings.Contains(err.Error(), "execute template") {
		t.Errorf("error missing execute context: %q", err.Error())
	}
}
