package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// scaffoldedProject writes a curated-default scaffold (10 core skills, 3 agents,
// 0 docs - no doc is core after ADR-0043 - no domains) and syncs it.
func scaffoldedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	b, _, err := project.ScaffoldConfig("example", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, string(b))
	if err := initializeProject(root, io.Discard); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return root
}

func readConfig(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunAddAcrossKinds(t *testing.T) {
	root := scaffoldedProject(t)

	// Catalog skill into a block-with-items array.
	if err := runEnable(root, "skill", "tdd", false, io.Discard); err != nil {
		t.Fatalf("add skill tdd: %v", err)
	}
	if !strings.Contains(readConfig(t, root), "- tdd") {
		t.Error("tdd not added")
	}
	// Opt-in doc.
	if err := runEnable(root, "doc", "pitfalls", false, io.Discard); err != nil {
		t.Fatalf("add doc pitfalls: %v", err)
	}
	// Freeform domain into an absent array (scaffold omits domains:).
	if err := runEnable(root, "domain", "payments", false, io.Discard); err != nil {
		t.Fatalf("add domain payments: %v", err)
	}
	if !strings.Contains(readConfig(t, root), "domains:") {
		t.Error("domains: block not created")
	}
	stubPath := filepath.Join(root, ".awf", "domains", "parts", "payments", "current-state.md")
	stub, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("read scaffolded current-state.md: %v", err)
	}
	if !strings.Contains(string(stub), `"payments" domain`) {
		t.Errorf("stub does not name the domain: %q", stub)
	}
	if !strings.Contains(string(stub), "doc-standard.md") {
		t.Errorf("stub does not point at the doc standard: %q", stub)
	}

	// Rejections.
	if err := runEnable(root, "bogus", "x", false, io.Discard); err == nil {
		t.Error("expected unknown-kind error")
	}
	if err := runEnable(root, "skill", "no-such", false, io.Discard); err == nil {
		t.Error("expected not-in-catalog error")
	}
	if err := runEnable(root, "domain", "bad/name", false, io.Discard); err == nil {
		t.Error("expected invalid-domain-name error")
	}
	if err := runEnable(root, "skill", "tdd", false, io.Discard); err == nil {
		t.Error("expected already-enabled error")
	}
}

// A project-local artifact (ADR-0068) lists under its declared kind with the
// state "local" - not "tuned", which would hide that it is not a catalog skill.
func TestRunListShowsLocalArtifactAsLocal(t *testing.T) {
	root := scaffoldedProject(t)
	if err := runNew(root, "skill", []string{"deploy-check", "Verify the deploy."}, io.Discard); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runList(root, "skill", &out); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.Contains(line, "deploy-check") {
			if !strings.Contains(line, "local") {
				t.Errorf("local skill listed as %q, want state local", strings.TrimSpace(line))
			}
			return
		}
	}
	t.Errorf("local skill not listed at all:\n%s", out.String())
}

// The driver must refuse add/remove BEFORE the config rewrite: a stale binary
// must not leave a modified config.yaml with nothing rendered (the half-mutated
// state the chained sync's gate could only catch after the write). Driven
// through run() because the gate now lives in the driver, ahead of the handler.
func TestRunAddRemoveGateBeforeConfigWrite(t *testing.T) {
	root := scaffoldedProject(t)
	lockPath := filepath.Join(root, ".awf", "awf.lock")
	lock, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	lock.AWFVersion = "99.0.0" // project rendered by a newer awf → binary is behind
	if err := lock.Save(lockPath); err != nil {
		t.Fatal(err)
	}
	before := readConfig(t, root)
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "enable", "skill", "tdd"}, &out, &errb); code != 1 {
		t.Errorf("expected exit 1 from enable on a behind binary, got %d (%s)", code, errb.String())
	}
	if got := readConfig(t, root); got != before {
		t.Errorf("enable modified config.yaml despite failing the gate:\n%s", got)
	}
	if code := runAt(t, root, []string{"awf", "disable", "skill", "brainstorming"}, &out, &errb); code != 1 {
		t.Errorf("expected exit 1 from disable on a behind binary, got %d (%s)", code, errb.String())
	}
	if got := readConfig(t, root); got != before {
		t.Errorf("disable modified config.yaml despite failing the gate:\n%s", got)
	}
}

