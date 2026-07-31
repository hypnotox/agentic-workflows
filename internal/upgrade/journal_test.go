package upgrade

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestJournalPresentSeparatesAbsenceFromFault pins that an unreadable journal
// location is reported as a fault rather than as absence. Folding the two
// together told the command-state guard there was no journal to recover from,
// so it permitted exactly the commands an unrecovered upgrade must block.
func TestJournalPresentSeparatesAbsenceFromFault(t *testing.T) {
	root := t.TempDir()
	found, err := JournalPresent(root)
	if found || err != nil {
		t.Fatalf("absent journal: found=%v err=%v, want false and no error", found, err)
	}

	awfDir := filepath.Join(root, ".awf")
	mustMkdir(t, awfDir)
	if err := os.WriteFile(JournalPath(root), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if found, err := JournalPresent(root); !found || err != nil {
		t.Fatalf("present journal: found=%v err=%v, want true and no error", found, err)
	}

	if err := os.Chmod(awfDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(awfDir, 0o755) })
	if found, err := JournalPresent(root); err == nil || found {
		t.Fatalf("unreadable journal location: found=%v err=%v, want a fault", found, err)
	}
}

// journalPresence answers JournalPresent for the tests that assert presence or
// absence and expect no fault reading it.
func journalPresence(t *testing.T, root string) bool {
	t.Helper()
	found, err := JournalPresent(root)
	if err != nil {
		t.Fatalf("JournalPresent(%s): %v", root, err)
	}
	return found
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRawJournal(t *testing.T, root string, j Journal) {
	t.Helper()
	mustMkdir(t, filepath.Join(root, ".awf"))
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, JournalPath(root), append(b, '\n'))
}

func presentImg(content string) Image {
	return Image{Present: true, Mode: 0o644, Content: []byte(content)}
}

// lockJournal builds a valid two-op journal (a.txt then the lock) in the given
// phase, with the fixed final lock content "FINAL".
func lockJournal(phase string) Journal {
	lockFinal := presentImg("FINAL")
	return Journal{
		Version:         JournalVersion,
		Phase:           phase,
		FinalLockSHA256: imageSHA(lockFinal),
		Operations: []Operation{
			{Path: "a.txt", Prior: Image{}, Replacement: presentImg("new")},
			{Path: LockRel(), Prior: presentImg("old-lock"), Replacement: lockFinal},
		},
	}
}

func TestJournalLoadRejections(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".awf"))
	mustWrite(t, JournalPath(root), []byte("{not json"))
	if _, err := LoadJournal(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed json: %v", err)
	}
	mustWrite(t, JournalPath(root), []byte(`{"version":1,"phase":"prepared","operations":[],"extra":1}`))
	if _, err := LoadJournal(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("unknown field: %v", err)
	}
	for _, tc := range []struct {
		name string
		mut  func(*Journal)
		want string
	}{
		{"version", func(j *Journal) { j.Version = 2 }, "version"},
		{"phase", func(j *Journal) { j.Phase = "bogus" }, "phase"},
		{"unsafe", func(j *Journal) { j.Operations[0].Path = "../escape" }, "unsafe"},
		{"unsorted", func(j *Journal) {
			j.Operations = []Operation{
				{Path: "b.txt", Replacement: presentImg("b")},
				{Path: "a.txt", Replacement: presentImg("a")},
				{Path: LockRel(), Prior: presentImg("old-lock"), Replacement: presentImg("FINAL")},
			}
		}, "sorted"},
		{"lock-not-last", func(j *Journal) {
			j.Operations = []Operation{
				{Path: LockRel(), Replacement: presentImg("FINAL")},
				{Path: LockRel(), Prior: presentImg("old-lock"), Replacement: presentImg("FINAL")},
			}
		}, "only last"},
		{"no-lock-last", func(j *Journal) { j.Operations = j.Operations[:1] }, "lock operation"},
		{"present-mode-zero", func(j *Journal) { j.Operations[0].Replacement = Image{Present: true, Mode: 0, Content: []byte("x")} }, "invalid mode"},
		{"present-mode-high", func(j *Journal) {
			j.Operations[0].Replacement = Image{Present: true, Mode: 0o4000, Content: []byte("x")}
		}, "invalid mode"},
		{"absent-with-content", func(j *Journal) { j.Operations[0].Prior = Image{Present: false, Content: []byte("x")} }, "carries"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := lockJournal(phasePrepared)
			tc.mut(&j)
			r := t.TempDir()
			writeRawJournal(t, r, j)
			if _, err := LoadJournal(r); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	if _, err := LoadJournal(t.TempDir()); err == nil {
		t.Fatal("missing journal accepted")
	}
}

func TestJournalCommitHappyPath(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".awf"))
	ops := []Operation{
		{Path: "a.txt", Prior: Image{}, Replacement: presentImg("alpha")},
		{Path: LockRel(), Prior: Image{}, Replacement: presentImg("lock-final")},
	}
	var log bytes.Buffer
	if err := commitTransaction(root, ops, &log); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(got) != "alpha" {
		t.Fatalf("a.txt: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, LockRel())); string(got) != "lock-final" {
		t.Fatalf("lock: %q", got)
	}
	if journalPresence(t, root) {
		t.Fatal("journal residue after success")
	}
	for _, want := range []string{"operation: applied a.txt", "operation: applied .awf/awf.lock", "operation: upgrade committed"} {
		if !strings.Contains(log.String(), want) {
			t.Fatalf("log missing %q: %s", want, log.String())
		}
	}
}

func TestJournalCommitRollsBackOnApplyFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".awf"))
	mustWrite(t, filepath.Join(root, "a.txt"), []byte("original"))
	mustMkdir(t, filepath.Join(root, "ro"))
	if err := os.Chmod(filepath.Join(root, "ro"), 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(filepath.Join(root, "ro"), 0o755) }()
	ops := []Operation{
		{Path: "a.txt", Prior: presentImg("original"), Replacement: presentImg("changed")},
		{Path: "ro/new.txt", Prior: Image{}, Replacement: presentImg("blocked")},
		{Path: LockRel(), Prior: Image{}, Replacement: presentImg("lock-final")},
	}
	var log bytes.Buffer
	if err := commitTransaction(root, ops, &log); err == nil || !strings.Contains(err.Error(), "ro/new.txt") {
		t.Fatalf("want apply failure, got %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(got) != "original" {
		t.Fatalf("a.txt not restored: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, LockRel())); !os.IsNotExist(err) {
		t.Fatal("lock written despite rollback")
	}
	if journalPresence(t, root) {
		t.Fatal("journal residue after rollback")
	}
	if !strings.Contains(log.String(), "operation: restored") || !strings.Contains(log.String(), "operation: rolled back") {
		t.Fatalf("rollback log: %s", log.String())
	}
}

func TestJournalRecoverTable(t *testing.T) {
	t.Run("precommit-rolls-back", func(t *testing.T) {
		root := t.TempDir()
		writeRawJournal(t, root, lockJournal(phaseApplying))
		mustWrite(t, filepath.Join(root, "a.txt"), []byte("new"))
		mustWrite(t, filepath.Join(root, LockRel()), []byte("old-lock"))
		var log bytes.Buffer
		if err := Recover(root, &log); err != nil {
			t.Fatalf("recover: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "a.txt")); !os.IsNotExist(err) {
			t.Fatal("a.txt not rolled back")
		}
		if journalPresence(t, root) {
			t.Fatal("journal residue")
		}
		if err := Recover(root, io.Discard); err == nil {
			t.Fatal("second recovery with no journal accepted")
		}
	})
	t.Run("precommit-lock-already-final", func(t *testing.T) {
		root := t.TempDir()
		writeRawJournal(t, root, lockJournal(phasePrepared))
		mustWrite(t, filepath.Join(root, LockRel()), []byte("FINAL"))
		mustWrite(t, filepath.Join(root, "a.txt"), []byte("new"))
		if err := Recover(root, io.Discard); err != nil {
			t.Fatalf("recover: %v", err)
		}
		if journalPresence(t, root) {
			t.Fatal("journal residue")
		}
		if got, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(got) != "new" {
			t.Fatal("cleanup rolled the tree back")
		}
	})
	t.Run("lock-committed-final", func(t *testing.T) {
		root := t.TempDir()
		writeRawJournal(t, root, lockJournal(phaseLockCommitted))
		mustWrite(t, filepath.Join(root, LockRel()), []byte("FINAL"))
		if err := Recover(root, io.Discard); err != nil {
			t.Fatalf("recover: %v", err)
		}
		if journalPresence(t, root) {
			t.Fatal("journal residue")
		}
	})
	t.Run("lock-committed-mismatch-refuses", func(t *testing.T) {
		root := t.TempDir()
		writeRawJournal(t, root, lockJournal(phaseLockCommitted))
		mustWrite(t, filepath.Join(root, LockRel()), []byte("DIFFERENT"))
		if err := Recover(root, io.Discard); err == nil || !strings.Contains(err.Error(), "refusing to roll committed authority back") {
			t.Fatalf("want refusal, got %v", err)
		}
		if !journalPresence(t, root) {
			t.Fatal("journal cleared despite refusal")
		}
	})
	t.Run("third-party-preserves-journal", func(t *testing.T) {
		root := t.TempDir()
		writeRawJournal(t, root, lockJournal(phaseApplying))
		mustWrite(t, filepath.Join(root, "a.txt"), []byte("TAMPERED"))
		mustWrite(t, filepath.Join(root, LockRel()), []byte("old-lock"))
		if err := Recover(root, io.Discard); err == nil || !strings.Contains(err.Error(), "a.txt") {
			t.Fatalf("want third-party halt, got %v", err)
		}
		if !journalPresence(t, root) {
			t.Fatal("journal cleared despite third-party edit")
		}
	})
}

func TestJournalHelpers(t *testing.T) {
	if !imagesEqual(Image{}, Image{}) || imagesEqual(Image{Present: true, Mode: 0o644}, Image{Present: true, Mode: 0o600}) {
		t.Fatal("imagesEqual")
	}
	if imagesEqual(Image{Present: true}, Image{Present: false}) {
		t.Fatal("presence mismatch")
	}
	root := t.TempDir()
	img, err := imageOf(root, "absent")
	if err != nil || img.Present {
		t.Fatalf("absent image: %#v %v", img, err)
	}
	mustMkdir(t, filepath.Join(root, "adir"))
	if _, err := imageOf(root, "adir"); err == nil {
		t.Fatal("directory imaged as a file")
	}
	mustWrite(t, filepath.Join(root, "adir", "child"), []byte("x"))
	if err := applyImage(root, "adir", Image{Present: false}); err == nil {
		t.Fatal("non-empty directory removed as absent image")
	}
	if journalPresence(t, root) {
		t.Fatal("phantom journal")
	}
	if safeRelPath("") || safeRelPath("/abs") || safeRelPath("a/../b") || !safeRelPath("a/b.txt") {
		t.Fatal("safeRelPath")
	}
	empty := t.TempDir()
	mustMkdir(t, filepath.Join(empty, ".awf"))
	mustWrite(t, JournalPath(empty), []byte(`{"version":1,"phase":"prepared","finalLockSHA256":"","operations":[]}`))
	if _, err := LoadJournal(empty); err == nil || !strings.Contains(err.Error(), "no operations") {
		t.Fatalf("empty ops: %v", err)
	}
}

func TestJournalCommitPreparedWriteFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	mustMkdir(t, awf)
	if err := os.Chmod(awf, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(awf, 0o755) }()
	ops := []Operation{{Path: LockRel(), Replacement: presentImg("final")}}
	if err := commitTransaction(root, ops, io.Discard); err == nil {
		t.Fatal("commit succeeded despite an unwritable journal directory")
	}
}

// treeOp builds a resident-tree quarantine operation for path, sending it to
// the given leaf name under the dedicated quarantine root.
func treeOp(path, leaf string) Operation {
	return Operation{Path: path, Kind: KindResidentTree, Quarantine: QuarantineRel() + "/" + leaf}
}

// residentJournal builds the canonical mixed transaction: one tracked file
// replacement, one quarantined resident leaf, one quarantined resident tree,
// and the lock last. The three non-lock paths are already sorted.
func residentJournal(phase string) Journal {
	lockFinal := presentImg("FINAL")
	return Journal{
		Version:         JournalVersion,
		Phase:           phase,
		FinalLockSHA256: imageSHA(lockFinal),
		Operations: []Operation{
			{Path: ".awf/config.yaml", Prior: presentImg("old-config"), Replacement: presentImg("new-config")},
			treeOp(".awf/efforts/legacy.json", "efforts-legacy.json"),
			treeOp(".awf/memory", "memory"),
			{Path: LockRel(), Prior: presentImg("old-lock"), Replacement: lockFinal},
		},
	}
}

// seedResidents materializes the residents residentJournal quarantines: a
// single-file leaf and a tree with a descendant, plus the pre-transaction
// config and lock bytes.
func seedResidents(t *testing.T, root string) {
	t.Helper()
	mustMkdir(t, filepath.Join(root, ".awf", "efforts"))
	mustMkdir(t, filepath.Join(root, ".awf", "memory"))
	mustWrite(t, filepath.Join(root, ".awf", "efforts", "legacy.json"), []byte(`{"schemaVersion":1}`))
	mustWrite(t, filepath.Join(root, ".awf", "memory", "notes.md"), []byte("standalone"))
	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), []byte("old-config"))
	mustWrite(t, filepath.Join(root, LockRel()), []byte("old-lock"))
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func TestJournalResidentKindRejections(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*Journal)
		want string
	}{
		{"unknown-kind", func(j *Journal) { j.Operations[1].Kind = "resident-file" }, "unknown operation kind"},
		{"file-carries-quarantine", func(j *Journal) {
			j.Operations[0].Quarantine = QuarantineRel() + "/config"
		}, "a file operation carries quarantine"},
		{"tree-carries-prior-image", func(j *Journal) { j.Operations[2].Prior = presentImg("bytes") }, "carries file images"},
		{"tree-carries-replacement-image", func(j *Journal) {
			j.Operations[2].Replacement = presentImg("bytes")
		}, "carries file images"},
		{"unsafe-quarantine", func(j *Journal) { j.Operations[2].Quarantine = "../outside" }, "unsafe quarantine path"},
		{"empty-quarantine", func(j *Journal) { j.Operations[2].Quarantine = "" }, "unsafe quarantine path"},
		{"unconfined-quarantine", func(j *Journal) { j.Operations[2].Quarantine = ".awf/elsewhere/memory" }, "is outside"},
		{"quarantine-root-itself", func(j *Journal) { j.Operations[2].Quarantine = QuarantineRel() }, "is outside"},
		{"duplicate-quarantine", func(j *Journal) {
			j.Operations[2].Quarantine = j.Operations[1].Quarantine
		}, "duplicate quarantine destination"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := residentJournal(phasePrepared)
			tc.mut(&j)
			root := t.TempDir()
			writeRawJournal(t, root, j)
			if _, err := LoadJournal(root); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	// The canonical mixed transaction itself must load: the resident operations
	// take part in the same sorted run as the file operation, and the lock stays
	// the distinguished last entry.
	root := t.TempDir()
	writeRawJournal(t, root, residentJournal(phasePrepared))
	if _, err := LoadJournal(root); err != nil {
		t.Fatalf("canonical resident journal rejected: %v", err)
	}
}

// lockWatcher reads the on-disk lock at the moment each operation line is
// emitted. The log alone cannot prove when the commit point landed, because a
// transaction that wrote the lock early would still print that line in its
// usual place; only the bytes on disk at each step show the real order.
type lockWatcher struct {
	root  string
	lines []string
	locks []string
}

func (w *lockWatcher) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimSuffix(string(p), "\n"), "\n") {
		if line == "" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(w.root, filepath.FromSlash(LockRel())))
		if err != nil && !os.IsNotExist(err) {
			return 0, err
		}
		w.lines = append(w.lines, line)
		w.locks = append(w.locks, string(raw))
	}
	return len(p), nil
}

// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestJournalResidentCommitQuarantinesThenDiscards(t *testing.T) {
	root := t.TempDir()
	seedResidents(t, root)
	j := residentJournal(phasePrepared)
	log := &lockWatcher{root: root}
	if err := commitTransaction(root, j.Operations, log); err != nil {
		t.Fatalf("commit: %v", err)
	}
	for _, gone := range []string{".awf/efforts/legacy.json", ".awf/memory", QuarantineRel()} {
		if exists(t, filepath.Join(root, filepath.FromSlash(gone))) {
			t.Fatalf("%s survived the committed transaction", gone)
		}
	}
	// The resident root that merely held a quarantined leaf is untouched.
	if !exists(t, filepath.Join(root, ".awf", "efforts")) {
		t.Fatal("the efforts resident root was removed with its leaf")
	}
	if got, _ := os.ReadFile(filepath.Join(root, LockRel())); string(got) != "FINAL" {
		t.Fatalf("lock: %q", got)
	}
	// The order is the contract, not just the membership.
	want := []string{
		"operation: applied .awf/config.yaml",
		"operation: applied .awf/efforts/legacy.json",
		"operation: applied .awf/memory",
		"operation: applied .awf/awf.lock",
		"operation: discarded .awf/efforts/legacy.json",
		"operation: discarded .awf/memory",
		"operation: upgrade committed",
	}
	if !slices.Equal(log.lines, want) {
		t.Fatalf("operation sequence:\ngot  %q\nwant %q", log.lines, want)
	}
	// The lock replacement is the commit point, so it must not reach the disk
	// until every resident has been quarantined. Writing it any earlier would let
	// a crash seal the new generation with the residents still in place, and
	// recovery would then take the committed branch and delete the journal
	// without ever resetting them: an unrecoverable half-migration. Watching the
	// bytes rather than the log is what makes that unbuildable, because a
	// transaction that wrote the lock first would still log it last.
	commit := slices.Index(want, "operation: applied "+LockRel())
	for i, line := range log.lines {
		got, expect := log.locks[i], "FINAL"
		if i < commit {
			expect = "old-lock"
		}
		if got != expect {
			t.Fatalf("lock was %q at %q (step %d of %d); the commit point is step %d", got, line, i, len(log.lines), commit)
		}
	}
	if journalPresence(t, root) {
		t.Fatal("journal residue after success")
	}
}

// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestJournalResidentRollbackRestoresQuarantinedBytes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	seedResidents(t, root)
	mustMkdir(t, filepath.Join(root, "ro"))
	if err := os.Chmod(filepath.Join(root, "ro"), 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(filepath.Join(root, "ro"), 0o755) }()
	// Sorted after both resident operations, so both renames happen first and
	// must be undone in reverse when this operation fails.
	ops := make([]Operation, 0, 5)
	ops = append(ops, residentJournal(phasePrepared).Operations[:3]...)
	ops = append(ops,
		Operation{Path: "ro/new.txt", Prior: Image{}, Replacement: presentImg("blocked")},
		Operation{Path: LockRel(), Prior: presentImg("old-lock"), Replacement: presentImg("FINAL")},
	)
	var log bytes.Buffer
	if err := commitTransaction(root, ops, &log); err == nil || !strings.Contains(err.Error(), "ro/new.txt") {
		t.Fatalf("want apply failure, got %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".awf", "memory", "notes.md")); string(got) != "standalone" {
		t.Fatalf("quarantined tree not restored: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".awf", "efforts", "legacy.json")); string(got) != `{"schemaVersion":1}` {
		t.Fatalf("quarantined leaf not restored: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".awf", "config.yaml")); string(got) != "old-config" {
		t.Fatalf("config not restored: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, LockRel())); string(got) != "old-lock" {
		t.Fatalf("lock replaced despite rollback: %q", got)
	}
	if exists(t, filepath.Join(root, filepath.FromSlash(QuarantineRel()))) {
		t.Fatal("quarantine residue after rollback")
	}
	if journalPresence(t, root) {
		t.Fatal("journal residue after rollback")
	}
}

// TestJournalResidentInterruption pins every crash point around the resident
// renames. Each case materializes the exact tree an interruption would leave
// and requires `awf upgrade --recover` to converge, twice.
// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestJournalResidentInterruption(t *testing.T) {
	const (
		leafRel  = ".awf/efforts/legacy.json"
		treeRel  = ".awf/memory"
		leafQrel = ".awf/.upgrade-quarantine/efforts-legacy.json"
		treeQrel = ".awf/.upgrade-quarantine/memory"
	)
	// quarantine moves a seeded resident to its quarantine destination, standing
	// in for a rename the interrupted run had already completed.
	quarantine := func(t *testing.T, root, from, to string) {
		t.Helper()
		mustMkdir(t, filepath.Join(root, filepath.FromSlash(QuarantineRel())))
		if err := os.Rename(filepath.Join(root, filepath.FromSlash(from)), filepath.Join(root, filepath.FromSlash(to))); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		// phase is the journal phase the interrupted run left behind.
		phase string
		// lockFinal writes the committed lock image before recovery.
		lockFinal bool
		setup     func(t *testing.T, root string)
		// wantResidents is true when recovery must leave both residents at their
		// original paths, false when it must leave them discarded.
		wantResidents bool
	}{
		{
			name:          "before-any-rename",
			phase:         phaseApplying,
			setup:         func(*testing.T, string) {},
			wantResidents: true,
		},
		{
			name:  "after-first-rename",
			phase: phaseApplying,
			setup: func(t *testing.T, root string) {
				quarantine(t, root, leafRel, leafQrel)
			},
			wantResidents: true,
		},
		{
			name:  "after-every-rename",
			phase: phaseApplying,
			setup: func(t *testing.T, root string) {
				quarantine(t, root, leafRel, leafQrel)
				quarantine(t, root, treeRel, treeQrel)
			},
			wantResidents: true,
		},
		{
			name:  "after-rename-before-lock-replacement",
			phase: phaseRollingBack,
			setup: func(t *testing.T, root string) {
				quarantine(t, root, leafRel, leafQrel)
				quarantine(t, root, treeRel, treeQrel)
			},
			wantResidents: true,
		},
		{
			// The lock landed before the phase advanced: authority is committed,
			// so recovery discards rather than restores.
			name:      "after-lock-replacement-before-phase-write",
			phase:     phaseApplying,
			lockFinal: true,
			setup: func(t *testing.T, root string) {
				quarantine(t, root, leafRel, leafQrel)
				quarantine(t, root, treeRel, treeQrel)
			},
		},
		{
			name:      "after-phase-write-before-cleanup",
			phase:     phaseLockCommitted,
			lockFinal: true,
			setup: func(t *testing.T, root string) {
				quarantine(t, root, leafRel, leafQrel)
				quarantine(t, root, treeRel, treeQrel)
			},
		},
		{
			// Cleanup had already discarded one tree when it was interrupted.
			name:      "after-partial-cleanup",
			phase:     phaseLockCommitted,
			lockFinal: true,
			setup: func(t *testing.T, root string) {
				quarantine(t, root, leafRel, leafQrel)
				quarantine(t, root, treeRel, treeQrel)
				if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(leafQrel))); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			seedResidents(t, root)
			tc.setup(t, root)
			if tc.lockFinal {
				mustWrite(t, filepath.Join(root, LockRel()), []byte("FINAL"))
				mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), []byte("new-config"))
			}
			writeRawJournal(t, root, residentJournal(tc.phase))
			var log bytes.Buffer
			if err := Recover(root, &log); err != nil {
				t.Fatalf("recover: %v", err)
			}
			leaf := filepath.Join(root, filepath.FromSlash(leafRel))
			tree := filepath.Join(root, filepath.FromSlash(treeRel))
			if exists(t, leaf) != tc.wantResidents || exists(t, tree) != tc.wantResidents {
				t.Fatalf("residents present=%v/%v, want %v", exists(t, leaf), exists(t, tree), tc.wantResidents)
			}
			if tc.wantResidents {
				if got, _ := os.ReadFile(filepath.Join(tree, "notes.md")); string(got) != "standalone" {
					t.Fatalf("restored tree lost its descendant: %q", got)
				}
			}
			if exists(t, filepath.Join(root, filepath.FromSlash(QuarantineRel()))) {
				t.Fatal("quarantine residue after recovery")
			}
			if journalPresence(t, root) {
				t.Fatal("journal residue after recovery")
			}
			// `awf upgrade --recover` is idempotent in effect: a second run has
			// no journal to act on and must not mutate the converged tree.
			if err := Recover(root, io.Discard); err == nil {
				t.Fatal("second recovery without a journal accepted")
			}
			if exists(t, leaf) != tc.wantResidents || exists(t, tree) != tc.wantResidents {
				t.Fatal("second recovery mutated the converged tree")
			}
		})
	}
}

