package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing (TestGuideRoutesNativeSkillsWithoutCatalog)
func TestGuideRoutesNativeSkillsWithoutCatalog(t *testing.T) {
	data := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "commitScopes": "", "gatedCommands": "", "skills": map[string]bool{}}
	out := renderGuide(t, data)
	for _, want := range []string{
		"Treat exposed native-skill descriptions as routing metadata.",
		"Before loading a skill, identify the next concrete action.",
		"a possible later edit, render, documentation update, review, or commit does not justify loading its skill now.",
		"Load multiple bodies only when each independently governs that same next action before another routing decision can occur.",
		"use `agentic-code-design` only for structural questions raised by agreed behavior",
		"Generic plans remain interaction-local by default; only deliberately effort-backed operational plans enter effort scratch.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("guide does not preserve progressive skill disclosure %q", want)
		}
	}
	for _, banned := range []string{"Enabled skills:", "example-brainstorming", "purpose", "Trigger:", "Usually follows:", "Common follow-ups:", "fallback", "<no value>", "awf_workflow", "only legal predecessor", "only legal successor", "mandatory successor", "must follow", "must be followed by", "mandatory transition"} {
		if strings.Contains(out, banned) {
			t.Errorf("guide retains catalog or routing residue %q", banned)
		}
	}

	emptyPrefix := renderGuide(t, map[string]any{"prefix": "", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "commitScopes": "", "gatedCommands": "", "skills": map[string]bool{}})
	for _, want := range []string{"# Project Agent Guide", "working in this repository"} {
		if !strings.Contains(emptyPrefix, want) {
			t.Errorf("empty-prefix guide missing fallback %q", want)
		}
	}
	for _, malformed := range []string{"#  Agent Guide", "``"} {
		if strings.Contains(emptyPrefix, malformed) {
			t.Errorf("empty-prefix guide contains malformed fallback %q", malformed)
		}
	}
}

func renderGuide(t *testing.T, data map[string]any) string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, "agents-doc/AGENTS.md.tmpl")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	withLayoutDefaults(data)
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil {
		t.Fatalf("expand includes: %v", err)
	}
	asm, parts := assemble(parseSections(expanded))
	out, err := render.Execute(asm, data, parts, "test")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "<no value>") {
		t.Errorf("rendered <no value>:\n%s", out)
	}
	if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
		t.Errorf("unrendered template action:\n%s", out)
	}
	return out
}

// conventionalCommitsBullet returns the single invariant bullet opening with
// "**Conventional Commits" and fails if there is not exactly one - a second
// would mean a hand-written scope entry returned to agents-doc.yaml.
func conventionalCommitsBullet(t *testing.T, out string) string {
	t.Helper()
	var found []string
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "- **Conventional Commits") {
			found = append(found, ln)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one Conventional-Commits invariant bullet, got %d:\n%s",
			len(found), strings.Join(found, "\n"))
	}
	return found[0]
}

// awfAgentsDocInvariants reads the data.invariants list from awf's own
// .awf/agents-doc.yaml as the template consumes it ([]any of map[string]any).
func awfAgentsDocInvariants(t *testing.T) []any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRootDir(t), ".awf", "agents-doc.yaml"))
	if err != nil {
		t.Fatalf("read agents-doc.yaml: %v", err)
	}
	var doc struct {
		Data struct {
			Invariants []map[string]any `yaml:"invariants"`
		} `yaml:"data"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse agents-doc.yaml: %v", err)
	}
	if len(doc.Data.Invariants) == 0 {
		t.Fatal("agents-doc.yaml carries no data.invariants")
	}
	out := make([]any, len(doc.Data.Invariants))
	for i, m := range doc.Data.Invariants {
		out[i] = m
	}
	return out
}

// repoRootDir ascends from the test's working directory to the directory
// holding go.mod.
func repoRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test working directory")
		}
		dir = parent
	}
}
