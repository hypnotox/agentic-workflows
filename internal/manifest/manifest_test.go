package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashStableAndPrefixed(t *testing.T) {
	h := Hash([]byte("abc"))
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("hash = %q", h)
	}
	if h != Hash([]byte("abc")) {
		t.Errorf("hash not stable")
	}
	if h == Hash([]byte("abd")) {
		t.Errorf("hash collision on different input")
	}
}

func TestLoadOldLockZeroSchema(t *testing.T) {
	// A lock JSON predating the schemaVersion field unmarshals with the zero value.
	p := filepath.Join(t.TempDir(), "awf.lock")
	old := `{"awfVersion":"0.1.0","files":{}}` + "\n"
	if err := os.WriteFile(p, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if l.SchemaVersion != 0 {
		t.Errorf("SchemaVersion = %d, want 0 for a lock with no schemaVersion field", l.SchemaVersion)
	}
}

func TestLoadMissingFile(t *testing.T) {
	// A non-existent lock path surfaces a wrapped read error.
	p := filepath.Join(t.TempDir(), "absent.lock")
	_, err := Load(p)
	if err == nil {
		t.Fatal("Load on a missing file: want error, got nil")
	}
	if !strings.Contains(err.Error(), "read lock") {
		t.Errorf("error = %q, want it to mention \"read lock\"", err)
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	// Invalid JSON content surfaces a wrapped parse error.
	p := filepath.Join(t.TempDir(), "awf.lock")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("Load on malformed JSON: want error, got nil")
	}
	if !strings.Contains(err.Error(), "parse lock") {
		t.Errorf("error = %q, want it to mention \"parse lock\"", err)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "awf.lock")
	if err := os.WriteFile(p, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("new content\n")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "new content\n" {
		t.Fatalf("content = %q, err = %v", b, err)
	}
	info, err := os.Stat(p)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("perm = %v, err = %v (want 0644 regardless of prior mode)", info.Mode().Perm(), err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("temp residue left behind: %v (err %v)", ents, err)
	}
}

func TestWriteFileAtomicFailureLeavesTargetUntouched(t *testing.T) {
	// Destination path is a directory: CreateTemp succeeds, the rename fails.
	// The original path must be untouched and no temp file may remain.
	dir := t.TempDir()
	p := filepath.Join(dir, "asdir")
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("x")); err == nil {
		t.Fatal("want error renaming onto a directory")
	}
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("temp residue after failure: %v (err %v)", ents, err)
	}
	// Absent parent directory: CreateTemp itself fails (ENOENT, root-proof).
	if err := WriteFileAtomic(filepath.Join(dir, "nope", "x"), []byte("x")); err == nil {
		t.Fatal("want error creating the temp file in an absent directory")
	}
}

func TestSaveDirectoryAtPath(t *testing.T) {
	// A directory squatting on the lock path makes WriteFile fail for all users (incl. root).
	dir := t.TempDir()
	p := filepath.Join(dir, "awf.lock")
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	l := &Lock{AWFVersion: "0.1.0"}
	if err := l.Save(p); err == nil {
		t.Fatal("Save onto a directory path: want error, got nil")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "awf.lock")
	l := &Lock{
		AWFVersion: "0.1.0",
		Files: map[string]Entry{
			".claude/skills/example-tdd/SKILL.md": {
				TemplateID: "skills/tdd/SKILL.md.tmpl", TemplateHash: "sha256:aa",
				ConfigHash: "sha256:bb", OutputHash: "sha256:cc",
			},
		},
	}
	if err := l.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.AWFVersion != "0.1.0" || got.Files[".claude/skills/example-tdd/SKILL.md"].OutputHash != "sha256:cc" {
		t.Errorf("round trip mismatch: %#v", got)
	}
	// Stable formatting: re-saving identical content yields identical bytes.
	b1, _ := os.ReadFile(p)
	_ = got.Save(p)
	b2, _ := os.ReadFile(p)
	if string(b1) != string(b2) {
		t.Errorf("lock serialization not stable")
	}
}
