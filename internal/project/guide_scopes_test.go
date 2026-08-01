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
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// invariant: rendering/guide-and-doc-templates:guide-scopes-derived (TestGuideCatalogRowsAreCompleteSafeAndAdvisory)
// invariant: rendering/workflow-skill-templates:workflow-transitions-advisory (TestGuideCatalogRowsAreCompleteSafeAndAdvisory)
//
// The agent guide renders its commit-scope mention from the $.commitScopes
// render key - never a hand-written token list - and degrades to generic
// Conventional-Commits prose when scopes are accept-any (ADR-0055, ADR-0045).
// The invariants are read from awf's OWN .awf/agents-doc.yaml, not a synthetic
// fixture: a re-introduced hand-written scope entry surfaces as a second
// Conventional-Commits invariant bullet and fails this test.
// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing (TestGuideCatalogRowsAreCompleteSafeAndAdvisory)
// invariant: rendering/workflow-skill-templates:workflow-transitions-advisory (TestGuideCatalogRowsAreCompleteSafeAndAdvisory)
func TestGuideCatalogRowsAreCompleteSafeAndAdvisory(t *testing.T) {
	profile := catalog.Standard.Skills["subagent-driven-development"].Profile
	if profile.Purpose != "Implement a plan through reviewed phase owners." || profile.Trigger != "Use when a plan phase benefits from delegated implementation ownership." {
		t.Fatalf("subagent-driven profile = %#v", profile)
	}
	for _, old := range []string{"reviewed subagent tasks", "delegated implementation tasks"} {
		if strings.Contains(profile.Purpose, old) || strings.Contains(profile.Trigger, old) {
			t.Errorf("subagent-driven profile retains %q", old)
		}
	}
	skills := maps.Clone(catalog.Standard.Skills)
	names := slices.Sorted(maps.Keys(skills))
	skills["local"] = catalog.SkillSpec{Profile: catalog.WorkflowProfile{}}
	names = append(names, "local")
	p := &Project{Cfg: &config.Config{Prefix: "example", Skills: names}, Cat: &catalog.Catalog{Skills: skills}}
	rows := p.skillRows()
	for _, name := range names {
		profile := skills[name].Profile
		kind, purpose, trigger := string(profile.Kind), profile.Purpose, profile.Trigger
		if kind == "" {
			kind = string(catalog.WorkflowTask)
		}
		if purpose == "" {
			purpose = "A project-local skill."
		}
		if trigger == "" {
			trigger = "Use when this skill fits the current work."
		}
		want := "`example-" + name + "` (" + kind + "): " + purpose + " Trigger: " + strings.TrimRight(trigger, ".") + "."
		if len(profile.UsuallyFollows) > 0 {
			want += " Usually follows: " + strings.Join(profile.UsuallyFollows, ", ") + "."
		}
		if len(profile.CommonFollowUps) > 0 {
			want += " Common follow-ups: " + strings.Join(profile.CommonFollowUps, ", ") + "."
		}
		if !strings.Contains(rows, want) {
			t.Errorf("row for %s = missing %q:\n%s", name, want, rows)
		}
	}
	data := map[string]any{"prefix": "example", "vars": map[string]any{}, "layout": testLayout(), "data": map[string]any{}, "commitScopes": "", "gatedCommands": "", "skills": map[string]bool{}, "skillRows": rows}
	out := renderGuide(t, data)
	if !strings.Contains(out, "Any enabled skill may be used whenever its purpose fits") {
		t.Error("guide does not state advisory skill selection")
	}
	p.Cfg.Prefix = ""
	if !strings.Contains(p.skillRows(), "`project-local`") {
		t.Fatal("missing-prefix fallback is not coherent")
	}
	banned := []string{"<no value>", "awf_workflow", "only legal predecessor", "only legal successor", "mandatory successor", "must follow", "must be followed by", "mandatory transition", "router"}
	for _, phrase := range banned {
		if strings.Contains(out, phrase) {
			t.Errorf("guide retains banned routing phrase %q", phrase)
		}
	}
	enabled := slices.Sorted(maps.Keys(catalog.Standard.Skills))
	agents := slices.Sorted(maps.Keys(catalog.Standard.Agents))
	root := scaffold(t, "prefix: example\nskills: ["+strings.Join(enabled, ", ")+"]\nagents: ["+strings.Join(agents, ", ")+"]\ndocs: [roadmap]\ntargets: [pi]\n")
	rendered, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := rendered.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	skillBanned := []string{"<no value>", "awf_workflow", "only legal predecessor", "only legal successor", "mandatory successor", "must follow", "must be followed by", "mandatory transition", "router"}
	for _, file := range files {
		if file.Path == "AGENTS.md" {
			if !strings.Contains(file.Content, "Any enabled skill may be used whenever its purpose fits") {
				t.Error("rendered guide does not state advisory skill selection")
			}
			for _, phrase := range banned {
				if strings.Contains(file.Content, phrase) {
					t.Errorf("rendered guide retains banned routing phrase %q", phrase)
				}
			}
		}
		if !strings.HasSuffix(file.Path, "/SKILL.md") {
			continue
		}
		for _, phrase := range skillBanned {
			if strings.Contains(file.Content, phrase) {
				t.Errorf("%s retains banned routing phrase %q", file.Path, phrase)
			}
		}
	}
	t.Run("skill advisory mutation is rejected", func(t *testing.T) {
		for _, file := range files {
			if !strings.HasSuffix(file.Path, "/SKILL.md") {
				continue
			}
			mutated := file.Content + "\nmust follow\n"
			rejected := false
			for _, phrase := range skillBanned {
				if strings.Contains(mutated, phrase) {
					rejected = true
					break
				}
			}
			if !rejected {
				t.Fatal("skill advisory mutation was not rejected")
			}
			return
		}
		t.Fatal("scaffold rendered no skill")
	})
}

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
