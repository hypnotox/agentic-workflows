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
	asm, parts := Assemble(ParseSections(tmpl), nil)
	out, err := Execute(asm, sampleData(), parts, "test")
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
	asm, parts := Assemble(ParseSections(tmpl), plan)
	out, err := Execute(asm, sampleData(), parts, "test")
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
	asm, parts := Assemble(ParseSections(tmpl), plan)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CUSTOM {{ .prefix }}") || strings.Contains(out, "NOTE") {
		t.Errorf("convention part substitution failed:\n%s", out)
	}
	if !strings.Contains(out, "<!-- awf:edit notes — from .awf/x.md -->") {
		t.Errorf("convention part pointer missing:\n%s", out)
	}
}

func TestEmptyPartRendersEmptyNotDropped(t *testing.T) {
	// ADR-0034 item 4: an empty part yields an empty section body (the section and
	// its awf:edit pointer remain), distinct from a drop which removes both.
	plan := map[string]SectionPlan{"notes": {HasPart: true, PartBody: "", EditPath: ".awf/x.md"}}
	asm, parts := Assemble(ParseSections(tmpl), plan)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- awf:edit notes — from .awf/x.md -->") {
		t.Errorf("empty part must keep the section pointer (not dropped):\n%s", out)
	}
	if strings.Contains(out, "NOTE") {
		t.Errorf("empty part must replace the default body, not keep it:\n%s", out)
	}
	if strings.Contains(out, "\x00") {
		t.Errorf("empty part's sentinel leaked instead of restoring to empty:\n%s", out)
	}
}

func TestExecuteParseError(t *testing.T) {
	_, err := Execute("{{ .prefix", sampleData(), nil, "test")
	if err == nil {
		t.Fatal("expected parse error from malformed template, got nil")
	}
	if !strings.Contains(err.Error(), "parse template") {
		t.Errorf("error missing parse context: %q", err.Error())
	}
}

func TestExecuteExecError(t *testing.T) {
	// .prefix is a string; ranging over it is a parse-valid but execution-time error.
	_, err := Execute("{{ range .prefix }}{{ end }}", sampleData(), nil, "test")
	if err == nil {
		t.Fatal("expected execution error, got nil")
	}
	if !strings.Contains(err.Error(), "execute template") {
		t.Errorf("error missing execute context: %q", err.Error())
	}
}

func TestPartBodyIsRawNeverTemplated(t *testing.T) {
	tmpl := "<!-- awf:section body -->\nDEFAULT {{ .prefix }}\n<!-- awf:end -->\n"
	plan := map[string]SectionPlan{"body": {
		HasPart:  true,
		PartBody: "Literal braces survive: {{ .vars.x }} {{ if }} }} and a mustache {{name}}.",
		EditPath: ".awf/x/parts/y/body.md",
	}}
	asm, parts := Assemble(ParseSections(tmpl), plan)
	out, err := Execute(asm, sampleData(), parts, "raw-test")
	if err != nil {
		t.Fatalf("Execute over a part with literal braces must not error: %v", err)
	}
	want := "Literal braces survive: {{ .vars.x }} {{ if }} }} and a mustache {{name}}."
	if !strings.Contains(out, want) {
		t.Fatalf("part body must render verbatim (not interpolated)\n got: %q\nwant substring: %q", out, want)
	}
	if strings.Contains(out, "<no value>") || strings.Contains(out, "\x00") {
		t.Fatalf("part body was interpolated or a sentinel leaked: %q", out)
	}
}
