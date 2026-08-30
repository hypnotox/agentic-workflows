package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// invariant: tooling/audit-and-snapshots:commit-gate-shared-rule (TestRunCommitGateAppliesSharedMessagePolicy)
func TestRunCommitGateAppliesSharedMessagePolicy(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(testContext(t), root, writeMsg(t, "feat: clean subject\n"), nil, &out); err != nil {
		t.Fatalf("clean message: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("clean output = %q", out.String())
	}
	if err := runCommitGate(testContext(t), root, writeMsg(t, "not conventional\n"), nil, &out); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("invalid message error = %v", err)
	}
}

func TestRunCommitGateDoesNotAuthorizeADRMerges(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runCommitGate(testContext(t), root, writeMsg(t, "Merge branch 'legacy'\n\nAWF-Allow-Version: legacy\n"), nil, &out); err != nil {
		t.Fatalf("merge message must not require ADR authorization: %v", err)
	}
}

func writeMsg(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func cite() string { return ".awf/efforts/example/memory.md" }
