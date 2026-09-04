package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

// preparePublicSyncLaterFailure makes public Publisher.Sync commit the earlier
// AGENTS.md mode correction before a later rendered output cannot be read.
func upgradeJournalPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(upgrade.JournalRel))
}

func snapshotUpgradeFixture(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == filepath.Join(root, ".git") && info.IsDir() {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = contents
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func assertUpgradeFixtureUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := snapshotUpgradeFixture(t, root)
	if len(after) != len(before) {
		t.Fatalf("fixture file count after refusal = %d, want %d: %#v", len(after), len(before), after)
	}
	for path, want := range before {
		if got, ok := after[path]; !ok || !bytes.Equal(got, want) {
			t.Fatalf("fixture file %s after refusal = %q, want byte-identical %q", path, got, want)
		}
	}
}

func preparePublicSyncLaterFailure(t *testing.T, root string) {
	t.Helper()
	if err := os.Chmod(filepath.Join(root, "AGENTS.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	unreadableOutput := filepath.Join(root, "CLAUDE.md")
	if err := os.Remove(unreadableOutput); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(unreadableOutput, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestPublicSyncPartialResultIsPresentedByEveryCommandBoundary(t *testing.T) {
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
		if got := stderr.String(); !strings.Contains(got, "committed effects: output-replaced AGENTS.md") || !strings.Contains(got, "publication partially committed") || !strings.Contains(got, "filesystem: read \"CLAUDE.md\"") || !strings.Contains(got, "recovery:") {
			t.Fatalf("upgrade failure diagnostic = %q, want complete committed effects and typed sync cause", got)
		}
	})

	t.Run("ordinary sync presents the real partial result", func(t *testing.T) {
		root := scaffoldProject(t)
		preparePublicSyncLaterFailure(t, root)
		var stdout bytes.Buffer
		if err := runSync(testContext(t), root, &stdout); err == nil {
			t.Fatal("ordinary sync accepted an unreplaceable later output")
		}
		if got := stdout.String(); !strings.Contains(got, "status: partially committed") || !strings.Contains(got, "output-replaced AGENTS.md") || !strings.Contains(got, "recovery:") {
			t.Fatalf("ordinary sync stdout = %q, want complete partial-effect presentation", got)
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
	if !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("error = %v, want below-floor refusal", err)
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
	if err := os.WriteFile(upgradeJournalPath(root), append(b, '\n'), 0o644); err != nil {
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

// TestGuardRefusesWhenProjectAuthorityIsUnreadable pins that the command-state
// guard refuses when project-presence inspection cannot complete. Reading the
// fault as absence would permit commands against an authority tree whose state
// was never established.
func TestGuardRefusesWhenJournalPresenceInspectionFails(t *testing.T) {
	root := scaffoldProject(t)
	journalPath := upgradeJournalPath(root)
	if err := os.Symlink(filepath.Base(journalPath), journalPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	var out, errb bytes.Buffer
	code := runAt(t, root, []string{"awf", "check"}, &out, &errb)
	if code == 0 || !strings.Contains(errb.String(), "current-state upgrade journal") {
		t.Fatalf("journal inspection failure not refused: code=%d\n%s", code, errb.String())
	}
}

func TestGuardRefusesWhenProjectAuthorityIsUnreadable(t *testing.T) {
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
	if code == 0 || !strings.Contains(errb.String(), "permission denied") {
		t.Fatalf("unreadable project authority not refused: code=%d\n%s", code, errb.String())
	}
}

func TestGuardMalformedJournalRefusesEveryMode(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	if err := os.WriteFile(upgradeJournalPath(root), []byte("{not json"), 0o644); err != nil {
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
	final := append(append([]byte{}, lockBytes...), []byte("\n# changed\n")...)
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
	if err := os.WriteFile(upgradeJournalPath(root), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runRecover(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if _, err := os.Stat(prepared); !os.IsNotExist(err) {
		t.Fatal("prepared.txt not rolled back")
	}
	if journalPresence(t, root) {
		t.Fatal("journal residue after rollback")
	}
}

func TestRunUpgradeLegacyAdopterRefusesBelowFloorWithoutMutation(t *testing.T) {
	ctx := testContext(t)
	// A retired single-file project is recognized only to produce the typed,
	// non-mutating unsupported-layout refusal.
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "prefix: example\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\n"
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: "0.1.0", Files: map[string]manifest.Entry{}}).Save(filepath.Join(claude, "awf.lock")); err != nil {
		t.Fatal(err)
	}
	before := snapshotUpgradeFixture(t, root)
	var out bytes.Buffer
	if err := runUpgrade(ctx, root, &out); !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("runUpgrade legacy = %v, want below-floor refusal", err)
	} else if !strings.Contains(err.Error(), "supports the retired layout") {
		t.Fatalf("legacy refusal presentation = %q, want recovery guidance", err)
	}
	assertUpgradeFixtureUnchanged(t, root, before)
}

// A schema-7 current-layout project is below the live floor and refuses before
// upgrade can mutate its authority or run terminal synchronization.
func TestRunUpgradeRefusesBelowFloorWithoutMutation(t *testing.T) {
	ctx := testContext(t)
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {gateCmd: make gate}\n")
	lock := &manifest.Lock{SchemaVersion: 7, Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(ctx, root, io.Discard); err == nil {
		t.Fatal("pre-upgrade check should refuse (schema gate)")
	}
	before := snapshotUpgradeFixture(t, root)
	var out bytes.Buffer
	if err := runUpgrade(ctx, root, &out); !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("runUpgrade = %v, want below-floor refusal", err)
	} else if !strings.Contains(err.Error(), "use a release that supports schema 7") {
		t.Fatalf("below-floor refusal presentation = %q, want recovery guidance", err)
	}
	assertUpgradeFixtureUnchanged(t, root, before)
}

func TestRunUpgradePresentsAheadAndPartialAuthorityRefusals(t *testing.T) {
	t.Run("schema ahead", func(t *testing.T) {
		root := scaffoldProject(t)
		lock, err := manifest.Load(config.LockPath(root))
		if err != nil {
			t.Fatal(err)
		}
		lock.SchemaVersion = migrate.Current() + 1
		if err := lock.Save(config.LockPath(root)); err != nil {
			t.Fatal(err)
		}
		err = runUpgrade(testContext(t), root, io.Discard)
		if !errors.Is(err, manifest.ErrUnsupportedLiveSource) || !strings.Contains(err.Error(), "update your pinned awf") {
			t.Fatalf("ahead refusal = %v", err)
		}
	})

	for _, tc := range []struct {
		name   string
		remove string
	}{
		{name: "config only", remove: ".awf/awf.lock"},
		{name: "lock only", remove: ".awf/config.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldProject(t)
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(tc.remove))); err != nil {
				t.Fatal(err)
			}
			err := runUpgrade(testContext(t), root, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "restore the complete .awf/config.yaml and .awf/awf.lock control pair") {
				t.Fatalf("partial refusal = %v", err)
			}
		})
	}
}

func TestRunUpgradeRendersSuccessfulFinalJournalMutation(t *testing.T) {
	root := scaffoldProject(t)
	var stdout bytes.Buffer
	if err := runUpgrade(testContext(t), root, &stdout); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "status: completed") {
		t.Fatalf("upgrade output = %q", stdout.String())
	}
}

// invariant: config/migrations-and-locks:skill-extraction-source-migration (TestRunUpgradeExtractsGenericSkillsAndPublishesFixedOutputs)
func TestRunUpgradeExtractsGenericSkillsAndPublishesFixedOutputs(t *testing.T) {
	root := scaffoldProject(t)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{".claude", ".pi"} {
		currentRel := target + "/skills/awf-maintenance/SKILL.md"
		retiredRel := target + "/skills/example-using-awf/SKILL.md"
		entry, found := lock.Files[currentRel]
		if !found {
			t.Fatalf("fixture lock lacks %s", currentRel)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(retiredRel))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(root, filepath.FromSlash(currentRel)), filepath.Join(root, filepath.FromSlash(retiredRel))); err != nil {
			t.Fatal(err)
		}
		delete(lock.Files, currentRel)
		entry.TemplateID = "skills/using-awf/SKILL.md.tmpl"
		lock.Files[retiredRel] = entry

		genericRel := target + "/skills/example-context/SKILL.md"
		genericBody := []byte("retired generic skill\n")
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(genericRel)), string(genericBody))
		lock.Files[genericRel] = manifest.Entry{TemplateID: "skills/context/SKILL.md.tmpl", OutputHash: manifest.Hash(genericBody), Mode: 0o644}
	}
	lock.SchemaVersion = 51
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}

	retainedPart := filepath.Join(root, ".awf/skills/parts/using-awf/generated-documents.md")
	testsupport.WriteFile(t, retainedPart, "Custom maintenance guidance.\n")
	if err := os.Chmod(retainedPart, 0o640); err != nil {
		t.Fatal(err)
	}
	removedPart := filepath.Join(root, ".awf/skills/parts/context/explore.md")
	testsupport.WriteFile(t, removedPart, "Custom generic exploration.\n")
	removedRolePart := filepath.Join(root, ".awf/agents/parts/reviewer/review.md")
	testsupport.WriteFile(t, removedRolePart, "Custom generic review role.\n")

	var stdout bytes.Buffer
	if err := runUpgrade(testContext(t), root, &stdout); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "renamed AWF skills and preserved extracted generic skill and role overrides") {
		t.Fatalf("upgrade output = %q", stdout.String())
	}
	lock, err = manifest.Load(config.LockPath(root))
	if err != nil || lock.SchemaVersion != migrate.Current() {
		t.Fatalf("final lock schema = %v, err=%v", lock, err)
	}
	currentPart := filepath.Join(root, ".awf/skills/parts/awf-maintenance/generated-documents.md")
	contents, err := os.ReadFile(currentPart)
	info, statErr := os.Stat(currentPart)
	if err != nil || statErr != nil || string(contents) != "Custom maintenance guidance.\n" || info.Mode().Perm() != 0o640 {
		t.Fatalf("migrated retained part = %q mode=%v errors=%v", contents, info, errors.Join(err, statErr))
	}
	if _, err := os.Stat(retainedPart); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired retained part remains: %v", err)
	}
	backup := removedPart + ".awf-bak"
	if contents, err := os.ReadFile(backup); err != nil || string(contents) != "Custom generic exploration.\n" {
		t.Fatalf("generic override backup = %q, %v", contents, err)
	}
	if _, err := os.Stat(removedPart); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed generic part remains: %v", err)
	}
	roleBackup := removedRolePart + ".awf-bak"
	if contents, err := os.ReadFile(roleBackup); err != nil || string(contents) != "Custom generic review role.\n" {
		t.Fatalf("generic role override backup = %q, %v", contents, err)
	}
	if _, err := os.Stat(removedRolePart); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed generic role part remains: %v", err)
	}
	for _, target := range []string{".claude", ".pi"} {
		current := filepath.Join(root, target, "skills/awf-maintenance/SKILL.md")
		rendered, err := os.ReadFile(current)
		if err != nil || !strings.Contains(string(rendered), "Custom maintenance guidance.") {
			t.Fatalf("rendered %s maintenance = %q, err=%v", target, rendered, err)
		}
		for _, retired := range []string{"skills/example-using-awf/SKILL.md", "skills/example-context/SKILL.md"} {
			if _, err := os.Stat(filepath.Join(root, target, retired)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("retired rendered output %s remains for %s: %v", retired, target, err)
			}
		}
	}
	if journalPresence(t, root) {
		t.Fatal("migration journal remains after successful upgrade")
	}
}

