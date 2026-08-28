package main

import (
	"bytes"
	"encoding/json"
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
	if !strings.Contains(human.String(), "qualification record v2") || !strings.Contains(human.String(), "aggregate fast-gate on local-linux-amd64: cache=warm samples=3 seconds=1.229") || !strings.Contains(human.String(), "class=diagnostic result=coverage-policy-refused-expanded-identity") || !strings.Contains(human.String(), "change=-14.155s (-93.5%)") {
		t.Fatalf("human=%q", human.String())
	}
	var report testperformance.Report
	if err := json.Unmarshal(machine.Bytes(), &report); err != nil {
		t.Fatalf("machine report: %v", err)
	}
	if len(report.Aggregates) != 1 {
		t.Fatalf("machine aggregates = %#v", report.Aggregates)
	}
	aggregate := report.Aggregates[0]
	if aggregate.Workload != "fast-gate" || aggregate.Environment != "local-linux-amd64" || aggregate.Cache != "warm" || aggregate.Samples != 3 || aggregate.Seconds != 1.229 {
		t.Fatalf("machine aggregate = %#v", aggregate)
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
			Sample: sample, EvidenceClass: "qualification", Result: "passed",
			Seconds: 100, Components: []testperformance.Component{component},
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
