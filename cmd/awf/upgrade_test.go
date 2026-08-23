package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

// preparePublicSyncLaterFailure makes public Publisher.Sync commit the earlier
// AGENTS.md mode correction before the later bridge path cannot be read.
func preparePublicSyncLaterFailure(t *testing.T, root string) {
	t.Helper()
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	bridge := filepath.Join(root, "CLAUDE.md")
	if err := os.Remove(bridge); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(bridge, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestPublicSyncPartialResultHasDivergentUpgradeAndSyncPresentation(t *testing.T) {
	t.Run("upgrade retains the real partial result", func(t *testing.T) {
		root := scaffoldProject(t)
		preparePublicSyncLaterFailure(t, root)
		var stdout, stderr bytes.Buffer
		if code := runAt(t, root, []string{"awf", "upgrade"}, &stdout, &stderr); code == 0 {
			t.Fatal("upgrade accepted an unreplaceable later output")
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want no partial success mutation", stdout.String())
		}
		if got := stderr.String(); !strings.Contains(got, "outputs: changed AGENTS.md (internal)") || !strings.Contains(got, "cause: filesystem: read \"CLAUDE.md\"") {
			t.Fatalf("upgrade failure diagnostic = %q, want committed output and sync cause", got)
		}
	})

	t.Run("ordinary sync discards the real partial result", func(t *testing.T) {
		root := scaffoldProject(t)
		preparePublicSyncLaterFailure(t, root)
		var stdout bytes.Buffer
		if err := runSync(testContext(t), root, &stdout); err == nil {
			t.Fatal("ordinary sync accepted an unreplaceable later output")
		}
		if stdout.Len() != 0 {
			t.Fatalf("ordinary sync stdout = %q, want no partial output", stdout.String())
		}
	})
}

func TestUpgradeMigrationAdaptsGroundingCollision(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: ex\nintegrationBranch: main\nskills: [brainstorming]\nagents: [grounding-checker]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "grounding.yaml"), "local: true\n")
	if err := (&manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 36, Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	_, err := upgradeMigration(testContext(t), root)
	var collision upgradeGroundingCollision
	if !errors.As(err, &collision) || collision.cause.Path != "skills/grounding.yaml" {
		t.Fatalf("error = %v, want adapted grounding collision", err)
	}
}

func TestUpgradeSyncMutationPropagatesLoaderFailures(t *testing.T) {
	root := gitfixture.InitRepo(t).Root()
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := upgradeSyncMutationWith(testContext(t), root, upgradeSyncDependencies{}); err == nil {
		t.Fatal("malformed repository accepted by upgrade sync loader")
	}
}

func TestUpgradeSyncMutationPropagatesProjectOpenFailures(t *testing.T) {
	if _, err := upgradeSyncMutationWith(testContext(t), t.TempDir(), upgradeSyncDependencies{}); err == nil {
		t.Fatal("missing project accepted by upgrade sync loader")
	}
}

func TestRunUpgradeGateStateError(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	testsupport.WriteFile(t, config.LockPath(root), `{"awfVersion":"0.19.0","schemaVersion":14,"files":{}}`)
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
	if err := runUpgrade(ctx, root, io.Discard); err == nil {
		t.Fatal("expected a GateState error from the corrupt legacy lock")
	}
}

// journalPresence answers upgrade.JournalPresent for the tests that assert
// presence or absence and expect no fault reading it.
func journalPresence(t *testing.T, root string) bool {
	t.Helper()
	found, err := upgrade.JournalPresent(root)
	if err != nil {
		t.Fatalf("JournalPresent(%s): %v", root, err)
	}
	return found
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
		BridgeAttestation: &manifest.BridgeAttestation{Version: 1, PreparedHead: "0000000000000000000000000000000000000000", TreeDigest: "sha256:0", ADRFormatV1From: 2, LegacyADRGaps: []int{}},
	}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
}

func TestGuardValidJournalPermitsOnlyRecover(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	if journalPresence(t, root) {
		t.Fatal("journal not cleaned by recovery")
	}
	if !strings.Contains(out.String(), "recovered: ") {
		t.Fatalf("no recovered line: %s", out.String())
	}
}

// TestGuardRefusesWhenJournalPresenceIsUnreadable pins that the command-state
// guard refuses when it cannot determine whether a journal exists. Reading the
// fault as absence permitted every command an unrecovered upgrade must block,
// including the ones that mutate the tree.
func TestGuardRefusesWhenJournalPresenceIsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := scaffoldProject(t)
	awfDir := filepath.Join(root, ".awf")
	if err := os.Chmod(awfDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(awfDir, 0o755) })
	var out, errb bytes.Buffer
	code := runAt(t, root, []string{"awf", "check"}, &out, &errb)
	if code == 0 || !strings.Contains(errb.String(), "current-state upgrade journal") {
		t.Fatalf("unreadable journal location not refused: code=%d\n%s", code, errb.String())
	}
}

func TestGuardMalformedJournalRefusesEveryMode(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
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
	ctx := testContext(t)
	_ = ctx
	// runUpgrade routes an attested lock into the final cutover verifier, which
	// rejects the bogus sealed facts rather than running a schema migration.
	root := scaffoldProject(t)
	attestLock(t, root)
	if err := runUpgrade(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "prepared head") {
		t.Fatalf("want seal verification, got %v", err)
	}
}

func TestValidJournalRecoveryRollsBackInterrupted(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// A precommit journal whose lock hash differs from the final hash rolls the
	// prepared write back to its prior image on recovery.
	root := scaffoldProject(t)
	newlineRoot := filepath.Join(t.TempDir(), "newline\nroot")
	if err := os.Rename(root, newlineRoot); err != nil {
		t.Fatal(err)
	}
	root = newlineRoot
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
	if journalPresence(t, root) {
		t.Fatal("journal residue after rollback")
	}
}
