package upgrade

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if JournalPresent(root) {
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
	if JournalPresent(root) {
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
		if JournalPresent(root) {
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
		if JournalPresent(root) {
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
		if JournalPresent(root) {
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
		if !JournalPresent(root) {
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
		if !JournalPresent(root) {
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
	if JournalPresent(root) {
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
	if !JournalPresent(root) {
		t.Fatal("journal cleared despite a halted rollback")
	}
}
