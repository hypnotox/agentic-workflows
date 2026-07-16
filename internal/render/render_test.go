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
	asm, parts := Assemble(ParseSections(tmpl), nil, HTMLComment)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# example") || !strings.Contains(out, "S:Logic") ||
		!strings.Contains(out, "run go test ./...") || !strings.Contains(out, "NOTE") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if !strings.Contains(out, "<!-- awf:edit surfaces: default;") ||
		!strings.Contains(out, "<!-- awf:edit notes: default;") {
		t.Errorf("default edit pointers missing:\n%s", out)
	}
	// invariant: no-section-marker-leak
	if strings.Contains(out, "awf:section") || strings.Contains(out, "awf:end") {
		t.Errorf("markers leaked into output:\n%s", out)
	}
}

func TestRenderDropsSection(t *testing.T) {
	plan := map[string]SectionPlan{"notes": {Drop: true}}
	asm, parts := Assemble(ParseSections(tmpl), plan, HTMLComment)
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
	asm, parts := Assemble(ParseSections(tmpl), plan, HTMLComment)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CUSTOM {{ .prefix }}") || strings.Contains(out, "NOTE") {
		t.Errorf("convention part substitution failed:\n%s", out)
	}
	if !strings.Contains(out, "<!-- awf:edit notes: from .awf/x.md -->") {
		t.Errorf("convention part pointer missing:\n%s", out)
	}
}

func TestEmptyPartRendersEmptyNotDropped(t *testing.T) {
	// ADR-0034 item 4: an empty part yields an empty section body (the section and
	// its awf:edit pointer remain), distinct from a drop which removes both.
	plan := map[string]SectionPlan{"notes": {HasPart: true, PartBody: "", EditPath: ".awf/x.md"}}
	asm, parts := Assemble(ParseSections(tmpl), plan, HTMLComment)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- awf:edit notes: from .awf/x.md -->") {
		t.Errorf("empty part must keep the section pointer (not dropped):\n%s", out)
	}
	if strings.Contains(out, "NOTE") {
		t.Errorf("empty part must replace the default body, not keep it:\n%s", out)
	}
	if strings.Contains(out, "\x00") {
		t.Errorf("empty part's sentinel leaked instead of restoring to empty:\n%s", out)
	}
}

func TestEditPointerStub(t *testing.T) {
	stubTmpl := "<!-- awf:section notes stub -->\nNOTE\n<!-- awf:end -->\n"
	asm, parts := Assemble(ParseSections(stubTmpl), map[string]SectionPlan{"notes": {EditPath: ".awf/x.md"}}, HTMLComment)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	// invariant: section-edit-pointer
	if !strings.Contains(out, "<!-- awf:edit notes: stub; replace by creating .awf/x.md -->") {
		t.Errorf("stub default must render the stub pointer:\n%s", out)
	}
	asm, parts = Assemble(ParseSections(stubTmpl),
		map[string]SectionPlan{"notes": {HasPart: true, PartBody: "CUSTOM", EditPath: ".awf/x.md"}}, HTMLComment)
	out, err = Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- awf:edit notes: from .awf/x.md -->") {
		t.Errorf("part-backed stub section must keep the from-pointer:\n%s", out)
	}
	// Non-stub default pointer unchanged (also asserted by TestRenderDefault).
	asm, parts = Assemble(ParseSections(tmpl), nil, HTMLComment)
	out, err = Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- awf:edit notes: default;") {
		t.Errorf("non-stub default pointer changed:\n%s", out)
	}
}

func TestAssembleInPlaceSection(t *testing.T) {
	src := "head\n<!-- awf:section body inplace -->\nDEFAULT\n<!-- awf:end -->\ntail\n"
	segs := ParseSections(src)

	// invariant: in-place-pointer-distinct
	// A non-empty read-back body is emitted verbatim (internal blank line kept)
	// after the distinct awf:edit-in-place pointer: no re-templating.
	body := "line one\n\nline two\n"
	asm, parts := Assemble(segs, map[string]SectionPlan{"body": {InPlace: true, InPlaceFound: true, InPlaceBody: body}}, HTMLComment)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- awf:edit-in-place body: your edits below are preserved across syncs; awf owns the rest -->") {
		t.Errorf("in-place section must render the awf:edit-in-place pointer:\n%s", out)
	}
	if !strings.Contains(out, body) {
		t.Errorf("in-place body must render verbatim (internal blank line kept):\n%s", out)
	}
	// The distinct pointer is not awf:section/awf:end-shaped, so it survives
	// assembly without tripping the residual-marker guard.
	if err := CheckResidualMarkers(asm); err != nil {
		t.Errorf("awf:edit-in-place pointer must not trip the residual-marker guard: %v", err)
	}

	// An empty read-back body (first render, absent output) falls to the
	// template default.
	asm, parts = Assemble(segs, map[string]SectionPlan{"body": {InPlace: true}}, HTMLComment)
	out, err = Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- awf:edit-in-place body:") || !strings.Contains(out, "DEFAULT") {
		t.Errorf("empty in-place body must fall to the template default:\n%s", out)
	}

	// A located but emptied region (InPlaceFound, empty body) stays empty - it is
	// NOT reverted to the template default, so emptying a region is a fixpoint.
	asm, parts = Assemble(segs, map[string]SectionPlan{"body": {InPlace: true, InPlaceFound: true, InPlaceBody: ""}}, HTMLComment)
	out, err = Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "DEFAULT") {
		t.Errorf("an emptied (found) in-place region must not revert to the default:\n%s", out)
	}
	if !strings.Contains(out, "<!-- awf:edit-in-place body:") {
		t.Errorf("an emptied in-place region must still render its pointer:\n%s", out)
	}
}

