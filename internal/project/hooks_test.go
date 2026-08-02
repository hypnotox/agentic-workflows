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
	p, err := Open(testContext(t), root)
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

// With the singleton enabled, exactly the four payloads render under
// .awf/hooks/; absent or disabled, none do. The expected set is spelled out
// rather than derived from hookNames, which would make the assertion agree with
// whatever that list happens to say: the claim names these paths, so the test
// has to name them too for a wrong set to be able to fail.
// invariant: rendering/singletons-and-payloads:hook-payloads-rendered (TestHookPayloadsRendered)
func TestHookPayloadsRendered(t *testing.T) {
	want := []string{"pre-commit", "commit-msg", "pre-push", "pre-merge-commit"}
	got := hookFiles(t, "prefix: example\nintegrationBranch: main\nhooks:\n  enabled: true\n")
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("expected .awf/hooks/%s.sh to render when enabled", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rendered %d payloads, want exactly %d: %v", len(got), len(want), got)
	}

	for _, cfg := range []string{
		"prefix: example\nintegrationBranch: main\n",
		"prefix: example\nintegrationBranch: main\nhooks:\n  enabled: false\n",
	} {
		if got := hookFiles(t, cfg); len(got) != 0 {
			t.Errorf("expected no hook payloads for config %q, got %v", cfg, got)
		}
	}
}

// With every command var unset, each payload degrades to a runnable script
// whose awf-verb commands resolve to ./awf forms when the runner singleton is
// enabled and to the generic awf forms otherwise, with no inline resolution
// shim and no unresolved-value token (ADR-0156 Decision 4).
// invariant: rendering/companion-scripts:hook-payloads-fallback-safe (TestHookPayloadsFallbackSafe)
func TestHookPayloadsFallbackSafe(t *testing.T) {
	for _, tc := range []struct {
		name, config, awf string
	}{
		{"runner enabled", "prefix: example\nintegrationBranch: main\nhooks:\n  enabled: true\nrunner:\n  enabled: true\n", "./awf"},
		{"runner disabled", "prefix: example\nintegrationBranch: main\nhooks:\n  enabled: true\n", "awf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := hookFiles(t, tc.config)
			wantCmds := map[string][]string{
				"pre-commit":       {tc.awf + " check\n"},
				"commit-msg":       {tc.awf + ` check staged commit "$1"` + "\n"},
				"pre-push":         {tc.awf + " check\n"},
				"pre-merge-commit": {tc.awf + " check staged\n"},
			}
			for name, f := range got {
				lines := strings.Split(f.Content, "\n")
				if lines[0] != "#!/usr/bin/env bash" {
					t.Errorf("%s: first line = %q, want the shebang", name, lines[0])
				}
				if !strings.Contains(f.Content, "set -euo pipefail") {
					t.Errorf("%s: missing set -euo pipefail", name)
				}
				for _, want := range wantCmds[name] {
					if !strings.Contains(f.Content, "\n"+want) {
						t.Errorf("%s: missing fallback command %q:\n%s", name, want, f.Content)
					}
				}
				if strings.Contains(f.Content, "awf() {") {
					t.Errorf("%s: payloads carry no inline resolution shim:\n%s", name, f.Content)
				}
				if strings.Contains(f.Content, "<no value>") {
					t.Errorf("%s: unresolved-value token in output:\n%s", name, f.Content)
				}
			}
		})
	}
}

// With the command vars set, each payload runs them verbatim and omits the
// pin-aware shim.
func TestHookPayloadsUseConfiguredCommands(t *testing.T) {
	got := hookFiles(t, `prefix: example
integrationBranch: main
vars:
  checkCmd: ./x check
  gateCmd: ./x gate
  gateCmdFull: ./x gate full
  commitGateCmd: ./x commit-gate
hooks:
  enabled: true
`)
	want := map[string][]string{
		"pre-commit":       {"./x check\n./x gate\n"},
		"commit-msg":       {"./x commit-gate \"$1\"\n"},
		"pre-push":         {"./x gate full\n"},
		"pre-merge-commit": {"./x check staged\n"},
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
	chain := hookFiles(t, "prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: ./x gate\nhooks:\n  enabled: true\n")
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
