package plan_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/plan"
)

const v1Plan = `---
format: plan-v1
date: 2026-08-02
adrs: []
status: Proposed
---
# Plan: Example

## Goal

Deliver the thing without widening its scope.

## Goal detail

This level-two heading remains opaque Goal Markdown.

## Architecture summary

Keep parsing and rendering in the model owner.

## Phase 1: Parse

**Execution mode: inline.**

### Task 1.1: Build it
Kind: batch
Latitude: exact
Paths: ["glob:internal/plan/*.go", "pathspec::(top)internal/plan", "docs/plans/template.md"]
Representative: Cover normal input.
Edge: Cover invalid input.
Post-check: go test ./internal/plan

Implement the parser.

### Task 1.2: Investigate
Kind: spike
Question: Which errors are stable?

### Phase close

Run the staged check and gate.

` + "```commit\nfeat(plans): parse plans\n```" + `

## Phase 2: Expose

**Execution mode: subagent-driven.**

### Task 2.1: Add the reader

Expose the parsed projection.

### Phase close

Run the staged check and gate.

` + "```commit\nfeat(plans): expose reads\n```" + `

## Definition of done

- A valid plan parses and projects.

## Notes

The spike established stable typed diagnostics.
`

// invariant: adr-system/plan-artifacts:plan-v1-structure-validated (TestPlanV1StructureValidated)
func TestPlanV1StructureValidated(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-example.md", v1Plan)
	plans, err := plan.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans = %#v", plans)
	}
	got := plans[0]
	if got.Format != "plan-v1" || got.Title != "Example" || len(got.Phases) != 2 {
		t.Fatalf("parsed plan = %#v", got)
	}
	if got.Phases[0].ExecutionMode != plan.ExecutionInline || got.Phases[1].ExecutionMode != plan.ExecutionSubagentDriven {
		t.Fatalf("execution modes = %q, %q", got.Phases[0].ExecutionMode, got.Phases[1].ExecutionMode)
	}
	batch := got.Phases[0].Tasks[0]
	if batch.Fields.Kind != plan.TaskBatch || batch.Fields.Latitude != plan.TaskExact || len(batch.Fields.Paths) != 3 {
		t.Fatalf("batch fields = %#v", batch.Fields)
	}
	if batch.Fields.Paths[0].Kind != plan.PathGlob || batch.Fields.Paths[0].Value != "internal/plan/*.go" ||
		batch.Fields.Paths[1].Kind != plan.PathPathspec || batch.Fields.Paths[1].Value != ":(top)internal/plan" ||
		batch.Fields.Paths[2].Kind != plan.PathLiteral {
		t.Fatalf("typed paths = %#v", batch.Fields.Paths)
	}
	if !strings.Contains(got.Goal, "## Goal detail") {
		t.Fatalf("opaque Goal lost nested heading: %q", got.Goal)
	}
	t.Run("diagnostics", TestPlanV1Diagnostics)
	t.Run("path grammar", TestPlanV1PathGrammar)
	t.Run("fenced list syntax", TestPlanV1FencedListSyntaxIsOpaque)
	t.Run("fence grammar", TestPlanV1FenceGrammarIsOpaque)
	t.Run("definition bullet markers", TestPlanV1DefinitionBulletMarkers)
	t.Run("legacy boundary", TestPlanV1AbsentFormatRemainsLegacy)
	t.Run("fenced headings", TestPlanV1RetiredHeadingInsideFenceIsOpaque)
}

func TestPlanV2BatchOptionalExamples(t *testing.T) {
	dir := t.TempDir()
	withoutExamples := strings.ReplaceAll(v1Plan,
		"Representative: Cover normal input.\nEdge: Cover invalid input.\n", "")
	withoutExamples = strings.Replace(withoutExamples, "format: plan-v1", "format: plan-v2", 1)
	withoutExamples = strings.Replace(withoutExamples, "- A valid plan parses and projects.", "- `dod: valid-plan` A valid plan parses and projects.", 1)
	writePlan(t, dir, "2026-08-02-example.md", withoutExamples)
	plans, err := plan.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir batch without optional examples: %v", err)
	}
	batch := plans[0].Phases[0].Tasks[0].Fields
	if batch.Kind != plan.TaskBatch || len(batch.Paths) == 0 || batch.PostCheck == "" {
		t.Fatalf("parsed batch = %#v", batch)
	}
	if batch.Representative != "" || batch.Edge != "" {
		t.Fatalf("optional examples unexpectedly populated: %#v", batch)
	}
}

