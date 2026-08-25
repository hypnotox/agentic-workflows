package main

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestRunRefusesFullOnlyCommandForCoreProfileAfterStateAdmission(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\n")
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "audit", "HEAD"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "awf audit is unavailable in the selected core governance footprint") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	for _, configYAML := range []string{
		"prefix: example\nprofile: core\nintegrationBranch: main\ndomains: [rendering]\n",
		"profile: [\n",
	} {
		testsupport.WriteAwfConfig(t, root, configYAML)
		stderr.Reset()
		if code := run([]string{"awf", "audit", "HEAD"}, &stdout, &stderr); code != 1 {
			t.Fatalf("invalid capability config exit = %d, stderr = %q", code, stderr.String())
		}
	}
}

func TestRunFullOnlyCapabilityFollowsLiveAuthorityAdmission(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, root, "profile: [\n")
	testsupport.WriteFile(t, config.LockPath(root), `{"awfVersion":"0.39.2","schemaVersion":45,"files":{},"bridgeAttestation":{"version":1,"adrFormatV1From":1,"legacyADRGaps":null}}`)

	var stdout, stderr bytes.Buffer
	if code := runAt(t, root, []string{"awf", "audit", "HEAD"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "below live floor") || strings.Contains(got, "parse config") {
		t.Fatalf("Full-only admission order = %q", got)
	}
}

func TestRunStagedFullOnlyCapabilityUsesIndexConfig(t *testing.T) {
	root := syncedGitProject(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\n")

	var stdout, stderr bytes.Buffer
	if code := runAt(t, root, []string{"awf", "check", "staged", "state"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "selected core governance footprint") {
		t.Fatalf("staged capability consulted working config: %q", stderr.String())
	}
}

func TestRunStagedFullOnlyCapabilityAllowsPreAdoptionIndex(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})

	var stdout, stderr bytes.Buffer
	if code := runAt(t, repo.Root(), []string{"awf", "check", "staged", "state"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "no staged .awf/awf.lock") || strings.Contains(got, "governance footprint") {
		t.Fatalf("pre-adoption staged capability = %q", got)
	}
}

func TestRunDispatchesEveryFullOnlyCommandFamilyByProfile(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		fullCode   int
		fullOutput string
	}{
		{"check repo state", []string{"awf", "check", "repo", "state"}, 0, "findings: 0 errors, 0 warnings"},
		{"check staged state", []string{"awf", "check", "staged", "state"}, 0, "findings: 0 errors, 0 warnings"},
		{"read plan", []string{"awf", "read", "plan", "missing", "1"}, 1, `plan name "missing" not found`},
		{"audit", []string{"awf", "audit", "HEAD"}, 1, "scope: 0 commit(s) in HEAD..HEAD"},
		{"adr", []string{"awf", "adr", "number"}, 1, "no pending ADR to number"},
		{"context", []string{"awf", "context", "README.md"}, 0, "context: live state for this project"},
		{"topic", []string{"awf", "topic", "rendering/missing"}, 1, `current-state topic "rendering/missing" not found`},
		{"new adr", []string{"awf", "new", "adr", "Dispatch Proof"}, 0, "status: created:"},
		{"new plan", []string{"awf", "new", "plan", "Dispatch Proof"}, 0, "status: created:"},
		{"new topic", []string{"awf", "new", "topic", "rendering", "Dispatch Proof"}, 1, `topic domain "rendering" is not configured`},
		{"new domain", []string{"awf", "new", "domain", "dispatch-proof"}, 0, "added docs/domains/dispatch-proof.md"},
		{"remove domain", []string{"awf", "remove", "domain", "rendering"}, 1, `domain "rendering" is not configured`},
	}
	var declared []string
	var visit func(string, []clispec.Command)
	visit = func(prefix string, commands []clispec.Command) {
		for _, command := range commands {
			name := strings.TrimSpace(prefix + " " + command.Name)
			if command.FullOnly {
				declared = append(declared, name)
			}
			visit(name, command.Children)
		}
	}
	visit("", clispec.Commands)
	var pinned []string
	for _, tc := range cases {
		pinned = append(pinned, tc.name)
	}
	slices.Sort(declared)
	slices.Sort(pinned)
	if !slices.Equal(pinned, declared) {
		t.Fatalf("Full-only dispatch matrix drifted: pinned=%v declared=%v", pinned, declared)
	}
	for _, tc := range cases {
		t.Run(tc.name+" core refusal", func(t *testing.T) {
			root := scaffoldProject(t)
			testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\n")
			gitfixture.Add(t, gitfixture.At(root), ".awf/config.yaml")
			testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "is unavailable in the selected core governance footprint") {
				t.Fatalf("exit/output did not prove capability refusal: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
		t.Run(tc.name+" full dispatch", func(t *testing.T) {
			root := scaffoldProject(t)
			testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			combined := stdout.String() + stderr.String()
			if code != tc.fullCode || !strings.Contains(combined, tc.fullOutput) || strings.Contains(stderr.String(), "is unavailable for the selected") {
				t.Fatalf("Full command did not produce its handler-specific result: exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunGetwdError(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return "", errors.New("boom") })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "render"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on getwd error, got %d", code)
	}
	if out.Len() != 0 || errb.String() != "condition: awf: boom\n" {
		t.Errorf("streams stdout=%q stderr=%q", out.String(), errb.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for unknown command, got %d", code)
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("missing unknown-command text: %q", errb.String())
	}
}

func TestRunDispatchError(t *testing.T) {
	// render in a bare dir: project.Open fails -> handler error -> exit 1.
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "render"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on dispatch error, got %d", code)
	}
	if !strings.HasPrefix(errb.String(), "condition: awf:") {
		t.Errorf("expected typed diagnostic, got %q", errb.String())
	}
}

// TestRunDispatchArms drives every switch arm through run() against a scaffolded
// project, covering the dispatch statements. The check children are spelled as
// full argv because they are subcommands, not top-level names: a single-token
// loop could no longer reach them.
func TestRunDispatchArms(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"render", []string{"awf", "render"}},
		{"check repo drift", []string{"awf", "check", "repo", "drift"}},
		{"check repo state", []string{"awf", "check", "repo", "state"}},
		{"list", []string{"awf", "list"}},
		{"upgrade", []string{"awf", "upgrade"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldProject(t)
			testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
			var out, errb bytes.Buffer
			if code := run(tc.args, &out, &errb); code != 0 {
				t.Fatalf("%s: expected exit 0, got %d (%s)", tc.name, code, errb.String())
			}
		})
	}
	t.Run("init", func(t *testing.T) {
		root := t.TempDir()
		testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
			t.Fatalf("init: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
}
