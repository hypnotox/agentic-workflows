package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// scaffoldedProject writes a curated-default scaffold (10 core skills, 3 agents,
// 3 docs, no domains) and syncs it.
func scaffoldedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := project.ScaffoldConfig("example", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
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
		!strings.Contains(out, "awf-bootstrap.sh") || !strings.Contains(out, "available") {
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
	if _, err := os.Stat(filepath.Join(root, "awf-bootstrap.sh")); err != nil {
		t.Errorf("awf-bootstrap.sh not rendered after add: %v", err)
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
	if _, err := os.Stat(filepath.Join(root, "awf-bootstrap.sh")); !os.IsNotExist(err) {
		t.Errorf("awf-bootstrap.sh not pruned after remove: err=%v", err)
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

func TestDispatchAddRemoveList(t *testing.T) {
	root := scaffoldedProject(t)
	swapGetwd(t, func() (string, error) { return root, nil })

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
