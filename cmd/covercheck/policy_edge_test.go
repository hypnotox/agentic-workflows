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

func writeReview(t *testing.T, root string, review coverage.Review) string {
	t.Helper()
	raw, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "review.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func reviewedAnalysis(t *testing.T, root, profile string) coverage.Review {
	t.Helper()
	t.Chdir(root)
	analysis, err := coverage.AnalyzeProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	review := coverage.Review{}
	for _, miss := range analysis.RawMisses {
		review.Misses = append(review.Misses, coverage.MissAdmission{Identity: miss, Reason: "reviewed behavior gap"})
	}
	for _, directive := range analysis.ProductionDirectives {
		review.Directives = append(review.Directives, coverage.DirectiveAdmission{
			Directive: directive,
			Class:     coverage.IgnoreImpossibleState,
			Evidence:  "direct behavior test",
		})
	}
	return review
}

func TestRunPolicyModeUsage(t *testing.T) {
	for name, args := range map[string][]string{
		"filtered": {"covercheck", "--emit-filtered", "one", "extra"},
		"policy":   {"covercheck", "--policy", "one"},
		"generate": {"covercheck", "--generate-policy", "one", "two"},
	} {
		t.Run(name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(args, &out, &errb); code != 2 {
				t.Fatalf("usage exit = %d", code)
			}
			if !strings.Contains(errb.String(), "usage:") {
				t.Fatalf("usage diagnostic = %q", errb.String())
			}
		})
	}
}

func TestRunPolicyReportsExecutedIgnoredBody(t *testing.T) {
	root := t.TempDir()
	markerText := "//" + " coverage-ignore: impossible"
	testsupport.WriteGoModule(t, root, "example.com/m", "package m\nfunc F() { "+markerText+"\n}\n")
	uncovered := testsupport.WriteProfile(t, root, "example.com/m/f.go:2.10,3.2 1 0\n")
	reviewPath := writeReview(t, root, reviewedAnalysis(t, root, uncovered))
	baselinePath := filepath.Join(root, "coverage-baseline.json")
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--generate-policy", uncovered, baselinePath, reviewPath}, &out, &errb); code != 0 {
		t.Fatalf("generation exit %d: %s", code, errb.String())
	}
	executed := testsupport.WriteProfile(t, root, "example.com/m/f.go:2.10,3.2 1 1\n")
	out.Reset()
	errb.Reset()
	if code := run([]string{"covercheck", "--policy", executed, baselinePath}, &out, &errb); code != 1 {
		t.Fatalf("executed-ignore exit = %d", code)
	}
	if !strings.Contains(errb.String(), "executed-ignore") {
		t.Fatalf("executed-ignore diagnostic = %q", errb.String())
	}
}

func TestRunGenerateRegeneratesExistingCanonicalBaseline(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteGoModule(t, root, "example.com/m", "package m\nfunc F() {}\n")
	profile := testsupport.WriteProfile(t, root, "example.com/m/f.go:2.1,2.5 1 0\n")
	reviewPath := writeReview(t, root, reviewedAnalysis(t, root, profile))
	baselinePath := filepath.Join(root, "coverage-baseline.json")
	for runIndex := range 2 {
		var out, errb bytes.Buffer
		if code := run([]string{"covercheck", "--generate-policy", profile, baselinePath, reviewPath}, &out, &errb); code != 0 {
			t.Fatalf("generation %d exit %d: %s", runIndex, code, errb.String())
		}
	}
}

func TestRunGenerateFailureDiagnostics(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteGoModule(t, root, "example.com/m", "package m\nfunc F() {}\n")
	profile := testsupport.WriteProfile(t, root, "example.com/m/f.go:2.1,2.5 1 0\n")
	reviewPath := writeReview(t, root, reviewedAnalysis(t, root, profile))
	baselinePath := filepath.Join(root, "coverage-baseline.json")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"covercheck", "--generate-policy", filepath.Join(root, "missing.out"), baselinePath, reviewPath}, want: "no such file"},
		{name: "missing review", args: []string{"covercheck", "--generate-policy", profile, baselinePath, filepath.Join(root, "missing-review.json")}, want: "read review"},
		{name: "unreviewed miss", args: []string{"covercheck", "--generate-policy", profile, baselinePath, writeReview(t, t.TempDir(), coverage.Review{})}, want: "requires review"},
		{name: "missing output directory", args: []string{"covercheck", "--generate-policy", profile, filepath.Join(root, "absent", "baseline.json"), reviewPath}, want: "open baseline directory"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(test.args, &out, &errb); code != 1 {
				t.Fatalf("failure exit = %d", code)
			}
			if !strings.Contains(errb.String(), test.want) {
				t.Fatalf("diagnostic = %q, want %q", errb.String(), test.want)
			}
		})
	}
}

func TestRunGenerateRejectsInvalidExistingAndUninspectableBaseline(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteGoModule(t, root, "example.com/m", "package m\nfunc F() {}\n")
	profile := testsupport.WriteProfile(t, root, "example.com/m/f.go:2.1,2.5 1 0\n")
	reviewPath := writeReview(t, root, reviewedAnalysis(t, root, profile))

	invalid := filepath.Join(root, "invalid.json")
	if err := os.WriteFile(invalid, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parentFile := filepath.Join(root, "not-directory")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, baselinePath := range map[string]string{
		"invalid existing": invalid,
		"uninspectable":    filepath.Join(parentFile, "baseline.json"),
	} {
		t.Run(name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run([]string{"covercheck", "--generate-policy", profile, baselinePath, reviewPath}, &out, &errb); code != 1 {
				t.Fatalf("failure exit = %d", code)
			}
		})
	}
}

func TestRunPolicyAnalysisError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteGoModule(t, root, "example.com/m", "package m\n")
	t.Chdir(root)
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", "--policy", filepath.Join(root, "missing.out"), filepath.Join(root, "missing.json")}, &out, &errb); code != 1 {
		t.Fatalf("analysis failure exit = %d", code)
	}
}

func TestWriteCompleteFailurePaths(t *testing.T) {
	if err := writeComplete(filepath.Join(t.TempDir(), "missing", "baseline.json"), []byte("x")); err == nil {
		t.Fatal("expected missing-directory error")
	}
	root := t.TempDir()
	target := filepath.Join(root, "baseline.json")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeComplete(target, []byte("x")); err == nil {
		t.Fatal("expected replacement-over-directory error")
	}
}
