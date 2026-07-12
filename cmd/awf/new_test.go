package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestRunNewScaffoldsADR(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runNew(root, "adr", []string{"My", "New", "Title"}, &out); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	want := filepath.Join(root, "docs", "decisions", "0001-my-new-title.md")
	got := strings.TrimSpace(out.String())
	if got != want {
		t.Errorf("runNew printed %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("created file not found: %v", err)
	}
}

func TestRunNewADRError(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "adr", []string{"!!!"}, os.Stdout); err == nil {
		t.Fatal("expected NewADR error for an all-punctuation title")
	}
}

func TestRunNewUnknownKind(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "widget", []string{"x"}, os.Stdout); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestRunNewScaffoldsPlan(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runNew(root, "plan", []string{"Some", "Plan", "Title"}, &out); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	got := strings.TrimSpace(out.String())
	// Date-prefixed under docs/plans (no sequential number); the date is today's,
	// so match on shape rather than couple the test to the wall clock.
	if dir := filepath.Dir(got); dir != filepath.Join(root, "docs", "plans") {
		t.Errorf("plan written to %q, want under docs/plans", got)
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-some-plan-title\.md$`).MatchString(filepath.Base(got)) {
		t.Errorf("plan filename %q not YYYY-MM-DD-some-plan-title.md", filepath.Base(got))
	}
	body, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("created file not found: %v", err)
	}
	if !strings.Contains(string(body), "# Plan: Some Plan Title") || !strings.Contains(string(body), "status: Proposed") {
		t.Errorf("plan not scaffolded from template:\n%s", body)
	}
}

func TestRunNewPlanMissingTitle(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "plan", nil, os.Stdout); err == nil {
		t.Fatal("expected usage error for a missing plan title")
	}
}

func TestRunNewPlanRefusesExisting(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "plan", []string{"Same", "Plan"}, io.Discard); err != nil {
		t.Fatalf("first runNew: %v", err)
	}
	if err := runNew(root, "plan", []string{"Same", "Plan"}, io.Discard); err == nil {
		t.Fatal("expected overwrite refusal for a same-day same-title plan")
	}
}

func TestRunNewPlanOpenError(t *testing.T) {
	root := scaffoldProject(t)
	// Passes the schema/version gate but fails project.Open (a ghost enabled doc),
	// covering newPlan's Open-error return.
	testsupport.WriteAwfConfig(t, root, minimalYAML+"docs: [ghost-doc]\n")
	if err := runNew(root, "plan", []string{"Some", "Plan"}, io.Discard); err == nil {
		t.Fatal("expected project.Open error")
	}
}

func TestRunNewScaffoldsDoc(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"guides/ci", "How", "CI", "runs"}, io.Discard); err != nil {
		t.Fatalf("new doc: %v", err)
	}
	sc := filepath.Join(root, ".awf", "docs", "guides", "ci.yaml")
	if b, err := os.ReadFile(sc); err != nil {
		t.Fatalf("sidecar not written: %v", err)
	} else if !strings.Contains(string(b), "title: Ci") || !strings.Contains(string(b), "description: How CI runs") {
		t.Errorf("sidecar content wrong:\n%s", b)
	}
	part := filepath.Join(root, ".awf", "docs", "parts", "guides", "ci", "content.md")
	if b, err := os.ReadFile(part); err != nil {
		t.Fatalf("part not written: %v", err)
	} else if !strings.Contains(string(b), "awf:stub") {
		t.Errorf("part missing stub marker:\n%s", b)
	}
	out := filepath.Join(root, "docs", "guides", "ci.md")
	if _, err := os.Stat(out); err != nil {
		t.Errorf("rendered doc missing: %v", err)
	}
}

func TestRunNewDocMissingDescription(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"ci"}, io.Discard); err == nil {
		t.Fatal("expected usage error for missing description")
	}
}

func TestRunNewDocEmptyDescription(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"ci", "   "}, io.Discard); err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestRunNewDocInvalidName(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"Bad", "desc"}, io.Discard); err == nil {
		t.Fatal("expected error for invalid doc name")
	}
}

func TestRunNewDocRefusesExisting(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"ci", "desc"}, io.Discard); err != nil {
		t.Fatalf("first new doc: %v", err)
	}
	// Disable ci in config but leave its sidecar+part on disk, so the second run's
	// catalog-collision check misses and the authored-files guard fires (mirrors
	// TestRunNewRefusesExistingLocalArtifactFiles).
	cfgPath := config.ConfigPath(root)
	src, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := config.SetArrayMember(src, "docs", "ci", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runNew(root, "doc", []string{"ci", "desc"}, io.Discard); err == nil {
		t.Fatal("expected refusal for existing doc files")
	}
}

func TestRunNewDocCollidesWithCatalog(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "doc", []string{"architecture", "desc"}, io.Discard); err == nil {
		t.Fatal("expected collision error for a catalog doc name")
	}
}

func TestRunNewDispatch(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "new", "adr", "Some", "Title"}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
}

func TestRunNewMissingArgs(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "new", "adr"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for missing title, got %d", code)
	}
}

// An unrecognized `new` subcommand is not a clispec child, so resolve leaves it
// in the positionals; the new handler reunites it as the kind and runNew reports
// the unknown-kind usage error (exit 2).
func TestRunNewUnknownKindDispatch(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "new", "widget", "x"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for unknown kind, got %d (%s)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "unknown kind") {
		t.Errorf("missing unknown-kind message: %q", errb.String())
	}
}

func TestRunNewScaffoldsSkill(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"deploy-check", "Verify the deploy is green."}, io.Discard); err != nil {
		t.Fatalf("runNew skill: %v", err)
	}
	sc, err := os.ReadFile(filepath.Join(root, ".awf", "skills", "deploy-check.yaml"))
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	if !strings.Contains(string(sc), "Verify the deploy is green.") {
		t.Errorf("sidecar missing description:\n%s", sc)
	}
	part, err := os.ReadFile(filepath.Join(root, ".awf", "skills", "parts", "deploy-check", "content.md"))
	if err != nil {
		t.Errorf("content part not written: %v", err)
	}
	if !strings.HasPrefix(string(part), "<!-- awf:stub -->\n") {
		t.Errorf("starter part must open with the awf:stub marker (ADR-0070):\n%s", part)
	}
	cfg, _ := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if !strings.Contains(string(cfg), "deploy-check") {
		t.Errorf("skill not enabled in config:\n%s", cfg)
	}
	rendered, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "example-deploy-check", "SKILL.md"))
	if err != nil {
		t.Errorf("rendered skill missing: %v", err)
	}
	if !strings.Contains(string(rendered), "<!-- awf:stub -->") {
		t.Errorf("stub-marked part must render verbatim, marker included:\n%s", rendered)
	}
	if err := runCheck(root, io.Discard); err != nil {
		t.Errorf("post-scaffold check not clean: %v", err)
	}
}

// awf new must refuse when the name already has files under .awf/, even when
// the name is not in the enable array (an enabled+declared local is caught by
// the catalog-pool guard; a disabled one left its sidecar and authored part on
// disk, and a re-run must not silently reset them to the stub).
func TestRunNewRefusesExistingLocalArtifactFiles(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"deploy-check", "Verify the deploy is green."}, io.Discard); err != nil {
		t.Fatalf("runNew skill: %v", err)
	}
	partPath := filepath.Join(root, ".awf", "skills", "parts", "deploy-check", "content.md")
	const authored = "Authored body — must survive a re-run.\n"
	if err := os.WriteFile(partPath, []byte(authored), 0o644); err != nil {
		t.Fatal(err)
	}
	// Disable the skill but keep its authored files, as `awf disable skill` would.
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(strings.ReplaceAll(string(cfg), "  - deploy-check\n", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runNew(root, "skill", []string{"deploy-check", "Other description."}, io.Discard); err == nil {
		t.Fatal("expected error re-running awf new over existing local artifact files")
	}
	part, err := os.ReadFile(partPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(part) != authored {
		t.Errorf("authored content part was overwritten:\n%s", part)
	}
	sc, err := os.ReadFile(filepath.Join(root, ".awf", "skills", "deploy-check.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sc), "Verify the deploy is green.") {
		t.Errorf("authored sidecar was overwritten:\n%s", sc)
	}
}

func TestRunNewScaffoldsAgent(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "agent", []string{"deploy-bot", "Runs the deploy checks."}, io.Discard); err != nil {
		t.Fatalf("runNew agent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "agents", "deploy-bot.yaml")); err != nil {
		t.Errorf("agent sidecar not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "agents", "deploy-bot.md")); err != nil {
		t.Errorf("rendered agent missing: %v", err)
	}
}

func TestRunNewSkillMissingDescription(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"lonely"}, io.Discard); err == nil {
		t.Fatal("expected usage error when description is missing")
	}
}

func TestRunNewSkillEmptyDescription(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"x", "   "}, io.Discard); err == nil {
		t.Fatal("expected error for a whitespace-only description")
	}
}

func TestRunNewSkillReservedName(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"_base", "desc"}, io.Discard); err == nil {
		t.Fatal("expected reserved-name rejection")
	}
}

func TestRunNewSkillCollision(t *testing.T) {
	root := scaffoldProject(t)
	if err := runNew(root, "skill", []string{"tdd", "desc"}, io.Discard); err == nil {
		t.Fatal("expected collision with the catalog skill tdd")
	}
}

func TestRunNewSkillOpenError(t *testing.T) {
	root := scaffoldProject(t)
	// Passes the schema/version gate (lock intact) but fails project.Open — an
	// enabled doc that is not in the catalog.
	testsupport.WriteAwfConfig(t, root, minimalYAML+"docs: [ghost-doc]\n")
	if err := runNew(root, "skill", []string{"newone", "a description"}, io.Discard); err == nil {
		t.Fatal("expected project.Open error")
	}
}

func TestRunNewDocOpenError(t *testing.T) {
	root := scaffoldProject(t)
	// Passes the schema/version gate but fails project.Open (a ghost enabled doc),
	// covering newLocalDoc's Open-error return.
	testsupport.WriteAwfConfig(t, root, minimalYAML+"docs: [ghost-doc]\n")
	if err := runNew(root, "doc", []string{"newdoc", "a description"}, io.Discard); err == nil {
		t.Fatal("expected project.Open error")
	}
}

// seedScaffoldVars: an absent referenced var is seeded empty, a present one is
// untouched, and a malformed source surfaces the editor's error.
// invariant: new-seeds-scaffold-vars
func TestSeedScaffoldVars(t *testing.T) {
	src := []byte("prefix: x\nvars:\n  kept: value\n")
	got, err := seedScaffoldVars(src, []string{"kept", "added"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kept: value", "added: \"\""} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if _, err := seedScaffoldVars([]byte(":\n:"), []string{"x"}); err == nil {
		t.Fatal("expected error on malformed config source")
	}
}

// The shipped base templates reference no vars, so awf new seeds nothing today
// — this pins the no-op so a future var-bearing base template consciously
// changes it (ADR-0087 Decision 4).
func TestRunNewSeedsNoVarsToday(t *testing.T) {
	for _, kind := range []string{"skill", "agent"} {
		refs, err := project.ScaffoldVarRefs(kind)
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != 0 {
			t.Errorf("base %s template gained var refs %v — confirm awf new seeding and update this pin", kind, refs)
		}
	}
}