func TestPlanDiagnosticRendering(t *testing.T) {
	withoutPath := (&plan.Diagnostic{Category: "structure", Detail: "broken"}).Error()
	if withoutPath != "plan structure: broken" {
		t.Fatalf("pathless diagnostic = %q", withoutPath)
	}
	single := (&plan.DiagnosticsError{Diagnostics: []*plan.Diagnostic{{Category: "field", Path: "one.md", Detail: "broken"}}}).Error()
	if single != "plan field at one.md: broken" {
		t.Fatalf("single diagnostic aggregate = %q", single)
	}
	multiple := (&plan.DiagnosticsError{Diagnostics: []*plan.Diagnostic{
		{Category: "field", Path: "one.md", Detail: "broken"},
		{Category: "paths", Path: "two.md", Detail: "escaped"},
	}}).Error()
	if multiple != "2 plan diagnostics (first: plan field at one.md: broken)" {
		t.Fatalf("multiple diagnostic aggregate = %q", multiple)
	}
}

func TestPlanV1Diagnostics(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		category string
		detail   string
	}{
		{"unknown format", replaceOnceForTest(v1Plan, "format: plan-v1", "format: plan-v3"), "frontmatter", "format must be exactly plan-v1 or plan-v2"},
		{"empty format", replaceOnceForTest(v1Plan, "format: plan-v1", "format: \"\""), "frontmatter", "format must be a nonempty string"},
		{"duplicate format", replaceOnceForTest(v1Plan, "format: plan-v1", "format: plan-v1\nformat: plan-v1"), "frontmatter", "duplicate format"},
		{"nonmapping frontmatter", replaceOnceForTest(v1Plan, "format: plan-v1\ndate: 2026-08-02\nadrs: []\nstatus: Proposed", "- format: plan-v1"), "frontmatter", "frontmatter must be a mapping"},
		{"malformed frontmatter", replaceOnceForTest(v1Plan, "adrs: []", "adrs: ["), "frontmatter", "yaml: line 2: did not find expected ',' or ']'"},
		{"missing title", replaceOnceForTest(v1Plan, "# Plan: Example", "# Plan:"), "structure", "expected # Plan: title"},
		{"top-level order", replaceOnceForTest(v1Plan, "## Goal", "## Architecture summary"), "structure", "expected ## Goal after title"},
		{"empty goal", replaceOnceForTest(v1Plan, "\nDeliver the thing without widening its scope.\n\n## Goal detail\n\nThis level-two heading remains opaque Goal Markdown.\n", "\n"), "structure", "Goal must be nonempty"},
		{"unexpected section after goal", replaceOnceForTest(v1Plan, "## Architecture summary", "## Definition of done"), "structure", "unexpected top-level section after Goal"},
		{"missing section after goal", truncateBefore(v1Plan, "## Architecture summary"), "structure", "missing section after Goal"},
		{"empty architecture", replaceOnceForTest(v1Plan, "\nKeep parsing and rendering in the model owner.\n", "\n"), "structure", "Architecture summary must be nonempty"},
		{"unexpected section after architecture", replaceOnceForTest(v1Plan, "## Phase 1: Parse", "## Notes"), "structure", "unexpected top-level section after Architecture summary"},
		{"missing section after architecture", truncateBefore(v1Plan, "## Phase 1: Parse"), "structure", "missing section after Architecture summary"},
		{"malformed phase heading", replaceOnceForTest(v1Plan, "## Phase 1: Parse", "## Phase broken"), "structure", "malformed phase heading"},
		{"nonsequential phase", replaceOnceForTest(v1Plan, "## Phase 1: Parse", "## Phase 2: Parse"), "numbering", "phase number 2, want 1"},
		{"missing execution mode", truncateAfter(v1Plan, "## Phase 1: Parse\n"), "structure", "phase 1 requires an execution mode"},
		{"malformed execution mode", replaceOnceForTest(v1Plan, "**Execution mode: inline.**", "**Execution mode: delegated.**"), "structure", "phase 1 requires exact execution mode"},
		{"duplicate execution mode", replaceOnceForTest(v1Plan, "Implement the parser.", "Implement the parser.\n\n**Execution mode: inline.**"), "structure", "phase 1 requires exactly one execution-mode declaration"},
		{"no tasks", replaceOnceForTest(v1Plan, "### Task 1.1: Build it", "Prose before tasks"), "structure", "phase 1 requires one or more tasks"},
		{"malformed task heading", replaceOnceForTest(v1Plan, "### Task 1.1: Build it", "### Task broken"), "structure", "malformed task heading"},
		{"nonsequential task", replaceOnceForTest(v1Plan, "### Task 1.1: Build it", "### Task 1.2: Build it"), "numbering", "task number 1.2, want 1.1"},
		{"unknown field", replaceOnceForTest(v1Plan, "Kind: batch", "Bogus: yes"), "field", "task 1.1 has unknown field Bogus"},
		{"duplicate field", replaceOnceForTest(v1Plan, "Kind: batch", "Kind: batch\nKind: batch"), "field", "task 1.1 duplicates field Kind"},
		{"empty field", replaceOnceForTest(v1Plan, "Kind: batch", "Kind:"), "field", "task 1.1 field Kind must be nonempty"},
		{"malformed field", replaceOnceForTest(v1Plan, "Kind: batch", "Kind:batch"), "field", "task 1.1 has malformed field Kind"},
		{"noncontiguous field", replaceOnceForTest(v1Plan, "Kind: batch", "\nKind: batch"), "field", "task 1.1 field Kind is not contiguous below its heading"},
		{"bad kind", replaceOnceForTest(v1Plan, "Kind: batch", "Kind: other"), "field", "task 1.1 Kind must be spike or batch"},
		{"bad latitude", replaceOnceForTest(v1Plan, "Latitude: exact", "Latitude: approximate"), "field", "task 1.1 Latitude must be exact"},
		{"batch relationship", replaceOnceForTest(v1Plan, "Paths: [\"glob:internal/plan/*.go\", \"pathspec::(top)internal/plan\", \"docs/plans/template.md\"]\n", ""), "relationship", "batch 1.1 requires Paths and Post-check"},
		{"glob post-check", withoutBatchOnlyFields(v1Plan), "relationship", "task 1.1 requires Post-check for batch, glob, or pathspec scope"},
		{"spike question missing", replaceOnceForTest(v1Plan, "Question: Which errors are stable?\n", ""), "relationship", "spike 1.2 requires Question"},
		{"spike batch field", replaceOnceForTest(v1Plan, "Question: Which errors are stable?", "Question: Which errors are stable?\nPaths: [\"internal/plan\"]"), "relationship", "spike 1.2 forbids batch fields"},
		{"spike body", replaceOnceForTest(v1Plan, "Question: Which errors are stable?", "Question: Which errors are stable?\n\nImplement an answer."), "relationship", "spike 1.2 has no prose body"},
		{"question without spike", replaceOnceForTest(v1Plan, "### Task 2.1: Add the reader", "### Task 2.1: Add the reader\nQuestion: Why?"), "relationship", "task 2.1 Question requires Kind: spike"},
		{"batch fields without batch", replaceOnceForTest(v1Plan, "### Task 2.1: Add the reader", "### Task 2.1: Add the reader\nRepresentative: Cover normal input."), "relationship", "task 2.1 Representative and Edge require Kind: batch"},
		{"spike alone", spikeOnlyPlan(v1Plan), "relationship", "spike cannot constitute a phase alone"},
		{"empty notes", replaceOnceForTest(v1Plan, "The spike established stable typed diagnostics.\n", ""), "relationship", "spike requires nonempty Notes"},
		{"retired section", replaceOnceForTest(v1Plan, "## Goal detail", "## File structure"), "structure", "File structure and Verification are not plan-v1 sections"},
		{"checkbox task", replaceOnceForTest(v1Plan, "### Task 1.1: Build it", "- [ ] **Task 1.1: Build it.**"), "structure", "task checkboxes are not plan-v1 declarations"},
		{"optional task", replaceOnceForTest(v1Plan, "### Task 2.1: Add the reader", "### Task 2.1: Add the reader (optional)"), "structure", "conditional and optional task declarations are forbidden"},
		{"missing phase close", replaceOnceForTest(replaceOnceForTest(v1Plan, "Kind: spike\nQuestion: Which errors are stable?", "Investigate stable errors."), "### Phase close", "### Closing work"), "structure", "phase 1 requires one final Phase close"},
		{"duplicate phase close", replaceOnceForTest(v1Plan, "Run the staged check and gate.\n", "### Phase close\n\nRun the staged check and gate.\n"), "phase-close", "Phase close must be the final child of phase 1"},
		{"unexpected section after phase close", replaceOnceForTest(v1Plan, "```commit\nfeat(plans): parse plans\n```\n\n## Phase 2", "```commit\nfeat(plans): parse plans\n```\n\n## Notes\n\nUnexpected.\n\n## Phase 2"), "structure", "unexpected top-level section after Phase close"},
		{"unexpected section inside task", replaceOnceForTest(v1Plan, "Expose the parsed projection.\n\n### Phase close", "Expose the parsed projection.\n\n## Notes\n\nUnexpected.\n\n### Phase close"), "structure", "unexpected top-level section inside task"},
		{"checkbox in task body", replaceOnceForTest(v1Plan, "Expose the parsed projection.", "Expose the parsed projection.\n\n- [ ] Nested work"), "structure", "task checkboxes are not plan-v1 declarations"},
		{"star checkbox in task body", replaceOnceForTest(v1Plan, "Expose the parsed projection.", "Expose the parsed projection.\n\n* [ ] Nested work"), "structure", "task checkboxes are not plan-v1 declarations"},
		{"plus checkbox in task body", replaceOnceForTest(v1Plan, "Expose the parsed projection.", "Expose the parsed projection.\n\n+ [x] Nested work"), "structure", "task checkboxes are not plan-v1 declarations"},
		{"optional prefix task", replaceOnceForTest(v1Plan, "### Task 2.1: Add the reader", "### Task 2.1: Optional add the reader"), "structure", "conditional and optional task declarations are forbidden"},
		{"conditional suffix task", replaceOnceForTest(v1Plan, "### Task 2.1: Add the reader", "### Task 2.1: Add the reader if needed"), "structure", "conditional and optional task declarations are forbidden"},
		{"as-needed task", replaceOnceForTest(v1Plan, "### Task 2.1: Add the reader", "### Task 2.1: Add the reader as needed"), "structure", "conditional and optional task declarations are forbidden"},
		{"if-required task", replaceOnceForTest(v1Plan, "### Task 2.1: Add the reader", "### Task 2.1: Add the reader if required"), "structure", "conditional and optional task declarations are forbidden"},
		{"missing commit fence", replaceOnceForTest(v1Plan, "```commit\nfeat(plans): parse plans\n```", "No commit fence."), "phase-close", "phase 1 requires exactly one non-ignored commit fence in Phase close"},
		{"unclosed commit fence", replaceOnceForTest(v1Plan, "```commit\nfeat(plans): parse plans\n```", "```commit\nfeat(plans): parse plans"), "phase-close", "phase 1 requires exactly one non-ignored commit fence in Phase close"},
		{"task commit fence", replaceOnceForTest(v1Plan, "Implement the parser.", "Implement the parser.\n\n```commit\nfix(plans): wrong fence\n```"), "phase-close", "phase 1 requires exactly one non-ignored commit fence in Phase close"},
		{"missing definition section", truncateBefore(v1Plan, "## Definition of done"), "structure", "expected ## Definition of done after final phase"},
		{"unexpected section before notes", replaceOnceForTest(v1Plan, "## Notes\n\nThe spike", "## Goal\n\nUnexpected.\n\n## Notes\n\nThe spike"), "structure", "unexpected top-level section before Notes"},
		{"no definition bullet", replaceOnceForTest(v1Plan, "- A valid plan parses and projects.", "A valid plan parses and projects."), "structure", "Definition of done requires a nonempty plain bullet"},
		{"fenced fake definition bullet", replaceOnceForTest(v1Plan, "- A valid plan parses and projects.", "```markdown\n- A fenced example is not completion.\n```"), "structure", "Definition of done requires a nonempty plain bullet"},
		{"definition checkbox", replaceOnceForTest(v1Plan, "- A valid plan parses and projects.", "- [ ] A valid plan parses and projects.\n- A second condition."), "structure", "Definition of done uses plain bullets, not checkboxes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePlan(t, dir, "2026-08-02-example.md", tc.body)
			_, err := plan.ParseDir(dir)
			var diagnostic *plan.Diagnostic
			if !errors.As(err, &diagnostic) {
				t.Fatalf("error = %v, want *plan.Diagnostic", err)
			}
			if diagnostic.Category != tc.category || diagnostic.Path != "2026-08-02-example.md" || diagnostic.Detail != tc.detail {
				t.Fatalf("diagnostic = %#v, want category=%q path=%q detail=%q", diagnostic, tc.category, "2026-08-02-example.md", tc.detail)
			}
		})
	}
}