// invariant: config/migrations-and-locks:workflow-surface-source-migration (TestRunUpgradeRetiresCodeDesignGuideAndRenamesAdvancedWorkflowSection)
func TestRunUpgradeRetiresCodeDesignGuideAndRenamesAdvancedWorkflowSection(t *testing.T) {
	root := scaffoldProject(t)
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.SchemaVersion = 52
	managedGuide := []byte("managed old guide\n")
	guideRel := "docs/maintainable-code-design.md"
	lock.Files[guideRel] = manifest.Entry{TemplateID: "docs/maintainable-code-design.md.tmpl", OutputHash: manifest.Hash(managedGuide), Mode: 0o644}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	guideOutput := filepath.Join(root, filepath.FromSlash(guideRel))
	testsupport.WriteFile(t, guideOutput, "adopter-customized old guide\n")
	testsupport.WriteFile(t, guideOutput+".awf-bak", "prior output backup\n")

	guideSidecar := filepath.Join(root, ".awf/maintainable-code-design.yaml")
	guidePart := filepath.Join(root, ".awf/parts/maintainable-code-design/readability.md")
	oldSection := filepath.Join(root, ".awf/parts/working-with-awf/model-selection.md")
	workingSidecar := filepath.Join(root, ".awf/working-with-awf.yaml")
	testsupport.WriteFile(t, guideSidecar, "data:\n  custom: doctrine\n")
	testsupport.WriteFile(t, guideSidecar+".awf-bak", "prior source backup\n")
	testsupport.WriteFile(t, guidePart, "custom doctrine part\n")
	testsupport.WriteFile(t, oldSection, "obsolete model policy must not render\n")
	testsupport.WriteFile(t, workingSidecar, "data:\n  retained: yes\nsections:\n  model-selection:\n    drop: true\n  commands:\n    drop: false\n")

	var stdout bytes.Buffer
	if err := runUpgrade(testContext(t), root, &stdout); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, want := range []string{
		"retire duplicate code-design guide and rename advanced workflow section",
		".awf/maintainable-code-design.yaml.awf-bak.1",
		".awf/parts/maintainable-code-design/readability.md.awf-bak",
		".awf/parts/working-with-awf/model-selection.md.awf-bak",
		".awf/working-with-awf.yaml.awf-bak",
		"docs/maintainable-code-design.md.awf-bak.1",
		"remove its retired parent directory if empty",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("upgrade output omits %q:\n%s", want, output)
		}
	}
	if _, err := os.Stat(guideOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired guide output remains: %v", err)
	}
	outputBackup := guideOutput + ".awf-bak.1"
	if contents, err := os.ReadFile(outputBackup); err != nil || string(contents) != "adopter-customized old guide\n" {
		t.Fatalf("retired customized output backup = %q err=%v", contents, err)
	}
	working, err := os.ReadFile(filepath.Join(root, "docs/working-with-awf.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(working), "## Advanced workflow") || strings.Contains(string(working), "obsolete model policy must not render") {
		t.Fatalf("renamed section output = %q", working)
	}
	cleanedSidecar, err := os.ReadFile(workingSidecar)
	if err != nil || !strings.Contains(string(cleanedSidecar), "retained: yes") || !strings.Contains(string(cleanedSidecar), "commands:") || strings.Contains(string(cleanedSidecar), "model-selection") || strings.Contains(string(cleanedSidecar), "advanced-workflow") {
		t.Fatalf("cleaned working sidecar = %q err=%v", cleanedSidecar, err)
	}
	if err := runCheck(testContext(t), root, io.Discard); err == nil {
		t.Fatal("check unexpectedly accepted named migration backups before operator review")
	}
	for _, backup := range []string{
		guideSidecar + ".awf-bak", guideSidecar + ".awf-bak.1", guidePart + ".awf-bak", oldSection + ".awf-bak",
		workingSidecar + ".awf-bak", guideOutput + ".awf-bak", outputBackup,
	} {
		if err := os.Remove(backup); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Dir(guidePart)); err != nil {
		t.Fatalf("remove reviewed empty retired source directory: %v", err)
	}
	if err := runCheck(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("reviewed schema-53 upgrade did not converge: %v", err)
	}
}