// The pointer comment style follows the target (ADR-0100 Decision 7): a
// #!-shebang target emits #-comment pointers, everything else HTML. All four
// awf:edit-family variants switch style; neither trips the residual-marker guard.
func TestCommentStyleForSourceAndPointers(t *testing.T) {
	if got := CommentStyleForSource("#!/usr/bin/env bash\ncase x in\n"); got != HashComment {
		t.Errorf("a #!-shebang source must sniff HashComment, got %v", got)
	}
	if got := CommentStyleForSource("# Markdown H1 is not a shebang\n"); got != HTMLComment {
		t.Errorf("a non-shebang source must sniff HTMLComment, got %v", got)
	}

	// invariant: in-place-pointer-distinct
	// The distinct awf:edit-in-place pointer renders in the target's comment
	// syntax - # for a shebang target, <!-- --> otherwise.
	src := "#!/usr/bin/env bash\n<!-- awf:section body inplace -->\nDEFAULT\n<!-- awf:end -->\n"
	segs := ParseSections(src)
	style := CommentStyleForSource(src)
	asm, parts := Assemble(segs, map[string]SectionPlan{"body": {InPlace: true, InPlaceFound: true, InPlaceBody: "echo hi\n"}}, style)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "# awf:edit-in-place body: your edits below are preserved across syncs; awf owns the rest\n") {
		t.Errorf("shell target must render a #-comment in-place pointer:\n%s", out)
	}
	if strings.Contains(out, "<!-- awf:edit-in-place") {
		t.Errorf("shell target must NOT render an HTML pointer:\n%s", out)
	}
	if err := CheckResidualMarkers(asm); err != nil {
		t.Errorf("#-style pointer must not trip the residual-marker guard: %v", err)
	}

	// The three ordinary awf:edit variants (from-part / stub / default) also
	// switch style. Each is exercised in HashComment vs HTMLComment.
	variants := []struct {
		name  string
		tmpl  string
		plan  map[string]SectionPlan
		token string
	}{
		{"from-part", "<!-- awf:section s -->\nD\n<!-- awf:end -->\n",
			map[string]SectionPlan{"s": {HasPart: true, PartBody: "P", EditPath: ".awf/s.md"}},
			"awf:edit s: from .awf/s.md"},
		{"stub", "<!-- awf:section s stub -->\nD\n<!-- awf:end -->\n",
			map[string]SectionPlan{"s": {EditPath: ".awf/s.md"}},
			"awf:edit s: stub; replace by creating .awf/s.md"},
		{"default", "<!-- awf:section s -->\nD\n<!-- awf:end -->\n",
			map[string]SectionPlan{},
			"awf:edit s: default; create .awf/s.md to override"},
	}
	for _, v := range variants {
		vsegs := ParseSections(v.tmpl)
		if v.name == "default" {
			v.plan = map[string]SectionPlan{"s": {EditPath: ".awf/s.md"}}
		}
		hashAsm, hp := Assemble(vsegs, v.plan, HashComment)
		hashOut, err := Execute(hashAsm, sampleData(), hp, "t")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(hashOut, "# "+v.token+"\n") {
			t.Errorf("%s: HashComment must render `# %s`:\n%s", v.name, v.token, hashOut)
		}
		htmlAsm, tp := Assemble(vsegs, v.plan, HTMLComment)
		htmlOut, err := Execute(htmlAsm, sampleData(), tp, "t")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(htmlOut, "<!-- "+v.token+" -->\n") {
			t.Errorf("%s: HTMLComment must render `<!-- %s -->`:\n%s", v.name, v.token, htmlOut)
		}
	}
}

