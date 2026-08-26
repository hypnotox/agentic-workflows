package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func JournalPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(JournalRel))
}

// TestJournalPresentSeparatesAbsenceFromFault pins that an unreadable journal
// location is reported as a fault rather than as absence. Folding the two
// together told the command-state guard there was no journal to recover from,
// so it permitted exactly the commands an unrecovered upgrade must block.
func TestJournalPresentSeparatesAbsenceFromFault(t *testing.T) {
	missingRoot := filepath.Join(t.TempDir(), "missing")
	if found, err := JournalPresent(missingRoot); found || err == nil {
		t.Fatalf("missing root: found=%v err=%v, want false and open failure", found, err)
	}

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

// TestApplyImageRestoresContentAndModeAtomically pins that a restore keeps the
// image's recorded mode and leaves no temp residue. applyImage writes through
// the same atomic path as the journal that records it, so a crash mid-restore
// cannot leave a truncated file where recovery promised a whole image.
func TestApplyImageRestoresContentAndModeAtomically(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), []byte("current"))
	before, err := os.Stat(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyImage(root, "a.txt", Image{Present: true, Mode: 0o600, Content: []byte("prior")}); err != nil {
		t.Fatal(err)
	}
	// A rename-replaced target is a different file; truncating the original in
	// place would keep identity, which is exactly the window a crash could catch
	// half-written. os.SameFile compares inode identity portably.
	after, err := os.Stat(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("restore truncated the existing file in place instead of renaming a replacement over it")
	}
	got, err := os.ReadFile(filepath.Join(root, "a.txt"))
	if err != nil || string(got) != "prior" {
		t.Fatalf("content = %q, err = %v", got, err)
	}
	info, err := os.Stat(filepath.Join(root, "a.txt"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %v, err = %v (want 0600)", info.Mode().Perm(), err)
	}
	ents, err := os.ReadDir(root)
	if err != nil || len(ents) != 1 {
		t.Fatalf("temp residue left behind: %v (err %v)", ents, err)
	}
	mustWrite(t, filepath.Join(root, "blocking-parent"), []byte("file"))
	if err := applyImage(root, "blocking-parent/child", Image{Present: true, Mode: 0o600, Content: []byte("prior")}); err == nil {
		t.Fatal("restore created a child beneath a regular file")
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

func testImageSHA(img Image) string {
	sum := sha256.Sum256(img.Content)
	return hex.EncodeToString(sum[:])
}

type treeEntry struct {
	path    string
	mode    fs.FileMode
	content string
}

func captureTree(t *testing.T, root string) []treeEntry {
	t.Helper()
	var state []treeEntry
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		item := treeEntry{path: filepath.ToSlash(rel), mode: info.Mode()}
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			item.content = string(b)
		}
		state = append(state, item)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return state
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

func TestRecoverRefusesControlCharacterOperationPathsBeforeMutation(t *testing.T) {
	for _, path := range []string{"a\r.txt", "a\n.txt"} {
		t.Run(strconv.Quote(path), func(t *testing.T) {
			root := t.TempDir()
			journal := lockJournal(phaseApplying)
			journal.Operations[0].Path = path
			writeRawJournal(t, root, journal)
			mustWrite(t, filepath.Join(root, "a.txt"), []byte("unchanged"))
			mustWrite(t, filepath.Join(root, LockRel()), []byte("old-lock"))

			if _, err := Recover(root); err == nil || !strings.Contains(err.Error(), "unsafe path") {
				t.Fatalf("recover control-character journal: %v", err)
			}
			if got, err := os.ReadFile(filepath.Join(root, "a.txt")); err != nil || string(got) != "unchanged" {
				t.Fatalf("output after refusal = %q, %v; want unchanged", got, err)
			}
			if got, err := os.ReadFile(filepath.Join(root, LockRel())); err != nil || string(got) != "old-lock" {
				t.Fatalf("lock after refusal = %q, %v; want old-lock", got, err)
			}
			if !journalPresence(t, root) {
				t.Fatal("journal removed despite parse refusal")
			}
		})
	}
}

func TestJournalCommitHappyPath(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".awf"))
	ops := []Operation{
		{Path: "a.txt", Prior: Image{}, Replacement: presentImg("alpha")},
		{Path: LockRel(), Prior: Image{}, Replacement: presentImg("lock-final")},
	}
	if _, err := commitTransaction(root, ops); err != nil {
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
	if _, err := commitTransaction(root, ops); err == nil || !strings.Contains(err.Error(), "ro/new.txt") {
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
}

func TestJournalRecoverTable(t *testing.T) {
	t.Run("precommit-rolls-back", func(t *testing.T) {
		root := t.TempDir()
		writeRawJournal(t, root, lockJournal(phaseApplying))
		mustWrite(t, filepath.Join(root, "a.txt"), []byte("new"))
		mustWrite(t, filepath.Join(root, LockRel()), []byte("old-lock"))
		if _, err := Recover(root); err != nil {
			t.Fatalf("recover: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "a.txt")); !os.IsNotExist(err) {
			t.Fatal("a.txt not rolled back")
		}
		if journalPresence(t, root) {
			t.Fatal("journal residue")
		}
		if _, err := Recover(root); err == nil {
			t.Fatal("second recovery with no journal accepted")
		}
	})
	t.Run("precommit-lock-already-final", func(t *testing.T) {
		root := t.TempDir()
		writeRawJournal(t, root, lockJournal(phasePrepared))
		mustWrite(t, filepath.Join(root, LockRel()), []byte("FINAL"))
		mustWrite(t, filepath.Join(root, "a.txt"), []byte("new"))
		if _, err := Recover(root); err != nil {
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
		if _, err := Recover(root); err != nil {
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
		outcome, err := Recover(root)
		if err == nil || !strings.Contains(err.Error(), "refusing to roll committed authority back") {
			t.Fatalf("want refusal, got %v", err)
		}
		want := []Evidence{retainedJournal(root)}
		if !slices.Equal(outcome.Changed, want) {
			t.Fatalf("changed = %#v, want %#v", outcome.Changed, want)
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
		if _, err := Recover(root); err == nil || !strings.Contains(err.Error(), "a.txt") {
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
	if safeRelPath("") || safeRelPath("/abs") || safeRelPath("a/../b") || safeRelPath("a\r/b") || safeRelPath("a\n/b") || !safeRelPath("a/b.txt") {
		t.Fatal("safeRelPath")
	}
	empty := t.TempDir()
	mustMkdir(t, filepath.Join(empty, ".awf"))
	mustWrite(t, JournalPath(empty), []byte(`{"version":1,"phase":"prepared","finalLockSHA256":"","operations":[]}`))
	if _, err := LoadJournal(empty); err == nil || !strings.Contains(err.Error(), "no operations") {
		t.Fatalf("empty ops: %v", err)
	}
}

func TestJournalCommitApplyingPhaseWriteFailure(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".awf"))
	failure := errors.New("applying phase write failed")
	writes := 0
	operation := productionJournalOperation()
	priorWrite := operation.write
	operation.write = func(root string, j Journal) error {
		writes++
		if writes == 2 {
			return failure
		}
		return priorWrite(root, j)
	}
	if _, err := commitTransactionWith(root, []Operation{{Path: LockRel(), Replacement: presentImg("final")}}, operation); !errors.Is(err, failure) {
		t.Fatalf("commit error = %v, want applying phase-write failure", err)
	}
}

func TestJournalCommitRetainsEvidenceWhenLockPhaseWriteFails(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".awf"))
	failure := errors.New("phase write failed")
	writes := 0
	operation := productionJournalOperation()
	priorWrite := operation.write
	operation.write = func(root string, j Journal) error {
		writes++
		if writes == 3 {
			return failure
		}
		return priorWrite(root, j)
	}
	ops := []Operation{
		{Path: "a.txt", Prior: Image{}, Replacement: presentImg("alpha")},
		{Path: LockRel(), Prior: Image{}, Replacement: presentImg("lock-final")},
	}
	outcome, err := commitTransactionWith(root, ops, operation)
	if !errors.Is(err, failure) {
		t.Fatalf("commit error = %v, want phase-write failure", err)
	}
	want := []Evidence{
		{Action: "applied", Path: "a.txt"},
		{Action: "applied", Path: LockRel()},
		{Action: "committed", Path: LockRel()},
		retainedJournal(root),
	}
	if !slices.Equal(outcome.Evidence, want) {
		t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, want)
	}
	if got, _ := os.ReadFile(filepath.Join(root, LockRel())); string(got) != "lock-final" {
		t.Fatalf("lock = %q, want committed bytes", got)
	}
	if !journalPresence(t, root) {
		t.Fatal("recoverable journal missing after committed lock")
	}
}

func TestRecoverPropagatesAppliedImageInspectionFailure(t *testing.T) {
	root := t.TempDir()
	writeRawJournal(t, root, lockJournal(phaseApplying))
	failure := errors.New("inspect applied image")
	operation := productionJournalOperation()
	operation.imageOf = func(string, string) (Image, error) { return Image{}, failure }
	outcome, err := recoverWith(root, operation)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if want := []Evidence{retainedJournal(root)}; !slices.Equal(outcome.Evidence, want) {
		t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, want)
	}
	if want := []Evidence{retainedJournal(root)}; !slices.Equal(outcome.Changed, want) {
		t.Fatalf("changed = %#v, want %#v", outcome.Changed, want)
	}
}

func TestRecoverPropagatesLockInspectionFailure(t *testing.T) {
	root := t.TempDir()
	writeRawJournal(t, root, lockJournal(phaseApplying))
	failure := errors.New("inspect lock")
	operation := productionJournalOperation()
	operation.imageOf = func(string, string) (Image, error) { return Image{}, failure }
	outcome, err := recoverWith(root, operation)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if want := []Evidence{retainedJournal(root)}; !slices.Equal(outcome.Changed, want) {
		t.Fatalf("changed = %#v, want %#v", outcome.Changed, want)
	}
}

func TestAppliedOperationsPropagatesResidentInspectionFailure(t *testing.T) {
	failure := errors.New("inspect quarantine")
	operation := productionJournalOperation()
	operation.lstat = func(string, string) (os.FileInfo, error) { return nil, failure }
	journal := Journal{Operations: []Operation{{Kind: KindResidentTree, Path: ".awf/efforts", Quarantine: ".awf/.upgrade-quarantine/efforts"}}}
	if _, err := appliedOperations(t.TempDir(), journal, operation); !errors.Is(err, failure) || !strings.Contains(err.Error(), "inspect quarantine .awf/.upgrade-quarantine/efforts") {
		t.Fatalf("error = %v, want wrapped inspection failure with quarantine path", err)
	}
}

func TestAppliedOperationsTreatsWrappedResidentAbsenceAsUnapplied(t *testing.T) {
	operation := productionJournalOperation()
	operation.lstat = func(string, string) (os.FileInfo, error) {
		return nil, fmt.Errorf("quarantine unavailable: %w", fs.ErrNotExist)
	}
	journal := Journal{Operations: []Operation{{Kind: KindResidentTree, Path: ".awf/efforts", Quarantine: ".awf/.upgrade-quarantine/efforts"}}}
	applied, err := appliedOperations(t.TempDir(), journal, operation)
	if err != nil {
		t.Fatalf("appliedOperations: %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("applied = %#v, want none", applied)
	}
}

func TestAppliedOperationsPropagatesFileInspectionFailure(t *testing.T) {
	failure := errors.New("inspect file")
	operation := productionJournalOperation()
	operation.imageOf = func(string, string) (Image, error) { return Image{}, failure }
	journal := Journal{Operations: []Operation{{Path: "a.txt", Prior: Image{}, Replacement: presentImg("new")}}}
	if _, err := appliedOperations(t.TempDir(), journal, operation); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
}

func TestRecoverRetainsJournalWhenAppliedInspectionFails(t *testing.T) {
	root := t.TempDir()
	writeRawJournal(t, root, lockJournal(phaseApplying))
	failure := errors.New("inspect applied image")
	operation := productionJournalOperation()
	operation.imageOf = func(_ string, path string) (Image, error) {
		if path == LockRel() {
			return Image{}, nil
		}
		return Image{}, failure
	}
	outcome, err := recoverWith(root, operation)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if want := []Evidence{retainedJournal(root)}; !slices.Equal(outcome.Changed, want) {
		t.Fatalf("changed = %#v, want %#v", outcome.Changed, want)
	}
}

func TestRecoveryJournalWriteFailureRetainsTerminalJournalAxis(t *testing.T) {
	root := t.TempDir()
	writeRawJournal(t, root, lockJournal(phaseApplying))
	failure := errors.New("recovery journal write")
	operation := productionJournalOperation()
	operation.write = func(string, Journal) error { return failure }
	outcome, err := recoverWith(root, operation)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	if want := []Evidence{{Action: "pending", Path: LockRel()}, {Action: "retained", Path: JournalRel}}; !slices.Equal(outcome.Changed, want) {
		t.Fatalf("changed = %#v, want %#v", outcome.Changed, want)
	}
}

func TestJournalWriteFailuresReportTerminalJournalAxis(t *testing.T) {
	for _, tc := range []struct {
		name   string
		failAt int
		want   []Evidence
	}{
		{"initial", 1, nil},
		{"applying", 2, []Evidence{{Action: "retained", Path: ""}}},
		{"rollback", 3, []Evidence{{Action: "applied", Path: "a.txt"}, {Action: "pending", Path: "blocked"}, {Action: "retained", Path: ""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mustMkdir(t, filepath.Join(root, ".awf"))
			failure := errors.New("journal write")
			writes := 0
			operation := productionJournalOperation()
			priorWrite := operation.write
			operation.write = func(root string, j Journal) error {
				writes++
				if writes == tc.failAt {
					return failure
				}
				return priorWrite(root, j)
			}
			ops := []Operation{{Path: "a.txt", Replacement: presentImg("a")}, {Path: "blocked", Prior: Image{}, Replacement: Image{Present: true, Mode: 0o644, Content: nil}}, {Path: LockRel(), Replacement: presentImg("final")}}
			// A directory makes the second file application fail after the applying phase.
			mustMkdir(t, filepath.Join(root, "blocked"))
			outcome, err := commitTransactionWith(root, ops, operation)
			if !errors.Is(err, failure) && tc.failAt < 3 {
				t.Fatalf("error = %v", err)
			}
			if tc.failAt == 3 && !errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
			for i := range tc.want {
				if tc.want[i].Path == "" {
					tc.want[i].Path = JournalRel
				}
			}
			if !slices.Equal(outcome.Changed, tc.want) {
				t.Fatalf("changed = %#v, want %#v", outcome.Changed, tc.want)
			}
		})
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
	if _, err := commitTransaction(root, ops); err == nil {
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

// invariant: config/migrations-and-locks:lock-atomic-save (TestJournalResidentCommitQuarantinesThenDiscards)
func TestJournalResidentCommitQuarantinesThenDiscards(t *testing.T) {
	root := t.TempDir()
	seedResidents(t, root)
	j := residentJournal(phasePrepared)
	outcome, err := commitTransaction(root, j.Operations)
	if err != nil {
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
	// Ordered terminal evidence is collected only after the lock commits.
	want := []Evidence{
		{Action: "applied", Path: ".awf/config.yaml"},
		{Action: "applied", Path: ".awf/efforts/legacy.json"},
		{Action: "applied", Path: ".awf/memory"},
		{Action: "applied", Path: LockRel()},
		{Action: "committed", Path: LockRel()},
		{Action: "discarded", Path: ".awf/efforts/legacy.json"},
		{Action: "discarded", Path: ".awf/memory"},
	}
	if !slices.Equal(outcome.Evidence, want) {
		t.Fatalf("evidence:\ngot  %#v\nwant %#v", outcome.Evidence, want)
	}
	if journalPresence(t, root) {
		t.Fatal("journal residue after success")
	}
}

// invariant: config/migrations-and-locks:lock-atomic-save (TestJournalResidentRollbackRestoresQuarantinedBytes)
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
	if _, err := commitTransaction(root, ops); err == nil || !strings.Contains(err.Error(), "ro/new.txt") {
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
// invariant: config/migrations-and-locks:lock-atomic-save (TestJournalResidentInterruption)
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
			if _, err := Recover(root); err != nil {
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
			if _, err := Recover(root); err == nil {
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
func TestJournalResidentRenameHelpersPreserveDestinationInspectionFailures(t *testing.T) {
	op := Operation{Kind: KindResidentTree, Path: ".awf/efforts", Quarantine: ".awf/.upgrade-quarantine/efforts"}
	failure := errors.New("inspect destination")

	t.Run("quarantine", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, filepath.FromSlash(op.Path)))
		operation := productionJournalOperation()
		lstat := operation.lstat
		operation.lstat = func(root, path string) (os.FileInfo, error) {
			if path == op.Quarantine {
				return nil, failure
			}
			return lstat(root, path)
		}
		if err := quarantineTree(root, op, operation); !errors.Is(err, failure) {
			t.Fatalf("quarantine destination error = %v, want %v", err, failure)
		}
	})

	t.Run("restore", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, filepath.FromSlash(op.Quarantine)))
		operation := productionJournalOperation()
		lstat := operation.lstat
		operation.lstat = func(root, path string) (os.FileInfo, error) {
			if path == op.Path {
				return nil, failure
			}
			return lstat(root, path)
		}
		if err := restoreTree(root, op, operation); !errors.Is(err, failure) {
			t.Fatalf("restore destination error = %v, want %v", err, failure)
		}
	})
}

func TestJournalResidentRenameHelpers(t *testing.T) {
	op := treeOp(".awf/efforts/legacy.json", "efforts-legacy.json")
	t.Run("absent-source-converges", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, ".awf", "efforts"))
		if err := quarantineTree(root, op, productionJournalOperation()); err != nil {
			t.Fatalf("quarantine an absent resident: %v", err)
		}
		if exists(t, filepath.Join(root, filepath.FromSlash(QuarantineRel()))) {
			t.Fatal("quarantine root created for an absent resident")
		}
		if err := restoreTree(root, op, productionJournalOperation()); err != nil {
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
		if err := quarantineTree(root, op, productionJournalOperation()); err == nil {
			t.Fatal("unreadable resident quarantined")
		}
		if err := restoreTree(root, op, productionJournalOperation()); err == nil {
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
	if _, err := commitTransaction(root, residentJournal(phasePrepared).Operations); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, LockRel())); string(got) != "FINAL" {
		t.Fatalf("lock: %q", got)
	}
	if exists(t, filepath.Join(root, filepath.FromSlash(QuarantineRel()))) {
		t.Fatal("quarantine residue")
	}
}

// invariant: config/migrations-and-locks:lock-atomic-save (TestJournalResidentCollisionRefusals)
func TestJournalResidentCollisionRefusals(t *testing.T) {
	t.Run("occupied-quarantine-destination", func(t *testing.T) {
		root := t.TempDir()
		seedResidents(t, root)
		// A previous interrupted run already put something at the destination;
		// overwriting it would destroy the only copy of those bytes.
		mustMkdir(t, filepath.Join(root, filepath.FromSlash(QuarantineRel())))
		mustWrite(t, filepath.Join(root, filepath.FromSlash(QuarantineRel()), "efforts-legacy.json"), []byte("earlier"))
		ops := residentJournal(phasePrepared).Operations
		_, err := commitTransaction(root, ops)
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
		_, err := Recover(root)
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
	_, err := commitTransaction(root, ops)
	if err == nil || !strings.Contains(err.Error(), "apply .awf/awf.lock") || !strings.Contains(err.Error(), "rollback halted") {
		t.Fatalf("want a halted rollback, got %v", err)
	}
	if !journalPresence(t, root) {
		t.Fatal("journal cleared despite a halted rollback")
	}
}

func TestJournalCleanupFaultOutcomes(t *testing.T) {
	t.Run("committed-transaction-records-terminal-discarded-residents", func(t *testing.T) {
		root := t.TempDir()
		seedResidents(t, root)
		outcome, err := commitTransaction(root, residentJournal(phasePrepared).Operations)
		if err != nil {
			t.Fatal(err)
		}
		wantEvidence := []Evidence{
			{Action: "applied", Path: ".awf/config.yaml"},
			{Action: "applied", Path: ".awf/efforts/legacy.json"},
			{Action: "applied", Path: ".awf/memory"},
			{Action: "applied", Path: LockRel()},
			{Action: "committed", Path: LockRel()},
			{Action: "discarded", Path: ".awf/efforts/legacy.json"},
			{Action: "discarded", Path: ".awf/memory"},
		}
		if !slices.Equal(outcome.Evidence, wantEvidence) {
			t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, wantEvidence)
		}
		wantChanged := []Evidence{
			{Action: "applied", Path: ".awf/config.yaml"},
			{Action: "applied", Path: LockRel()},
			{Action: "committed", Path: LockRel()},
			{Action: "discarded", Path: ".awf/efforts/legacy.json"},
			{Action: "discarded", Path: ".awf/memory"},
		}
		if !slices.Equal(outcome.Changed, wantChanged) {
			t.Fatalf("changed = %#v, want %#v", outcome.Changed, wantChanged)
		}
	})

	t.Run("committed-recovery-records-terminal-discarded-residents", func(t *testing.T) {
		root := t.TempDir()
		seedResidents(t, root)
		mustWrite(t, filepath.Join(root, LockRel()), []byte("FINAL"))
		writeRawJournal(t, root, residentJournal(phaseLockCommitted))
		outcome, err := Recover(root)
		if err != nil {
			t.Fatal(err)
		}
		wantEvidence := []Evidence{
			{Action: "applied", Path: ".awf/config.yaml"},
			{Action: "applied", Path: ".awf/efforts/legacy.json"},
			{Action: "applied", Path: ".awf/memory"},
			{Action: "applied", Path: LockRel()},
			{Action: "committed", Path: LockRel()},
			{Action: "discarded", Path: ".awf/efforts/legacy.json"},
			{Action: "discarded", Path: ".awf/memory"},
			{Action: "recovered", Path: JournalRel},
		}
		if !slices.Equal(outcome.Evidence, wantEvidence) {
			t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, wantEvidence)
		}
		wantChanged := []Evidence{
			{Action: "applied", Path: ".awf/config.yaml"},
			{Action: "applied", Path: LockRel()},
			{Action: "committed", Path: LockRel()},
			{Action: "discarded", Path: ".awf/efforts/legacy.json"},
			{Action: "discarded", Path: ".awf/memory"},
		}
		if !slices.Equal(outcome.Changed, wantChanged) {
			t.Fatalf("changed = %#v, want %#v", outcome.Changed, wantChanged)
		}
	})

	t.Run("committed-transaction-retains-partial-discard", func(t *testing.T) {
		root := t.TempDir()
		seedResidents(t, root)
		failure := errors.New("discard second quarantine")
		operation := productionJournalOperation()
		priorRemoveAll := operation.removeAll
		calls := 0
		operation.removeAll = func(root, path string) error {
			calls++
			if calls == 2 {
				return failure
			}
			return priorRemoveAll(root, path)
		}

		outcome, err := commitTransactionWith(root, residentJournal(phasePrepared).Operations, operation)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v, want %v", err, failure)
		}
		wantEvidence := []Evidence{
			{Action: "applied", Path: ".awf/config.yaml"},
			{Action: "applied", Path: ".awf/efforts/legacy.json"},
			{Action: "applied", Path: ".awf/memory"},
			{Action: "applied", Path: LockRel()},
			{Action: "committed", Path: LockRel()},
			{Action: "discarded", Path: ".awf/efforts/legacy.json"},
			retainedJournal(root),
		}
		if !slices.Equal(outcome.Evidence, wantEvidence) {
			t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, wantEvidence)
		}
		wantChanged := []Evidence{
			{Action: "applied", Path: ".awf/config.yaml"},
			{Action: "applied", Path: ".awf/memory"},
			{Action: "applied", Path: LockRel()},
			{Action: "committed", Path: LockRel()},
			{Action: "discarded", Path: ".awf/efforts/legacy.json"},
			retainedJournal(root),
		}
		if !slices.Equal(outcome.Changed, wantChanged) {
			t.Fatalf("changed = %#v, want %#v", outcome.Changed, wantChanged)
		}
	})

	t.Run("post-commit-removal-retains-committed-axes", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, ".awf"))
		failure := errors.New("remove committed journal")
		operation := productionJournalOperation()
		operation.remove = func(string, string) error { return failure }

		outcome, err := commitTransactionWith(root, lockJournal(phasePrepared).Operations, operation)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v, want %v", err, failure)
		}
		wantEvidence := []Evidence{{Action: "applied", Path: "a.txt"}, {Action: "applied", Path: LockRel()}, {Action: "committed", Path: LockRel()}, retainedJournal(root)}
		if !slices.Equal(outcome.Evidence, wantEvidence) {
			t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, wantEvidence)
		}
		if !slices.Equal(outcome.Changed, wantEvidence) {
			t.Fatalf("changed = %#v, want %#v", outcome.Changed, wantEvidence)
		}
	})

	t.Run("rollback-removal-retains-only-journal", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, ".awf"))
		mustWrite(t, filepath.Join(root, "a.txt"), []byte("new"))
		failure := errors.New("remove rollback journal")
		operation := productionJournalOperation()
		operation.remove = func(string, string) error { return failure }

		outcome, err := rollBack(root, lockJournal(phaseApplying), errors.New("apply blocked"), []Evidence{{Action: "applied", Path: "a.txt"}}, operation)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v, want %v", err, failure)
		}
		wantEvidence := []Evidence{{Action: "applied", Path: "a.txt"}, {Action: "restored", Path: "a.txt"}, retainedJournal(root)}
		if !slices.Equal(outcome.Evidence, wantEvidence) {
			t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, wantEvidence)
		}
		if want := []Evidence{retainedJournal(root)}; !slices.Equal(outcome.Changed, want) {
			t.Fatalf("changed = %#v, want %#v", outcome.Changed, want)
		}
	})

	t.Run("committed-recovery-quarantine-failure", func(t *testing.T) {
		root := t.TempDir()
		seedResidents(t, root)
		mustWrite(t, filepath.Join(root, LockRel()), []byte("FINAL"))
		writeRawJournal(t, root, residentJournal(phaseLockCommitted))
		failure := errors.New("discard second quarantine")
		operation := productionJournalOperation()
		priorRemoveAll := operation.removeAll
		calls := 0
		operation.removeAll = func(root, path string) error {
			calls++
			if calls == 2 {
				return failure
			}
			return priorRemoveAll(root, path)
		}

		outcome, err := recoverWith(root, operation)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v, want %v", err, failure)
		}
		wantEvidence := []Evidence{
			{Action: "applied", Path: ".awf/config.yaml"},
			{Action: "applied", Path: ".awf/efforts/legacy.json"},
			{Action: "applied", Path: ".awf/memory"},
			{Action: "applied", Path: LockRel()},
			{Action: "committed", Path: LockRel()},
			{Action: "discarded", Path: ".awf/efforts/legacy.json"},
			retainedJournal(root),
		}
		if !slices.Equal(outcome.Evidence, wantEvidence) {
			t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, wantEvidence)
		}
		wantChanged := []Evidence{
			{Action: "applied", Path: ".awf/config.yaml"},
			{Action: "applied", Path: ".awf/memory"},
			{Action: "applied", Path: LockRel()},
			{Action: "committed", Path: LockRel()},
			{Action: "discarded", Path: ".awf/efforts/legacy.json"},
			retainedJournal(root),
		}
		if !slices.Equal(outcome.Changed, wantChanged) {
			t.Fatalf("changed = %#v, want %#v", outcome.Changed, wantChanged)
		}
	})

	t.Run("cleanup-journal-failure-retains-input-axes", func(t *testing.T) {
		root := t.TempDir()
		failure := errors.New("remove recovery journal")
		operation := productionJournalOperation()
		operation.remove = func(string, string) error { return failure }

		evidence := []Evidence{{Action: "applied", Path: "a.txt"}}
		changed := []Evidence{{Action: "applied", Path: "a.txt"}}
		outcome, err := cleanupJournal(root, evidence, changed, operation)
		if !errors.Is(err, failure) || !strings.Contains(err.Error(), "remove current-state upgrade journal") {
			t.Fatalf("error = %v, want wrapped journal removal failure", err)
		}
		wantEvidence := appendEvidence(evidence, retainedJournal(root))
		if !slices.Equal(outcome.Evidence, wantEvidence) {
			t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, wantEvidence)
		}
		if want := appendEvidence(changed, retainedJournal(root)); !slices.Equal(outcome.Changed, want) {
			t.Fatalf("changed = %#v, want %#v", outcome.Changed, want)
		}
	})

	t.Run("wrapped-absence-is-idempotent", func(t *testing.T) {
		operation := productionJournalOperation()
		operation.remove = func(string, string) error {
			return fmt.Errorf("journal already removed: %w", fs.ErrNotExist)
		}
		outcome, err := cleanupJournal(t.TempDir(), nil, nil, operation)
		if err != nil {
			t.Fatalf("cleanupJournal: %v", err)
		}
		if want := []Evidence{{Action: "recovered", Path: JournalRel}}; !slices.Equal(outcome.Evidence, want) {
			t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, want)
		}
	})
}