func TestPlanV1PathGrammar(t *testing.T) {
	valid := []string{
		`["internal/plan/structure.go"]`,
		`["glob:**/*.go"]`,
		`["pathspec::/internal/plan"]`,
		`["pathspec::!vendor"]`,
		`["pathspec::^vendor"]`,
		`["pathspec::/:internal/plan"]`,
		`["pathspec::(top,icase)Internal/Plan"]`,
		`["pathspec::(attr:vendored)internal/plan"]`,
	}
	for _, paths := range valid {
		t.Run(paths, func(t *testing.T) {
			body := simplePlanWithPaths(paths, "Post-check: git diff --name-only")
			dir := t.TempDir()
			writePlan(t, dir, "2026-08-02-paths.md", body)
			if _, err := plan.ParseDir(dir); err != nil {
				t.Fatalf("ParseDir: %v", err)
			}
		})
	}

	invalid := []struct{ paths, detail string }{
		{`[`, "paths must be a nonempty JSON array of strings"},
		{`[]`, "paths must be a nonempty JSON array of strings"},
		{`[1]`, "paths entries must be nonempty strings"},
		{`[""]`, "paths entries must be nonempty strings"},
		{`["a", "a"]`, "paths entries must be unique after JSON decoding"},
		{`["../outside"]`, "literal path escapes repository"},
		{`["/outside"]`, "literal path escapes repository"},
		{`["C:\\\\outside"]`, "literal path escapes repository"},
		{`[":/literal"]`, "literal path contains glob or Git pathspec magic syntax"},
		{`["*.go"]`, "literal path contains glob or Git pathspec magic syntax"},
		{`["glob:../outside"]`, "glob path escapes repository"},
		{`["glob:["]`, `glob "[" is malformed`},
		{`["pathspec:"]`, "empty pathspec"},
		{`["pathspec::(top"]`, "pathspec magic prefix is missing terminator"},
		{`["pathspec::()file"]`, "unrecognized or malformed pathspec magic prefix"},
		{`["pathspec::(unknown)file"]`, "unrecognized or malformed pathspec magic prefix"},
		{`["pathspec::(glob,literal)file"]`, "pathspec magic glob and literal are incompatible"},
		{`["pathspec::zfile"]`, "unrecognized or malformed pathspec magic prefix"},
		{`["pathspec::!!file"]`, "unrecognized or malformed pathspec magic prefix"},
		{`["pathspec::!^file"]`, "unrecognized or malformed pathspec magic prefix"},
		{`["pathspec::!../outside"]`, "pathspec path escapes repository"},
	}
	for _, tc := range invalid {
		t.Run(tc.paths, func(t *testing.T) {
			dir := t.TempDir()
			writePlan(t, dir, "2026-08-02-paths.md", simplePlanWithPaths(tc.paths, "Post-check: git diff --name-only"))
			_, err := plan.ParseDir(dir)
			var diagnostic *plan.Diagnostic
			if !errors.As(err, &diagnostic) || diagnostic.Category != "paths" || diagnostic.Path != "2026-08-02-paths.md" || diagnostic.Detail != tc.detail {
				t.Fatalf("error = %v, diagnostic = %#v", err, diagnostic)
			}
		})
	}
}

