package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testperformance"
)

func trackedRecord(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../test-performance.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "record.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateTrackedRecordDoesNotRewriteIt(t *testing.T) {
	path := trackedRecord(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"testperformance", "validate", path}, &out, &errb); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errb.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !strings.Contains(out.String(), "valid") {
		t.Fatalf("validate changed=%t out=%q", !bytes.Equal(before, after), out.String())
	}
}

func TestReportEmitsHumanAndMachineFromSameRecord(t *testing.T) {
	path := trackedRecord(t)
	var human, machine, errb bytes.Buffer
	if code := run([]string{"testperformance", "report", path}, &human, &errb); code != 0 {
		t.Fatalf("human=%d: %s", code, errb.String())
	}
	if code := run([]string{"testperformance", "report", "--machine", path}, &machine, &errb); code != 0 {
		t.Fatalf("machine=%d: %s", code, errb.String())
	}
	if !strings.Contains(human.String(), "qualification record v1") || !strings.Contains(machine.String(), `"record_version": 1`) {
		t.Fatalf("human=%q machine=%q", human.String(), machine.String())
	}
}

func TestReportBlocksDeterministicComponentRegression(t *testing.T) {
	record, err := testperformance.Load("../../test-performance.json")
	if err != nil {
		t.Fatal(err)
	}
	component := testperformance.Component{Stage: "go-test", Package: "internal/project", Test: "package-total", Seconds: 2}
	for i := range record.Budgets {
		if record.Budgets[i].Workload == "ordinary-full" {
			limit := component
			limit.Seconds = 1
			record.Budgets[i].ComponentMaximums = []testperformance.Component{limit}
		}
	}
	for sample := 1; sample <= 3; sample++ {
		record.Observations = append(record.Observations, testperformance.Observation{
			Workload: "ordinary-full", Environment: record.Environments[0], Cache: "warm",
			Sample: sample, Seconds: 100, Components: []testperformance.Component{component},
		})
	}
	data, err := testperformance.Canonical(record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "regression.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"testperformance", "report", "--machine", path}, &out, &errb); code != 1 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), `"component_regressions"`) || !strings.Contains(errb.String(), "blocks qualification") {
		t.Fatalf("stdout=%s stderr=%s", out.String(), errb.String())
	}
}

func TestRunRejectsMalformedRecordAndUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"version": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"testperformance", "validate", path}, &out, &errb); code != 1 {
		t.Fatalf("malformed code=%d", code)
	}
	for _, args := range [][]string{
		{"testperformance"},
		{"testperformance", "report", "--machine", "--machine", path},
		{"testperformance", "wat"},
		{"testperformance", "validate", "--machine"},
		{"testperformance", "report", path, path},
	} {
		if code := run(args, &out, &errb); code != 2 {
			t.Fatalf("usage %v code=%d", args, code)
		}
	}
}
