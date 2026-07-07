package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJSON writes body to a temp file and returns its path.
func writeJSON(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mutants"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for missing arg, got %d", code)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("missing usage text: %q", errb.String())
	}
}

// ./x mutants pre-creates the report via mktemp, so a nonexistent path is a
// caller error (typo, changed script) — it must fail loudly, not read as a
// clean run. Only a present-but-empty file means "nothing to report".
// invariant: mutants-missing-report-errors
func TestRunMissingFileErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", filepath.Join(t.TempDir(), "nope.json")}, &out, &errb); code == 0 {
		t.Fatalf("expected non-zero exit for missing report file, got 0 (%s)", out.String())
	}
	if strings.Contains(out.String(), "no survived mutants") {
		t.Errorf("missing file must not read as a clean run: %q", out.String())
	}
}

func TestRunEmptyFile(t *testing.T) {
	for _, body := range []string{"", "   \n"} {
		var out, errb bytes.Buffer
		if code := run([]string{"mutants", writeJSON(t, body)}, &out, &errb); code != 0 {
			t.Fatalf("body %q: expected exit 0, got %d", body, code)
		}
		if !strings.Contains(out.String(), "no survived mutants") {
			t.Errorf("body %q: expected empty-run message, got %q", body, out.String())
		}
	}
}

func TestRunReadError(t *testing.T) {
	// A directory path is not IsNotExist but os.ReadFile still errors.
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", t.TempDir()}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on read error, got %d", code)
	}
	if !strings.Contains(errb.String(), "mutants:") {
		t.Errorf("expected error prefix, got %q", errb.String())
	}
}

func TestRunMalformed(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", writeJSON(t, "{not json")}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on malformed json, got %d", code)
	}
	if !strings.Contains(errb.String(), "parsing") {
		t.Errorf("expected parse error, got %q", errb.String())
	}
}

// invariant: mutants-timeout-untrusted
func TestRunTimedOutIsUntrusted(t *testing.T) {
	// A LIVED survivor is present, but the timeout makes the whole run untrustworthy.
	const j = `{"files":[{"file_name":"refs.go","mutations":[
		{"type":"CONDITIONALS_BOUNDARY","status":"LIVED","line":85},
		{"type":"ARITHMETIC_BASE","status":"TIMED OUT","line":92}]}]}`
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 when a mutant timed out, got %d", code)
	}
	if !strings.Contains(errb.String(), "timed out") {
		t.Errorf("expected timeout message, got %q", errb.String())
	}
}

func TestRunReportsOnlyLived(t *testing.T) {
	const j = `{"files":[{"file_name":"refs.go","mutations":[
		{"type":"ARITHMETIC_BASE","status":"KILLED","line":63},
		{"type":"CONDITIONALS_NEGATION","status":"NOT COVERED","line":60},
		{"type":"CONDITIONALS_BOUNDARY","status":"NOT VIABLE","line":70},
		{"type":"ARITHMETIC_BASE","status":"LIVED","line":92}]}]}`
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
	o := out.String()
	if !strings.Contains(o, "refs.go:92  ARITHMETIC_BASE") {
		t.Errorf("LIVED mutant not reported: %q", o)
	}
	if strings.Contains(o, "NOT COVERED") || strings.Contains(o, ":60") || strings.Contains(o, ":70") {
		t.Errorf("only LIVED should be reported, got %q", o)
	}
}

func TestRunNoSurvivors(t *testing.T) {
	const j = `{"files":[{"file_name":"refs.go","mutations":[
		{"type":"ARITHMETIC_BASE","status":"KILLED","line":63},
		{"type":"CONDITIONALS_NEGATION","status":"NOT COVERED","line":60}]}]}`
	var out, errb bytes.Buffer
	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "no survived mutants") {
		t.Errorf("expected no-survivors message, got %q", out.String())
	}
}
