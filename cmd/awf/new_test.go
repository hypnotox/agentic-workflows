package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if err := runNew(root, "doc", []string{"x"}, os.Stdout); err == nil {
		t.Fatal("expected error for unknown kind")
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
	if _, err := os.Stat(filepath.Join(root, ".awf", "skills", "parts", "deploy-check", "content.md")); err != nil {
		t.Errorf("content part not written: %v", err)
	}
	cfg, _ := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if !strings.Contains(string(cfg), "deploy-check") {
		t.Errorf("skill not enabled in config:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "example-deploy-check", "SKILL.md")); err != nil {
		t.Errorf("rendered skill missing: %v", err)
	}
	if err := runCheck(root, io.Discard); err != nil {
		t.Errorf("post-scaffold check not clean: %v", err)
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
