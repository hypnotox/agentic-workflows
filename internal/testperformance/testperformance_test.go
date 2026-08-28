package testperformance

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validRecord(t *testing.T) Record {
	t.Helper()
	record, err := Load("../../test-performance.json")
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestCanonicalTrackedRecord(t *testing.T) {
	record := validRecord(t)
	canonical, err := Canonical(record)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Parse(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != Version || !bytes.Equal(canonical, mustCanonical(t, loaded)) {
		t.Fatalf("canonical record did not round trip: %s", canonical)
	}
	tracked, err := os.ReadFile("../../test-performance.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tracked, canonical) {
		t.Fatal("tracked record is not canonical")
	}
}

func TestParseRejectsMalformedRecords(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"version":1}`,
		`{"version":2}`,
		`{"version":1,"unknown":true}`,
		`{"version":1`,
	} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("Parse(%s) succeeded", raw)
		}
	}
}

func TestRecordDeclaresCompleteWorkloadsAndEnvironments(t *testing.T) {
	record := validRecord(t)
	if len(record.Workloads) != 5 {
		t.Fatalf("workloads = %d", len(record.Workloads))
	}
	seen := map[string]bool{}
	for _, environment := range record.Environments {
		seen[environment.Kind] = true
		for _, value := range []string{environment.CPU, environment.OS, environment.Architecture, environment.Filesystem, environment.Memory, environment.RunnerImage, environment.Toolchain} {
			if value == "" {
				t.Fatalf("incomplete environment: %#v", environment)
			}
		}
	}
	if !seen["local"] || !seen["hosted"] {
		t.Fatalf("environment kinds = %v", seen)
	}
}

func TestAggregatePreservesComponentsAndEvaluatesDeterministically(t *testing.T) {
	record := validRecord(t)
	environment := record.Environments[0]
	record.Observations = []Observation{
		{Workload: "ordinary-full", Environment: environment, Cache: "warm", Sample: 1, EvidenceClass: "qualification", Result: "passed", Seconds: 100, Components: []Component{{Stage: "go-test", Package: "internal/project", Test: "TestRunner", Seconds: 40}}},
		{Workload: "ordinary-full", Environment: environment, Cache: "warm", Sample: 2, EvidenceClass: "qualification", Result: "passed", Seconds: 120, Components: []Component{{Stage: "go-test", Package: "internal/project", Test: "TestRunner", Seconds: 50}}},
		{Workload: "ordinary-full", Environment: environment, Cache: "warm", Sample: 3, EvidenceClass: "qualification", Result: "passed", Seconds: 110, Components: []Component{{Stage: "go-test", Package: "internal/project", Test: "TestRunner", Seconds: 45}}},
	}
	for i := range record.Budgets {
		if record.Budgets[i].Workload == "ordinary-full" && record.Budgets[i].Environment == environment.ID {
			record.Budgets[i].ComponentMaximums = []Component{{Stage: "go-test", Package: "internal/project", Test: "TestRunner", Seconds: 44}}
		}
	}
	aggregates := Aggregates(record)
	if len(aggregates) != 1 || aggregates[0].Seconds != 110 || len(aggregates[0].Components) != 3 || aggregates[0].Components[0].Test != "TestRunner" {
		t.Fatalf("aggregate = %#v", aggregates)
	}
	evaluations := Evaluate(record, aggregates)
	if len(evaluations) != 1 || !evaluations[0].MeetsMaximum || evaluations[0].WallTime != "evidence-not-correctness" || len(evaluations[0].ComponentRegressions) != 1 {
		t.Fatalf("evaluations = %#v", evaluations)
	}
}

func TestValidationRefusesObservationWithMismatchedEnvironment(t *testing.T) {
	record := validRecord(t)
	candidate := record.Environments[0]
	candidate.CPU = "different CPU"
	record.Observations = []Observation{{Workload: "fast-gate", Environment: candidate, Cache: "warm", Sample: 1, EvidenceClass: "qualification", Result: "passed"}}
	if err := Validate(record); err == nil || !strings.Contains(err.Error(), "not like-for-like") {
		t.Fatalf("validation error = %v", err)
	}
}

func TestValidationRejectsInventedUnqualifiedBudget(t *testing.T) {
	record := validRecord(t)
	for i := range record.Budgets {
		if record.Budgets[i].Qualification == "unqualified" {
			record.Budgets[i].MaximumSeconds = 1
		}
	}
	if err := Validate(record); err == nil || !strings.Contains(err.Error(), "must not invent") {
		t.Fatalf("validation error = %v", err)
	}
}

func TestMatchEnvironmentRefusesUnlikeEvidence(t *testing.T) {
	record := validRecord(t)
	if err := MatchEnvironment(record.Environments[0], record.Environments[1]); err == nil || !strings.Contains(err.Error(), "not like-for-like") {
		t.Fatalf("mismatch error = %v", err)
	}
	if err := MatchEnvironment(record.Environments[0], record.Environments[0]); err != nil {
		t.Fatal(err)
	}
}

func TestBuildReportFeedsBothRenderingsFromObservations(t *testing.T) {
	record := validRecord(t)
	record.Observations = []Observation{{Workload: "ordinary-full", Environment: record.Environments[0], Cache: "warm", Sample: 1, EvidenceClass: "diagnostic", Result: "coverage-policy-refused", Seconds: 2, Components: []Component{{Stage: "gate", Package: "repository", Test: "fast", Seconds: 2}}}}
	report := BuildReport(record)
	machine, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var human bytes.Buffer
	WriteHuman(&human, report)
	if !strings.Contains(string(machine), `"evidence_class":"diagnostic"`) || !strings.Contains(string(machine), `"runner_image":"local checkout"`) || !strings.Contains(human.String(), "seconds=2.000") || !strings.Contains(human.String(), "workload common-feedback") || !strings.Contains(human.String(), "environment local-linux-amd64") {
		t.Fatalf("machine=%s human=%s", machine, human.String())
	}
}

func TestValidationRequiresEveryWorkload(t *testing.T) {
	base := validRecord(t)
	for missing := range requiredWorkloads {
		t.Run(missing, func(t *testing.T) {
			record := cloneRecord(t, base)
			filtered := record.Workloads[:0]
			for _, workload := range record.Workloads {
				if workload.ID != missing {
					filtered = append(filtered, workload)
				}
			}
			record.Workloads = filtered
			if err := Validate(record); err == nil || !strings.Contains(err.Error(), "required set") {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidationRejectsInvalidContracts(t *testing.T) {
	base := validRecord(t)
	cases := []struct {
		name   string
		mutate func(*Record)
		want   string
	}{
		{"required collections", func(r *Record) { r.Workloads = nil }, "required"},
		{"incomplete workload", func(r *Record) { r.Workloads[0].Description = "" }, "not a complete"},
		{"duplicate workload", func(r *Record) { r.Workloads[len(r.Workloads)-1] = r.Workloads[0] }, "duplicate workload"},
		{"invalid workload kind", func(r *Record) { r.Workloads[0].Kind = "nonsense" }, "classification must"},
		{"invalid workload mutation", func(r *Record) { r.Workloads[0].Mutation = "banana" }, "classification must"},
		{"incomplete environment", func(r *Record) { r.Environments[0].CPU = "" }, "full identity"},
		{"invalid environment kind", func(r *Record) { r.Environments[0].Kind = "other" }, "full identity"},
		{"duplicate environment", func(r *Record) { r.Environments = append(r.Environments, r.Environments[0]) }, "duplicate environment"},
		{"invalid sample method", func(r *Record) { r.SampleMethod.Aggregation = "mean" }, "sample_method"},
		{"unknown baseline workload", func(r *Record) { r.Baselines[0].Workload = "missing" }, "unknown workload"},
		{"unknown baseline environment", func(r *Record) { r.Baselines[0].Environment = "missing" }, "unknown environment"},
		{"negative baseline", func(r *Record) { r.Baselines[0].Seconds = -1 }, "must not be negative"},
		{"invalid baseline component", func(r *Record) { r.Baselines[0].Components[0].Stage = "" }, "component requires"},
		{"duplicate baseline component", func(r *Record) {
			r.Baselines[0].Components = append(r.Baselines[0].Components, r.Baselines[0].Components[0])
		}, "duplicate component"},
		{"duplicate baseline", func(r *Record) { r.Baselines = append(r.Baselines, r.Baselines[0]) }, "duplicate baseline"},
		{"unknown budget workload", func(r *Record) { r.Budgets[0].Workload = "missing" }, "unknown workload"},
		{"unknown budget environment", func(r *Record) { r.Budgets[0].Environment = "missing" }, "unknown environment"},
		{"duplicate budget", func(r *Record) { r.Budgets = append(r.Budgets, r.Budgets[0]) }, "duplicate budget"},
		{"minimum without baseline", func(r *Record) { r.Budgets[0].Qualification = "minimum" }, "no like-for-like baseline"},
		{"invalid minimum targets", func(r *Record) { r.Budgets[1].StrongerTargetSeconds = 200 }, "invalid targets"},
		{"invalid target", func(r *Record) { r.Budgets[0].MaximumSeconds = 0 }, "invalid targets"},
		{"invalid qualification", func(r *Record) { r.Budgets[0].Qualification = "other" }, "invalid qualification"},
		{"invalid budget component", func(r *Record) { r.Budgets[0].ComponentMaximums = []Component{{}} }, "component requires"},
		{"unknown observation environment", func(r *Record) { r.Observations[0].Environment.ID = "missing" }, "unknown environment"},
		{"unknown observation workload", func(r *Record) { r.Observations[0].Workload = "missing" }, "unknown workload"},
		{"invalid observation cache", func(r *Record) { r.Observations[0].Cache = "other" }, "cache"},
		{"invalid evidence class", func(r *Record) { r.Observations[0].EvidenceClass = "other" }, "evidence class"},
		{"qualification failure", func(r *Record) { r.Observations[0].Result = "failed" }, "must be passed"},
		{"diagnostic without result", func(r *Record) {
			r.Observations[0].EvidenceClass = "diagnostic"
			r.Observations[0].Result = ""
		}, "requires a result"},
		{"invalid observation sample", func(r *Record) { r.Observations[0].Sample = 0 }, "positive sample"},
		{"duplicate observation sample", func(r *Record) { r.Observations = append(r.Observations, r.Observations[0]) }, "duplicate observation sample"},
		{"invalid observation component", func(r *Record) { r.Observations[0].Components[0].Test = "" }, "component requires"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := cloneRecord(t, base)
			tc.mutate(&record)
			if err := Validate(record); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadParseAndCanonicalErrorPaths(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil || !strings.Contains(err.Error(), "read qualification record") {
		t.Fatalf("Load error = %v", err)
	}
	if _, err := Parse([]byte(`null null`)); err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("Parse multiple values error = %v", err)
	}
	invalid := validRecord(t)
	invalid.Version = 0
	if _, err := Canonical(invalid); err == nil {
		t.Fatal("Canonical accepted invalid record")
	}
	unencodable := validRecord(t)
	unencodable.Baselines[0].Seconds = math.NaN()
	if _, err := Canonical(unencodable); err == nil {
		t.Fatal("Canonical encoded a non-JSON number")
	}
	for _, raw := range []string{"", "{", `{"x":`, "[", `[{"x":`} {
		if err := rejectDuplicateKeys([]byte(raw)); err == nil {
			t.Errorf("rejectDuplicateKeys(%q) succeeded", raw)
		}
	}
}

func TestAggregationAndQualificationEdges(t *testing.T) {
	if median(nil) != 0 || medianValues([]float64{1, 3}) != 2 {
		t.Fatal("median edge calculation failed")
	}
	record := validRecord(t)
	record.Observations = []Observation{
		{Workload: "fast-gate", Environment: record.Environments[0], Cache: "cold", Sample: 1, EvidenceClass: "qualification", Result: "passed", Seconds: 2},
		{Workload: "fast-gate", Environment: record.Environments[0], Cache: "cold", Sample: 2, EvidenceClass: "qualification", Result: "passed", Seconds: 4},
		{Workload: "selected-mutation", Environment: record.Environments[0], Cache: "warm", Sample: 1, EvidenceClass: "exceptional", Result: "passed", Seconds: 9},
	}
	aggregates := Aggregates(record)
	if len(aggregates) != 1 || aggregates[0].Seconds != 3 {
		t.Fatalf("aggregates = %#v", aggregates)
	}
	if evaluations := Evaluate(record, aggregates); len(evaluations) != 0 {
		t.Fatalf("unbudgeted or unqualified evaluations = %#v", evaluations)
	}
	if HasComponentRegressions(BuildReport(record)) {
		t.Fatal("report invented a component regression")
	}
}

func TestMissingComponentBlocksQualification(t *testing.T) {
	record := validRecord(t)
	record.SampleMethod.WarmSamples = 2
	record.Observations = []Observation{
		{Workload: "ordinary-full", Environment: record.Environments[0], Cache: "warm", Sample: 1, EvidenceClass: "qualification", Result: "passed", Seconds: 100},
		{Workload: "ordinary-full", Environment: record.Environments[0], Cache: "warm", Sample: 2, EvidenceClass: "qualification", Result: "passed", Seconds: 101, Components: []Component{{Stage: "go-test", Package: "internal/project", Test: "package-total", Seconds: 100}}},
	}
	for i := range record.Budgets {
		if record.Budgets[i].Workload == "ordinary-full" {
			record.Budgets[i].ComponentMaximums = []Component{{Stage: "go-test", Package: "internal/project", Test: "package-total", Seconds: 200}}
		}
	}
	report := BuildReport(record)
	if !HasComponentRegressions(report) || !strings.Contains(report.Evaluations[0].ComponentRegressions[0], "missing") {
		t.Fatalf("report = %#v", report)
	}
}

func TestHumanReportIncludesComponentRegression(t *testing.T) {
	report := Report{RecordVersion: 1, Evaluations: []Evaluation{{
		Workload: "ordinary-full", Environment: "local", WallTime: "evidence-not-correctness",
		MaximumSeconds: 10, ObservedSeconds: 11, ComponentRegressions: []string{"go-test/pkg/test"},
	}}}
	if !HasComponentRegressions(report) {
		t.Fatal("component regression was not detected")
	}
	var out bytes.Buffer
	WriteHuman(&out, report)
	if !strings.Contains(out.String(), "component regression: go-test/pkg/test") {
		t.Fatalf("human report = %q", out.String())
	}
}

func cloneRecord(t *testing.T, record Record) Record {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var clone Record
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func mustCanonical(t *testing.T, record Record) []byte {
	t.Helper()
	data, err := Canonical(record)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
