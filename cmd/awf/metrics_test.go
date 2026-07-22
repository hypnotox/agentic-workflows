package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/telemetry"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestMetricsProtocolAndGrammar(t *testing.T) {
	var out bytes.Buffer
	c := &cmdCtx{sub: "protocol", inv: invocation{bools: map[string]bool{"--json": true}}, stdout: &out}
	if err := runMetrics(c); err != nil {
		t.Fatal(err)
	}
	wantProtocol := fmt.Sprintf("{\"schemaVersion\":1,\"protocol\":{\"major\":1,\"minor\":0},\"compatibleMajor\":1,\"descriptorSha256\":%q,\"awfVersion\":%q,\"projectVersion\":%q}\n", telemetry.DescriptorSHA256(), awfVersion(), awfVersion())
	if out.String() != wantProtocol {
		t.Fatalf("protocol output = %q, want %q", out.String(), wantProtocol)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	metricsSpec, ok := clispec.Lookup("metrics")
	if !ok {
		t.Fatal("metrics command missing")
	}
	wantHelp := map[string]string{
		"export":    "Usage: awf metrics export [selectors] --format <json|jsonl>",
		"protocol":  "Usage: awf metrics protocol --json",
		"lifecycle": "Usage: awf metrics lifecycle --request <FILE|-> [--json]",
		"retain":    "Usage: awf metrics retain [--dry-run] [--json]",
		"purge":     "Usage: awf metrics purge --effort <ID> --confirm [--json]",
	}
	for name, usage := range wantHelp {
		child, found := metricsSpec.Child(name)
		if !found || !strings.HasPrefix(child.HelpBody, usage+"\n") {
			t.Errorf("metrics %s help = %q, found=%v", name, child.HelpBody, found)
		}
	}
	for _, c := range []*cmdCtx{
		{sub: "", inv: invocation{}, stdout: io.Discard},
		{sub: "unknown", inv: invocation{}, stdout: io.Discard},
		{sub: "protocol", inv: invocation{bools: map[string]bool{}}, stdout: io.Discard},
	} {
		if err := runMetrics(c); err == nil {
			t.Fatal("expected metrics usage error")
		}
	}
}

// invariant: tooling/cli:metrics-and-doctor-command-contract
func TestMetricsAndDoctorCommandContract(t *testing.T) {
	root := telemetryProject(t)
	originalNow := telemetryNow
	telemetryNow = func() time.Time { return time.Date(2026, 7, 22, 1, 2, 3, 0, time.UTC) }
	t.Cleanup(func() { telemetryNow = originalNow })

	selector, err := parseTelemetrySelector(invocation{values: map[string]string{
		"--effort": "effort", "--session": "session", "--phase": "implementation",
		"--since": "2026-07-22T00:00:00Z", "--until": "2026-07-22T00:00:00.000000001Z",
	}})
	if err != nil || selector.EffortID == nil || selector.SessionID == nil || selector.Phase == nil || selector.Since == nil || selector.Until == nil {
		t.Fatalf("selector = %#v, %v", selector, err)
	}
	for _, values := range []map[string]string{
		{"--phase": "unknown"},
		{"--since": "not-time"},
		{"--since": "2026-07-22T00:00:01Z", "--until": "2026-07-22T00:00:01Z"},
	} {
		if _, err := parseTelemetrySelector(invocation{values: values}); err == nil {
			t.Fatalf("invalid selector accepted: %v", values)
		}
	}

	var out bytes.Buffer
	query := &cmdCtx{root: root, inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &out}
	if err := runMetrics(query); err != nil {
		t.Fatal(err)
	}
	wantMetrics := "{\"schemaVersion\":1,\"protocolMajor\":1,\"generatedAt\":\"2026-07-22T01:02:03Z\",\"selector\":{},\"efforts\":[],\"retention\":{\"maxAgeDays\":90,\"maxCount\":100,\"terminalEffortCount\":0,\"candidates\":[]},\"integrity\":[]}\n"
	if out.String() != wantMetrics {
		t.Fatalf("metrics JSON = %q, want %q", out.String(), wantMetrics)
	}
	out.Reset()
	query.inv.bools["--json"] = false
	if err := runMetrics(query); err != nil || out.String() != "workflow metrics schema 1 protocol 1 generated 2026-07-22T01:02:03Z\nretention terminal=0 candidates=0 max-age-days=90 max-count=100\n" {
		t.Fatalf("metrics human = %q, %v", out.String(), err)
	}

	out.Reset()
	doctor := &cmdCtx{root: root, inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &out}
	if err := runDoctor(doctor); err != nil {
		t.Fatal(err)
	}
	wantDoctor := "{\"schemaVersion\":1,\"protocolMajor\":1,\"generatedAt\":\"2026-07-22T01:02:03Z\",\"selector\":{},\"findings\":[],\"integrity\":[]}\n"
	if out.String() != wantDoctor {
		t.Fatalf("doctor JSON = %q, want %q", out.String(), wantDoctor)
	}
	out.Reset()
	doctor.inv.bools["--json"] = false
	if err := runDoctor(doctor); err != nil || out.String() != "workflow doctor schema 1 protocol 1 generated 2026-07-22T01:02:03Z\n" {
		t.Fatalf("doctor human = %q, %v", out.String(), err)
	}
}

