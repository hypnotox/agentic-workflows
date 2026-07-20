package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

// invariant: tooling/quality-gates:deadcode-gate
func TestRunFiltersTestsupportOnly(t *testing.T) {
	const j = `[{"Path":"x","Funcs":[
		{"Name":"Helper","Position":{"File":"internal/testsupport/x.go","Line":10}},
		{"Name":"Dead","Position":{"File":"internal/project/p.go","Line":42}}]}]`
	var out, errb bytes.Buffer
	if code := run(strings.NewReader(j), &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 with a non-testsupport finding, got %d", code)
	}
	if !strings.Contains(errb.String(), "internal/project/p.go:42") ||
		!strings.Contains(errb.String(), "Dead") {
		t.Errorf("offender not reported: %q", errb.String())
	}
	if strings.Contains(errb.String(), "testsupport") {
		t.Errorf("testsupport finding should be ignored, got %q", errb.String())
	}
}

func TestRunAllTestsupport(t *testing.T) {
	const j = `[{"Path":"x","Funcs":[
		{"Name":"H","Position":{"File":"internal/testsupport/gitfixture/g.go","Line":1}}]}]`
	var out, errb bytes.Buffer
	if code := run(strings.NewReader(j), &out, &errb); code != 0 {
		t.Fatalf("expected exit 0 when all findings are testsupport, got %d (%s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "no production dead code") {
		t.Errorf("expected clean message, got %q", out.String())
	}
}

func TestRunEmptyInputs(t *testing.T) {
	for _, in := range []string{"", "  \n", "null", "[]"} {
		var out, errb bytes.Buffer
		if code := run(strings.NewReader(in), &out, &errb); code != 0 {
			t.Fatalf("input %q: expected exit 0, got %d (%s)", in, code, errb.String())
		}
	}
}

func TestRunMalformed(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(strings.NewReader("{not json"), &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on malformed json, got %d", code)
	}
	if !strings.Contains(errb.String(), "parsing") {
		t.Errorf("expected parse error, got %q", errb.String())
	}
}

func TestRunReadError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(errReader{}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on read error, got %d", code)
	}
}
