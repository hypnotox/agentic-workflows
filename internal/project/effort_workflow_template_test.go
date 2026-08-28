package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type effortSignatureFinding struct {
	path     string
	line     int
	offset   int
	contract string
	lineText string
}

type effortSignaturePattern struct {
	contract string
	pattern  *regexp.Regexp
}

func effortSignaturePatterns() []effortSignaturePattern {
	return []effortSignaturePattern{
		{"title-only creation signature", regexp.MustCompile("awf effort " + `new[^<]*<(confirmed title|outcome|outcome-title)>`)},
		{"title-derived creation guidance", regexp.MustCompile("[Ee]ffort (creation )?" + `deriv(e|es|ed|ing)[^\r\n]{0,40}slug`)},
		{"title-derived creation guidance", regexp.MustCompile("[Dd]eriv" + `(e|es|ed|ing) an immutable slug`)},
		{"two-field confirmation", regexp.MustCompile("outcome/title " + `(pair|confirmation)`)},
		{"two-field confirmation", regexp.MustCompile("labeled outcome and " + `(proposed )?(effort )?title`)},
		{"two-field confirmation", regexp.MustCompile("confirms? the " + `pair`)},
		{"two-field confirmation", regexp.MustCompile("both " + `fields`)},
	}
}

func activeEffortSignatureFindings(t *testing.T, root string) []effortSignatureFinding {
	t.Helper()
	patterns := effortSignaturePatterns()
	var findings []effortSignatureFinding
	scan := func(relative string, raw []byte) {
		for _, candidate := range patterns {
			for _, match := range candidate.pattern.FindAllIndex(raw, -1) {
				lineStart := bytes.LastIndexByte(raw[:match[0]], '\n') + 1
				lineEnd := bytes.IndexByte(raw[match[0]:], '\n')
				if lineEnd < 0 {
					lineEnd = len(raw)
				} else {
					lineEnd += match[0]
				}
				findings = append(findings, effortSignatureFinding{
					path: relative, line: bytes.Count(raw[:match[0]], []byte("\n")) + 1,
					offset: match[0], contract: candidate.contract, lineText: string(raw[lineStart:lineEnd]),
				})
			}
		}
	}
	historical := func(relative string) bool {
		return relative == "docs/decisions" || strings.HasPrefix(relative, "docs/decisions/") ||
			relative == "docs/plans" || strings.HasPrefix(relative, "docs/plans/") ||
			relative == "changelog" || strings.HasPrefix(relative, "changelog/")
	}
	scanRoot := func(relativeRoot string) {
		start := filepath.Join(root, filepath.FromSlash(relativeRoot))
		info, err := os.Lstat(start)
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			raw, err := os.ReadFile(start)
			if err != nil {
				t.Fatal(err)
			}
			scan(relativeRoot, raw)
			return
		}
		testsupport.WalkRepoFiles(t, start, func(relative string) bool {
			full := filepath.ToSlash(filepath.Join(relativeRoot, filepath.FromSlash(relative)))
			resident := full == ".awf/efforts" || strings.HasPrefix(full, ".awf/efforts/") || strings.Contains(full, "/.awf/efforts/") ||
				full == ".awf/worktrees" || strings.HasPrefix(full, ".awf/worktrees/") || strings.Contains(full, "/.awf/worktrees/")
			return !historical(full) && !resident
		}, func(relative string, raw []byte) {
			scan(filepath.ToSlash(filepath.Join(relativeRoot, filepath.FromSlash(relative))), raw)
		})
	}
	for _, relativeRoot := range []string{"cmd", "internal", ".awf/parts", ".awf/docs", ".awf/skills", ".awf/topics", "templates", "AGENTS.md", "README.md", "docs", ".pi", ".claude", "examples"} {
		scanRoot(relativeRoot)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "examples")); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			for _, hidden := range []string{".awf", ".pi", ".claude"} {
				scanRoot(filepath.ToSlash(filepath.Join("examples", entry.Name(), hidden)))
			}
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].path != findings[j].path {
			return findings[i].path < findings[j].path
		}
		if findings[i].offset != findings[j].offset {
			return findings[i].offset < findings[j].offset
		}
		return findings[i].contract < findings[j].contract
	})
	return findings
}

func formatEffortSignatureFindings(findings []effortSignatureFinding) string {
	var lines []string
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("%s:%d:%d: %s", finding.path, finding.line, finding.offset, finding.contract))
	}
	return strings.Join(lines, "\n")
}