func TestMetricsJSONLGoldenOrderingAndNoCorruptRaw(t *testing.T) {
	root := telemetryProject(t)
	createRequest(t, root, "z-effort", "z-create")
	createRequest(t, root, "a-effort", "a-create")
	var out bytes.Buffer
	c := &cmdCtx{root: root, sub: "export", inv: invocation{values: map[string]string{"--format": "jsonl"}}, stdout: &out}
	if err := runMetrics(c); err != nil {
		t.Fatal(err)
	}
	wantJSONL := "{\"version\":{\"major\":1,\"minor\":0},\"eventId\":\"a-create\",\"idempotencyKey\":\"key-a-create\",\"effortId\":\"a-effort\",\"sessionId\":\"session-id\",\"timestamp\":\"2026-07-22T00:00:00Z\",\"kind\":\"effort_created\",\"predecessors\":[],\"payload\":{\"checkpointId\":\"checkpoint.md\",\"creationMode\":\"independent\"}}\n" +
		"{\"version\":{\"major\":1,\"minor\":0},\"eventId\":\"z-create\",\"idempotencyKey\":\"key-z-create\",\"effortId\":\"z-effort\",\"sessionId\":\"session-id\",\"timestamp\":\"2026-07-22T00:00:00Z\",\"kind\":\"effort_created\",\"predecessors\":[],\"payload\":{\"checkpointId\":\"checkpoint.md\",\"creationMode\":\"independent\"}}\n"
	if out.String() != wantJSONL {
		t.Fatalf("JSONL = %q, want %q", out.String(), wantJSONL)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for _, line := range lines {
		var event telemetry.EventEnvelope
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid normalized JSONL %q: %v", line, err)
		}
		if _, err := telemetry.ValidateEvent([]byte(line)); err != nil {
			t.Fatalf("unvalidated JSONL %q: %v", line, err)
		}
	}

	out.Reset()
	c.inv.values["--format"] = "json"
	if err := runMetrics(c); err != nil || !strings.Contains(out.String(), `"schemaVersion":1`) {
		t.Fatalf("JSON export = %q, %v", out.String(), err)
	}
	for _, format := range []string{"", "yaml"} {
		c.inv.values["--format"] = format
		if err := runMetrics(c); err == nil {
			t.Fatalf("format %q accepted", format)
		}
	}
}

