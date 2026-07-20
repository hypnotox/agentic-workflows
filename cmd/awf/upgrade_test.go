package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

// TestRunUpgradeGateStateError covers the GateState error branch in runUpgrade:
// an old-tree (.claude/awf/) project whose legacy lock is corrupt. The new-tree
// lock is absent (so the earlier LoadOptional passes), and the fault surfaces
// when GateState reads the legacy lock to compute the generation.
func TestRunUpgradeAuthorityRefusalsDoNotMutate(t *testing.T) {
	for _, tc := range []struct{ name, lock, want string }{
		{"missing", "", "use the bridge release to attest"},
		{"pre-tracking", `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`, "use the bridge release to attest"},
		{"invalid", `{"awfVersion":"0.19.0","schemaVersion":14,"files":{},"adrFormatV1From":1}`, "restore"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteAwfConfig(t, root, minimalYAML)
			if tc.lock != "" {
				testsupport.WriteFile(t, config.LockPath(root), tc.lock)
			}
			before := snapshotTree(t, root)
			err := runUpgrade(root, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
			if after := snapshotTree(t, root); after != before {
				t.Fatal("refused upgrade mutated the tree")
			}
		})
	}
}

func TestRunUpgradeGateStateError(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, ".claude", "awf")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "config.yaml"), []byte("prefix: ex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "awf.lock"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runUpgrade(root, io.Discard); err == nil {
		t.Fatal("expected a GateState error from the corrupt legacy lock")
	}
}

// writeValidJournal writes a minimal valid single-op (lock) journal in the given
// phase. When finalMatchesLock, its final hash matches the on-disk lock so
// recovery treats it as committed and cleans it up.
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
	j := upgrade.Journal{
		Version:         upgrade.JournalVersion,
		Phase:           phase,
		FinalLockSHA256: fmt.Sprintf("%x", sha256.Sum256(final)),
		Operations: []upgrade.Operation{
			{Path: upgrade.LockRel(), Prior: upgrade.Image{Present: true, Mode: uint32(info.Mode().Perm()), Content: lockBytes}, Replacement: upgrade.Image{Present: true, Mode: 0o644, Content: final}},
		},
	}
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(upgrade.JournalPath(root), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// attestLock writes a bridge attestation into the project's lock so the guard
// and the seal-consumption routing observe an attested lock. The sealed facts
// are deliberately bogus: the tests assert only routing, not a passing seal.
func attestLock(t *testing.T, root string) {
	t.Helper()
	lock, found, err := manifest.LoadOptional(config.LockPath(root))
	if err != nil || !found {
		t.Fatalf("load lock: %v found=%t", err, found)
	}
	lock = &manifest.Lock{
		AWFVersion: lock.AWFVersion, SchemaVersion: lock.SchemaVersion, Files: lock.Files,
		BridgeAttestation: &manifest.BridgeAttestation{Version: 1, PreparedHead: "0000000000000000000000000000000000000000", TreeDigest: "sha256:0", ADRFormatV1From: 137},
	}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
}

func TestGuardValidJournalPermitsOnlyRecover(t *testing.T) {
	root := scaffoldProject(t)
	writeValidJournal(t, root, "lock-committed", true)
	// Every non-recover command refuses with the run-recover diagnostic.
	for _, args := range [][]string{{"awf", "check"}, {"awf", "upgrade"}} {
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
	if upgrade.JournalPresent(root) {
		t.Fatal("journal not cleaned by recovery")
	}
	if !strings.Contains(out.String(), "operation: recovered") {
		t.Fatalf("no recovered line: %s", out.String())
	}
}

func TestGuardMalformedJournalRefusesEveryMode(t *testing.T) {
	root := scaffoldProject(t)
	if err := os.WriteFile(upgrade.JournalPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"awf", "upgrade", "--recover"}, {"awf", "check"}} {
		var out, errb bytes.Buffer
		if code := runAt(t, root, args, &out, &errb); code == 0 || !strings.Contains(errb.String(), "restore the working tree from Git") {
			t.Fatalf("%v not refused with restoration guidance: code=%d\n%s", args, code, errb.String())
		}
	}
}

func TestGuardRecoverWithoutJournal(t *testing.T) {
	root := scaffoldProject(t)
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "upgrade", "--recover"}, &out, &errb); code == 0 || !strings.Contains(errb.String(), "no current-state upgrade journal to recover") {
		t.Fatalf("recover-without-journal: code=%d\n%s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := runAt(t, t.TempDir(), []string{"awf", "upgrade", "--recover"}, &out, &errb); code == 0 || !strings.Contains(errb.String(), "not an awf project") {
		t.Fatalf("recover outside tree: code=%d\n%s", code, errb.String())
	}
}

func TestGuardAttestedLockPermitsUpgradeRefusesOthers(t *testing.T) {
	root := scaffoldProject(t)
	attestLock(t, root)
	// Ordinary commands refuse with the consume-the-attestation diagnostic.
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "check"}, &out, &errb); code == 0 || !strings.Contains(errb.String(), "run `awf upgrade` to consume it") {
		t.Fatalf("check not refused: code=%d\n%s", code, errb.String())
	}
	// Plain upgrade is permitted by the guard and reaches the handler, which
	// verifies the seal and refuses the bogus prepared head (not a guard message).
	out.Reset()
	errb.Reset()
	code := runAt(t, root, []string{"awf", "upgrade"}, &out, &errb)
	if code == 0 || strings.Contains(errb.String(), "run `awf upgrade` to consume it") {
		t.Fatalf("upgrade should reach the handler: code=%d\n%s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "prepared head") {
		t.Fatalf("want a seal-verification failure, got: %s", errb.String())
	}
}

func TestUpgradeConsumesAttestationRouting(t *testing.T) {
	// runUpgrade routes an attested lock into the final cutover verifier, which
	// rejects the bogus sealed facts rather than running a schema migration.
	root := scaffoldProject(t)
	attestLock(t, root)
	if err := runUpgrade(root, io.Discard); err == nil || !strings.Contains(err.Error(), "prepared head") {
		t.Fatalf("want seal verification, got %v", err)
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
	prepared := filepath.Join(root, "prepared.txt")
	if err := os.WriteFile(prepared, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	j := upgrade.Journal{
		Version:         upgrade.JournalVersion,
		Phase:           "applying",
		FinalLockSHA256: fmt.Sprintf("%x", sha256.Sum256(final)),
		Operations: []upgrade.Operation{
			{Path: "prepared.txt", Prior: upgrade.Image{Present: false}, Replacement: upgrade.Image{Present: true, Mode: 0o644, Content: []byte("new")}},
			{Path: upgrade.LockRel(), Prior: upgrade.Image{Present: true, Mode: uint32(info.Mode().Perm()), Content: lockBytes}, Replacement: upgrade.Image{Present: true, Mode: 0o644, Content: final}},
		},
	}
	b, _ := json.MarshalIndent(j, "", "  ")
	if err := os.WriteFile(upgrade.JournalPath(root), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runRecover(root, io.Discard); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if _, err := os.Stat(prepared); !os.IsNotExist(err) {
		t.Fatal("prepared.txt not rolled back")
	}
	if upgrade.JournalPresent(root) {
		t.Fatal("journal residue after rollback")
	}
}