func TestPlanV1FencedListSyntaxIsOpaque(t *testing.T) {
	body := replaceOnceForTest(v1Plan, "Expose the parsed projection.", "Expose the parsed projection.\n\n```markdown\n- [ ] A fenced checkbox example.\n```")
	body = replaceOnceForTest(body, "- A valid plan parses and projects.", "- A real completion condition.\n\n```markdown\n- [ ] A fenced Definition-of-done example.\n```")
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-fenced-lists.md", body)
	if _, err := plan.ParseDir(dir); err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
}

func TestPlanV1FenceGrammarIsOpaque(t *testing.T) {
	body := replaceOnceForTest(v1Plan, "Expose the parsed projection.", "Expose the parsed projection.\n\n~~~markdown\n### Task 9.9: Optional fenced task\nKind: spike\n- [ ] Fenced work.\n## Verification\n~~~")
	body = replaceOnceForTest(body, "Run the staged check and gate.", "~~~markdown\n```commit\nfix(plans): fenced example\n```\n~~~\n\nRun the staged check and gate.")
	body = replaceOnceForTest(body, "feat(plans): parse plans\n```", "feat(plans): parse plans\n````commit\nThis longer run remains content.\n```")
	body = replaceOnceForTest(body, "- A valid plan parses and projects.", "````markdown\n- A fenced example is not completion.\n```\n+ Still fenced after the shorter run.\n````\n\n* A real completion condition.")
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-fence-grammar.md", body)
	if _, err := plan.ParseDir(dir); err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
}

