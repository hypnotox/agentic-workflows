package main

import (
	"bytes"
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
	if err := runNew(root, "skill", []string{"x"}, os.Stdout); err == nil {
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
