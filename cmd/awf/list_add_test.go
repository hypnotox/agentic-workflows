package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// scaffoldedProject writes a curated-default scaffold (10 core skills, 3 agents,
// 0 docs — no doc is core after ADR-0043 — no domains) and syncs it.
func scaffoldedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	b, err := project.ScaffoldConfig("example", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteAwfConfig(t, root, string(b))
	if err := runSync(root, io.Discard); err != nil {
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
	if err := runAdd(root, "skill", "tdd", io.Discard); err != nil {
		t.Fatalf("add skill tdd: %v", err)
	}
	if !strings.Contains(readConfig(t, root), "- tdd") {
		t.Error("tdd not added")
	}
	// Opt-in doc.
	if err := runAdd(root, "doc", "pitfalls", io.Discard); err != nil {
		t.Fatalf("add doc pitfalls: %v", err)
	}
	// Freeform domain into an absent array (scaffold omits domains:).
	if err := runAdd(root, "domain", "payments", io.Discard); err != nil {
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
	if err := runAdd(root, "bogus", "x", io.Discard); err == nil {
		t.Error("expected unknown-kind error")
	}
	if err := runAdd(root, "skill", "no-such", io.Discard); err == nil {
		t.Error("expected not-in-catalog error")
	}
	if err := runAdd(root, "domain", "bad/name", io.Discard); err == nil {
		t.Error("expected invalid-domain-name error")
	}
	if err := runAdd(root, "skill", "tdd", io.Discard); err == nil {
		t.Error("expected already-enabled error")
	}
}

// A project-local artifact (ADR-0068) lists under its declared kind with the
// state "local" — not "tuned", which would hide that it is not a catalog skill.
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

// The version gate must refuse add/remove BEFORE the config rewrite: a stale
// binary must not leave a modified config.yaml with nothing rendered (the
// half-mutated state the chained sync's gate can only catch after the write).
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
	if err := runAdd(root, "skill", "tdd", io.Discard); err == nil {
		t.Error("expected gate error from runAdd on a behind binary")
	}
	if got := readConfig(t, root); got != before {
		t.Errorf("runAdd modified config.yaml despite failing the gate:\n%s", got)
	}
	if err := runRemove(root, "skill", "brainstorming", io.Discard); err == nil {
		t.Error("expected gate error from runRemove on a behind binary")
	}
	if got := readConfig(t, root); got != before {
		t.Errorf("runRemove modified config.yaml despite failing the gate:\n%s", got)
	}
}

// TestRunAddDomainScaffoldIdempotent confirms add domain never clobbers an
// existing current-state.md — pre-authored content (e.g. from a prior
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

	if err := runAdd(root, "domain", "billing", io.Discard); err != nil {
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
// invariant: target-cli
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
	if err := runAdd(root, "target", "cursor", io.Discard); err != nil {
		t.Fatalf("add target cursor: %v", err)
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "- claude") || !strings.Contains(cfg, "- cursor") {
		t.Errorf("expected targets [claude, cursor]:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(root, ".cursor", "skills")); err != nil {
		t.Errorf("cursor tree not rendered after add: %v", err)
	}

	// Rejections.
	if err := runAdd(root, "target", "nope", io.Discard); err == nil {
		t.Error("expected unknown-target error")
	}
	if err := runAdd(root, "target", "cursor", io.Discard); err == nil {
		t.Error("expected already-enabled error")
	}
	if err := runRemove(root, "target", "nope", io.Discard); err == nil {
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
	if err := runAdd(broken, "target", "cursor", io.Discard); err == nil {
		t.Error("expected project.Open error to surface from add target")
	}

	// remove claude → [cursor]; removing the last target is refused.
	if err := runRemove(root, "target", "claude", io.Discard); err != nil {
		t.Fatalf("remove target claude: %v", err)
	}
	if strings.Contains(readConfig(t, root), "- claude") {
		t.Error("claude not removed")
	}
	if err := runRemove(root, "target", "claude", io.Discard); err == nil {
		t.Error("expected not-enabled error")
	}
	if err := runRemove(root, "target", "cursor", io.Discard); err == nil {
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
		!strings.Contains(out, ".awf/bootstrap.sh") || !strings.Contains(out, "available") {
		t.Errorf("list bootstrap (initial):\n%s", out)
	}

	// remove when disabled errors.
	if err := runRemove(root, "bootstrap", "", io.Discard); err == nil ||
		!strings.Contains(err.Error(), "is not enabled") {
		t.Errorf("expected is-not-enabled error, got %v", err)
	}

	// add enables it (config gains enabled: true, sync runs).
	if err := runAdd(root, "bootstrap", "", io.Discard); err != nil {
		t.Fatalf("add bootstrap: %v", err)
	}
	cfg := readConfig(t, root)
	if !strings.Contains(cfg, "bootstrap:") || !strings.Contains(cfg, "enabled: true") {
		t.Errorf("bootstrap not enabled in config:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "bootstrap.sh")); err != nil {
		t.Errorf("bootstrap.sh not rendered after add: %v", err)
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
	if err := runAdd(root, "bootstrap", "", io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already enabled") {
		t.Errorf("expected already-enabled error, got %v", err)
	}

	// remove disables it and prunes the rendered file.
	if err := runRemove(root, "bootstrap", "", io.Discard); err != nil {
		t.Fatalf("remove bootstrap: %v", err)
	}
	if !strings.Contains(readConfig(t, root), "enabled: false") {
		t.Errorf("bootstrap not disabled in config:\n%s", readConfig(t, root))
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "bootstrap.sh")); !os.IsNotExist(err) {
		t.Errorf("bootstrap.sh not pruned after remove: err=%v", err)
	}

	// A broken config surfaces the project.Open error from addRemoveBootstrap.
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broken, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, ".awf", "config.yaml"), []byte("prefix: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runAdd(broken, "bootstrap", "", io.Discard); err == nil {
		t.Error("expected project.Open error to surface from add bootstrap")
	}
}

// TestDispatchBootstrap covers run()'s nameless-bootstrap dispatch branches
// (ADR-0040): `awf add bootstrap` / `awf remove bootstrap` carry no <name> arg, so
// they reach the handler via the bespoke len==3 cases rather than the len==4 path.
func TestDispatchBootstrap(t *testing.T) {
	root := scaffoldProject(t) // minimalYAML: no bootstrap key (disabled)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })

	// add bootstrap (3 args, no name) enables it.
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "add", "bootstrap"}, &out, &errb); code != 0 {
		t.Fatalf("add bootstrap dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "enabled: true") {
		t.Errorf("bootstrap not enabled after add dispatch:\n%s", cfg)
	}

	// remove bootstrap (3 args, no name) disables it.
	errb.Reset()
	if code := run([]string{"awf", "remove", "bootstrap"}, &out, &errb); code != 0 {
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
	if err := runRemove(root, "hooks", "", io.Discard); err == nil ||
		!strings.Contains(err.Error(), "is not enabled") {
		t.Errorf("expected is-not-enabled error, got %v", err)
	}

	// add enables it (config gains enabled: true, sync renders the payloads).
	if err := runAdd(root, "hooks", "", io.Discard); err != nil {
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
	if err := runAdd(root, "hooks", "", io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already enabled") {
		t.Errorf("expected already-enabled error, got %v", err)
	}

	// remove disables it and prunes the rendered files and their directory.
	if err := runRemove(root, "hooks", "", io.Discard); err != nil {
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
	if code := run([]string{"awf", "add", "hooks"}, &out, &errb); code != 0 {
		t.Fatalf("add hooks dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "hooks:") || !strings.Contains(cfg, "enabled: true") {
		t.Errorf("hooks not enabled after add dispatch:\n%s", cfg)
	}

	errb.Reset()
	if code := run([]string{"awf", "remove", "hooks"}, &out, &errb); code != 0 {
		t.Fatalf("remove hooks dispatch: code=%d err=%q", code, errb.String())
	}
	if cfg := readConfig(t, root); !strings.Contains(cfg, "enabled: false") {
		t.Errorf("hooks not disabled after remove dispatch:\n%s", cfg)
	}
}

func TestRunAddRemoveFlowStyle(t *testing.T) {
	root := scaffoldProject(t)
	if err := runAdd(root, "skill", "brainstorming", io.Discard); err != nil {
		t.Fatalf("add to flow-style array: %v", err)
	}
	cfg := readConfig(t, root)
	if !strings.Contains(cfg, "- brainstorming") || !strings.Contains(cfg, "- tdd") {
		t.Errorf("expected block-style skills with both members:\n%s", cfg)
	}
	if err := runRemove(root, "skill", "tdd", io.Discard); err != nil {
		t.Fatalf("remove from (now block) array: %v", err)
	}
	if strings.Contains(readConfig(t, root), "- tdd") {
		t.Error("tdd not removed")
	}
}

func TestRunAddDocGatedSkillWarns(t *testing.T) {
	root := scaffoldedProject(t) // roadmap doc is not enabled
	var out bytes.Buffer
	if err := runAdd(root, "skill", "roadmap-graduation", &out); err != nil {
		t.Fatalf("add roadmap-graduation: %v", err)
	}
	if !strings.Contains(out.String(), "requires the \"roadmap\" doc") {
		t.Errorf("expected doc-gate warning, got %q", out.String())
	}
}

func TestRunRemove(t *testing.T) {
	root := scaffoldedProject(t)

	// Remove a core skill.
	if err := runRemove(root, "skill", "brainstorming", io.Discard); err != nil {
		t.Fatalf("remove brainstorming: %v", err)
	}
	if strings.Contains(readConfig(t, root), "- brainstorming") {
		t.Error("brainstorming not removed")
	}
	// Rejections.
	if err := runRemove(root, "bogus", "x", io.Discard); err == nil {
		t.Error("expected unknown-kind error")
	}
	if err := runRemove(root, "skill", "brainstorming", io.Discard); err == nil {
		t.Error("expected not-enabled error")
	}
}

func TestRunRemoveNotesOrphan(t *testing.T) {
	root := scaffoldedProject(t)
	// Give an enabled skill a sidecar, then remove it.
	if err := os.MkdirAll(filepath.Join(root, ".awf", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "skills", "writing-plans.yaml"), []byte("data: {x: 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runRemove(root, "skill", "writing-plans", &out); err != nil {
		t.Fatalf("remove writing-plans: %v", err)
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
	if err := runAdd(root, "domain", "payments", io.Discard); err != nil {
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

// `awf remove agent` refuses upfront — before any config rewrite — while an
// enabled, non-local skill requires the agent (ADR-0050).
// invariant: remove-agent-pairing-guard
func TestRunRemoveAgentPairingGuard(t *testing.T) {
	root := scaffoldedProject(t) // 10 core skills incl. the reviewing four; all 3 agents
	before := readConfig(t, root)
	err := runRemove(root, "agent", "code-reviewer", io.Discard)
	if err == nil || !strings.Contains(err.Error(), `skill "reviewing-impl" requires agent "code-reviewer"`) {
		t.Fatalf("expected pairing refusal, got %v", err)
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
	if err := runRemove(root, "agent", "adr-reviewer", io.Discard); err != nil {
		t.Fatalf("remove agent with only a local requirer: %v", err)
	}
	// The sync inside that removal pruned the formerly-managed rendered file
	// (no longer produced once the skill is local); restore it so later syncs
	// pass the local-frontmatter contract (inv: local-frontmatter, ADR-0037).
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "skills", "example-reviewing-adr", "SKILL.md"),
		"---\nname: example-reviewing-adr\ndescription: local reviewing skill\n---\nbody\n")

	// Disabling the requiring skill unblocks the removal.
	if err := runRemove(root, "skill", "reviewing-impl", io.Discard); err != nil {
		t.Fatalf("remove skill reviewing-impl: %v", err)
	}
	if err := runRemove(root, "agent", "code-reviewer", io.Discard); err != nil {
		t.Fatalf("remove agent after disabling its skill: %v", err)
	}
}

// `awf add skill` enables the skill's required agent in the same config
// rewrite, announced by a note (ADR-0050).
// invariant: add-skill-pairs-agent
func TestRunAddSkillPairsAgent(t *testing.T) {
	root := scaffoldProject(t) // minimalYAML: skills [tdd], agents []
	var out bytes.Buffer
	if err := runAdd(root, "skill", "reviewing-impl", &out); err != nil {
		t.Fatalf("add skill reviewing-impl: %v", err)
	}
	if !strings.Contains(out.String(), `note: also enabled agent "code-reviewer" (required by skill "reviewing-impl")`) {
		t.Errorf("missing pairing note, got %q", out.String())
	}
	cfg := readConfig(t, root)
	if !strings.Contains(cfg, "- reviewing-impl") || !strings.Contains(cfg, "- code-reviewer") {
		t.Errorf("expected both skill and agent enabled:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "agents", "code-reviewer.md")); err != nil {
		t.Errorf("code-reviewer not rendered after paired add: %v", err)
	}

	// Second paired add enables the shared plan-reviewer once; a skill whose
	// agent is already enabled adds without a note.
	out.Reset()
	if err := runAdd(root, "skill", "reviewing-plan", &out); err != nil {
		t.Fatalf("add skill reviewing-plan: %v", err)
	}
	if !strings.Contains(out.String(), `note: also enabled agent "plan-reviewer"`) {
		t.Errorf("expected plan-reviewer note, got %q", out.String())
	}
	out.Reset()
	if err := runAdd(root, "skill", "reviewing-plan-resync", &out); err != nil {
		t.Fatalf("add skill reviewing-plan-resync: %v", err)
	}
	if strings.Contains(out.String(), "also enabled agent") {
		t.Errorf("no note expected when the agent is already enabled, got %q", out.String())
	}
}

func TestDispatchAddRemoveList(t *testing.T) {
	root := scaffoldedProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })

	// add with kind.
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "add", "skill", "tdd"}, &out, &errb); code != 0 {
		t.Fatalf("add dispatch: %s", errb.String())
	}
	// add with a single arg → targeted migration message.
	errb.Reset()
	if code := run([]string{"awf", "add", "tdd"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "requires a kind") {
		t.Fatalf("expected migration hint, code=%d err=%q", code, errb.String())
	}
	// add with no args → usage.
	errb.Reset()
	if code := run([]string{"awf", "add"}, &out, &errb); code != 2 {
		t.Fatalf("expected usage error, code=%d", code)
	}
	// remove with kind.
	errb.Reset()
	if code := run([]string{"awf", "remove", "skill", "tdd"}, &out, &errb); code != 0 {
		t.Fatalf("remove dispatch: %s", errb.String())
	}
	// remove missing args → usage.
	errb.Reset()
	if code := run([]string{"awf", "remove", "skill"}, &out, &errb); code != 2 {
		t.Fatalf("expected remove usage error, code=%d", code)
	}
	// remove with extra positionals → usage (Phase 3: not silently ignored).
	errb.Reset()
	if code := run([]string{"awf", "remove", "skill", "tdd", "extra"}, &out, &errb); code != 2 {
		t.Fatalf("expected remove extra-positional usage error, code=%d", code)
	}
	// list with kind.
	errb.Reset()
	if code := run([]string{"awf", "list", "skill"}, &out, &errb); code != 0 {
		t.Fatalf("list dispatch: %s", errb.String())
	}
}

// Bare `awf list` covers every kind — the four catalog/domain kinds plus
// target, bootstrap, and hooks — and an empty kind prints (none) under its
// header. A single-kind filter still prints only that kind.
func TestRunListBareShowsAllKinds(t *testing.T) {
	root := scaffoldedProject(t)
	var out bytes.Buffer
	if err := runList(root, "", &out); err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{
		"skills:", "agents:", "docs:", "domains:\n  (none)",
		"targets:", "bootstrap:", ".awf/bootstrap.sh", "hooks:", ".awf/hooks/pre-commit.sh",
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