func TestPlanV1DefinitionBulletMarkers(t *testing.T) {
	for _, bullet := range []string{"* Asterisk completion condition.", "+ Plus completion condition."} {
		t.Run(bullet[:1], func(t *testing.T) {
			body := replaceOnceForTest(v1Plan, "- A valid plan parses and projects.", bullet)
			dir := t.TempDir()
			writePlan(t, dir, "2026-08-02-bullet.md", body)
			if _, err := plan.ParseDir(dir); err != nil {
				t.Fatalf("ParseDir: %v", err)
			}
		})
	}
}

func TestPlanV1AbsentFormatRemainsLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := "---\ndate: 2026-08-02\nadrs: []\nstatus: Proposed\n---\n# Plan: Old\n\n## File structure\n\nLegacy shape.\n"
	writePlan(t, dir, "2026-08-02-old.md", legacy)
	plans, err := plan.ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].Format != "" || len(plans[0].Phases) != 0 || string(plans[0].Source) != legacy {
		t.Fatalf("legacy = %#v", plans[0])
	}
}

func TestPlanV1RetiredHeadingInsideFenceIsOpaque(t *testing.T) {
	body := replaceOnceForTest(v1Plan, "This level-two heading remains opaque Goal Markdown.", "```markdown\n## Verification\n```")
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-fenced.md", body)
	if _, err := plan.ParseDir(dir); err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
}