// TestJournalResidentRenameHelpers pins the two rename helpers directly: both
// treat an absent source as already-converged so a restarted run makes
// progress, and both surface an unreadable source rather than silently
// treating it as absent.
func TestJournalResidentRenameHelpers(t *testing.T) {
	op := treeOp(".awf/efforts/legacy.json", "efforts-legacy.json")
	t.Run("absent-source-converges", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, ".awf", "efforts"))
		if err := quarantineTree(root, op); err != nil {
			t.Fatalf("quarantine an absent resident: %v", err)
		}
		if exists(t, filepath.Join(root, filepath.FromSlash(QuarantineRel()))) {
			t.Fatal("quarantine root created for an absent resident")
		}
		if err := restoreTree(root, op); err != nil {
			t.Fatalf("restore an absent quarantine: %v", err)
		}
		if exists(t, filepath.Join(root, filepath.FromSlash(op.Path))) {
			t.Fatal("resident recreated from an absent quarantine")
		}
	})
	t.Run("unreadable-source-refuses", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		root := t.TempDir()
		sealed := []string{filepath.Join(root, ".awf", "efforts"), filepath.Join(root, filepath.FromSlash(QuarantineRel()))}
		for _, dir := range sealed {
			mustMkdir(t, dir)
		}
		for _, dir := range sealed {
			if err := os.Chmod(dir, 0o000); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Chmod(dir, 0o755) }()
		}
		if err := quarantineTree(root, op); err == nil {
			t.Fatal("unreadable resident quarantined")
		}
		if err := restoreTree(root, op); err == nil {
			t.Fatal("unreadable quarantine restored")
		}
	})
}

