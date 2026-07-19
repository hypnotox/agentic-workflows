package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/bridge"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// readyBridgeProject returns a synced, committed project whose readiness passes
// with no live invariants, matching the ready fixture in upgrade_test.go.
func readyBridgeProject(t *testing.T) string {
	t.Helper()
	root := scaffoldProject(t)
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  sources: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "current-state-migration.yaml"), []byte("version: 1\ninvariantApprovals: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}, {"add", "."}, {"commit", "-qm", "fixture"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
	}
	return root
}

// writeValidJournal writes a minimal valid lock-committed journal whose final
// hash matches the current lock, so recovery cleans it up.
func writeValidJournal(t *testing.T, root, phase string, finalMatchesLock bool) {
	t.Helper()
	lockPath := config.LockPath(root)
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	final := lockBytes
	if !finalMatchesLock {
		final = append(append([]byte{}, lockBytes...), '\n')
	}
	j := bridge.Journal{
		Version:         bridge.JournalVersion,
		Phase:           phase,
		FinalLockSHA256: fmt.Sprintf("%x", sha256.Sum256(final)),
		Operations: []bridge.Operation{
			{Path: bridge.LockRel(), Prior: bridge.Image{Present: true, Mode: uint32(info.Mode().Perm()), Content: lockBytes}, Replacement: bridge.Image{Present: true, Mode: 0o644, Content: final}},
		},
	}
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bridge.JournalPath(root), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAttestSealsAndRefusesOrdinaryCommands(t *testing.T) {
	root := readyBridgeProject(t)
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "upgrade", "--attest-current-state"}, &out, &errb); code != 0 {
		t.Fatalf("attest failed code=%d\n%s\n%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "operation: attestation committed") {
		t.Fatalf("no committed line: %s", out.String())
	}
	// The lock now carries the attestation and the journal is gone.
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if lock.BridgeAttestation == nil || lock.BridgeAttestation.Version != 1 || lock.BridgeAttestation.PreparedHead == "" || lock.BridgeAttestation.TreeDigest == "" {
		t.Fatalf("attestation not sealed: %#v", lock.BridgeAttestation)
	}
	if bridge.JournalPresent(root) {
		t.Fatal("journal residue after commit")
	}
	// The legacy ACTIVE.md index was deleted by the terminal projection.
	if _, err := os.Stat(filepath.Join(root, "docs", "decisions", "ACTIVE.md")); !os.IsNotExist(err) {
		t.Fatalf("ACTIVE.md not pruned: %v", err)
	}
	// An ordinary command now refuses with the install-the-release diagnostic.
	out.Reset()
	errb.Reset()
	if code := runAt(t, root, []string{"awf", "check"}, &out, &errb); code == 0 || !strings.Contains(errb.String(), "install and run the current-state release") {
		t.Fatalf("ordinary command not refused: code=%d\n%s", code, errb.String())
	}
	// Re-attestation and recovery refuse with the same diagnostic.
	for _, args := range [][]string{{"awf", "upgrade", "--attest-current-state"}, {"awf", "upgrade", "--recover"}, {"awf", "upgrade"}} {
		out.Reset()
		errb.Reset()
		if code := runAt(t, root, args, &out, &errb); code == 0 || !strings.Contains(errb.String(), "install and run the current-state release") {
			t.Fatalf("%v not refused: code=%d\n%s", args, code, errb.String())
		}
	}
	// upgrade --check remains permitted (read-only inspection).
	out.Reset()
	errb.Reset()
	_ = runAt(t, root, []string{"awf", "upgrade", "--check"}, &out, &errb)
	if strings.Contains(out.String()+errb.String(), "install and run the current-state release") {
		t.Fatalf("--check was guard-refused: %s", out.String()+errb.String())
	}
}

func TestAttestRefusesDirtyHead(t *testing.T) {
	root := readyBridgeProject(t)
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "upgrade", "--attest-current-state"}, &out, &errb); code == 0 || !strings.Contains(errb.String(), "not clean") {
		t.Fatalf("dirty head accepted: code=%d\n%s", code, errb.String())
	}
	if bridge.JournalPresent(root) {
		t.Fatal("journal written despite a dirty head")
	}
}

func TestAttestRefusesUnreadyAndMissingLock(t *testing.T) {
	// Not an awf project.
	var out bytes.Buffer
	if err := runAttest(t.TempDir(), &out); err == nil || !strings.Contains(err.Error(), "not an awf project") {
		t.Fatalf("non-project attest: %v", err)
	}
	// Not ready: a bare scaffold has no committed HEAD or migration file; readiness
	// fails and the findings print before the refusal.
	root := scaffoldProject(t)
	out.Reset()
	if err := runAttest(root, &out); err == nil || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("unready attest: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "finding:") {
		t.Fatalf("findings not printed: %s", out.String())
	}
	// Ready with a committed lock removal: HEAD is clean but no lock remains.
	ready := readyBridgeProject(t)
	if err := os.Remove(config.LockPath(ready)); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "drop lock"}} {
		cmd := exec.Command("git", append([]string{"-C", ready}, args...)...)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
	}
	out.Reset()
	if err := runAttest(ready, &out); err == nil || !strings.Contains(err.Error(), "no .awf/awf.lock to attest") {
		t.Fatalf("missing-lock attest: %v", err)
	}
	// Ready with a committed but corrupt lock: the ADR-0076 hard refusal fires.
	corrupt := readyBridgeProject(t)
	if err := os.WriteFile(config.LockPath(corrupt), []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "corrupt lock"}} {
		cmd := exec.Command("git", append([]string{"-C", corrupt}, args...)...)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
	}
	out.Reset()
	if err := runAttest(corrupt, &out); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("corrupt-lock attest: %v", err)
	}
}