func simplePlanWithPaths(paths, postCheck string) string {
	return `---
format: plan-v1
date: 2026-08-02
adrs: []
status: Proposed
---
# Plan: Paths

## Goal

Validate paths without widening scope.

## Architecture summary

Keep path parsing in internal/plan.

## Phase 1: Parse

**Execution mode: inline.**

### Task 1.1: Parse paths
Paths: ` + paths + "\n" + postCheck + `

Implement the path parser.

### Phase close

Run the gate.

` + "```commit\nfeat(plans): parse paths\n```" + `

## Definition of done

- Paths parse deterministically.
`
}

func withoutBatchOnlyFields(body string) string {
	for _, line := range []string{
		"Kind: batch\n", "Representative: Cover normal input.\n", "Edge: Cover invalid input.\n", "Post-check: go test ./internal/plan\n",
	} {
		body = replaceOnceForTest(body, line, "")
	}
	return body
}

// invariant: adr-system/plan-artifacts:plan-v2-decision-references (TestPlanV2DecisionReferences)
func TestPlanV2DecisionReferences(t *testing.T) {
	body := strings.Replace(v1Plan, "format: plan-v1", "format: plan-v2", 1)
	body = strings.Replace(body, "### Task 1.1: Build it\n", "### Task 1.1: Build it\nApplying: [\"task-scoped-plan-decision-context-and-phase-outcomes:plan-v2\"]\nContext: [\"0001:#1\"]\n", 1)
	body = strings.Replace(body, "- A valid plan parses and projects.", "- `dod: complete` A valid plan parses and projects.", 1)
	body = strings.Replace(body, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"complete\"]", 1)
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-v2.md", body)
	plans, err := plan.ParseDir(dir)
	if err != nil || len(plans) != 1 || len(plans[0].Phases[0].Tasks[0].Fields.Applying) != 1 || plans[0].Phases[0].Tasks[0].Fields.Applying[0].Selector != "plan-v2" {
		t.Fatalf("ParseDir = %#v, %v", plans, err)
	}
	leadingDigitSlug := strings.Replace(body, "task-scoped-plan-decision-context-and-phase-outcomes:plan-v2", "3d-context:2fa-rule", 1)
	dir = t.TempDir()
	writePlan(t, dir, "2026-08-02-leading-digit.md", leadingDigitSlug)
	if parsed, err := plan.ParseDir(dir); err != nil || parsed[0].Phases[0].Tasks[0].Fields.Applying[0].ADR != "3d-context" || parsed[0].Phases[0].Tasks[0].Fields.Applying[0].Selector != "2fa-rule" {
		t.Fatalf("lowercase-kebab references with leading digits = %#v, %v", parsed, err)
	}
	for name, reference := range map[string]string{
		"numeric identity is four digits": "12:plan-v2",
		"repeated selector hyphen":        "task-scoped-plan-decision-context-and-phase-outcomes:plan--v2",
		"trailing selector hyphen":        "task-scoped-plan-decision-context-and-phase-outcomes:plan-v2-",
		"noncanonical ordinal":            "0001:#01",
	} {
		t.Run(name, func(t *testing.T) {
			bad := strings.Replace(body, "task-scoped-plan-decision-context-and-phase-outcomes:plan-v2", reference, 1)
			dir := t.TempDir()
			writePlan(t, dir, "2026-08-02-bad.md", bad)
			if _, err := plan.ParseDir(dir); err == nil {
				t.Fatalf("invalid reference %q accepted", reference)
			}
		})
	}
	for name, tc := range map[string]struct{ field, replacement string }{
		"whitespace before colon":           {"Applying", "Applying : ["},
		"missing space after colon":         {"Applying", "Applying:["},
		"tab after colon":                   {"Applying", "Applying:\t["},
		"context whitespace before colon":   {"Context", "Context : ["},
		"context missing space after colon": {"Context", "Context:["},
		"context tab after colon":           {"Context", "Context:\t["},
	} {
		t.Run(name, func(t *testing.T) {
			malformed := strings.Replace(body, tc.field+": [", tc.replacement, 1)
			dir := t.TempDir()
			writePlan(t, dir, "2026-08-02-malformed.md", malformed)
			if _, err := plan.ParseDir(dir); err == nil {
				t.Fatalf("malformed field %q accepted", tc.replacement)
			}
		})
	}
	prose := strings.Replace(body, "\n\nImplement the parser.", "\n\nApplying lessons: keep task prose legal.\nContext remains prose without a field separator.\n\nImplement the parser.", 1)
	dir = t.TempDir()
	writePlan(t, dir, "2026-08-02-prose.md", prose)
	if _, err := plan.ParseDir(dir); err != nil {
		t.Fatalf("ordinary prose beginning with reserved words: %v", err)
	}
}

