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
	// invariant: config/migrations-and-locks:lock-atomic-save
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("temp residue after failure: %v (err %v)", ents, err)
	}
	// Absent parent directory: CreateTemp itself fails (ENOENT, root-proof).
	if err := WriteFileAtomic(filepath.Join(dir, "nope", "x"), []byte("x")); err == nil {
		t.Fatal("want error creating the temp file in an absent directory")
	}
}

func TestLoadOptional(t *testing.T) {
	dir := t.TempDir()
	// Missing → found=false, no error.
	l, found, err := LoadOptional(filepath.Join(dir, "absent.lock"))
	if l != nil || found || err != nil {
		t.Fatalf("missing: lock=%v found=%v err=%v, want nil/false/nil", l, found, err)
	}
	// Corrupt → error carrying the recovery hint; never a lock.
	p := filepath.Join(dir, "awf.lock")
	if err := os.WriteFile(p, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, found, err = LoadOptional(p)
	if l != nil || found || err == nil {
		t.Fatalf("corrupt: lock=%v found=%v err=%v, want nil lock + error", l, found, err)
	}
	for _, want := range []string{"unreadable .awf/awf.lock", "restore it from version control", "delete it deliberately"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("hint missing %q in %q", want, err)
		}
	}
	// Valid → the lock.
	good := &Lock{AWFVersion: "0.1.0", SchemaVersion: 6, Files: map[string]Entry{}}
	if err := good.Save(p); err != nil {
		t.Fatal(err)
	}
	l, found, err = LoadOptional(p)
	if err != nil || !found || l == nil || l.SchemaVersion != 6 {
		t.Fatalf("valid: lock=%v found=%v err=%v", l, found, err)
	}
}

func TestBridgeAttestationOptionalAndRoundTrip(t *testing.T) {
	// An old lock with no bridgeAttestation field parses with a nil pointer.
	p := filepath.Join(t.TempDir(), "awf.lock")
	if err := os.WriteFile(p, []byte(`{"awfVersion":"0.1.0","schemaVersion":6,"files":{}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if old.BridgeAttestation != nil {
		t.Fatalf("old lock gained an attestation: %#v", old.BridgeAttestation)
	}
	// A lock without an attestation omits the key entirely.
	plain := &Lock{AWFVersion: "0.1.0", SchemaVersion: 6, Files: map[string]Entry{}}
	if err := plain.Save(p); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "bridgeAttestation") {
		t.Fatalf("absent attestation still serialized: %s", b)
	}
	// A populated attestation round-trips byte-stably.
	l := &Lock{AWFVersion: "0.1.0", SchemaVersion: 6, Files: map[string]Entry{}, BridgeAttestation: &BridgeAttestation{Version: 1, PreparedHead: "abc123", TreeDigest: "sha256:deadbeef", ADRFormatV1From: 137, LegacyADRGaps: []int{12, 44}}}
	if err := l.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.BridgeAttestation == nil || got.BridgeAttestation.Version != 1 || got.BridgeAttestation.PreparedHead != "abc123" || got.BridgeAttestation.TreeDigest != "sha256:deadbeef" || got.BridgeAttestation.ADRFormatV1From != 137 || len(got.BridgeAttestation.LegacyADRGaps) != 2 || got.BridgeAttestation.LegacyADRGaps[1] != 44 {
		t.Fatalf("attestation round trip mismatch: %#v", got.BridgeAttestation)
	}
	b1, _ := os.ReadFile(p)
	_ = got.Save(p)
	b2, _ := os.ReadFile(p)
	if string(b1) != string(b2) {
		t.Errorf("attested lock serialization not stable")
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
