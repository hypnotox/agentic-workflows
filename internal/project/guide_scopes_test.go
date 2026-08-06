package project

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing (TestGuideRoutesNativeSkillsWithoutCatalog)
func TestGuideRoutesNativeSkillsWithoutCatalog(t *testing.T) {
	data := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "commitScopes": "", "gatedCommands": "", "skills": map[string]bool{}}
	out := renderGuide(t, data)
	if !strings.Contains(out, "Use any enabled native skill whose exposed description fits the current work.") {
		t.Error("guide does not route selection through native skill descriptions")
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

// invariant: rendering/workflow-skill-templates:workflow-transitions-advisory (TestWorkflowSkillRelationshipsStayAdvisory)
func TestWorkflowSkillRelationshipsStayAdvisory(t *testing.T) {
	skills := slices.Sorted(maps.Keys(catalog.Standard.Skills))
	agents := slices.Sorted(maps.Keys(catalog.Standard.Agents))
	const advisorySelection = "Use any enabled native skill whose exposed description fits the current work."
	mandatoryRelationships := []string{"only legal predecessor", "only legal successor", "mandatory successor", "must follow", "must be followed by", "mandatory transition"}
	operativeControls := []string{" must ", " never ", " requires ", "Do not ", "Stop ", "stop ", "forbidden"}

	for _, target := range KnownTargets() {
		t.Run(target, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: ["+strings.Join(skills, ", ")+"]\nagents: ["+strings.Join(agents, ", ")+"]\ndocs: [roadmap]\ntargets: ["+target+"]\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			files, err := p.RenderAll()
			if err != nil {
				t.Fatal(err)
			}
			missingSkills := make(map[string]bool, len(skills))
			for _, skill := range skills {
				missingSkills[skill] = true
			}
			for _, file := range files {
				if file.Path == "AGENTS.md" && !strings.Contains(file.Content, advisorySelection) {
					t.Errorf("guide does not keep skill selection advisory")
				}
				if !strings.HasSuffix(file.Path, "/SKILL.md") {
					continue
				}
				for _, skill := range skills {
					if strings.HasSuffix(file.Path, "/example-"+skill+"/SKILL.md") {
						delete(missingSkills, skill)
						break
					}
				}
				for _, phrase := range mandatoryRelationships {
					if strings.Contains(file.Content, phrase) {
						t.Errorf("%s turns an advisory relationship into %q", file.Path, phrase)
					}
				}
				if !containsAny(file.Content, operativeControls) {
					t.Errorf("%s carries no mandatory operative control", file.Path)
				}
			}
			if len(missingSkills) != 0 {
				t.Fatalf("missing rendered standard skills: %v", slices.Sorted(maps.Keys(missingSkills)))
			}
		})
	}

	t.Run("mandatory relationship mutation is rejected", func(t *testing.T) {
		mutated := "A selected skill must follow its catalog predecessor."
		if !containsAny(mutated, mandatoryRelationships) {
			t.Fatal("representative mandatory relationship mutation was not rejected")
		}
	})
}

func containsAny(value string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}

// invariant: rendering/guide-and-doc-templates:guide-scopes-derived (TestGuideScopesDerived)
func TestGuideScopesDerived(t *testing.T) {
	invs := awfAgentsDocInvariants(t)

	base := func(scopes string) map[string]any {
		return map[string]any{
			"prefix":        "awf",
			"vars":          map[string]any{"gateCmd": "./x gate"},
			"layout":        testLayout(),
			"data":          map[string]any{"invariants": invs},
			"commitScopes":  scopes,
			"gatedCommands": gatedCommandsDisplay(),
			"skills":        map[string]bool{},
		}
	}

	// Scopes configured: the list and the allowed-scopes clause render.
	pop := renderGuide(t, base("`config`, `rendering`"))
	wantPop := "- **Conventional Commits, scopes `config`, `rendering`.** One concern per commit; " +
		"stage explicitly, no `git add -A`; the allowed-scope list lives in `audit.allowedScopes`."
	if got := conventionalCommitsBullet(t, pop); got != wantPop {
		t.Errorf("populated scope bullet:\n got: %s\nwant: %s", got, wantPop)
	}

	// Accept-any (scopes unset): the whole scope apparatus collapses to generic
	// prose - no scope list, no audit.allowedScopes clause, no dangling comma.
	empty := renderGuide(t, base(""))
	wantEmpty := "- **Conventional Commits.** One concern per commit; stage explicitly, no `git add -A`."
	if got := conventionalCommitsBullet(t, empty); got != wantEmpty {
		t.Errorf("accept-any scope bullet:\n got: %s\nwant: %s", got, wantEmpty)
	}
}

// renderGuide renders the agents-doc template with the given data. Unlike
// renderGolden it does not assert marker-freedom: awf's real invariant prose
// legitimately quotes `awf:section` as content, which the leak check would flag.
// It still guards against unresolved values and unrendered actions.
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
	asm, parts := render.Assemble(render.ParseSections(expanded), nil, render.HTMLComment)
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