func TestStubSections(t *testing.T) {
	src := "<!-- awf:section dropped stub -->\nD\n<!-- awf:end -->\n" +
		"<!-- awf:section parted -->\nP\n<!-- awf:end -->\n" +
		"<!-- awf:section stubbed stub -->\nS\n<!-- awf:end -->\n" +
		"<!-- awf:section plain -->\nN\n<!-- awf:end -->\n"
	plan := map[string]SectionPlan{
		"dropped": {Drop: true},
		"parted":  {HasPart: true, PartBody: "<!-- awf:stub -->\nwip\n", PartStub: true},
		"stubbed": {},
		"plain":   {},
	}
	defaults, parts := StubSections(ParseSections(src), plan)
	if len(defaults) != 1 || defaults[0] != "stubbed" {
		t.Errorf("defaults = %#v, want [stubbed]", defaults)
	}
	if len(parts) != 1 || parts[0] != "parted" {
		t.Errorf("parts = %#v, want [parted]", parts)
	}
}

func TestAssembleStubPartRendersVerbatim(t *testing.T) {
	plan := map[string]SectionPlan{"notes": {
		HasPart:  true,
		PartBody: "<!-- awf:stub -->\nstarter prose\n",
		PartStub: true,
		EditPath: ".awf/x.md",
	}}
	asm, parts := Assemble(ParseSections(tmpl), plan, HTMLComment)
	out, err := Execute(asm, sampleData(), parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!-- awf:stub -->\nstarter prose") {
		t.Errorf("stub-marked part must render verbatim, marker included:\n%s", out)
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
	asm, parts := Assemble(ParseSections(tmpl), plan, HTMLComment)
	out, err := Execute(asm, sampleData(), parts, "raw-test")
	if err != nil {
		t.Fatalf("Execute over a part with literal braces must not error: %v", err)
	}
	want := "Literal braces survive: {{ .vars.x }} {{ if }} }} and a mustache {{name}}."
	// invariant: parts-raw-except-authoring-comments
	if !strings.Contains(out, want) {
		t.Fatalf("part body must render verbatim (not interpolated)\n got: %q\nwant substring: %q", out, want)
	}
	if strings.Contains(out, "<no value>") || strings.Contains(out, "\x00") {
		t.Fatalf("part body was interpolated or a sentinel leaked: %q", out)
	}
}

func TestSectionDefaultSplice(t *testing.T) {
	tmpl := "<!-- awf:section body -->\ndefault={{ .v }}\n<!-- awf:end -->\n"
	segs := ParseSections(tmpl)
	data := map[string]any{"v": "R"}
	cases := []struct{ name, part, want string }{
		{"append", SectionDefaultSentinel + "\nEXTRA", "default=R\nEXTRA"},
		{"prepend", "PRE\n" + SectionDefaultSentinel, "PRE\ndefault=R"},
		{"wrap", "PRE\n" + SectionDefaultSentinel + "\nPOST", "PRE\ndefault=R\nPOST"},
		{"multi", "A" + SectionDefaultSentinel + "B" + SectionDefaultSentinel + "C", "Adefault=RBdefault=RC"},
		{"fragment-raw", "{{ .v }}" + SectionDefaultSentinel, "{{ .v }}default=R"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := map[string]SectionPlan{"body": {HasPart: true, PartBody: c.part}}
			asm, parts := Assemble(segs, plan, HTMLComment)
			out, err := Execute(asm, data, parts, "t")
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("%s: got %q, want substring %q", c.name, out, c.want)
			}
		})
	}
}

func TestSectionDefaultSpliceEmptyDefault(t *testing.T) {
	segs := ParseSections("<!-- awf:section body -->\n<!-- awf:end -->\n")
	plan := map[string]SectionPlan{"body": {HasPart: true, PartBody: "PRE" + SectionDefaultSentinel + "POST"}}
	asm, parts := Assemble(segs, plan, HTMLComment)
	out, err := Execute(asm, map[string]any{}, parts, "t")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "PREPOST") {
		t.Errorf("empty-default re-injection: got %q, want substring %q", out, "PREPOST")
	}
}

func TestCheckSectionDefaultStubs(t *testing.T) {
	stubSegs := ParseSections("<!-- awf:section body stub -->\nprompt\n<!-- awf:end -->\n")
	// A stub section whose part re-injects the default is an error.
	err := CheckSectionDefaultStubs(stubSegs, map[string]SectionPlan{"body": {HasPart: true, PartBody: "x" + SectionDefaultSentinel}})
	if err == nil {
		t.Fatal("stub default re-injection: want error, got nil")
	}
	// A stub section with a plain (non-re-injecting) part is fine.
	if err := CheckSectionDefaultStubs(stubSegs, map[string]SectionPlan{"body": {HasPart: true, PartBody: "authored"}}); err != nil {
		t.Errorf("plain stub part: unexpected error %v", err)
	}
	// A non-stub section re-injecting its default is fine; a literal (non-section) segment is skipped.
	okSegs := ParseSections("lead\n<!-- awf:section body -->\ndef\n<!-- awf:end -->\n")
	if err := CheckSectionDefaultStubs(okSegs, map[string]SectionPlan{"body": {HasPart: true, PartBody: SectionDefaultSentinel}}); err != nil {
		t.Errorf("non-stub re-injection: unexpected error %v", err)
	}
}
