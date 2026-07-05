package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// modWith builds a temp module (go.mod + f.go) and a coverprofile, then chdir's
// into it so coverage.CheckProfile resolves this module. Returns the profile path.
func modWith(t *testing.T, profileBody string) string {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteGoModule(t, root, "example.com/m", "package m\nfunc F() {}\n")
	prof := testsupport.WriteProfile(t, root, profileBody)
	t.Chdir(root)
	return prof
}

func TestRunUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for missing arg, got %d", code)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("missing usage text: %q", errb.String())
	}
}

func TestRunHundredPercent(t *testing.T) {
	prof := modWith(t, "example.com/m/f.go:2.1,2.5 1 1\n")
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", prof}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "100.0%") {
		t.Errorf("expected 100%% report, got %q", out.String())
	}
}

func TestRunBelowHundred(t *testing.T) {
	prof := modWith(t, "example.com/m/f.go:2.1,2.5 1 0\n")
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", prof}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 below 100%%, got %d", code)
	}
	if !strings.Contains(errb.String(), "below 100%") {
		t.Errorf("expected below-100 message, got %q", errb.String())
	}
}

func TestRunCheckError(t *testing.T) {
	prof := modWith(t, "example.com/m/ghost.go:2.1,2.5 1 0\n") // source file missing
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", prof}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on Check error, got %d", code)
	}
}

func TestRunEmitFiltered(t *testing.T) {
	// --emit-filtered writes the covered profile (ignored blocks dropped) to stdout.
	root := t.TempDir()
	src := "package m\nvar x = 1 //" + " coverage-ignore: defensive\nvar y = 2\n"
	testsupport.WriteGoModule(t, root, "example.com/m", src)
	prof := testsupport.WriteProfile(t, root,
		"example.com/m/f.go:2.1,2.10 1 0\nexample.com/m/f.go:3.1,3.10 1 1\n")
	t.Chdir(root)
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--emit-filtered", prof}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
	want := "mode: set\nexample.com/m/f.go:3.1,3.10 1 1\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestRunEmitFilteredMissingArg(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--emit-filtered"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for missing profile arg, got %d", code)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("missing usage text: %q", errb.String())
	}
}

func TestRunEmitFilteredError(t *testing.T) {
	prof := modWith(t, "example.com/m/ghost.go:2.1,2.5 1 0\n") // source file missing
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--emit-filtered", prof}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on filter error, got %d", code)
	}
}