func explicitSlugADRStatus(t *testing.T, root string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "docs", "decisions", "*require-explicit-short-effort-slugs.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("explicit-slug ADR matches = %v, err=%v", matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^status: ([^\r\n]+)$`).FindSubmatch(raw)
	if len(match) != 2 {
		t.Fatalf("explicit-slug ADR has no status: %s", matches[0])
	}
	return string(match[1])
}

func TestActiveEffortCreationSignaturesStaySynchronized(t *testing.T) {
	root := filepath.Join("..", "..")
	findings := activeEffortSignatureFindings(t, root)
	switch status := explicitSlugADRStatus(t, root); status {
	case "Implementing", "Implemented":
		if len(findings) != 0 {
			t.Fatalf("%s ADR requires zero active findings:\n%s", status, formatEffortSignatureFindings(findings))
		}
	default:
		t.Fatalf("explicit-slug signature test does not permit ADR status %q", status)
	}

	fixture := t.TempDir()
	writeFixture := func(path, body string) {
		t.Helper()
		full := filepath.Join(fixture, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		body     string
		contract string
	}{
		{"awf effort " + "new <outcome-title>", "title-only creation signature"},
		{"Effort creation " + "derives a slug", "title-derived creation guidance"},
		{"Deriving" + " an immutable slug", "title-derived creation guidance"},
		{"outcome/title " + "confirmation", "two-field confirmation"},
		{"labeled outcome and " + "proposed effort title", "two-field confirmation"},
		{"labeled outcome and " + "proposed title receive", "two-field confirmation"},
		{"confirms the " + "pair", "two-field confirmation"},
		{"both " + "fields", "two-field confirmation"},
	}
	var expected []string
	for index, test := range cases {
		path := fmt.Sprintf("cmd/stale-%d.md", index)
		writeFixture(path, test.body)
		expected = append(expected, fmt.Sprintf("%s:1:0: %s", path, test.contract))
	}
	multiplePath := "cmd/stale-multiple.md"
	multiple := cases[5].body + " / " + cases[5].body + "\n" + cases[5].body
	writeFixture(multiplePath, multiple)
	secondOffset := len(cases[5].body) + len(" / ")
	thirdOffset := secondOffset + len(cases[5].body) + 1
	expected = append(expected,
		fmt.Sprintf("%s:1:0: %s", multiplePath, cases[5].contract),
		fmt.Sprintf("%s:1:%d: %s", multiplePath, secondOffset, cases[5].contract),
		fmt.Sprintf("%s:2:%d: %s", multiplePath, thirdOffset, cases[5].contract),
	)
	for _, path := range []string{
		"cmd/active.md", "internal/active.md",
		".awf/parts/active.md", ".awf/docs/active.md", ".awf/skills/active.md", ".awf/topics/active.md",
		"templates/active.md", "AGENTS.md", "README.md", "docs/active.md",
		".pi/active.md", ".claude/active.md", "examples/demo/active.md",
		"examples/demo/.awf/active.md", "examples/demo/.pi/active.md", "examples/demo/.claude/active.md",
	} {
		writeFixture(path, cases[0].body)
		expected = append(expected, path+":1:0: "+cases[0].contract)
	}
	for _, path := range []string{
		"docs/decisions/historical.md", "docs/plans/historical.md", "changelog/historical.md",
		".awf/efforts/ignored.md", ".awf/worktrees/ignored.md",
		"examples/demo/.awf/efforts/ignored.md", "examples/demo/.awf/worktrees/ignored.md",
	} {
		writeFixture(path, cases[0].body)
	}
	sort.Strings(expected)
	if got := formatEffortSignatureFindings(activeEffortSignatureFindings(t, fixture)); got != strings.Join(expected, "\n") {
		t.Fatalf("closed active-path diagnostics =\n%s\nwant\n%s", got, strings.Join(expected, "\n"))
	}
}

// TestWorkingMemorySingleHomeSurfaces asserts the workflow doc remains the
// detailed protocol home while guides and skills carry executable routing.
// invariant: rendering/guide-and-doc-templates:working-memory-single-home (TestWorkingMemorySingleHomeSurfaces)
func TestWorkingMemorySingleHomeSurfaces(t *testing.T) {
	data := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "skills": map[string]bool{"effort-workflow": true}, "targetSessionHandoff": true}
	workflow := renderGolden(t, "docs/workflow.md.tmpl", data)
	assertOrderedPhrases(t, workflow,
		"## Working memory", "Session context is volatile", "`effort-workflow` alone chooses", "awf effort new --slug <slug>", "reports the allocated immutable identity", "rendered orienting skill's resume-revalidation section is the procedural home", "One effort has one user-managed writer")
	guide := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for surface, body := range map[string]string{"workflow": workflow, "agent guide": guide} {
		for _, forbidden := range []string{"`handoff_session`", "`Continue with effort <slug>.`", "`[session context]`", "session-context facts", "active-branch compaction", "Handoff validates only dual-format"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("generic %s retains Pi-only protocol %q", surface, forbidden)
			}
		}
	}
	effort := renderSkillGolden(t, "effort-workflow", data)
	assertContainsAll := func(body string, wants ...string) {
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("missing %q:\n%s", want, body)
			}
		}
	}
	assertContainsAll(effort, "sole owner of the effort lifecycle", "all effort checkpoints", "integration, divergence handling, topology removal, retrospective routing, and finish")
	orienting := renderSkillGolden(t, "orienting", data)
	assertContainsAll(orienting, "## Resume revalidation", "verify every load-bearing claim against repository truth", "A discrepancy resolves in favor of the repository", "a dispatched child never edits it")
	for _, other := range []string{"executing-direct", "reviewing-impl"} {
		body := renderSkillGolden(t, other, data)
		if strings.Contains(body, "awf effort finish <slug>") {
			t.Errorf("%s steals effort lifecycle closure", other)
		}
	}
}

