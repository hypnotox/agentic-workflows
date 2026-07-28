package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/telemetry"
)

type metricsFailWriter struct{ err error }

func (w metricsFailWriter) Write([]byte) (int, error) { return 0, w.err }

func metricsInvocation(values map[string]string, bools map[string]bool) invocation {
	if values == nil {
		values = map[string]string{}
	}
	if bools == nil {
		bools = map[string]bool{}
	}
	return invocation{values: values, bools: bools}
}

func metricsStream(id string, observations ...string) string {
	lines := []string{`{"record":"header","schemaVersion":1,"sessionId":"` + id + `","createdAt":"2026-07-27T00:00:00Z"}`}
	lines = append(lines, observations...)
	return strings.Join(lines, "\n") + "\n"
}

func metricsWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func metricsFixture(t *testing.T) (root, effortID string) {
	t.Helper()
	root = commandRepo(t)
	effortID = strings.Fields(runEffortCommand(t, root, "new", []string{"Metric effort"}, map[string]bool{"--no-memory": true}))[1]
	if err := runEffort(&cmdCtx{root: root, sub: "assign", inv: invocation{positionals: []string{effortID}, values: map[string]string{"--session": "session-a"}, bools: map[string]bool{}}, stdout: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}
	metrics := filepath.Join(root, ".awf", "metrics")
	metricsWrite(t, filepath.Join(metrics, "sessions", "session-a.jsonl"), metricsStream("session-a",
		`{"record":"observation","schemaVersion":1,"observationId":"123e4567-e89b-42d3-a456-426614174000","timestamp":"2026-07-27T00:00:00Z","kind":"usage","payload":{"inputTokens":2,"outputTokens":3,"cacheReadTokens":4,"cacheWriteTokens":5,"costUsd":1.5}}`,
		`{"record":"observation","schemaVersion":1,"observationId":"123e4567-e89b-42d3-a456-426614174001","timestamp":"2026-07-27T00:00:01Z","kind":"gate","payload":{"gate":"gate","outcome":"failure","durationMs":6}}`,
	))
	metricsWrite(t, filepath.Join(metrics, "sessions", "session-free.jsonl"), metricsStream("session-free",
		`{"record":"observation","schemaVersion":1,"observationId":"123e4567-e89b-42d3-a456-426614174002","timestamp":"2026-07-27T00:00:00Z","kind":"usage","payload":{"inputTokens":9,"outputTokens":0,"cacheReadTokens":0,"cacheWriteTokens":0,"costUsd":0}}`,
	))
	metricsWrite(t, filepath.Join(metrics, "efforts", effortID, "effort.json"), `{"legacy":true}`+"\n")
	metricsWrite(t, filepath.Join(metrics, "efforts", effortID, "sessions", "legacy.jsonl"), `{"kind":"usage_observed","payload":{"inputTokens":7,"outputTokens":8,"cacheReadTokens":9,"cacheWriteTokens":10,"costUsd":2.5}}`+"\n")
	return root, effortID
}

func TestMetricsReportDoctorListAndExports(t *testing.T) {
	root, id := metricsFixture(t)
	second := strings.Fields(runEffortCommand(t, root, "new", []string{"Second effort"}, map[string]bool{"--no-memory": true}))[1]
	var out bytes.Buffer
	if err := runMetrics(&cmdCtx{root: root, inv: metricsInvocation(nil, nil), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "effort "+id+` title="Metric effort" state=active`) || !strings.Contains(out.String(), "current input=2 output=3 cost=1.5") || !strings.Contains(out.String(), "legacy input=7 output=8 cost=2.5") {
		t.Fatalf("human report = %q", out.String())
	}
	out.Reset()
	if err := runMetrics(&cmdCtx{root: root, inv: metricsInvocation(map[string]string{"--session": "session-free"}, map[string]bool{"--json": true}), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	var selected telemetry.Report
	if err := json.Unmarshal(out.Bytes(), &selected); err != nil {
		t.Fatal(err)
	}
	if len(selected.Efforts) != 0 || len(selected.Sessions) != 1 || selected.Sessions[0].EffortID != nil || selected.Sessions[0].Counters.InputTokens != 9 {
		t.Fatalf("unassigned JSON report = %#v", selected)
	}

	metricsWrite(t, filepath.Join(root, ".awf", "metrics", "sessions", "bad.jsonl"), "bad\n")
	out.Reset()
	if err := runMetrics(&cmdCtx{root: root, sub: "doctor", inv: metricsInvocation(map[string]string{"--session": "bad"}, nil), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "session-v1 session=bad code=invalid-header\n" {
		t.Fatalf("doctor filtered report = %q", out.String())
	}
	metricsWrite(t, filepath.Join(root, ".awf", "metrics", "sessions", "bad-two.jsonl"), "bad\n")
	metricsWrite(t, filepath.Join(root, ".awf", "metrics", "sessions", "multi.jsonl"), metricsStream("multi",
		`{"record":"observation","schemaVersion":1,"observationId":"123e4567-e89b-42d3-a456-426614174009","timestamp":"2026-07-27T00:00:00Z","kind":"compaction","payload":{"inputTokensBefore":1,"inputTokensAfter":0}}`,
		`{"record":"observation","schemaVersion":1,"observationId":"123e4567-e89b-42d3-a456-426614174009","timestamp":"2026-07-27T00:00:00Z","kind":"compaction","payload":{"inputTokensBefore":1,"inputTokensAfter":0}}`,
		"broken",
	))
	metricsWrite(t, filepath.Join(root, ".awf", "metrics", "efforts", "not-a-directory"), "x")
	out.Reset()
	if err := runMetrics(&cmdCtx{root: root, sub: "doctor", inv: metricsInvocation(map[string]string{"--session": "bad"}, nil), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "session-v1 session=bad code=invalid-header\n" {
		t.Fatalf("doctor selector after extra findings = %q", out.String())
	}
	out.Reset()
	if err := runMetrics(&cmdCtx{root: root, sub: "doctor", inv: metricsInvocation(nil, map[string]bool{"--json": true}), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	var doctor telemetry.DoctorReport
	if err := json.Unmarshal(out.Bytes(), &doctor); err != nil || len(doctor.Findings) != 5 || doctor.Findings[0].Source != "legacy" || doctor.Findings[len(doctor.Findings)-1].SessionID != "multi" {
		t.Fatalf("doctor JSON = %#v, %v", doctor, err)
	}

	out.Reset()
	if err := runMetrics(&cmdCtx{root: root, sub: "list", inv: metricsInvocation(nil, nil), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "effort "+id+` title="Metric effort" state=active sessions=session-a`) || !strings.Contains(out.String(), "effort "+second) {
		t.Fatalf("list output = %q", out.String())
	}
	out.Reset()
	if err := runMetrics(&cmdCtx{root: root, sub: "list", inv: metricsInvocation(nil, map[string]bool{"--json": true}), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	var list struct {
		SchemaVersion int `json:"schemaVersion"`
		Efforts       []struct {
			ID       string   `json:"id"`
			Sessions []string `json:"sessions"`
		} `json:"efforts"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil || list.SchemaVersion != telemetry.SchemaVersion || len(list.Efforts) != 2 {
		t.Fatalf("list JSON = %#v, %v", list, err)
	}

	out.Reset()
	if err := runMetrics(&cmdCtx{root: root, sub: "export", inv: metricsInvocation(map[string]string{"--format": "json", "--effort": id}, nil), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	var exported telemetry.Report
	if err := json.Unmarshal(out.Bytes(), &exported); err != nil || len(exported.Efforts) != 1 || exported.Efforts[0].ID != id {
		t.Fatalf("JSON export = %#v, %v", exported, err)
	}
	out.Reset()
	if err := runMetrics(&cmdCtx{root: root, sub: "export", inv: metricsInvocation(map[string]string{"--format": "jsonl", "--effort": id}, nil), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 5 || !strings.Contains(lines[0], `"source":"session-v1"`) || !strings.Contains(lines[4], `"source":"legacy-protocol-2"`) {
		t.Fatalf("JSONL export = %q", out.String())
	}
}

func TestMetricsGrammarAndErrorBoundaries(t *testing.T) {
	if telemetryNow().IsZero() {
		t.Fatal("zero time")
	}
	cmd, _, sub, rest, ok := resolve([]string{"metrics", "doctor", "--session", "session", "--since", "2026-07-27T00:00:00Z", "--json"})
	if !ok || sub != "doctor" {
		t.Fatalf("metrics grammar resolution = %#v %q %v", cmd, sub, ok)
	}
	inv, err := parseArgs(cmd, rest)
	if err != nil || inv.values["--session"] != "session" || !inv.bools["--json"] {
		t.Fatalf("metrics grammar parse = %#v, %v", inv, err)
	}
	root, _ := metricsFixture(t)
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"awf", "metrics", "--json"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"schemaVersion":1`) {
		t.Fatalf("metrics CLI = code %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, args := range [][]string{
		{"awf", "metrics", "doctor", "--format", "json"},
		{"awf", "metrics", "export"},
		{"awf", "metrics", "export", "--format", "csv"},
		{"awf", "metrics", "--since", "not-time"},
		{"awf", "metrics", "--since", "2026-07-27T00:00:01Z", "--until", "2026-07-27T00:00:00Z"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("%v exit code = %d, stderr=%q", args, code, stderr.String())
		}
	}
	for _, inv := range []invocation{
		metricsInvocation(map[string]string{"--since": "not-time"}, nil),
		metricsInvocation(map[string]string{"--effort": "../bad"}, nil),
		metricsInvocation(map[string]string{"--since": "2026-07-27T00:00:01Z", "--until": "2026-07-27T00:00:00Z"}, nil),
	} {
		if _, err := parseTelemetrySelector(inv); err == nil {
			t.Errorf("parseTelemetrySelector accepted %#v", inv)
		}
	}
	selector, err := parseTelemetrySelector(metricsInvocation(map[string]string{"--effort": "effort", "--session": "session", "--since": "2026-07-27T00:00:00Z", "--until": "2026-07-27T00:00:01Z"}, nil))
	if err != nil || selector.EffortID == nil || selector.SessionID == nil || selector.Since == nil || selector.Until == nil {
		t.Fatalf("valid selector = %#v, %v", selector, err)
	}
	if err := runMetrics(&cmdCtx{root: root, sub: "unknown", inv: metricsInvocation(nil, nil), stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("unknown metrics command accepted")
	}
	if err := runMetrics(&cmdCtx{root: root, sub: "doctor", inv: metricsInvocation(map[string]string{"--since": "bad"}, nil), stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("doctor accepted invalid selector")
	}
	if err := runMetrics(&cmdCtx{root: root, sub: "export", inv: metricsInvocation(nil, nil), stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("export without format accepted")
	}
	if err := runMetrics(&cmdCtx{root: root, sub: "export", inv: metricsInvocation(map[string]string{"--format": "json", "--since": "bad"}, nil), stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("export accepted invalid selector")
	}
	for _, tc := range []struct {
		sub string
		inv invocation
	}{
		{"", metricsInvocation(nil, nil)},
		{"doctor", metricsInvocation(nil, nil)},
		{"list", metricsInvocation(nil, nil)},
		{"export", metricsInvocation(map[string]string{"--format": "json"}, nil)},
	} {
		err := runMetrics(&cmdCtx{root: t.TempDir(), sub: tc.sub, inv: tc.inv, stdout: &bytes.Buffer{}})
		var bounded *telemetryCommandError
		if !errors.As(err, &bounded) || bounded.Unwrap() == nil || bounded.Error() == "" {
			t.Fatalf("metrics %s read error = %#v", tc.sub, err)
		}
	}
	long := strings.Repeat("x", 600) + "\nnext"
	cause := errors.New(long)
	bounded := &telemetryCommandError{}
	ok = errors.As(boundedTelemetryError(cause), &bounded)
	if !ok || len(bounded.Error()) != 512 || strings.Contains(bounded.Error(), "\n") || !strings.HasSuffix(bounded.Error(), "...") || !errors.Is(bounded, cause) {
		t.Fatalf("bounded telemetry error = %q", bounded.Error())
	}
	if got := boundedTelemetryError(errors.New("short")); got.Error() != "short" {
		t.Fatalf("short bounded error = %q", got.Error())
	}
	if err := writeMetricsJSON(&bytes.Buffer{}, map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	if err := writeMetricsJSON(&bytes.Buffer{}, func() {}); err == nil {
		t.Fatal("writeMetricsJSON accepted unmarshalable value")
	}
	writeFailure := errors.New("write failure")
	if err := writeMetricsJSON(metricsFailWriter{writeFailure}, map[string]string{"a": "b"}); !errors.Is(err, writeFailure) {
		t.Fatalf("writeMetricsJSON error = %v", err)
	}
}

func TestMetricsAggregateAndExportErrorSeams(t *testing.T) {
	root, _ := metricsFixture(t)
	failure := errors.New("aggregate or export failure")
	t.Run("report-aggregate", func(t *testing.T) {
		original := metricsAggregate
		metricsAggregate = func(telemetry.ReadSet, telemetry.Selector) (telemetry.Report, error) {
			return telemetry.Report{}, failure
		}
		t.Cleanup(func() { metricsAggregate = original })
		if err := runMetrics(&cmdCtx{root: root, inv: metricsInvocation(nil, nil), stdout: &bytes.Buffer{}}); !errors.Is(err, failure) {
			t.Fatalf("report aggregate error = %v", err)
		}
	})
	t.Run("json-export-aggregate", func(t *testing.T) {
		original := metricsAggregate
		metricsAggregate = func(telemetry.ReadSet, telemetry.Selector) (telemetry.Report, error) {
			return telemetry.Report{}, failure
		}
		t.Cleanup(func() { metricsAggregate = original })
		err := runMetrics(&cmdCtx{root: root, sub: "export", inv: metricsInvocation(map[string]string{"--format": "json"}, nil), stdout: &bytes.Buffer{}})
		if !errors.Is(err, failure) {
			t.Fatalf("JSON export aggregate error = %v", err)
		}
	})
	t.Run("jsonl-export", func(t *testing.T) {
		original := metricsExport
		metricsExport = func(telemetry.ReadSet, telemetry.Selector) ([][]byte, error) { return nil, failure }
		t.Cleanup(func() { metricsExport = original })
		err := runMetrics(&cmdCtx{root: root, sub: "export", inv: metricsInvocation(map[string]string{"--format": "jsonl"}, nil), stdout: &bytes.Buffer{}})
		if !errors.Is(err, failure) {
			t.Fatalf("JSONL export error = %v", err)
		}
	})
}

func TestMetricsOutputErrorsPropagate(t *testing.T) {
	root, _ := metricsFixture(t)
	metricsWrite(t, filepath.Join(root, ".awf", "metrics", "sessions", "bad.jsonl"), "bad\n")
	failure := errors.New("output failure")
	for _, tc := range []struct {
		sub string
		inv invocation
	}{
		{"", metricsInvocation(nil, nil)},
		{"doctor", metricsInvocation(nil, nil)},
		{"list", metricsInvocation(nil, nil)},
		{"export", metricsInvocation(map[string]string{"--format": "json"}, nil)},
		{"export", metricsInvocation(map[string]string{"--format": "jsonl"}, nil)},
	} {
		t.Run(tc.sub+tc.inv.values["--format"], func(t *testing.T) {
			err := runMetrics(&cmdCtx{root: root, sub: tc.sub, inv: tc.inv, stdout: metricsFailWriter{failure}})
			if !errors.Is(err, failure) {
				t.Fatalf("%s output error = %v", tc.sub, err)
			}
		})
	}
}

// invariant: tooling/cli:metrics-command-contract