// invariant: adr-system/plan-artifacts:plan-v2-phase-outcomes (TestPlanV2PhaseOutcomes)
func TestPlanV2PhaseOutcomes(t *testing.T) {
	body := strings.Replace(v1Plan, "format: plan-v1", "format: plan-v2", 1)
	body = strings.Replace(body, "- A valid plan parses and projects.", "- `dod: complete` A valid plan parses and projects.", 1)
	body = strings.Replace(body, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nAdvances: [\"complete\"]", 1)
	dir := t.TempDir()
	writePlan(t, dir, "2026-08-02-v2.md", body)
	plans, err := plan.ParseDir(dir)
	if err != nil || len(plans) != 1 || len(plans[0].DoD) != 1 || plans[0].Phases[0].Advances[0] != "complete" {
		t.Fatalf("ParseDir = %#v, %v", plans, err)
	}
	t.Run("DoD source ranges and grammar", func(t *testing.T) {
		multiline := strings.Replace(body, "- `dod: complete` A valid plan parses and projects.", "- `dod: one` One.\n\n  Continuation.\n  - nested plain bullet\n\n  ```text\n  - fenced plain bullet\n  ```\n- `dod: two` Two.\n", 1)
		multiline = strings.Replace(multiline, "Advances: [\"complete\"]", "Advances: [\"one\"]", 1)
		dir := t.TempDir()
		writePlan(t, dir, "2026-08-02-ranges.md", multiline)
		parsed, err := plan.ParseDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		want := "- `dod: one` One.\n\n  Continuation.\n  - nested plain bullet\n\n  ```text\n  - fenced plain bullet\n  ```\n"
		if got := parsed[0].DoD[0].Content; got != want {
			t.Fatalf("DoD source range = %q, want %q", got, want)
		}
		noBullets := strings.Replace(multiline, "- `dod: one` One.\n\n  Continuation.\n  - nested plain bullet\n\n  ```text\n  - fenced plain bullet\n  ```\n- `dod: two` Two.\n", "No outcome bullet.\n", 1)
		dir = t.TempDir()
		writePlan(t, dir, "2026-08-02-empty.md", noBullets)
		if _, err := plan.ParseDir(dir); err == nil {
			t.Fatal("missing DoD bullets accepted")
		}
		for _, replacement := range []string{"- Unmarked.\n", "- `dod: Bad` malformed.\n", "- `dod: one` duplicate.\n"} {
			bad := strings.Replace(multiline, "- `dod: two` Two.\n", replacement, 1)
			dir := t.TempDir()
			writePlan(t, dir, "2026-08-02-bad.md", bad)
			if _, err := plan.ParseDir(dir); err == nil {
				t.Fatalf("invalid DoD bullet %q accepted", replacement)
			}
		}
	})
}

func TestPlanV2RejectsPhaseFieldAndCrossPhaseOutcomeErrors(t *testing.T) {
	base := strings.Replace(v1Plan, "format: plan-v1", "format: plan-v2", 1)
	base = strings.Replace(base, "- A valid plan parses and projects.", "- `dod: one` One.", 1)
	parse := func(t *testing.T, body string) error {
		t.Helper()
		dir := t.TempDir()
		writePlan(t, dir, "2026-08-02-v2.md", body)
		_, err := plan.ParseDir(dir)
		return err
	}
	phaseOne := "**Execution mode: inline.**"
	for name, replacement := range map[string]string{
		"unknown field":            phaseOne + "\n\nUnknown: [\"one\"]",
		"empty field":              phaseOne + "\n\nAdvances:",
		"invalid JSON":             phaseOne + "\n\nAdvances: not-json",
		"duplicate Advances":       phaseOne + "\n\nAdvances: [\"one\"]\nAdvances: [\"one\"]",
		"Advances after Completes": phaseOne + "\n\nCompletes: [\"one\"]\nAdvances: [\"one\"]",
	} {
		t.Run(name, func(t *testing.T) {
			if err := parse(t, strings.Replace(base, phaseOne, replacement, 1)); err == nil {
				t.Fatal("invalid phase field was accepted")
			}
		})
	}
	bothComplete := strings.Replace(base, phaseOne, phaseOne+"\n\nCompletes: [\"one\"]", 1)
	bothComplete = strings.Replace(bothComplete, "**Execution mode: subagent-driven.**", "**Execution mode: subagent-driven.**\n\nCompletes: [\"one\"]", 1)
	if err := parse(t, bothComplete); err == nil || !strings.Contains(err.Error(), "duplicate Completes owner") {
		t.Fatalf("duplicate phase outcome error = %v", err)
	}
}

func TestPlanV2RejectsFieldAndOutcomeRelationships(t *testing.T) {
	base := strings.Replace(v1Plan, "format: plan-v1", "format: plan-v2", 1)
	base = strings.Replace(base, "- A valid plan parses and projects.", "- `dod: one` One.\n- `dod: two` Two.", 1)
	base = strings.Replace(base, "**Execution mode: inline.**", "**Execution mode: inline.**\n\nCompletes: [\"one\"]", 1)
	valid := func(t *testing.T, body string) error {
		t.Helper()
		dir := t.TempDir()
		writePlan(t, dir, "2026-08-02-v2.md", body)
		_, err := plan.ParseDir(dir)
		return err
	}
	for name, body := range map[string]string{
		"empty applying":      strings.Replace(base, "### Task 1.1: Build it\n", "### Task 1.1: Build it\nApplying: []\n", 1),
		"duplicate applying":  strings.Replace(base, "### Task 1.1: Build it\n", "### Task 1.1: Build it\nApplying: [\"x:one\", \"x:one\"]\n", 1),
		"malformed reference": strings.Replace(base, "### Task 1.1: Build it\n", "### Task 1.1: Build it\nContext: [\"bad\"]\n", 1),
		"misplaced field":     strings.Replace(base, "Implement the parser.", "Implement the parser.\nApplying: [\"x:one\"]", 1),
		"duplicate outcomes":  strings.Replace(base, "Completes: [\"one\"]", "Completes: [\"one\"]\nCompletes: [\"two\"]", 1),
		"overlap outcomes":    strings.Replace(base, "Completes: [\"one\"]", "Advances: [\"one\"]\nCompletes: [\"one\"]", 1),
		"unknown dod":         strings.Replace(base, "Completes: [\"one\"]", "Completes: [\"missing\"]", 1),
		"duplicate dod":       strings.Replace(base, "- `dod: two` Two.", "- `dod: one` Two.", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := valid(t, body); err == nil {
				t.Fatal("invalid plan was accepted")
			}
		})
	}
	if err := valid(t, base); err != nil {
		t.Fatalf("valid v2 rejected: %v", err)
	}
}