func TestRecoverRestoreWriteHaltRetainsAppliedAxes(t *testing.T) {
	root := t.TempDir()
	writeRawJournal(t, root, lockJournal(phaseApplying))
	mustWrite(t, filepath.Join(root, "a.txt"), []byte("new"))
	mustWrite(t, filepath.Join(root, LockRel()), []byte("old-lock"))
	failure := errors.New("restore image")
	operation := productionJournalOperation()
	operation.applyImage = func(string, string, Image) error { return failure }
	outcome, err := recoverWith(root, operation)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	wantEvidence := []Evidence{{Action: "applied", Path: "a.txt"}, retainedJournal(root)}
	if !slices.Equal(outcome.Evidence, wantEvidence) {
		t.Fatalf("evidence = %#v, want %#v", outcome.Evidence, wantEvidence)
	}
	if !slices.Equal(outcome.Changed, wantEvidence) {
		t.Fatalf("changed = %#v, want %#v", outcome.Changed, wantEvidence)
	}
}

func TestBoundJournalOperationRejectsMissingRootAndInitializesInjectedState(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if err := withBoundJournalOperation(missing, journalOperation{}, func(journalOperation) error { return nil }); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing root error = %v, want not-exist", err)
	}
	if err := withBoundJournalOperation(t.TempDir(), journalOperation{}, func(bound journalOperation) error {
		if bound.state == nil || bound.state.files == nil {
			t.Fatal("injected operation did not receive bound filesystem state")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestJournalTransactionBindsFilesystemToOriginallyOpenedRoot(t *testing.T) {
	original, rebound := t.TempDir(), t.TempDir()
	for _, tree := range []string{original, rebound} {
		mustMkdir(t, filepath.Join(tree, ".awf"))
		mustWrite(t, filepath.Join(tree, LockRel()), []byte("prior"))
	}
	root := filepath.Join(t.TempDir(), "project")
	if err := os.Symlink(original, root); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	operation := productionJournalOperation()
	write, writes := operation.write, 0
	operation.write = func(path string, journal Journal) error {
		writes++
		if err := write(path, journal); err != nil {
			return err
		}
		if writes == 1 {
			if err := os.Remove(root); err != nil {
				return err
			}
			return os.Symlink(rebound, root)
		}
		return nil
	}
	if _, err := commitTransactionWith(root, []Operation{{Path: LockRel(), Prior: presentImg("prior"), Replacement: presentImg("final")}}, operation); err != nil {
		t.Fatal(err)
	}
	for tree, want := range map[string]string{original: "final", rebound: "prior"} {
		if got, err := os.ReadFile(filepath.Join(tree, LockRel())); err != nil || string(got) != want {
			t.Fatalf("%s lock = %q, %v; want %q", tree, got, err, want)
		}
	}
}

func TestJournalResidentRenameHelpersPropagateParentCreationFailures(t *testing.T) {
	op := treeOp(".awf/efforts/legacy.json", "efforts-legacy.json")
	for _, tc := range []struct {
		name string
		run  func(string, Operation, journalOperation) error
		seed string
	}{{"quarantine", quarantineTree, op.Path}, {"restore", restoreTree, op.Quarantine}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mustMkdir(t, filepath.Join(root, filepath.FromSlash(tc.seed)))
			failure := errors.New("create parent")
			operation := productionJournalOperation()
			operation.mkdirAll = func(string, string, os.FileMode) error { return failure }
			if err := tc.run(root, op, operation); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want %v", err, failure)
			}
			if !exists(t, filepath.Join(root, filepath.FromSlash(tc.seed))) {
				t.Fatalf("%s moved after parent creation failure", tc.seed)
			}
		})
	}
}

