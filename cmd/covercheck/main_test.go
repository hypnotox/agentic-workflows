package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/coverage"

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
	prof := modWith(t, "example.com/m/f.go:2.1,2.5 1 1\nexample.com/m/f.go:3.1,3.5 1 0\n")
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", prof}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 below 100%%, got %d", code)
	}
	if got, want := errb.String(), "covercheck: coverage below 100% (1 uncovered statement(s))\n"; got != want {
		t.Errorf("diagnostic = %q, want %q", got, want)
	}
}

func TestRunReportsRawAndFilteredPercentages(t *testing.T) {
	root := t.TempDir()
	src := "package m\nvar x = 1 //" + " coverage-ignore: defensive\nvar y = 2\n"
	testsupport.WriteGoModule(t, root, "example.com/m", src)
	prof := testsupport.WriteProfile(t, root,
		"example.com/m/f.go:2.1,2.10 1 0\nexample.com/m/f.go:3.1,3.10 1 1\n")
	t.Chdir(root)
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", prof}, &out, &errb); code != 0 {
		t.Fatalf("expected old filtered gate to pass, got %d (%s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "raw coverage: 50.0% (1/2 statements)") ||
		!strings.Contains(out.String(), "filtered coverage: 100.0% (1/1 statements)") {
		t.Fatalf("missing reports: %q", out.String())
	}
}

func TestRunGeneratesAndEvaluatesCanonicalPolicy(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteGoModule(t, root, "example.com/m", "package m\nfunc F() {}\nfunc G() {}\n")
	prof := testsupport.WriteProfile(t, root,
		"example.com/m/f.go:2.1,2.5 1 0\nexample.com/m/f.go:3.1,3.5 1 1\n")
	reviewPath := filepath.Join(root, "review.json")
	review := coverage.Review{Misses: []coverage.MissAdmission{{
		Identity: coverage.Identity{File: "f.go", Start: coverage.Position{Line: 2, Column: 1}, End: coverage.Position{Line: 2, Column: 5}, Statements: 1},
		Reason:   "reviewed behavior gap",
	}}}
	rawReview, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reviewPath, rawReview, 0o644); err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(root, "coverage-baseline.json")
	t.Chdir(root)
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--generate-policy", prof, baselinePath, reviewPath}, &out, &errb); code != 0 {
		t.Fatalf("generation exit %d: %s", code, errb.String())
	}
	if _, err := coverage.LoadBaseline(baselinePath); err != nil {
		t.Fatalf("generated baseline: %v", err)
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"covercheck", "--policy", prof, baselinePath}, &out, &errb); code != 0 {
		t.Fatalf("policy exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "raw coverage: 50.0%") || !strings.Contains(out.String(), "filtered coverage: 50.0%") {
		t.Fatalf("policy reports = %q", out.String())
	}

	swapped := testsupport.WriteProfile(t, root,
		"example.com/m/f.go:2.1,2.5 1 1\nexample.com/m/f.go:3.1,3.5 1 0\n")
	out.Reset()
	errb.Reset()
	if code := run([]string{"covercheck", "--policy", swapped, baselinePath}, &out, &errb); code != 1 {
		t.Fatalf("identity swap exit = %d", code)
	}
	if !strings.Contains(errb.String(), "raw-identity-added") {
		t.Fatalf("identity swap diagnostic = %q", errb.String())
	}
}

func TestRunPolicyRejectsUnavailableEvidence(t *testing.T) {
	prof := modWith(t, "example.com/m/f.go:2.1,2.5 1 0\n")
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--policy", prof, filepath.Join(t.TempDir(), "missing.json")}, &out, &errb); code != 1 {
		t.Fatalf("missing baseline exit = %d", code)
	}
	if !strings.Contains(errb.String(), "read baseline") {
		t.Fatalf("missing baseline diagnostic = %q", errb.String())
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

func TestRunMergeProfiles(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.out")
	second := filepath.Join(root, "second.out")
	testsupport.WriteFile(t, first, "mode: set\nexample.com/m/f.go:2.1,2.5 1 0\n")
	testsupport.WriteFile(t, second, "mode: set\nexample.com/m/f.go:2.1,2.5 1 1\n")
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--merge", first, second}, &out, &errb); code != 0 {
		t.Fatalf("merge exit = %d: %s", code, errb.String())
	}
	if got, want := out.String(), "mode: set\nexample.com/m/f.go:2.1,2.5 1 1\n"; got != want {
		t.Fatalf("merged output = %q, want %q", got, want)
	}
}

func TestRunMergeProfilesRejectsUsageAndInvalidInput(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--merge", "only.out"}, &out, &errb); code != 2 {
		t.Fatalf("merge usage exit = %d, want 2", code)
	}
	if got, want := errb.String(), "usage: covercheck --merge <coverprofile> <coverprofile> [...]\n"; got != want {
		t.Fatalf("merge usage = %q, want %q", got, want)
	}

	root := t.TempDir()
	first := filepath.Join(root, "first.out")
	second := filepath.Join(root, "second.out")
	testsupport.WriteFile(t, first, "mode: set\nexample.com/m/f.go:2.1,2.5 1 0\n")
	testsupport.WriteFile(t, second, "mode: count\nexample.com/m/f.go:2.1,2.5 1 0\n")
	out.Reset()
	errb.Reset()
	if code := run([]string{"covercheck", "--merge", first, second}, &out, &errb); code != 1 {
		t.Fatalf("invalid merge exit = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "coverage: mixed profile modes") {
		t.Fatalf("invalid merge diagnostic = %q", errb.String())
	}
}

func TestRunEmitFilteredError(t *testing.T) {
	prof := modWith(t, "example.com/m/ghost.go:2.1,2.5 1 0\n") // source file missing
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--emit-filtered", prof}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on filter error, got %d", code)
	}
}
