package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/bridge"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write") }

func TestUpgradeCheckJSONSchema(t *testing.T) {
	report := bridge.Report{Ready: false, Findings: []bridge.Finding{{Code: "invariant-approval", Path: bridge.ApprovalPath, Detail: "required"}}, InvariantAdjudications: []bridge.Adjudication{{Key: "ADR-0001#x", Disposition: "live", Destination: "core/x:x", Origin: "ADR-0001", Backing: "test", Approved: true}}, PlannedMutations: []bridge.Mutation{{Path: "x", BeforePresent: false, BeforeSHA256: strings.Repeat("0", 64), AfterPresent: true, AfterMode: 0o644, AfterSHA256: strings.Repeat("1", 64)}}}
	var out bytes.Buffer
	if err := writeReadinessJSON(&out, report); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ready", "findings", "invariantAdjudications", "plannedMutations"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %s: %s", key, out.String())
		}
	}
	if strings.Contains(out.String(), "Before") || strings.Contains(out.String(), "After\"") {
		t.Errorf("unstable field names: %s", out.String())
	}
	out.Reset()
	writeReadinessHuman(&out, report)
	for _, want := range []string{"finding: invariant-approval", "invariant: ADR-0001#x", "mutation: x"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("human missing %q: %s", want, out.String())
		}
	}
	if err := writeReadinessJSON(failingWriter{}, report); err == nil {
		t.Fatal("writer failure ignored")
	}
}
func TestUpgradeFlagRules(t *testing.T) {
	var out bytes.Buffer
	if err := runUpgradeFlags(t.TempDir(), false, true, false, false, &out); err == nil || !strings.Contains(err.Error(), "requires --check") {
		t.Fatalf("%v", err)
	}
	if err := runUpgradeFlags(t.TempDir(), true, false, false, false, &out); err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("%v", err)
	}
	root := scaffoldProject(t)
	out.Reset()
	if err := runUpgradeFlags(root, true, false, false, false, &out); err == nil || !strings.Contains(out.String(), "ready: false") {
		t.Fatalf("human: %v %s", err, out.String())
	}
	out.Reset()
	if err := runUpgradeFlags(root, true, true, false, false, &out); err == nil || !strings.HasPrefix(out.String(), "{\"ready\":false") {
		t.Fatalf("json: %v %s", err, out.String())
	}
	ready := scaffoldProject(t)
	if err := os.WriteFile(filepath.Join(ready, ".awf", "config.yaml"), []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  sources: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ready, ".awf", "current-state-migration.yaml"), []byte("version: 1\ninvariantApprovals: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}, {"add", "."}, {"commit", "-qm", "fixture"}} {
		cmd := exec.Command("git", append([]string{"-C", ready}, args...)...)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git: %v %s", e, b)
		}
	}
	out.Reset()
	if err := runUpgradeFlags(ready, true, false, false, false, &out); err != nil {
		t.Fatalf("ready human: %v\n%s", err, out.String())
	}
	out.Reset()
	if err := runUpgradeFlags(ready, true, true, false, false, &out); err != nil {
		t.Fatalf("ready json: %v\n%s", err, out.String())
	}
	if err := runUpgradeFlags(ready, true, true, false, false, failingWriter{}); err == nil {
		t.Fatal("upgrade ignored JSON writer failure")
	}
}
