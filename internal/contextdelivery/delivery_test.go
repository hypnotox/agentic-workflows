package contextdelivery

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// invariant: tooling/context-and-topic:context-terminal-output-cap (TestTerminalOutputCapDeliveryContract)
func TestTerminalOutputCapDeliveryContract(t *testing.T) {
	t.Run("exact boundary and secure spill", testDeliverBoundaryAndSpill)
	t.Run("unsafe temporary locations", testDeliverRejectsUnsafeLocations)
	t.Run("cleanup and primary errors", testDeliverFailuresCleanUpAndPreservePrimary)
}

func testDeliverBoundaryAndSpill(t *testing.T) {
	direct := bytes.Repeat([]byte("x"), 8192)
	var out bytes.Buffer
	if err := Deliver(direct, t.TempDir(), &out); err != nil || !bytes.Equal(out.Bytes(), direct) {
		t.Fatalf("direct err=%v len=%d", err, out.Len())
	}
	root := t.TempDir()
	tmp := t.TempDir()
	old := tempDir
	tempDir = func() string { return tmp }
	t.Cleanup(func() { tempDir = old })
	rendered := bytes.Repeat([]byte("z"), 8193)
	out.Reset()
	if err := Deliver(rendered, root, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out.String(), "\n")
	if len(lines) != 3 || lines[0] != "AWF_CONTEXT_SPILL_V1 bytes=8193 format=text" || !filepath.IsAbs(lines[1]) {
		t.Fatalf("notice %q", out.String())
	}
	wantNotice := []byte(fmt.Sprintf("AWF_CONTEXT_SPILL_V1 bytes=8193 format=text\n%s\n", lines[1]))
	if !bytes.Equal(out.Bytes(), wantNotice) {
		t.Fatalf("spill protocol bytes = %q, want %q", out.Bytes(), wantNotice)
	}
	info, err := os.Stat(lines[1])
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode err=%v info=%v", err, info)
	}
	got, err := os.ReadFile(lines[1])
	if err != nil || !bytes.Equal(got, rendered) {
		t.Fatalf("spill err=%v len=%d", err, len(got))
	}
	os.Remove(lines[1])
}

func testDeliverRejectsUnsafeLocations(t *testing.T) {
	oldTemp, oldCanon := tempDir, canonicalPath
	t.Cleanup(func() { tempDir = oldTemp; canonicalPath = oldCanon })
	root := t.TempDir()
	for _, candidate := range []string{root, filepath.Join(root, "child"), root + "\nchild"} {
		t.Run(strings.ReplaceAll(candidate, "/", "_"), func(t *testing.T) {
			tempDir = func() string { return candidate }
			canonicalPath = func(p string) (string, error) { return p, nil }
			if err := Deliver(bytes.Repeat([]byte("x"), 8193), root, io.Discard); err == nil {
				t.Fatal("accepted unsafe temp")
			}
		})
	}
	canonicalPath = func(string) (string, error) { return "", errors.New("canon") }
	if err := Deliver(bytes.Repeat([]byte("x"), 8193), root, io.Discard); err == nil || !strings.Contains(err.Error(), "canonicalize temporary") {
		t.Fatal(err)
	}
	calls := 0
	canonicalPath = func(p string) (string, error) {
		calls++
		if calls == 2 {
			return "", errors.New("root")
		}
		return p, nil
	}
	tempDir = func() string { return t.TempDir() }
	if err := Deliver(bytes.Repeat([]byte("x"), 8193), root, io.Discard); err == nil || !strings.Contains(err.Error(), "repository root") {
		t.Fatal(err)
	}
}

type fakeInfo struct{ mode os.FileMode }

