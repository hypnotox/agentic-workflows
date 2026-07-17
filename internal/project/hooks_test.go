package project

import (
	"strings"
	"testing"
)

// hookFiles renders a project with the given config and returns the
// .awf/hooks/*.sh RenderedFiles keyed by payload name.
func hookFiles(t *testing.T, configYAML string) map[string]RenderedFile {
	t.Helper()
	root := scaffold(t, configYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]RenderedFile{}
	for _, f := range out {
		if rest, ok := strings.CutPrefix(f.Path, ".awf/hooks/"); ok {
			found[strings.TrimSuffix(rest, ".sh")] = f
		}
	}
	return found
}

// With the singleton enabled, exactly the three payloads render under
// .awf/hooks/; absent or disabled, none do.
// invariant: hook-payloads-rendered
func TestHookPayloadsRendered(t *testing.T) {
	got := hookFiles(t, "prefix: example\nhooks:\n  enabled: true\n")
	for _, name := range hookNames {
		if _, ok := got[name]; !ok {
			t.Errorf("expected .awf/hooks/%s.sh to render when enabled", name)
		}
	}
	if len(got) != len(hookNames) {
		t.Errorf("rendered %d payloads, want exactly %d: %v", len(got), len(hookNames), got)
	}

	for _, cfg := range []string{
		"prefix: example\n",
		"prefix: example\nhooks:\n  enabled: false\n",
	} {
		if got := hookFiles(t, cfg); len(got) != 0 {
			t.Errorf("expected no hook payloads for config %q, got %v", cfg, got)
		}
	}
}

// With every command var unset, each payload degrades to runnable generic awf
// forms - pin-aware shim plus `awf check` / `awf commit-gate "$1"` - with no
// unresolved-value token (the ADR-0045 fallback contract).
// invariant: hook-payloads-fallback-safe
func TestHookPayloadsFallbackSafe(t *testing.T) {
	got := hookFiles(t, "prefix: example\nhooks:\n  enabled: true\n")
	wantCmd := map[string]string{
		"pre-commit": "awf check",
		"commit-msg": `awf commit-gate "$1"`,
		"pre-push":   "awf check",
	}
	for name, f := range got {
		lines := strings.Split(f.Content, "\n")
		if lines[0] != "#!/usr/bin/env bash" {
			t.Errorf("%s: first line = %q, want the shebang", name, lines[0])
		}
		if !strings.Contains(f.Content, "set -euo pipefail") {
			t.Errorf("%s: missing set -euo pipefail", name)
		}
		if !strings.Contains(f.Content, `[ -f .awf/bootstrap.sh ]`) {
			t.Errorf("%s: missing the pin-aware fallback shim:\n%s", name, f.Content)
		}
		if !strings.Contains(f.Content, wantCmd[name]) {
			t.Errorf("%s: missing fallback command %q:\n%s", name, wantCmd[name], f.Content)
		}
		if strings.Contains(f.Content, "<no value>") {
			t.Errorf("%s: unresolved-value token in output:\n%s", name, f.Content)
		}
	}
}

// With the command vars set, each payload runs them verbatim and omits the
// pin-aware shim.
func TestHookPayloadsUseConfiguredCommands(t *testing.T) {
	got := hookFiles(t, `prefix: example
vars:
  checkCmd: ./x check
  gateCmd: ./x gate
  gateCmdFull: ./x gate full
  commitGateCmd: ./x commit-gate
  proseGateCmd: ./x prose-gate
hooks:
  enabled: true
`)
	want := map[string][]string{
		"pre-commit": {"./x check\n./x gate\n./x prose-gate\n"},
		"commit-msg": {"./x commit-gate \"$1\"\n"},
		"pre-push":   {"./x gate full\n"},
	}
	for name, f := range got {
		for _, w := range want[name] {
			if !strings.Contains(f.Content, w) {
				t.Errorf("%s: missing %q:\n%s", name, w, f.Content)
			}
		}
		if strings.Contains(f.Content, "bootstrap.sh") {
			t.Errorf("%s: pin-aware shim should be omitted when commands are set:\n%s", name, f.Content)
		}
	}
	// pre-push falls back through the chain: gateCmd when gateCmdFull is unset.
	chain := hookFiles(t, "prefix: example\nvars:\n  gateCmd: ./x gate\nhooks:\n  enabled: true\n")
	if f := chain["pre-push"]; !strings.Contains(f.Content, "./x gate\n") {
		t.Errorf("pre-push: want gateCmd fallback, got:\n%s", f.Content)
	}
}

// Hook payload template ids label as their script name in advisories.
func TestHookPayloadLabel(t *testing.T) {
	if got, want := artifactLabel("hooks/pre-commit.sh.tmpl"), "hooks pre-commit"; got != want {
		t.Errorf("artifactLabel = %q, want %q", got, want)
	}
}
