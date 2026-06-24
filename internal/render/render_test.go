package render

import (
	"strings"
	"testing"

	"agentic-workflows/internal/config"
)

func noParts(name string) (string, error) { return "", nil }

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
	out, err := Render(tmpl, nil, noParts, sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# example") || !strings.Contains(out, "S:Logic") ||
		!strings.Contains(out, "run go test ./...") || !strings.Contains(out, "NOTE") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if strings.Contains(out, "awf:section") || strings.Contains(out, "awf:end") {
		t.Errorf("markers leaked into output:\n%s", out)
	}
}

func TestRenderDropsSection(t *testing.T) {
	ov := map[string]config.SectionOverride{"notes": {Drop: true}}
	out, err := Render(tmpl, ov, noParts, sampleData())
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

func TestRenderReplaceWith(t *testing.T) {
	ov := map[string]config.SectionOverride{"notes": {ReplaceWith: "parts/notes.md"}}
	parts := func(name string) (string, error) {
		if name != "parts/notes.md" {
			t.Fatalf("unexpected part %q", name)
		}
		return "CUSTOM {{ .prefix }}", nil
	}
	out, err := Render(tmpl, ov, parts, sampleData())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CUSTOM example") || strings.Contains(out, "NOTE") {
		t.Errorf("replaceWith failed:\n%s", out)
	}
}