func TestMetricsJSONLRejectsMalformedCompleteInputWithoutOutput(t *testing.T) {
	root := telemetryProject(t)
	createRequest(t, root, "effort", "create")
	stream := filepath.Join(root, ".awf", "metrics", "efforts", "effort", "sessions", "session-id.jsonl")
	file, err := os.OpenFile(stream, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("{}\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	export := &cmdCtx{root: root, sub: "export", inv: invocation{values: map[string]string{"--format": "jsonl"}}, stdout: &out}
	if err := runMetrics(export); err == nil || out.Len() != 0 {
		t.Fatalf("malformed JSONL export err=%v output=%q", err, out.String())
	}
	out.Reset()
	query := &cmdCtx{root: root, inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &out}
	if err := runMetrics(query); err != nil || !strings.Contains(out.String(), `"code":"malformed-complete-line"`) {
		t.Fatalf("metrics must report readable corruption: err=%v output=%q", err, out.String())
	}
}

func TestDoctorUsesConfiguredHeuristicsAndIsReadOnly(t *testing.T) {
	root := telemetryProject(t)
	createRequest(t, root, "effort", "create")
	ledger, err := telemetry.OpenLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	observation := `{"version":{"major":1,"minor":0},"eventId":"compact","observationId":"compact-observation","effortId":"effort","sessionId":"session-id","timestamp":"2026-07-22T00:00:01Z","kind":"compaction_observed","predecessors":["create"],"payload":{"count":3}}`
	if _, err := ledger.Append(context.Background(), []byte(observation)); err != nil {
		t.Fatal(err)
	}
	before := snapshotTelemetryTree(t, root)
	var out bytes.Buffer
	if err := runDoctor(&cmdCtx{root: root, inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"code":"WFH1-COMPACTIONS"`) {
		t.Fatalf("configured heuristic absent: %s", out.String())
	}
	after := snapshotTelemetryTree(t, root)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("doctor mutated resident telemetry: before=%v after=%v", before, after)
	}
}

func snapshotTelemetryTree(t *testing.T, root string) map[string]string {
	t.Helper()
	base := filepath.Join(root, ".awf", "metrics")
	result := map[string]string{}
	if err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			result[relative] = "directory"
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[relative] = string(content)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTelemetryQueryInputAndSelectorFailures(t *testing.T) {
	for _, command := range []*cmdCtx{
		{root: telemetryProject(t), inv: invocation{values: map[string]string{"--phase": "unknown"}}, stdout: io.Discard},
		{root: telemetryProject(t), sub: "export", inv: invocation{values: map[string]string{"--format": "jsonl", "--phase": "unknown"}}, stdout: io.Discard},
	} {
		if err := runMetrics(command); err == nil {
			t.Fatal("invalid selector reached telemetry query")
		}
	}

	unsafe := telemetryProject(t)
	if err := os.WriteFile(filepath.Join(unsafe, ".awf", "metrics", "efforts", "unsafe"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runMetrics(&cmdCtx{root: unsafe, sub: "export", inv: invocation{values: map[string]string{"--format": "jsonl"}}, stdout: io.Discard}); err == nil {
		t.Fatal("export accepted unreadable telemetry inputs")
	}

	lstatRoot := telemetryProject(t)
	originalLstat := telemetryStorageLstat
	t.Cleanup(func() { telemetryStorageLstat = originalLstat })
	telemetryStorageLstat = func(string) (os.FileInfo, error) { return nil, errors.New("injected lstat failure") }
	if _, _, err := readTelemetryQueryInputs(lstatRoot); err == nil || !strings.Contains(err.Error(), "inspect telemetry storage") {
		t.Fatalf("telemetry storage inspection failure = %v", err)
	}
	telemetryStorageLstat = originalLstat

	emptyStorage := telemetryProject(t)
	if err := os.RemoveAll(filepath.Join(emptyStorage, ".awf", "metrics")); err != nil {
		t.Fatal(err)
	}
	reads, cfg, err := readTelemetryQueryInputs(emptyStorage)
	if err != nil || len(reads) != 0 || cfg == nil {
		t.Fatalf("missing telemetry storage reads=%#v cfg=%v err=%v", reads, cfg, err)
	}

	invalidStorage := telemetryProject(t)
	metricsPath := filepath.Join(invalidStorage, ".awf", "metrics")
	if err := os.RemoveAll(metricsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metricsPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readTelemetryQueryInputs(invalidStorage); err == nil {
		t.Fatal("invalid telemetry storage opened")
	}
}

func TestTelemetryQueryFailuresWriteNoPartialOutputAndAreBounded(t *testing.T) {
	root := telemetryProject(t)
	if err := os.WriteFile(filepath.Join(root, ".awf", "metrics", "efforts", "unsafe"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runMetrics(&cmdCtx{root: root, inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &out})
	if err == nil || out.Len() != 0 || len(err.Error()) > 512 {
		t.Fatalf("query failure err=%v output=%q", err, out.String())
	}
	out.Reset()
	err = runDoctor(&cmdCtx{root: root, inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &out})
	if err == nil || out.Len() != 0 {
		t.Fatalf("doctor failure err=%v output=%q", err, out.String())
	}
	cause := errors.New(root + "/private\n" + strings.Repeat("x", 600))
	long := boundedTelemetryError(root, cause)
	if len(long.Error()) != 512 || strings.Contains(long.Error(), "\n") || strings.Contains(long.Error(), root) || !errors.Is(long, cause) {
		t.Fatalf("bounded error = %q", long)
	}
}

func TestMetricsAndDoctorExitMappingAndBinaryGating(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errOut bytes.Buffer
	if code := run([]string{"awf", "doctor", "--phase", "unknown"}, &out, &errOut); code != 2 || out.Len() != 0 {
		t.Fatalf("doctor usage exit=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"awf", "doctor", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("doctor findings/query must be non-blocking: exit=%d stderr=%q", code, errOut.String())
	}

	ahead := gateFixture(t, "99.0.0", migrate.Current())
	testsupport.SwapVar(t, &getwd, func() (string, error) { return ahead, nil })
	for _, command := range []string{"metrics", "doctor"} {
		out.Reset()
		errOut.Reset()
		if code := run([]string{"awf", command, "--json"}, &out, &errOut); code != 1 || out.Len() != 0 {
			t.Fatalf("%s gating exit=%d stdout=%q stderr=%q", command, code, out.String(), errOut.String())
		}
	}
}

func TestProtocolLedgerContract(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	request := `{"action":"create","idempotencyKey":"create-key","eventId":"create-event","effortId":"effort-id","sessionId":"session-id","timestamp":"2026-07-22T00:00:00Z","predecessors":[],"checkpointId":"checkpoint.md","creationMode":"independent"}`
	requestPath := filepath.Join(root, "request.json")
	if err := os.WriteFile(requestPath, []byte(request), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	c := &cmdCtx{root: root, sub: "lifecycle", inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{"--request": requestPath}}, stdout: &out}
	if err := runMetrics(c); err != nil {
		t.Fatal(err)
	}
	want := "{\"schemaVersion\":1,\"eventId\":\"create-event\",\"effortId\":\"effort-id\",\"sessionId\":\"session-id\",\"idempotent\":false}\n"
	if out.String() != want {
		t.Fatalf("lifecycle JSON = %q, want %q", out.String(), want)
	}
	out.Reset()
	c.inv.bools["--json"] = false
	if err := runMetrics(c); err != nil {
		t.Fatal(err)
	}
	if out.String() != "recorded create-event for effort effort-id session session-id\n" {
		t.Fatalf("human retry = %q", out.String())
	}
}

func TestMetricsLifecycleStdinAndErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	request := `{"action":"create","idempotencyKey":"key","eventId":"event","effortId":"effort","sessionId":"session","timestamp":"2026-07-22T00:00:00Z","predecessors":[],"checkpointId":"checkpoint.md","creationMode":"independent"}`
	c := &cmdCtx{root: root, sub: "lifecycle", inv: invocation{bools: map[string]bool{}, values: map[string]string{"--request": "-"}}, stdin: strings.NewReader(request), stdout: io.Discard}
	if err := runMetrics(c); err != nil {
		t.Fatal(err)
	}
	cases := []*cmdCtx{
		{root: root, sub: "lifecycle", inv: invocation{values: map[string]string{}}, stdin: strings.NewReader(""), stdout: io.Discard},
		{root: root, sub: "lifecycle", inv: invocation{values: map[string]string{"--request": filepath.Join(root, "missing")}}, stdout: io.Discard},
		{root: root, sub: "lifecycle", inv: invocation{values: map[string]string{"--request": "-"}}, stdin: strings.NewReader("{}"), stdout: io.Discard},
	}
	for _, tc := range cases {
		if err := runMetrics(tc); err == nil {
			t.Fatal("expected lifecycle error")
		}
	}
}

func TestMetricsRetentionAndPurge(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(minimalYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	ledger, err := telemetry.NewLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	staleLease := filepath.Join(root, ".awf", "metrics", "leases", "recovered.append.json")
	if err := os.WriteFile(staleLease, []byte(`{"nonce":"00000000000000000000000000000000","owner":"test","expiresAt":"2020-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = ledger
	var out bytes.Buffer
	retain := &cmdCtx{root: root, sub: "retain", inv: invocation{bools: map[string]bool{"--dry-run": true, "--json": true}}, stdout: &out}
	if err := runMetrics(retain); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{\"schemaVersion\":1,\"dryRun\":true,\"candidates\":[],\"pruned\":[],\"recovered\":[\"recovered\"]}\n" {
		t.Fatalf("retain = %q", out.String())
	}
	out.Reset()
	retain.inv.bools["--json"] = false
	if err := runMetrics(retain); err != nil || out.String() != "retention candidates 0, pruned 0, recovered 0\n" {
		t.Fatalf("human retain = %q, %v", out.String(), err)
	}
	for _, c := range []*cmdCtx{
		{root: root, sub: "purge", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: io.Discard},
		{root: root, sub: "purge", inv: invocation{bools: map[string]bool{"--confirm": true}, values: map[string]string{"--effort": "missing"}}, stdout: io.Discard},
	} {
		if err := runMetrics(c); err == nil {
			t.Fatal("expected purge error")
		}
	}
}

type failingMetricsWriter struct{}

func (failingMetricsWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestMetricsWriteFailure(t *testing.T) {
	c := &cmdCtx{sub: "protocol", inv: invocation{bools: map[string]bool{"--json": true}}, stdout: failingMetricsWriter{}}
	if err := runMetrics(c); err == nil {
		t.Fatal("expected write failure")
	}
}

func TestMetricsLifecycleStorageTransitionAndTrajectoryBranches(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if err := runMetrics(&cmdCtx{root: missing, sub: "lifecycle", inv: invocation{values: map[string]string{"--request": "-"}}, stdin: strings.NewReader(validCreateRequest("effort", "create")), stdout: io.Discard}); err == nil {
		t.Fatal("expected ledger creation failure")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	createRequest(t, root, "effort-id", "create")
	invalid := `{"action":"complete","idempotencyKey":"complete-key","eventId":"complete","effortId":"effort-id","sessionId":"session-id","timestamp":"2026-07-22T00:00:01Z","predecessors":["create"]}`
	if err := runMetrics(&cmdCtx{root: root, sub: "lifecycle", inv: invocation{values: map[string]string{"--request": "-"}}, stdin: strings.NewReader(invalid), stdout: io.Discard}); err == nil {
		t.Fatal("expected invalid transition")
	}
	trajectory := `{"action":"start-trajectory","idempotencyKey":"trajectory-key","eventId":"trajectory-event","effortId":"effort-id","sessionId":"session-id","timestamp":"2026-07-22T00:00:02Z","predecessors":["create"],"trajectoryId":"trajectory-id","anchorId":"anchor"}`
	var out bytes.Buffer
	if err := runMetrics(&cmdCtx{root: root, sub: "lifecycle", inv: invocation{values: map[string]string{"--request": "-"}}, stdin: strings.NewReader(trajectory), stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "recorded trajectory-event for effort effort-id session session-id trajectory trajectory-id\n" {
		t.Fatalf("trajectory output = %q", out.String())
	}
}

func TestMetricsMaintenanceFailuresAndSuccessfulPurge(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if err := runMetrics(&cmdCtx{root: missing, sub: "retain", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: io.Discard}); err == nil {
		t.Fatal("expected retention ledger error")
	}
	noConfig := t.TempDir()
	if err := os.MkdirAll(filepath.Join(noConfig, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := runMetrics(&cmdCtx{root: noConfig, sub: "retain", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: io.Discard}); err == nil {
		t.Fatal("expected retention config error")
	}
	badEffort := telemetryProject(t)
	if err := os.WriteFile(filepath.Join(badEffort, ".awf", "metrics", "efforts", "unsafe"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runMetrics(&cmdCtx{root: badEffort, sub: "retain", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: io.Discard}); err == nil {
		t.Fatal("expected retention selection error")
	}

	root := telemetryProject(t)
	createTerminalEffort(t, root, "json-effort")
	stalePurgeLease := filepath.Join(root, ".awf", "metrics", "leases", "recovered-purge.append.json")
	if err := os.WriteFile(stalePurgeLease, []byte(`{"nonce":"00000000000000000000000000000000","owner":"test","expiresAt":"2020-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	purge := &cmdCtx{root: root, sub: "purge", inv: invocation{bools: map[string]bool{"--confirm": true, "--json": true}, values: map[string]string{"--effort": "json-effort"}}, stdout: &out}
	if err := runMetrics(purge); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{\"schemaVersion\":1,\"dryRun\":false,\"candidates\":[\"json-effort\"],\"pruned\":[\"json-effort\"],\"recovered\":[\"recovered-purge\"]}\n" {
		t.Fatalf("purge JSON = %q", out.String())
	}
	createTerminalEffort(t, root, "human-effort")
	out.Reset()
	purge.inv.bools["--json"] = false
	purge.inv.values["--effort"] = "human-effort"
	if err := runMetrics(purge); err != nil || out.String() != "purged effort human-effort\n" {
		t.Fatalf("human purge = %q, %v", out.String(), err)
	}
}

func TestTelemetryRecoveryFailureAndAmbiguity(t *testing.T) {
	originalRecover := recoverTelemetryLedger
	t.Cleanup(func() { recoverTelemetryLedger = originalRecover })
	missing := filepath.Join(t.TempDir(), "missing")
	purge := func(root string) error {
		return runMetrics(&cmdCtx{root: root, sub: "purge", inv: invocation{bools: map[string]bool{"--confirm": true}, values: map[string]string{"--effort": "effort"}}, stdout: io.Discard})
	}
	if err := purge(missing); err == nil {
		t.Fatal("expected purge ledger error")
	}
	root := telemetryProject(t)
	recoverTelemetryLedger = func(*telemetry.Ledger) (telemetry.RecoveryReport, error) {
		return telemetry.RecoveryReport{}, errors.New("recover failed")
	}
	if err := purge(root); err == nil {
		t.Fatal("expected telemetry recovery error")
	}
	recoverTelemetryLedger = originalRecover
	ambiguous := telemetryProject(t)
	if err := os.Mkdir(filepath.Join(ambiguous, ".awf", "metrics", "leases", ".operations.lock"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := telemetryLedgerWithRecovery(ambiguous); err == nil {
		t.Fatal("expected ambiguous recovery error")
	}
}

func telemetryProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(minimalYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	ledger, err := telemetry.NewLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = ledger
	return root
}

func validCreateRequest(effortID, eventID string) string {
	return fmt.Sprintf(`{"action":"create","idempotencyKey":"key-%s","eventId":"%s","effortId":"%s","sessionId":"session-id","timestamp":"2026-07-22T00:00:00Z","predecessors":[],"checkpointId":"checkpoint.md","creationMode":"independent"}`, eventID, eventID, effortID)
}

func createRequest(t *testing.T, root, effortID, eventID string) {
	t.Helper()
	if err := runMetrics(&cmdCtx{root: root, sub: "lifecycle", inv: invocation{values: map[string]string{"--request": "-"}}, stdin: strings.NewReader(validCreateRequest(effortID, eventID)), stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
}

func createTerminalEffort(t *testing.T, root, effortID string) {
	t.Helper()
	ledger, err := telemetry.NewLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	create := telemetry.CreateLifecycleRequest{LifecycleRequestBase: telemetry.LifecycleRequestBase{Action: "create", IdempotencyKey: "create-key-" + effortID, EventID: "create-" + effortID, EffortID: effortID, SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}, CheckpointID: "checkpoint.md", CreationMode: "independent"}
	if _, err := ledger.ApplyLifecycle(context.Background(), create); err != nil {
		t.Fatal(err)
	}
	abandon := telemetry.TerminalLifecycleRequest{LifecycleRequestBase: telemetry.LifecycleRequestBase{Action: "abandon", IdempotencyKey: "abandon-key-" + effortID, EventID: "abandon-" + effortID, EffortID: effortID, SessionID: "session", Timestamp: "2026-07-22T00:00:01Z", Predecessors: []string{"create-" + effortID}}}
	if _, err := ledger.ApplyLifecycle(context.Background(), abandon); err != nil {
		t.Fatal(err)
	}
}
