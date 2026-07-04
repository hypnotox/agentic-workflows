package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// projectWithScopes opens a scaffolded project with two meaning-bearing scopes
// and both gate-command vars set (every registry key populated).
func projectWithScopes(t *testing.T) *Project {
	t.Helper()
	root := scaffold(t, "prefix: awftest\n"+
		"vars:\n  gateCmd: ./x gate\n  checkCmd: ./x check\n"+
		"skills: []\nagents: []\n"+
		"audit:\n  allowedScopes:\n"+
		"    - {name: adr, meaning: ADR docs}\n"+
		"    - {name: rendering, meaning: the render engine}\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// projectAcceptAny opens a scaffolded project with no audit config (accept-any
// scopes) and no gate vars — the scope keys and gate keys are all absent.
func projectAcceptAny(t *testing.T) *Project {
	t.Helper()
	p, err := Open(scaffold(t, "prefix: bare\nvars: {}\nskills: []\nagents: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlaceholderRegistry(t *testing.T) {
	reg := projectWithScopes(t).placeholderRegistry()
	for _, k := range []string{"commitScopeList", "commitScopeTable", "commitScopeSentence", "prefix", "gateCmd", "checkCmd"} {
		if reg[k] == "" {
			t.Errorf("populated registry missing key %q", k)
		}
	}

	bare := projectAcceptAny(t).placeholderRegistry()
	if bare["prefix"] != "bare" {
		t.Errorf("prefix = %q, want bare", bare["prefix"])
	}
	for _, k := range []string{"commitScopeList", "commitScopeTable", "commitScopeSentence", "gateCmd", "checkCmd"} {
		if _, ok := bare[k]; ok {
			t.Errorf("accept-any/no-vars registry should not carry %q", k)
		}
	}
}

func TestCommitScopeTableAndSentence(t *testing.T) {
	p := projectWithScopes(t)
	table := p.commitScopeTable()
	if !strings.Contains(table, "| `adr` | ADR docs |") || !strings.Contains(table, "| `rendering` | the render engine |") {
		t.Errorf("table missing rows:\n%s", table)
	}
	if s := p.commitScopeSentence(); !strings.Contains(s, "`adr`, `rendering`") {
		t.Errorf("sentence = %q", s)
	}

	empty := projectAcceptAny(t)
	if empty.commitScopeTable() != "" {
		t.Error("accept-any commitScopeTable must be empty")
	}
	if empty.commitScopeSentence() != "" {
		t.Error("accept-any commitScopeSentence must be empty")
	}
}

func TestSubstitutePlaceholders(t *testing.T) {
	p := projectWithScopes(t)
	reg := p.placeholderRegistry()

	// Fast path: no placeholder token.
	if out, err := p.substitutePlaceholders("x", "plain prose, no tokens", reg); err != nil || out != "plain prose, no tokens" {
		t.Errorf("fast path: out=%q err=%v", out, err)
	}

	// Known keys substitute.
	out, err := p.substitutePlaceholders("x", "Scopes: {{=awf:commitScopeList}}\n{{=awf:commitScopeTable}}", reg)
	if err != nil {
		t.Fatalf("known keys: %v", err)
	}
	if !strings.Contains(out, "`adr`, `rendering`") || !strings.Contains(out, "| `adr` | ADR docs |") {
		t.Errorf("known keys not substituted:\n%s", out)
	}

	// Error cases.
	for _, tc := range []struct{ name, body, want string }{
		{"unknown", "{{=awf:nope}}", "unknown or empty placeholder"},
		{"two-unknown", "{{=awf:nope}} {{=awf:alsobad}}", "unknown or empty placeholder"},
		{"residual-space", "{{= awf:commitScopeList}}", "malformed"},
		{"residual-empty-ident", "{{=awf:}}", "malformed"},
		{"residual-hyphen", "{{=awf:commit-scope}}", "malformed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.substitutePlaceholders("part", tc.body, reg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s: err=%v, want containing %q", tc.name, err, tc.want)
			}
		})
	}

	// Empty-value key (accept-any project has no commitScopeTable) → hard error.
	bareReg := projectAcceptAny(t).placeholderRegistry()
	if _, err := projectAcceptAny(t).substitutePlaceholders("part", "{{=awf:commitScopeTable}}", bareReg); err == nil {
		t.Error("empty-value key: want error, got nil")
	}
}

// TestPlaceholderSubstitutionInSync drives the planSections wiring end to end:
// a workflow part using {{=awf:commitScopeTable}} renders the table, and a part
// with an unknown placeholder fails Sync.
func TestPlaceholderSubstitutionInSync(t *testing.T) {
	cfg := "prefix: awftest\nvars: {}\nskills: []\nagents: []\n" +
		"audit:\n  allowedScopes:\n    - {name: adr, meaning: ADR docs}\n"

	good := scaffoldFiles(t, cfg, map[string]string{
		"parts/workflow/commit-discipline.md": "## Commit discipline\n\n{{=awf:commitScopeTable}}\n",
	})
	p, err := Open(good)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(good, "docs", "workflow.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "| `adr` | ADR docs |") {
		t.Errorf("rendered workflow.md missing derived table:\n%s", b)
	}

	bad := scaffoldFiles(t, cfg, map[string]string{
		"parts/workflow/commit-discipline.md": "## Commit discipline\n\n{{=awf:bogus}}\n",
	})
	bp, err := Open(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := bp.Sync(); err == nil {
		t.Error("sync with unknown placeholder: want error, got nil")
	}
}