func spikeOnlyPlan(body string) string {
	start := strings.Index(body, "### Task 1.1: Build it")
	spike := strings.Index(body, "### Task 1.2: Investigate")
	if start < 0 || spike < 0 {
		panic("test fixture lacks phase-one tasks")
	}
	body = body[:start] + body[spike:]
	return replaceOnceForTest(body, "### Task 1.2: Investigate", "### Task 1.1: Investigate")
}

func truncateBefore(body, marker string) string {
	at := strings.Index(body, marker)
	if at < 0 {
		panic("test fixture missing " + marker)
	}
	return body[:at]
}

func truncateAfter(body, marker string) string {
	at := strings.Index(body, marker)
	if at < 0 {
		panic("test fixture missing " + marker)
	}
	return body[:at+len(marker)]
}

func TestPlanTaskHeadingAndFieldPlacementDiagnostics(t *testing.T) {
	cases := []struct{ name, old, replacement, detail string }{
		{"malformed heading", "### Task 1.1: Build it", "### Task invalid", "malformed task heading"},
		{"wrong task number", "### Task 1.1: Build it", "### Task 1.2: Build it", "task number 1.2, want 1.1"},
		{"malformed field", "Kind: batch", "Kind:batch", "malformed field Kind"},
		{"unknown field", "Kind: batch", "Unknown: value", "unknown field Unknown"},
		{"duplicate field", "Kind: batch", "Kind: batch\nKind: batch", "duplicates field Kind"},
		{"empty field", "Kind: batch", "Kind:", "field Kind must be nonempty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePlan(t, dir, "2026-08-02-bad.md", replaceOnceForTest(v1Plan, tc.old, tc.replacement))
			_, err := plan.ParseDir(dir)
			var diagnostics *plan.DiagnosticsError
			if !errors.As(err, &diagnostics) || len(diagnostics.Diagnostics) != 1 || !strings.Contains(diagnostics.Diagnostics[0].Detail, tc.detail) {
				t.Fatalf("ParseDir error = %v (%#v)", err, err)
			}
		})
	}
}

func replaceOnceForTest(body, old, replacement string) string {
	if !strings.Contains(body, old) {
		panic("test fixture missing " + old)
	}
	return strings.Replace(body, old, replacement, 1)
}