func TestRecoverRefusesUnboundOrTrailingJournalBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		phase       string
		currentLock string
		mut         func(*Journal)
		tail        string
	}{
		{name: "trailing JSON", phase: phaseApplying, currentLock: "old-lock", tail: "\n{}"},
		{name: "trailing malformed input", phase: phaseApplying, currentLock: "old-lock", tail: "\nnot-json"},
		{name: "final operation is not a file lock", phase: phaseApplying, currentLock: "old-lock", mut: func(j *Journal) {
			j.Operations[len(j.Operations)-1].Kind = KindResidentTree
			j.Operations[len(j.Operations)-1].Quarantine = QuarantineRel() + "/lock"
		}},
		{name: "missing final replacement", phase: phaseApplying, currentLock: "old-lock", mut: func(j *Journal) {
			j.Operations[len(j.Operations)-1].Replacement = Image{}
			j.FinalLockSHA256 = ""
		}},
		{name: "missing digest", phase: phaseApplying, currentLock: "old-lock", mut: func(j *Journal) { j.FinalLockSHA256 = "" }},
		{name: "mismatched digest", phase: phaseApplying, currentLock: "old-lock", mut: func(j *Journal) { j.FinalLockSHA256 = strings.Repeat("0", 64) }},
		{name: "forged committed hash", phase: phaseLockCommitted, currentLock: "forged", mut: func(j *Journal) {
			j.FinalLockSHA256 = testImageSHA(presentImg("forged"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			j := lockJournal(tc.phase)
			if tc.mut != nil {
				tc.mut(&j)
			}
			writeRawJournal(t, root, j)
			if tc.tail != "" {
				f, err := os.OpenFile(JournalPath(root), os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.WriteString(tc.tail); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			mustWrite(t, filepath.Join(root, "a.txt"), []byte("unchanged"))
			mustWrite(t, filepath.Join(root, LockRel()), []byte(tc.currentLock))
			mustMkdir(t, filepath.Join(root, QuarantineRel(), "resident"))
			mustWrite(t, filepath.Join(root, QuarantineRel(), "resident", "kept.txt"), []byte("quarantined"))
			before := captureTree(t, root)
			if _, err := Recover(root); err == nil {
				t.Fatal("accepted unbound journal")
			}
			if after := captureTree(t, root); !slices.Equal(after, before) {
				t.Fatalf("tree mutated on refusal\nbefore: %#v\nafter:  %#v", before, after)
			}
		})
	}
}

func FuzzParseJournal(f *testing.F) {
	validJournal := lockJournal(phaseApplying)
	valid, err := json.Marshal(validJournal)
	if err != nil {
		f.Fatal(err)
	}
	mismatched := validJournal
	mismatched.FinalLockSHA256 = strings.Repeat("0", 64)
	mismatchedBytes, err := json.Marshal(mismatched)
	if err != nil {
		f.Fatal(err)
	}
	unsorted := validJournal
	unsorted.Operations = append([]Operation{{Path: "z.txt", Replacement: presentImg("z")}}, unsorted.Operations...)
	unsortedBytes, err := json.Marshal(unsorted)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{valid, append(append([]byte(nil), valid...), []byte("\n{}")...), mismatchedBytes, unsortedBytes, []byte(`{"version":1,"phase":"prepared","finalLockSHA256":"","operations":[]}`), []byte(`{"version":1`)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		// Fuzzing intentionally spans phase, digest, operation ordering, truncation,
		// and trailing values through arbitrary serialized journal bytes. Parsing is
		// pure, so an independent oracle can inspect every accepted value.
		j, err := ParseJournal(input)
		if err != nil {
			return
		}
		dec := json.NewDecoder(strings.NewReader(string(input)))
		var one json.RawMessage
		if err := dec.Decode(&one); err != nil {
			t.Fatalf("accepted input that an independent decoder rejects: %v", err)
		}
		var extra any
		if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
			t.Fatalf("accepted trailing journal input: %v", err)
		}
		if len(j.Operations) == 0 {
			t.Fatal("accepted journal without operations")
		}
		last := j.Operations[len(j.Operations)-1]
		if last.Path != LockRel() || last.Kind != KindFile || !last.Replacement.Present || j.FinalLockSHA256 != testImageSHA(last.Replacement) {
			t.Fatalf("accepted unbound final lock: %#v", j)
		}
		for i, op := range j.Operations[:len(j.Operations)-1] {
			if op.Path == LockRel() || (i > 0 && op.Path <= j.Operations[i-1].Path) {
				t.Fatalf("accepted invalid operation order: %#v", j.Operations)
			}
		}
	})
}
