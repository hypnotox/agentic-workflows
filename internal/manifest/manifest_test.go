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
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "legacyAdrGaps") {
		t.Fatalf("pre-cutover lock gained permanent gap authority: %s", b)
	}
	permanent := &Lock{AWFVersion: "0.19.0", SchemaVersion: 14, Files: map[string]Entry{}, ADRFormatV1From: 137, LegacyADRGaps: []int{}}
	if err := permanent.Save(p); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); !strings.Contains(string(b), `"legacyAdrGaps": []`) {
		t.Fatalf("post-cutover empty gap set is not explicit: %s", b)
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

func TestV2CutoffCanonicalRoundTrip(t *testing.T) {
	lock := &Lock{AWFVersion: "0.20.0", SchemaVersion: 15, Files: map[string]Entry{}, ADRFormatV1From: 4, ADRFormatV2From: 9, LegacyADRGaps: []int{2}, InitializedWithVersion: "0.19.0"}
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	v1 := strings.Index(text, `"adrFormatV1From"`)
	v2 := strings.Index(text, `"adrFormatV2From"`)
	gaps := strings.Index(text, `"legacyAdrGaps"`)
	initialized := strings.Index(text, `"initializedWithVersion"`)
	if v1 >= v2 || v2 >= gaps || gaps >= initialized {
		t.Fatalf("noncanonical authority order:\n%s", text)
	}
	got, err := Parse(b)
	if err != nil || got.ADRFormatV2From != 9 {
		t.Fatalf("V2 round trip = %#v, %v", got, err)
	}
	again, err := got.Marshal()
	if err != nil || string(again) != text {
		t.Fatalf("V2 round trip changed bytes: %v\n%s", err, again)
	}

	preV2 := &Lock{AWFVersion: "0.19.0", SchemaVersion: 14, Files: map[string]Entry{}, ADRFormatV1From: 4, LegacyADRGaps: []int{2}}
	before, err := preV2.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(before)
	if err != nil {
		t.Fatal(err)
	}
	after, err := parsed.Marshal()
	if err != nil || string(before) != string(after) || strings.Contains(string(after), "adrFormatV2From") {
		t.Fatalf("pre-V2 serialization changed: %v\n%s", err, after)
	}
}

func TestSchema15PermanentAuthorityRequiresV2Cutoff(t *testing.T) {
	fields := `"awfVersion":"0.20.0","files":{},"adrFormatV1From":4,"legacyAdrGaps":[]`
	if _, err := Parse([]byte(`{` + fields + `,"schemaVersion":14}`)); err != nil {
		t.Fatalf("schema 14 omission must remain compatible: %v", err)
	}
	if _, err := Parse([]byte(`{` + fields + `,"schemaVersion":15}`)); err == nil || !strings.Contains(err.Error(), "requires adrFormatV2From") {
		t.Fatalf("schema 15 omission error = %v", err)
	}
}

func TestAuthorityStateMatrix(t *testing.T) {
	bridge := `"bridgeAttestation":{"version":1,"preparedHead":"head","treeDigest":"sha256:x","adrFormatV1From":4,"legacyADRGaps":[2]}`
	tests := []struct {
		name, fields string
		want         AuthorityState
		wantErr      bool
	}{
		{"bridge", bridge, AuthorityBridge, false},
		{"permanent-migrated", `"adrFormatV1From":4,"legacyAdrGaps":[2]`, AuthorityPermanent, false},
		{"permanent-initialized", `"adrFormatV1From":4,"legacyAdrGaps":[2],"initializedWithVersion":"0.20.0"`, AuthorityPermanent, false},
		{"permanent-v-prefixed", `"adrFormatV1From":4,"legacyAdrGaps":[],"initializedWithVersion":"v0.20.0"`, AuthorityPermanent, false},
		{"permanent-v2", `"adrFormatV1From":4,"adrFormatV2From":9,"legacyAdrGaps":[]`, AuthorityPermanent, false},
		{"pre-tracking", "", AuthorityPreTracking, false},
		{"v2-positive-only", `"adrFormatV2From":9`, 0, true},
		{"v2-explicit-zero-only", `"adrFormatV2From":0`, 0, true},
		{"mixed", bridge + `,"adrFormatV1From":4,"legacyAdrGaps":[]`, 0, true},
		{"bridge-v2-mixing", bridge + `,"adrFormatV2From":9`, 0, true},
		{"v2-zero", `"adrFormatV1From":4,"adrFormatV2From":0,"legacyAdrGaps":[]`, 0, true},
		{"v2-negative", `"adrFormatV1From":4,"adrFormatV2From":-1,"legacyAdrGaps":[]`, 0, true},
		{"v2-reversed", `"adrFormatV1From":4,"adrFormatV2From":3,"legacyAdrGaps":[]`, 0, true},
		{"init-without-cutoff", `"initializedWithVersion":"0.20.0"`, 0, true},
		{"gaps-without-cutoff", `"legacyAdrGaps":[]`, 0, true},
		{"cutoff-without-gaps", `"adrFormatV1From":1`, 0, true},
		{"null-gaps", `"adrFormatV1From":1,"legacyAdrGaps":null`, 0, true},
		{"negative-cutoff", `"adrFormatV1From":-1,"legacyAdrGaps":[]`, 0, true},
		{"bad-gaps-duplicate", `"adrFormatV1From":4,"legacyAdrGaps":[2,2]`, 0, true},
		{"bad-gaps-descending", `"adrFormatV1From":4,"legacyAdrGaps":[3,2]`, 0, true},
		{"bad-gaps-zero", `"adrFormatV1From":4,"legacyAdrGaps":[0]`, 0, true},
		{"bad-gaps-cutoff", `"adrFormatV1From":4,"legacyAdrGaps":[4]`, 0, true},
		{"bad-init-version", `"adrFormatV1From":4,"legacyAdrGaps":[],"initializedWithVersion":"nope"`, 0, true},
		{"bad-awf-version", `"adrFormatV1From":4,"legacyAdrGaps":[],"initializedWithVersion":"0.20.0"`, 0, true},
		{"future-init-version", `"adrFormatV1From":4,"legacyAdrGaps":[],"initializedWithVersion":"0.21.0"`, 0, true},
		{"bridge-null-gaps", `"bridgeAttestation":{"version":1,"preparedHead":"head","treeDigest":"sha256:x","adrFormatV1From":4,"legacyADRGaps":null}`, 0, true},
		{"bridge-zero-cutoff", `"bridgeAttestation":{"version":1,"preparedHead":"head","treeDigest":"sha256:x","adrFormatV1From":0,"legacyADRGaps":[]}`, 0, true},
		{"bridge-bad-gaps", `"bridgeAttestation":{"version":1,"preparedHead":"head","treeDigest":"sha256:x","adrFormatV1From":4,"legacyADRGaps":[2,2]}`, 0, true},
		{"bridge-descending-gaps", `"bridgeAttestation":{"version":1,"preparedHead":"head","treeDigest":"sha256:x","adrFormatV1From":4,"legacyADRGaps":[3,2]}`, 0, true},
		{"bridge-gap-zero", `"bridgeAttestation":{"version":1,"preparedHead":"head","treeDigest":"sha256:x","adrFormatV1From":4,"legacyADRGaps":[0]}`, 0, true},
		{"bridge-gap-cutoff", `"bridgeAttestation":{"version":1,"preparedHead":"head","treeDigest":"sha256:x","adrFormatV1From":4,"legacyADRGaps":[4]}`, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			comma := ""
			if tc.fields != "" {
				comma = "," + tc.fields
			}
			awfVersion := "0.20.0"
			if tc.name == "permanent-v-prefixed" {
				awfVersion = "v0.20.0"
			}
			if tc.name == "bad-awf-version" {
				awfVersion = "broken"
			}
			lock, err := Parse([]byte(`{"awfVersion":"` + awfVersion + `","schemaVersion":14,"files":{}` + comma + `}`))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected invalid authority")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := lock.AuthorityState()
			if err != nil || got != tc.want {
				t.Fatalf("state=%v err=%v, want %v", got, err, tc.want)
			}
			b, err := lock.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == AuthorityPermanent && !strings.Contains(string(b), `"legacyAdrGaps"`) {
				t.Fatalf("permanent gaps omitted: %s", b)
			}
			if tc.name == "permanent-migrated" && strings.Contains(string(b), "initializedWithVersion") {
				t.Fatalf("old lock gained init provenance: %s", b)
			}
		})
	}
	if _, err := Parse([]byte(`{"awfVersion":"0.20.0","schemaVersion":"bad","files":{}}`)); err == nil {
		t.Fatal("typed lock mismatch parsed")
	}
	invalid := &Lock{AWFVersion: "0.20.0", InitializedWithVersion: "0.20.0"}
	if _, err := invalid.Marshal(); err == nil {
		t.Fatal("invalid programmatic lock marshaled")
	}
	if err := invalid.Save(filepath.Join(t.TempDir(), "invalid.lock")); err == nil {
		t.Fatal("invalid programmatic lock saved")
	}
	cutoffWithoutExplicitGaps := &Lock{AWFVersion: "0.20.0", ADRFormatV1From: 4}
	if _, err := cutoffWithoutExplicitGaps.Marshal(); err == nil || !strings.Contains(err.Error(), "non-nil legacyAdrGaps") {
		t.Fatalf("cutoff plus nil gaps Marshal error = %v", err)
	}
	validProgrammatic := &Lock{AWFVersion: "v0.20.0", ADRFormatV1From: 4, LegacyADRGaps: []int{}, InitializedWithVersion: "v0.19.0"}
	if _, err := validProgrammatic.Marshal(); err != nil {
		t.Fatalf("v-prefixed programmatic authority rejected: %v", err)
	}
	if _, found, err := LoadOptional(filepath.Join(t.TempDir(), "missing.lock")); found || err != nil {
		t.Fatalf("missing lock is pre-tracking at the project boundary: found=%v err=%v", found, err)
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
