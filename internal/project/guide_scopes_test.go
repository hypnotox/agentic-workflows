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

// invariant: rendering/guide-and-doc-templates:guide-scopes-derived
//
// The agent guide renders its commit-scope mention from the $.commitScopes
// render key - never a hand-written token list - and degrades to generic
// Conventional-Commits prose when scopes are accept-any (ADR-0055, ADR-0045).
// The invariants are read from awf's OWN .awf/agents-doc.yaml, not a synthetic
// fixture: a re-introduced hand-written scope entry surfaces as a second
// Conventional-Commits invariant bullet and fails this test.
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