// TestRunAddDomainScaffoldIdempotent confirms add domain never clobbers an
// existing current-state.md - pre-authored content (e.g. from a prior
// add/remove cycle, or hand-authored before the domain was ever enabled)
// survives a fresh add.
func TestRunAddDomainScaffoldIdempotent(t *testing.T) {
	root := scaffoldedProject(t)

	stubDir := filepath.Join(root, ".awf", "domains", "parts", "billing")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const handAuthored = "Billing settled onto Stripe in 2026Q1; see ADR-0031.\n"
	stubPath := filepath.Join(stubDir, "current-state.md")
	if err := os.WriteFile(stubPath, []byte(handAuthored), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runEnable(root, "domain", "billing", false, io.Discard); err != nil {
		t.Fatalf("add domain billing: %v", err)
	}

	got, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != handAuthored {
		t.Errorf("add domain overwrote hand-authored current-state.md: got %q, want %q", got, handAuthored)
	}
}

// TestRunAddRemoveFlowStyle confirms a hand-edited flow-style array is now edited
// (not refused): SetArrayMember normalizes it to block style. minimalYAML uses
// flow-style `skills: [tdd]`. brainstorming references no vars and is not
// doc-gated, so the post-add sync renders cleanly under minimalYAML's seed.
// invariant: tooling/cli:target-cli
func TestRunTargetCLI(t *testing.T) {
	root := scaffoldedProject(t) // no targets: key → defaults to claude

	// list target before any change: claude enabled, cursor available.
	var buf bytes.Buffer
	if err := runList(root, "target", &buf); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "claude") || !strings.Contains(out, "enabled") ||
		!strings.Contains(out, "cursor") || !strings.Contains(out, "available") {
		t.Errorf("list target (initial):\n%s", out)
	}

	// add cursor must materialize the full resolved list, not drop the defaulted claude.
	if err := runEnable(root, "target", "cursor", false, io.Discard); err != nil {
		t.Fatalf("add target cursor: %v", err)
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "- claude") || !strings.Contains(cfg, "- cursor") {
		t.Errorf("expected targets [claude, cursor]:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(root, ".cursor", "skills")); err != nil {
		t.Errorf("cursor tree not rendered after add: %v", err)
	}

	// Rejections.
	if err := runEnable(root, "target", "nope", false, io.Discard); err == nil {
		t.Error("expected unknown-target error")
	}
	if err := runEnable(root, "target", "cursor", false, io.Discard); err == nil {
		t.Error("expected already-enabled error")
	}
	if err := runDisable(root, "target", "nope", false, false, io.Discard); err == nil {
		t.Error("expected unknown-target error on remove")
	}

	// A known target name on a broken config surfaces the project.Open error.
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broken, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, ".awf", "config.yaml"), []byte("prefix: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runEnable(broken, "target", "cursor", false, io.Discard); err == nil {
		t.Error("expected project.Open error to surface from add target")
	}

	// remove claude → [cursor]; removing the last target is refused.
	if err := runDisable(root, "target", "claude", false, false, io.Discard); err != nil {
		t.Fatalf("remove target claude: %v", err)
	}
	if strings.Contains(readConfig(t, root), "- claude") {
		t.Error("claude not removed")
	}
	if err := runDisable(root, "target", "claude", false, false, io.Discard); err == nil {
		t.Error("expected not-enabled error")
	}
	if err := runDisable(root, "target", "cursor", false, false, io.Discard); err == nil {
		t.Error("expected cannot-remove-last-target error")
	}
}