func TestGuardValidJournalPermitsOnlyRecover(t *testing.T) {
	root := scaffoldProject(t)
	writeValidJournal(t, root, "lock-committed", true)
	// Every non-recover mode refuses with the run-recover diagnostic.
	for _, args := range [][]string{{"awf", "check"}, {"awf", "upgrade"}, {"awf", "upgrade", "--check"}, {"awf", "upgrade", "--attest-current-state"}} {
		var out, errb bytes.Buffer
		if code := runAt(t, root, args, &out, &errb); code == 0 || !strings.Contains(errb.String(), "awf upgrade --recover") {
			t.Fatalf("%v not refused: code=%d\n%s", args, code, errb.String())
		}
	}
	// version and changelog bypass the transaction state.
	for _, args := range [][]string{{"awf", "version"}, {"awf", "changelog"}} {
		var out, errb bytes.Buffer
		if code := runAt(t, root, args, &out, &errb); code != 0 {
			t.Fatalf("%v was guarded: code=%d\n%s", args, code, errb.String())
		}
	}
	// Recovery is permitted and cleans up the committed journal.
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "upgrade", "--recover"}, &out, &errb); code != 0 {
		t.Fatalf("recover failed: code=%d\n%s", code, errb.String())
	}
	if bridge.JournalPresent(root) {
		t.Fatal("journal not cleaned by recovery")
	}
	if !strings.Contains(out.String(), "operation: recovered") {
		t.Fatalf("no recovered line: %s", out.String())
	}
}

func TestGuardMalformedJournalRefusesEveryMode(t *testing.T) {
	root := scaffoldProject(t)
	if err := os.WriteFile(bridge.JournalPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"awf", "upgrade", "--recover"}, {"awf", "check"}, {"awf", "upgrade", "--check"}} {
		var out, errb bytes.Buffer
		if code := runAt(t, root, args, &out, &errb); code == 0 || !strings.Contains(errb.String(), "restore the working tree from Git") {
			t.Fatalf("%v not refused with restoration guidance: code=%d\n%s", args, code, errb.String())
		}
	}
}

func TestGuardRecoverWithoutJournal(t *testing.T) {
	// A synced project with neither a journal nor an attestation: recovery has
	// nothing to do.
	root := scaffoldProject(t)
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "upgrade", "--recover"}, &out, &errb); code == 0 || !strings.Contains(errb.String(), "no current-state upgrade journal to recover") {
		t.Fatalf("recover-without-journal: code=%d\n%s", code, errb.String())
	}
	// Outside an adopted tree the guard is a no-op and the handler owns the hint.
	out.Reset()
	errb.Reset()
	if code := runAt(t, t.TempDir(), []string{"awf", "upgrade", "--recover"}, &out, &errb); code == 0 || !strings.Contains(errb.String(), "not an awf project") {
		t.Fatalf("recover outside tree: code=%d\n%s", code, errb.String())
	}
}

func TestUpgradeModeMutualExclusion(t *testing.T) {
	var out bytes.Buffer
	if err := runUpgradeFlags(t.TempDir(), true, false, true, false, &out); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("check+attest: %v", err)
	}
	if err := runUpgradeFlags(t.TempDir(), false, false, true, true, &out); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("attest+recover: %v", err)
	}
}

func TestValidJournalRecoveryRollsBackInterrupted(t *testing.T) {
	// A precommit journal whose lock hash differs from the final hash rolls the
	// prepared write back to its prior image on recovery.
	root := scaffoldProject(t)
	lockPath := config.LockPath(root)
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(lockPath)
	final := append(append([]byte{}, lockBytes...), []byte("\n# attested\n")...)
	// A prepared file whose replacement is on disk; its prior is absent.
	prepared := filepath.Join(root, "prepared.txt")
	if err := os.WriteFile(prepared, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	j := bridge.Journal{
		Version:         bridge.JournalVersion,
		Phase:           "applying",
		FinalLockSHA256: fmt.Sprintf("%x", sha256.Sum256(final)),
		Operations: []bridge.Operation{
			{Path: "prepared.txt", Prior: bridge.Image{Present: false}, Replacement: bridge.Image{Present: true, Mode: 0o644, Content: []byte("new")}},
			{Path: bridge.LockRel(), Prior: bridge.Image{Present: true, Mode: uint32(info.Mode().Perm()), Content: lockBytes}, Replacement: bridge.Image{Present: true, Mode: 0o644, Content: final}},
		},
	}
	b, _ := json.MarshalIndent(j, "", "  ")
	if err := os.WriteFile(bridge.JournalPath(root), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runRecover(root, io.Discard); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if _, err := os.Stat(prepared); !os.IsNotExist(err) {
		t.Fatal("prepared.txt not rolled back")
	}
	if bridge.JournalPresent(root) {
		t.Fatal("journal residue after rollback")
	}
}