func (f fakeInfo) Name() string       { return "x" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() os.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

type fakeFile struct {
	name                        string
	writeErr, statErr, closeErr error
	mode                        os.FileMode
	short                       bool
	closeCalls                  int
}

func (f *fakeFile) Name() string               { return f.name }
func (f *fakeFile) Stat() (os.FileInfo, error) { return fakeInfo{f.mode}, f.statErr }
func (f *fakeFile) Close() error               { f.closeCalls++; return f.closeErr }
func (f *fakeFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	if f.short {
		return 0, nil
	}
	return len(p), nil
}

type failWriter struct {
	calls, failAt int
	partial       bool
}

func (w *failWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		if w.partial && len(p) > 0 {
			return 1, errors.New("write")
		}
		return 0, errors.New("write")
	}
	return len(p), nil
}

func testDeliverFailuresCleanUpAndPreservePrimary(t *testing.T) {
	oldTemp, oldCanon, oldCreate, oldRemove := tempDir, canonicalPath, createTemp, removeFile
	t.Cleanup(func() { tempDir = oldTemp; canonicalPath = oldCanon; createTemp = oldCreate; removeFile = oldRemove })
	root, tmp := t.TempDir(), t.TempDir()
	tempDir = func() string { return tmp }
	canonicalPath = func(p string) (string, error) { return filepath.Clean(p), nil }
	data := bytes.Repeat([]byte("x"), 8193)
	createTemp = func(string, string) (spillFile, error) { return nil, errors.New("create") }
	if err := Deliver(data, root, io.Discard); err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		file      *fakeFile
		canonFail bool
		stdout    io.Writer
		want      string
	}{
		{"stat", &fakeFile{name: filepath.Join(tmp, "x"), statErr: errors.New("stat"), mode: 0o600}, false, io.Discard, "inspect"},
		{"mode", &fakeFile{name: filepath.Join(tmp, "x"), mode: 0o644}, false, io.Discard, "secure"},
		{"canonical", &fakeFile{name: filepath.Join(tmp, "x"), mode: 0o600}, true, io.Discard, "canonicalize"},
		{"unsafe-name", &fakeFile{name: filepath.Join(root, "x"), mode: 0o600}, false, io.Discard, "unsafe"},
		{"write", &fakeFile{name: filepath.Join(tmp, "x"), mode: 0o600, writeErr: errors.New("write")}, false, io.Discard, "write context spill"},
		{"short", &fakeFile{name: filepath.Join(tmp, "x"), mode: 0o600, short: true}, false, io.Discard, "short write"},
		{"close", &fakeFile{name: filepath.Join(tmp, "x"), mode: 0o600, closeErr: errors.New("close")}, false, io.Discard, "close"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			removed := false
			createTemp = func(string, string) (spillFile, error) { return tc.file, nil }
			canonicalPath = func(p string) (string, error) {
				if tc.canonFail && p == tc.file.name {
					return "", errors.New("canon")
				}
				return filepath.Clean(p), nil
			}
			removeFile = func(string) error { removed = true; return errors.New("remove") }
			err := Deliver(data, root, tc.stdout)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !removed || tc.file.closeCalls != 1 {
				t.Fatalf("err=%v removed=%t closeCalls=%d", err, removed, tc.file.closeCalls)
			}
		})
	}
	for i, name := range []string{"first-line", "second-line", "final-newline"} {
		t.Run(name, func(t *testing.T) {
			f := &fakeFile{name: filepath.Join(tmp, "x"), mode: 0o600}
			createTemp = func(string, string) (spillFile, error) { return f, nil }
			canonicalPath = func(p string) (string, error) { return filepath.Clean(p), nil }
			removed := false
			removeFile = func(string) error { removed = true; return nil }
			err := Deliver(data, root, &failWriter{failAt: i + 1, partial: true})
			if err == nil || !removed || f.closeCalls != 1 {
				t.Fatalf("err=%v removed=%t closeCalls=%d", err, removed, f.closeCalls)
			}
		})
	}
	if err := Deliver([]byte("x"), root, &failWriter{failAt: 1}); err == nil || !strings.Contains(err.Error(), "write context") {
		t.Fatal(err)
	}
}