func TestRunBootstrapCLI(t *testing.T) {
	// scaffoldProject uses minimalYAML, which carries no bootstrap key (disabled).
	root := scaffoldProject(t)

	// list bootstrap before any change: available.
	var buf bytes.Buffer
	if err := runList(root, "bootstrap", &buf); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "bootstrap:") ||
		!strings.Contains(out, ".awf/bootstrap.sh") || !strings.Contains(out, ".awf/upgrade.sh") ||
		!strings.Contains(out, "available") {
		t.Errorf("list bootstrap (initial):\n%s", out)
	}

	// remove when disabled errors.
	if err := runDisable(root, "bootstrap", "", false, false, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "is not enabled") {
		t.Errorf("expected is-not-enabled error, got %v", err)
	}

	// add enables it (config gains enabled: true, sync runs).
	if err := runEnable(root, "bootstrap", "", false, io.Discard); err != nil {
		t.Fatalf("add bootstrap: %v", err)
	}
	cfg := readConfig(t, root)
	if !strings.Contains(cfg, "bootstrap:") || !strings.Contains(cfg, "enabled: true") {
		t.Errorf("bootstrap not enabled in config:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "bootstrap.sh")); err != nil {
		t.Errorf("bootstrap.sh not rendered after add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "upgrade.sh")); err != nil {
		t.Errorf("upgrade.sh not rendered after add: %v", err)
	}

	// list bootstrap now reports enabled.
	buf.Reset()
	if err := runList(root, "bootstrap", &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "enabled") {
		t.Errorf("list bootstrap (enabled):\n%s", buf.String())
	}

	// add when already enabled errors.
	if err := runEnable(root, "bootstrap", "", false, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already enabled") {
		t.Errorf("expected already-enabled error, got %v", err)
	}

	// remove disables it and prunes the rendered file.
	if err := runDisable(root, "bootstrap", "", false, false, io.Discard); err != nil {
		t.Fatalf("remove bootstrap: %v", err)
	}
	if !strings.Contains(readConfig(t, root), "enabled: false") {
		t.Errorf("bootstrap not disabled in config:\n%s", readConfig(t, root))
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "bootstrap.sh")); !os.IsNotExist(err) {
		t.Errorf("bootstrap.sh not pruned after remove: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "upgrade.sh")); !os.IsNotExist(err) {
		t.Errorf("upgrade.sh not pruned after remove: err=%v", err)
	}

	// A broken config surfaces the project.Open error from addRemoveBootstrap.
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broken, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, ".awf", "config.yaml"), []byte("prefix: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runEnable(broken, "bootstrap", "", false, io.Discard); err == nil {
		t.Error("expected project.Open error to surface from add bootstrap")
	}
}

// TestDispatchBootstrap covers run()'s nameless-bootstrap dispatch branches
// (ADR-0040): `awf enable bootstrap` / `awf disable bootstrap` carry no <name> arg, so
// they reach the handler via the bespoke len==3 cases rather than the len==4 path.
func TestDispatchBootstrap(t *testing.T) {
	root := scaffoldProject(t) // minimalYAML: no bootstrap key (disabled)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })

	// add bootstrap (3 args, no name) enables it.
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "enable", "bootstrap"}, &out, &errb); code != 0 {
		t.Fatalf("add bootstrap dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "enabled: true") {
		t.Errorf("bootstrap not enabled after add dispatch:\n%s", cfg)
	}

	// remove bootstrap (3 args, no name) disables it.
	errb.Reset()
	if code := run([]string{"awf", "disable", "bootstrap"}, &out, &errb); code != 0 {
		t.Fatalf("remove bootstrap dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "enabled: false") {
		t.Errorf("bootstrap not disabled after remove dispatch:\n%s", cfg)
	}
}

// TestRunHooksCLI mirrors TestRunBootstrapCLI for the git-hook payloads
// singleton (ADR-0048): list state, the add/remove toggle round-trip with
// render/prune, and the guard errors.
func TestRunHooksCLI(t *testing.T) {
	// scaffoldProject uses minimalYAML, which carries no hooks key (disabled).
	root := scaffoldProject(t)

	// list hooks before any change: available, one row per payload.
	var buf bytes.Buffer
	if err := runList(root, "hooks", &buf); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "hooks:") ||
		!strings.Contains(out, ".awf/hooks/pre-commit.sh") ||
		!strings.Contains(out, ".awf/hooks/commit-msg.sh") ||
		!strings.Contains(out, ".awf/hooks/pre-push.sh") ||
		!strings.Contains(out, "available") {
		t.Errorf("list hooks (initial):\n%s", out)
	}

	// remove when disabled errors.
	if err := runDisable(root, "hooks", "", false, false, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "is not enabled") {
		t.Errorf("expected is-not-enabled error, got %v", err)
	}

	// add enables it (config gains enabled: true, sync renders the payloads).
	if err := runEnable(root, "hooks", "", false, io.Discard); err != nil {
		t.Fatalf("add hooks: %v", err)
	}
	cfg := readConfig(t, root)
	if !strings.Contains(cfg, "hooks:") || !strings.Contains(cfg, "enabled: true") {
		t.Errorf("hooks not enabled in config:\n%s", cfg)
	}
	for _, n := range []string{"pre-commit", "commit-msg", "pre-push"} {
		if _, err := os.Stat(filepath.Join(root, ".awf", "hooks", n+".sh")); err != nil {
			t.Errorf("%s.sh not rendered after add: %v", n, err)
		}
	}

	// list hooks now reports enabled.
	buf.Reset()
	if err := runList(root, "hooks", &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "enabled") {
		t.Errorf("list hooks (enabled):\n%s", buf.String())
	}

	// add when already enabled errors.
	if err := runEnable(root, "hooks", "", false, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already enabled") {
		t.Errorf("expected already-enabled error, got %v", err)
	}

	// remove disables it and prunes the rendered files and their directory.
	if err := runDisable(root, "hooks", "", false, false, io.Discard); err != nil {
		t.Fatalf("remove hooks: %v", err)
	}
	if !strings.Contains(readConfig(t, root), "enabled: false") {
		t.Errorf("hooks not disabled in config:\n%s", readConfig(t, root))
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "hooks")); !os.IsNotExist(err) {
		t.Errorf(".awf/hooks/ not pruned after remove: err=%v", err)
	}
}