// TestJournalResidentAbsentResidentCommits pins the whole transaction for the
// ordinary adopter that simply has no such resident: the operation converges,
// still reports itself, and the lock still commits.
func TestJournalResidentAbsentResidentCommits(t *testing.T) {
	root := t.TempDir()
	seedResidents(t, root)
	if err := os.RemoveAll(filepath.Join(root, ".awf", "memory")); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if err := commitTransaction(root, residentJournal(phasePrepared).Operations, &log); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, LockRel())); string(got) != "FINAL" {
		t.Fatalf("lock: %q", got)
	}
	if exists(t, filepath.Join(root, filepath.FromSlash(QuarantineRel()))) {
		t.Fatal("quarantine residue")
	}
	for _, want := range []string{"operation: applied .awf/memory", "operation: discarded .awf/memory"} {
		if !strings.Contains(log.String(), want) {
			t.Fatalf("log missing %q: %s", want, log.String())
		}
	}
}

// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestJournalResidentCollisionRefusals(t *testing.T) {
	t.Run("occupied-quarantine-destination", func(t *testing.T) {
		root := t.TempDir()
		seedResidents(t, root)
		// A previous interrupted run already put something at the destination;
		// overwriting it would destroy the only copy of those bytes.
		mustMkdir(t, filepath.Join(root, filepath.FromSlash(QuarantineRel())))
		mustWrite(t, filepath.Join(root, filepath.FromSlash(QuarantineRel()), "efforts-legacy.json"), []byte("earlier"))
		ops := residentJournal(phasePrepared).Operations
		err := commitTransaction(root, ops, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "quarantine destination") || !strings.Contains(err.Error(), gitRestorationGuidance) {
			t.Fatalf("want a quarantine-collision refusal, got %v", err)
		}
		// Rolling this operation back would have to choose between the occupied
		// destination and the untouched resident, so the rollback halts on it and
		// preserves the journal instead of guessing.
		if !strings.Contains(err.Error(), "rollback halted") {
			t.Fatalf("want a halted rollback, got %v", err)
		}
		if !journalPresence(t, root) {
			t.Fatal("journal cleared despite a halted rollback")
		}
		if got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(QuarantineRel()), "efforts-legacy.json")); string(got) != "earlier" {
			t.Fatalf("earlier quarantined bytes destroyed: %q", got)
		}
		if got, _ := os.ReadFile(filepath.Join(root, ".awf", "efforts", "legacy.json")); string(got) != `{"schemaVersion":1}` {
			t.Fatalf("untouched resident destroyed: %q", got)
		}
		if got, _ := os.ReadFile(filepath.Join(root, ".awf", "memory", "notes.md")); string(got) != "standalone" {
			t.Fatalf("later resident destroyed: %q", got)
		}
		if got, _ := os.ReadFile(filepath.Join(root, LockRel())); string(got) != "old-lock" {
			t.Fatalf("lock replaced despite the refusal: %q", got)
		}
	})
	t.Run("occupied-resident-path-halts-restore", func(t *testing.T) {
		root := t.TempDir()
		seedResidents(t, root)
		// The rename already happened and something recreated the resident path,
		// so restoring would have to overwrite foreign bytes.
		mustMkdir(t, filepath.Join(root, filepath.FromSlash(QuarantineRel())))
		if err := os.Rename(filepath.Join(root, ".awf", "memory"), filepath.Join(root, filepath.FromSlash(QuarantineRel()), "memory")); err != nil {
			t.Fatal(err)
		}
		mustMkdir(t, filepath.Join(root, ".awf", "memory"))
		mustWrite(t, filepath.Join(root, ".awf", "memory", "foreign.md"), []byte("recreated"))
		writeRawJournal(t, root, residentJournal(phaseApplying))
		err := Recover(root, io.Discard)
		if err == nil || !strings.Contains(err.Error(), ".awf/memory") || !strings.Contains(err.Error(), gitRestorationGuidance) {
			t.Fatalf("want a restore refusal, got %v", err)
		}
		if !journalPresence(t, root) {
			t.Fatal("journal cleared despite a halted restore")
		}
		if got, _ := os.ReadFile(filepath.Join(root, ".awf", "memory", "foreign.md")); string(got) != "recreated" {
			t.Fatalf("foreign bytes destroyed: %q", got)
		}
		if got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(QuarantineRel()), "memory", "notes.md")); string(got) != "standalone" {
			t.Fatalf("quarantined bytes destroyed: %q", got)
		}
	})
}

func TestJournalCommitLockFailureHaltsRollback(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".awf"))
	mustMkdir(t, filepath.Join(root, LockRel()))
	ops := []Operation{
		{Path: "a.txt", Prior: Image{}, Replacement: presentImg("alpha")},
		{Path: LockRel(), Prior: Image{}, Replacement: presentImg("final")},
	}
	var log bytes.Buffer
	err := commitTransaction(root, ops, &log)
	if err == nil || !strings.Contains(err.Error(), "apply .awf/awf.lock") || !strings.Contains(err.Error(), "rollback halted") {
		t.Fatalf("want a halted rollback, got %v", err)
	}
	if !journalPresence(t, root) {
		t.Fatal("journal cleared despite a halted rollback")
	}
}