// invariant: rendering/workflow-skill-templates:memory-log-consumer-coverage (TestMemoryLogConsumerCoverage)
func TestMemoryLogConsumerCoverage(t *testing.T) {
	data := map[string]any{
		"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"},
		"layout": testLayout(), "data": map[string]any{},
		"skills": map[string]bool{},
	}
	for _, agent := range []string{"adr-reviewer", "plan-reviewer", "code-reviewer"} {
		out := renderAgentGolden(t, agent, data)
		for _, want := range []string{
			"## Consensus adherence",
			"user-decision",
			"`location` cites the deviating",
			"`issue` names the deviation",
			"`suggested_fix` carries the escalation phrasing",
			"we decided X; during <phase> we found Z; recommend Y, approve?",
			"explicitly approved design summary",
			"Removing an unaccepted surplus commitment",
			"authority-preserving `reasoned` correction",
			"A brief without either form of consent evidence leaves this check idle",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing consensus-adherence phrase %q:\n%s", agent, want, out)
			}
		}
	}
	for _, skill := range []string{"reviewing-adr", "reviewing-plan", "reviewing-impl"} {
		out := renderSkillGolden(t, skill, data)
		for _, want := range []string{"verbatim", "including whatever `Record:` blocks exist"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing decision-log paste phrase %q:\n%s", skill, want, out)
			}
		}
	}
	adrReview := renderSkillGolden(t, "reviewing-adr", data)
	for _, want := range []string{"For effort-backed work", "For effort-free work", "explicitly approved design summary", "repository facts do not establish consent", "reviewer must not infer it"} {
		if !strings.Contains(adrReview, want) {
			t.Errorf("reviewing-adr missing consent-evidence branch %q:\n%s", want, adrReview)
		}
	}
	effortFreeOmission := map[string]string{
		"reviewing-plan": "otherwise omit effort and memory fields",
		"reviewing-impl": "absence of an effort omits those fields",
	}
	for skill, want := range effortFreeOmission {
		out := renderSkillGolden(t, skill, data)
		if !strings.Contains(out, want) {
			t.Errorf("%s does not preserve effort-free memory omission %q:\n%s", skill, want, out)
		}
	}
	retrospective := renderSkillGolden(t, "retrospective", data)
	for _, want := range []string{"`## Observations`", "`## Decision log`", "as primary input", "across the effort's sessions"} {
		if !strings.Contains(retrospective, want) {
			t.Errorf("retrospective missing memory-log phrase %q:\n%s", want, retrospective)
		}
	}
}

func TestEffortWorkflowTemplate(t *testing.T) {
	out := renderSkillGolden(t, "effort-workflow", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}})
	if !strings.Contains(out, "sole owner of the effort lifecycle") {
		t.Fatal("effort contract missing")
	}
}

func TestRoadmapGraduationTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
	}

	out := renderSkillGolden(t, "roadmap-graduation", data)

	// Assert frontmatter name line
	if !strings.Contains(out, "name: example-roadmap-graduation") {
		t.Errorf("expected 'name: example-roadmap-graduation' in output:\n%s", out)
	}

	// Assert load-bearing phrases unique to roadmap-graduation
	loadBearing := []string{
		"same commit",
		"roadmap",
		"benchmark",
		"docs(roadmap): drop",
	}
	for _, phrase := range loadBearing {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
}