// TestDispatchHooks covers run()'s nameless-hooks dispatch branches (ADR-0048),
// mirroring TestDispatchBootstrap.
func TestDispatchHooks(t *testing.T) {
	root := scaffoldProject(t) // minimalYAML: no hooks key (disabled)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })

	var out, errb bytes.Buffer
	if code := run([]string{"awf", "enable", "hooks"}, &out, &errb); code != 0 {
		t.Fatalf("add hooks dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "hooks:") || !strings.Contains(cfg, "enabled: true") {
		t.Errorf("hooks not enabled after add dispatch:\n%s", cfg)
	}

	errb.Reset()
	if code := run([]string{"awf", "disable", "hooks"}, &out, &errb); code != 0 {
		t.Fatalf("remove hooks dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "enabled: false") {
		t.Errorf("hooks not disabled after remove dispatch:\n%s", cfg)
	}
}

// TestRunRunnerCLI mirrors TestRunBootstrapCLI for the command-runner singleton
// (ADR-0101): list state, the add/remove toggle round-trip with render/prune, and
// the guard errors.
func TestRunRunnerCLI(t *testing.T) {
	root := scaffoldProject(t) // minimalYAML: no runner key (disabled)

	// list runner before any change: available.
	var buf bytes.Buffer
	if err := runList(root, "runner", &buf); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "runner:") ||
		!strings.Contains(out, "x") || !strings.Contains(out, "available") {
		t.Errorf("list runner (initial):\n%s", out)
	}

	// remove when disabled errors.
	if err := runDisable(root, "runner", "", false, false, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "is not enabled") {
		t.Errorf("expected is-not-enabled error, got %v", err)
	}

	// add enables it (config gains enabled: true, sync renders x at the root).
	if err := runEnable(root, "runner", "", false, io.Discard); err != nil {
		t.Fatalf("add runner: %v", err)
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "runner:") || !strings.Contains(cfg, "enabled: true") {
		t.Errorf("runner not enabled in config:\n%s", readConfig(t, root))
	}
	if _, err := os.Stat(filepath.Join(root, "x")); err != nil {
		t.Errorf("x not rendered after add: %v", err)
	}

	// list runner now reports enabled.
	buf.Reset()
	if err := runList(root, "runner", &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "enabled") {
		t.Errorf("list runner (enabled):\n%s", buf.String())
	}

	// add when already enabled errors.
	if err := runEnable(root, "runner", "", false, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already enabled") {
		t.Errorf("expected already-enabled error, got %v", err)
	}

	// remove disables it and prunes the rendered runner.
	if err := runDisable(root, "runner", "", false, false, io.Discard); err != nil {
		t.Fatalf("remove runner: %v", err)
	}
	if !strings.Contains(readConfig(t, root), "enabled: false") {
		t.Errorf("runner not disabled in config:\n%s", readConfig(t, root))
	}
	if _, err := os.Stat(filepath.Join(root, "x")); !os.IsNotExist(err) {
		t.Errorf("x not pruned after remove: err=%v", err)
	}
}

// TestDispatchRunner mirrors TestDispatchBootstrap: the runner is a nameless
// singleton reached through the enable/disable dispatch, and handing it a name is
// a usage error.
func TestDispatchRunner(t *testing.T) {
	root := scaffoldProject(t) // minimalYAML: no runner key (disabled)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })

	var out, errb bytes.Buffer
	if code := run([]string{"awf", "enable", "runner"}, &out, &errb); code != 0 {
		t.Fatalf("add runner dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "runner:") || !strings.Contains(cfg, "enabled: true") {
		t.Errorf("runner not enabled after add dispatch:\n%s", cfg)
	}

	// A name is a usage error - the runner is a nameless singleton.
	errb.Reset()
	if code := run([]string{"awf", "enable", "runner", "x"}, &out, &errb); code == 0 ||
		!strings.Contains(errb.String(), "takes no name") {
		t.Errorf("expected takes-no-name usage error, got code=%d err=%q", code, errb.String())
	}

	errb.Reset()
	if code := run([]string{"awf", "disable", "runner"}, &out, &errb); code != 0 {
		t.Fatalf("remove runner dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "enabled: false") {
		t.Errorf("runner not disabled after remove dispatch:\n%s", cfg)
	}
}

func TestRunAddRemoveFlowStyle(t *testing.T) {
	root := scaffoldProject(t)
	if err := runEnable(root, "skill", "bugfix", false, io.Discard); err != nil {
		t.Fatalf("add to flow-style array: %v", err)
	}
	cfg := readConfig(t, root)
	if !strings.Contains(cfg, "- bugfix") || !strings.Contains(cfg, "- tdd") {
		t.Errorf("expected block-style skills with both members:\n%s", cfg)
	}
	if err := runDisable(root, "skill", "tdd", false, false, io.Discard); err != nil {
		t.Fatalf("remove from (now block) array: %v", err)
	}
	if strings.Contains(readConfig(t, root), "- tdd") {
		t.Error("tdd not removed")
	}
}

// Adding an unclosed skill enables its full missing forward closure in one
// rewrite, printing a provenance plan (ADR-0081 Decision 4). This replaces
// the ADR-0050 pairing note and the ADR-0013 doc advisory.
// invariant: tooling/cli:add-applies-closure-plan
func TestRunAddAppliesClosurePlan(t *testing.T) {
	root := scaffoldProject(t) // minimalYAML: skills [tdd], agents []
	var out bytes.Buffer
	if err := runEnable(root, "skill", "reviewing-impl", false, &out); err != nil {
		t.Fatalf("add skill reviewing-impl: %v", err)
	}
	// invariant: tooling/cli:add-skill-pairs-agent
	for _, line := range []string{
		"plan: + skill reviewing-impl\n",
		"plan: + skill executing-plans (required by reviewing-impl)\n",
		"plan: + skill retrospective (required by reviewing-impl)\n",
		"plan: + agent code-reviewer (required by reviewing-impl)\n",
	} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("plan output missing %q, got:\n%s", line, out.String())
		}
	}
	cfg := readConfig(t, root)
	for _, want := range []string{"- reviewing-impl", "- executing-plans", "- subagent-driven-development", "- retrospective", "- code-reviewer"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("closure member %q missing from config:\n%s", want, cfg)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "agents", "code-reviewer.md")); err != nil {
		t.Errorf("code-reviewer not rendered after closure add: %v", err)
	}

	// Adding a doc-gated skill enables its doc as a plan op.
	out.Reset()
	if err := runEnable(root, "skill", "roadmap-graduation", false, &out); err != nil {
		t.Fatalf("add roadmap-graduation: %v", err)
	}
	if !strings.Contains(out.String(), "plan: + doc roadmap (required by roadmap-graduation)") {
		t.Errorf("expected the doc plan op, got %q", out.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "- roadmap") {
		t.Errorf("roadmap doc missing from config:\n%s", cfg)
	}
}

// --dry-run prints the plan and leaves the config byte-identical (ADR-0081
// Decision 6); a graph-only flag on a non-graph kind is a usage error.
func TestRunAddRemoveDryRunAndFlagGuard(t *testing.T) {
	root := scaffoldedProject(t)
	before := readConfig(t, root)
	var out bytes.Buffer
	if err := runEnable(root, "skill", "roadmap-graduation", true, &out); err != nil {
		t.Fatalf("add --dry-run: %v", err)
	}
	if !strings.Contains(out.String(), "plan: + skill roadmap-graduation") {
		t.Errorf("dry-run should print the plan, got %q", out.String())
	}
	if got := readConfig(t, root); got != before {
		t.Errorf("dry-run must not touch the config:\n%s", got)
	}
	out.Reset()
	if err := runDisable(root, "skill", "retrospective", false, true, &out); err != nil {
		t.Fatalf("remove --dry-run: %v", err)
	}
	if !strings.Contains(out.String(), "plan: - skill retrospective") {
		t.Errorf("remove dry-run should print the plan, got %q", out.String())
	}
	if got := readConfig(t, root); got != before {
		t.Errorf("remove dry-run must not touch the config:\n%s", got)
	}
	if err := runEnable(root, "domain", "payments", true, io.Discard); err == nil || !strings.Contains(err.Error(), "graph flags") {
		t.Errorf("expected graph-flag usage error for domain, got %v", err)
	}
	if err := runDisable(root, "hooks", "", true, false, io.Discard); err == nil || !strings.Contains(err.Error(), "graph flags") {
		t.Errorf("expected graph-flag usage error for hooks, got %v", err)
	}
}

// A cascade removal prints the plan, notes orphaned sidecars per removed
// node, and notes still-enabled agents nothing requires anymore (ADR-0081
// Decision 5); local-sidecar skills are outside the requirement scan.
func TestRunRemoveCascade(t *testing.T) {
	root := scaffoldedProject(t)
	if err := os.MkdirAll(filepath.Join(root, ".awf", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	// adr-lifecycle survives the cascade; its local sidecar exercises the
	// requirement scan's local skip (its rendered file already exists and
	// carries valid frontmatter from the scaffold sync).
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "adr-lifecycle.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// writing-plans is cascade-removed; its sidecar triggers the orphan note.
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "writing-plans.yaml"), []byte("data: {x: 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runDisable(root, "skill", "executing-plans", true, false, &out); err != nil {
		t.Fatalf("cascade remove: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"plan: - skill executing-plans\n",
		"plan: - agent plan-reviewer (required by reviewing-plan-resync)",
		`note: skill "writing-plans" still has a sidecar`,
		`note: agent "adr-reviewer" is no longer required by any enabled skill`,
		`note: agent "code-reviewer" is no longer required by any enabled skill`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("cascade output missing %q:\n%s", want, got)
		}
	}
	cfg := readConfig(t, root)
	for _, gone := range []string{"- executing-plans", "- reviewing-impl", "- brainstorming", "- plan-reviewer"} {
		if strings.Contains(cfg, gone) {
			t.Errorf("cascade should have removed %q:\n%s", gone, cfg)
		}
	}
	for _, kept := range []string{"- retrospective", "- adr-lifecycle", "- adr-reviewer", "- code-reviewer"} {
		if !strings.Contains(cfg, kept) {
			t.Errorf("cascade should have kept %q:\n%s", kept, cfg)
		}
	}
}

// Removing a domain with authored parts prints the orphan note (the domain
// path is outside the graph plan loop).
func TestRunRemoveDomainNotesOrphan(t *testing.T) {
	root := scaffoldedProject(t)
	if err := runEnable(root, "domain", "payments", false, io.Discard); err != nil {
		t.Fatalf("add domain: %v", err)
	}
	var out bytes.Buffer
	if err := runDisable(root, "domain", "payments", false, false, &out); err != nil {
		t.Fatalf("remove domain: %v", err)
	}
	if !strings.Contains(out.String(), `note: domain "payments" still has a sidecar`) {
		t.Errorf("expected the domain orphan note, got %q", out.String())
	}
}

// Removing a doc refuses while a doc-gated skill requires it (ADR-0081
// Decision 5 covers docs).
func TestRunRemoveDocRefusesWithDependentSkill(t *testing.T) {
	root := scaffoldedProject(t)
	if err := runEnable(root, "skill", "roadmap-graduation", false, io.Discard); err != nil {
		t.Fatalf("add roadmap-graduation: %v", err)
	}
	err := runDisable(root, "doc", "roadmap", false, false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "re-run with --with-dependents") {
		t.Fatalf("expected dependent refusal removing the doc, got %v", err)
	}
	if err := runDisable(root, "doc", "roadmap", true, false, io.Discard); err != nil {
		t.Fatalf("remove doc --with-dependents: %v", err)
	}
	cfg := readConfig(t, root)
	if strings.Contains(cfg, "- roadmap-graduation") || strings.Contains(cfg, "- roadmap") {
		t.Errorf("cascade should remove the doc and its dependent skill:\n%s", cfg)
	}
}

func TestRunRemove(t *testing.T) {
	root := scaffoldedProject(t)

	// Remove a core skill.
	if err := runDisable(root, "skill", "brainstorming", false, false, io.Discard); err != nil {
		t.Fatalf("remove brainstorming: %v", err)
	}
	if strings.Contains(readConfig(t, root), "- brainstorming") {
		t.Error("brainstorming not removed")
	}
	// Rejections.
	if err := runDisable(root, "bogus", "x", false, false, io.Discard); err == nil {
		t.Error("expected unknown-kind error")
	}
	if err := runDisable(root, "skill", "brainstorming", false, false, io.Discard); err == nil {
		t.Error("expected not-enabled error")
	}
}

func TestRunRemoveNotesOrphan(t *testing.T) {
	root := scaffoldedProject(t)
	// Give an enabled skill a sidecar, then remove it. brainstorming is the
	// chain's pure source (ADR-0081): nothing requires it, so its removal
	// leaves the enabled set closed.
	if err := os.MkdirAll(filepath.Join(root, ".awf", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "brainstorming.yaml"), []byte("data: {x: 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runDisable(root, "skill", "brainstorming", false, false, &out); err != nil {
		t.Fatalf("remove brainstorming: %v", err)
	}
	if !strings.Contains(out.String(), "orphaned") {
		t.Errorf("expected orphan note, got %q", out.String())
	}
}

func TestRunListStatesAndKinds(t *testing.T) {
	root := scaffoldedProject(t)
	// Craft a local and a tuned sidecar on two enabled skills.
	if err := os.MkdirAll(filepath.Join(root, ".awf", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "adr-lifecycle.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "proposing-adr.yaml"), []byte("data: {x: 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A configured domain exercises the freeform list path.
	if err := runEnable(root, "domain", "payments", false, io.Discard); err != nil {
		t.Fatalf("add domain: %v", err)
	}

	var all bytes.Buffer
	if err := runList(root, "", &all); err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{"skills:", "agents:", "docs:", "domains:", "available", "enabled", "local", "tuned", "payments", "configured"} {
		if !strings.Contains(all.String(), want) {
			t.Errorf("list output missing %q:\n%s", want, all.String())
		}
	}

	// Single-kind filter.
	var one bytes.Buffer
	if err := runList(root, "doc", &one); err != nil {
		t.Fatalf("list doc: %v", err)
	}
	if strings.Contains(one.String(), "skills:") {
		t.Errorf("list doc should not show skills:\n%s", one.String())
	}
	if err := runList(root, "bogus", io.Discard); err == nil {
		t.Error("expected unknown-kind error from list")
	}
}

// `awf disable agent` refuses upfront - before any config rewrite - while an
// enabled, non-local skill requires the agent (ADR-0050).
// invariant: tooling/cli:remove-agent-pairing-guard
func TestRunRemoveAgentPairingGuard(t *testing.T) {
	root := scaffoldedProject(t) // 10 core skills incl. the reviewing four; all 3 agents
	before := readConfig(t, root)
	var out bytes.Buffer
	err := runDisable(root, "agent", "code-reviewer", false, false, &out)
	if err == nil || !strings.Contains(err.Error(), "re-run with --with-dependents") {
		t.Fatalf("expected dependent-plan refusal, got %v", err)
	}
	if !strings.Contains(out.String(), "plan: - agent code-reviewer\n") ||
		!strings.Contains(out.String(), "plan: - skill reviewing-impl (required by code-reviewer)") {
		t.Errorf("expected the dependent plan printed before the refusal, got:\n%s", out.String())
	}
	if got := readConfig(t, root); got != before {
		t.Errorf("config must be untouched on refusal:\n%s", got)
	}

	// A local sidecar takes the requiring skill out of the pairing's scope,
	// mirroring the validator exactly.
	if err := os.MkdirAll(filepath.Join(root, ".awf", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "reviewing-adr.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runDisable(root, "agent", "adr-reviewer", false, false, io.Discard); err != nil {
		t.Fatalf("remove agent with only a local requirer: %v", err)
	}
	// The sync inside that removal pruned the formerly-managed rendered file
	// (no longer produced once the skill is local); restore it so later syncs
	// pass the local-frontmatter contract (inv: local-frontmatter, ADR-0037).
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "skills", "example-reviewing-adr", "SKILL.md"),
		"---\nname: example-reviewing-adr\ndescription: local reviewing skill\n---\nbody\n")

	// Local-declaring the requiring skill unblocks the removal too - removing
	// the skill outright is impossible one-at-a-time inside the chain's
	// mutually-requiring core (ADR-0081; Phase 2's cascade flag covers it).
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "reviewing-impl.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "skills", "example-reviewing-impl", "SKILL.md"),
		"---\nname: example-reviewing-impl\ndescription: local reviewing skill\n---\nbody\n")
	if err := runDisable(root, "agent", "code-reviewer", false, false, io.Discard); err != nil {
		t.Fatalf("remove agent after local-declaring its skill: %v", err)
	}
}

func TestDispatchAddRemoveList(t *testing.T) {
	root := scaffoldedProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })

	// add with kind.
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "enable", "skill", "tdd"}, &out, &errb); code != 0 {
		t.Fatalf("add dispatch: %s", errb.String())
	}
	// add with a single non-kind arg → "requires a kind" (pos[0] is the name).
	errb.Reset()
	if code := run([]string{"awf", "enable", "tdd"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "requires a kind") {
		t.Fatalf("expected migration hint, code=%d err=%q", code, errb.String())
	}
	// add with a lone kind token → "requires a name", not "requires a kind".
	errb.Reset()
	if code := run([]string{"awf", "enable", "skill"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "requires a name") {
		t.Fatalf("expected enable-kind name hint, code=%d err=%q", code, errb.String())
	}
	// add a singleton with an extra name → rejected, not silently dropped.
	errb.Reset()
	if code := run([]string{"awf", "enable", "bootstrap", "extra"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "takes no name") {
		t.Fatalf("expected singleton-name rejection, code=%d err=%q", code, errb.String())
	}
	// lone `target` (the non-descriptor kind) → "requires a name" too.
	errb.Reset()
	if code := run([]string{"awf", "enable", "target"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "requires a name") {
		t.Fatalf("expected enable-target name hint, code=%d err=%q", code, errb.String())
	}
	// add with no args → usage.
	errb.Reset()
	if code := run([]string{"awf", "enable"}, &out, &errb); code != 2 {
		t.Fatalf("expected usage error, code=%d", code)
	}
	// remove with kind.
	errb.Reset()
	if code := run([]string{"awf", "disable", "skill", "tdd"}, &out, &errb); code != 0 {
		t.Fatalf("remove dispatch: %s", errb.String())
	}
	// remove with a lone kind token → "requires a name" (mirrors enable).
	errb.Reset()
	if code := run([]string{"awf", "disable", "skill"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "requires a name") {
		t.Fatalf("expected remove name hint, code=%d err=%q", code, errb.String())
	}
	// remove a non-kind single token → generic disable usage.
	errb.Reset()
	if code := run([]string{"awf", "disable", "tdd"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "usage: awf disable") {
		t.Fatalf("expected disable usage, code=%d err=%q", code, errb.String())
	}
	// remove with extra positionals → usage (Phase 3: not silently ignored).
	errb.Reset()
	if code := run([]string{"awf", "disable", "skill", "tdd", "extra"}, &out, &errb); code != 2 {
		t.Fatalf("expected remove extra-positional usage error, code=%d", code)
	}
	// list with kind.
	errb.Reset()
	if code := run([]string{"awf", "list", "skill"}, &out, &errb); code != 0 {
		t.Fatalf("list dispatch: %s", errb.String())
	}
}

// Bare `awf list` covers every kind - the four catalog/domain kinds plus
// target, bootstrap, and hooks - and an empty kind prints (none) under its
// header. A single-kind filter still prints only that kind.
func TestRunListBareShowsAllKinds(t *testing.T) {
	root := scaffoldedProject(t)
	var out bytes.Buffer
	if err := runList(root, "", &out); err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{
		"skills:", "agents:", "docs:", "domains:\n  (none)",
		"targets:", "bootstrap:", ".awf/bootstrap.sh", ".awf/upgrade.sh",
		"hooks:", ".awf/hooks/pre-commit.sh",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("bare list missing %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if err := runList(root, "skill", &out); err != nil {
		t.Fatalf("list skill: %v", err)
	}
	if strings.Contains(out.String(), "targets:") || strings.Contains(out.String(), "hooks:") {
		t.Errorf("filtered list must not append the singleton kinds:\n%s", out.String())
	}
}

// noteUnrequiredAgents edge cases unreachable through the CLI with the
// shipped catalog (the reviewing-plan/resync pair is mutually requiring, so a
// real cascade never splits an agent's requirers): an agent still required by
// a remaining non-local skill gets no note, and a local-sidecar agent is
// outside the scan, mirroring the resolver's skip.
func TestNoteUnrequiredAgentsEdgeCases(t *testing.T) {
	root := scaffoldedProject(t)
	if err := os.MkdirAll(filepath.Join(root, ".awf", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "agents", "adr-reviewer.yaml"), []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := project.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	plan := []project.PlanOp{
		{Node: catalog.Node{Kind: "skill", Name: "reviewing-adr"}, Enable: false},
		{Node: catalog.Node{Kind: "skill", Name: "reviewing-plan"}, Enable: false, RequiredBy: "reviewing-adr"},
	}
	var out bytes.Buffer
	noteUnrequiredAgents(p, plan, &out)
	// adr-reviewer: required by the removed reviewing-adr, but local - skipped.
	// plan-reviewer: required by the removed reviewing-plan, but the remaining
	// reviewing-plan-resync still requires it - no note.
	if out.String() != "" {
		t.Errorf("expected no notes, got %q", out.String())
	}
}
