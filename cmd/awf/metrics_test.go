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

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/telemetry"
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