func TestSchema47JournalRecoveryRestoresLegacyAuthorityThenRefuses(t *testing.T) {
	// The journal format remains able to recover the last retired source shape,
	// even though normal operations no longer migrate it. Recovery must restore
	// its exact bytes before the normal live-floor admission refuses schema 47.
	writeJournal := func(t *testing.T, root, phase string, priorConfig, finalConfig, priorLock, finalLock []byte) {
		t.Helper()
		journal := upgrade.Journal{
			Version:         upgrade.JournalVersion,
			Phase:           phase,
			FinalLockSHA256: fmt.Sprintf("%x", sha256.Sum256(finalLock)),
			Operations: []upgrade.Operation{
				{Path: ".awf/config.yaml", Prior: upgrade.Image{Present: true, Mode: 0o644, Content: priorConfig}, Replacement: upgrade.Image{Present: true, Mode: 0o644, Content: finalConfig}},
				{Path: upgrade.LockRel(), Prior: upgrade.Image{Present: true, Mode: 0o644, Content: priorLock}, Replacement: upgrade.Image{Present: true, Mode: 0o644, Content: finalLock}},
			},
		}
		encoded, err := json.MarshalIndent(journal, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(upgradeJournalPath(root), append(encoded, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	legacy := func(t *testing.T) (legacyConfig, finalConfig, legacyLock, finalLock []byte) {
		t.Helper()
		// This is a real schema-47-to-50 image shape from v0.44: schema 47 still
		// allowed the workflow profile and retired empty workflow variable, while
		// schema 50 removes both and retains the rest of the current config.
		legacyConfig = []byte("prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  activeMdRegenCmd: \"\"\n")
		finalConfig = []byte("prefix: example\nintegrationBranch: main\nvars: {}\n")
		var err error
		legacyLock, err = (&manifest.Lock{AWFVersion: "0.40.0", SchemaVersion: 47, Files: map[string]manifest.Entry{"prior": {}}}).Marshal()
		if err != nil {
			t.Fatal(err)
		}
		finalLock, err = (&manifest.Lock{AWFVersion: "0.44.0", SchemaVersion: 50, Files: map[string]manifest.Entry{"prior": {}}}).Marshal()
		if err != nil {
			t.Fatal(err)
		}
		return legacyConfig, finalConfig, legacyLock, finalLock
	}

	t.Run("pre-commit restores schema-47 bytes then normal upgrade refuses", func(t *testing.T) {
		root := scaffoldProject(t)
		legacyConfig, finalConfig, legacyLock, finalLock := legacy(t)
		testsupport.WriteFile(t, config.ConfigPath(root), string(finalConfig)) // migration image landed
		testsupport.WriteFile(t, config.LockPath(root), string(legacyLock))    // lock did not
		writeJournal(t, root, "applying", legacyConfig, finalConfig, legacyLock, finalLock)

		if err := runRecover(testContext(t), root, io.Discard); err != nil {
			t.Fatalf("recover pre-commit schema-47 journal: %v", err)
		}
		if got, err := os.ReadFile(config.ConfigPath(root)); err != nil || !bytes.Equal(got, legacyConfig) {
			t.Fatalf("restored config = %q, err=%v; want exact schema-47 bytes", got, err)
		}
		if got, err := os.ReadFile(config.LockPath(root)); err != nil || !bytes.Equal(got, legacyLock) {
			t.Fatalf("restored lock = %q, err=%v; want exact schema-47 bytes", got, err)
		}
		if journalPresence(t, root) {
			t.Fatal("pre-commit journal residue after recovery")
		}
		if err := runUpgrade(testContext(t), root, io.Discard); !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
			t.Fatalf("normal operation after schema-47 recovery = %v, want below-floor refusal", err)
		}
	})

	t.Run("post-commit keeps schema-50 authority and removes residue", func(t *testing.T) {
		root := scaffoldProject(t)
		legacyConfig, finalConfig, legacyLock, finalLock := legacy(t)
		testsupport.WriteFile(t, config.ConfigPath(root), string(finalConfig))
		testsupport.WriteFile(t, config.LockPath(root), string(finalLock))
		writeJournal(t, root, "lock-committed", legacyConfig, finalConfig, legacyLock, finalLock)

		if err := runRecover(testContext(t), root, io.Discard); err != nil {
			t.Fatalf("recover post-commit schema-50 journal: %v", err)
		}
		lock, err := manifest.Load(config.LockPath(root))
		if err != nil {
			t.Fatalf("load post-commit lock: %v", err)
		}
		if lock.SchemaVersion != 50 {
			t.Fatalf("post-commit lock schema = %d, want 50", lock.SchemaVersion)
		}
		if got, err := os.ReadFile(config.ConfigPath(root)); err != nil || !bytes.Equal(got, finalConfig) {
			t.Fatalf("post-commit config = %q, err=%v; want unchanged schema-50 bytes", got, err)
		}
		if got, err := os.ReadFile(config.LockPath(root)); err != nil || !bytes.Equal(got, finalLock) {
			t.Fatalf("post-commit lock = %q, err=%v; want unchanged schema-50 bytes", got, err)
		}
		if journalPresence(t, root) {
			t.Fatal("post-commit journal residue after recovery")
		}
	})
}

func TestRunUpgradeRetiredLayoutDoesNotDecodeMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".claude", "awf.yaml"), ": : malformed")
	before := snapshotUpgradeFixture(t, root)
	err := runUpgrade(testContext(t), root, io.Discard)
	if !errors.Is(err, manifest.ErrUnsupportedLiveSource) || !strings.Contains(err.Error(), "retired layout") {
		t.Fatalf("retired layout error = %v", err)
	}
	assertUpgradeFixtureUnchanged(t, root, before)
}

// invariant: tooling/cli:upgrade-always-syncs (TestRunUpgradeAlreadyCurrentStillSyncs)
func TestRunUpgradeAlreadyCurrentStillSyncs(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runUpgrade(ctx, root, &out); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if !strings.Contains(out.String(), "migration changes:\n      config schema already current") {
		t.Errorf("expected the schema-current fact, got %q", out.String())
	}
	// The zero-migrations path must still sync: a same-schema binary bump
	// re-renders every managed file and re-pins the bootstrap (ADR-0085).
	if !strings.Contains(out.String(), "status: completed") || !strings.Contains(out.String(), "continue with the rendered project state") {
		t.Errorf("expected the schema-current upgrade to run a sync, got %q", out.String())
	}
}

func TestRunRecoverHonorsCommandContextWhileWaitingForLease(t *testing.T) {
	root := scaffoldProject(t)
	lease, err := filesystem.AcquireTrackedLease(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Release(); err != nil {
			t.Errorf("release recover test lease: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(testContext(t), 50*time.Millisecond)
	defer cancel()
	err = runRecover(ctx, root, io.Discard)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("recover lease error = %v, want command deadline", err)
	}
}
