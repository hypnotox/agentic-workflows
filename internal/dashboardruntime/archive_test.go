package dashboardruntime

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func archiveBytes(t *testing.T, headers ...tar.Header) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for _, header := range headers {
		header := header
		if err := writer.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			if _, err := writer.Write(bytes.Repeat([]byte("x"), int(header.Size))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestExtractArchiveShapesAndFaults(t *testing.T) {
	for _, name := range []string{".", "../escape", "/absolute"} {
		t.Run("unsafe-"+strings.ReplaceAll(name, "/", "-"), func(t *testing.T) {
			content := archiveBytes(t, tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755})
			if err := extractArchive(bytes.NewReader(content), t.TempDir()); !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	t.Run("invalid tar", func(t *testing.T) {
		if err := extractArchive(strings.NewReader("not a tar"), t.TempDir()); !errors.Is(err, ErrBuild) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("extended headers", func(t *testing.T) {
		content := archiveBytes(t,
			tar.Header{Name: "pax", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "x"}},
			tar.Header{Name: "dir", Typeflag: tar.TypeDir, Mode: 0o755},
			tar.Header{Name: "dir/file", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1},
		)
		root := t.TempDir()
		if err := extractArchive(bytes.NewReader(content), root); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(root, "dir", "file")); err != nil || string(got) != "x" {
			t.Fatalf("file = %q, %v", got, err)
		}
	})
	t.Run("directory creation", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "blocked"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		content := archiveBytes(t, tar.Header{Name: "blocked/dir", Typeflag: tar.TypeDir, Mode: 0o755})
		if err := extractArchive(bytes.NewReader(content), root); !errors.Is(err, ErrBuild) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("parent creation", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "blocked"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		content := archiveBytes(t, tar.Header{Name: "blocked/file", Typeflag: tar.TypeReg, Mode: 0o644})
		if err := extractArchive(bytes.NewReader(content), root); !errors.Is(err, ErrBuild) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("exclusive file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		content := archiveBytes(t, tar.Header{Name: "file", Typeflag: tar.TypeReg, Mode: 0o644})
		if err := extractArchive(bytes.NewReader(content), root); !errors.Is(err, ErrBuild) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("truncated file", func(t *testing.T) {
		content := archiveBytes(t, tar.Header{Name: "file", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1024})
		content = content[:512+100]
		if err := extractArchive(bytes.NewReader(content), t.TempDir()); !errors.Is(err, ErrBuild) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("unsupported member", func(t *testing.T) {
		content := archiveBytes(t, tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "target"})
		if err := extractArchive(bytes.NewReader(content), t.TempDir()); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("reader failure", func(t *testing.T) {
		if err := extractArchive(errorReader{}, t.TempDir()); !errors.Is(err, ErrBuild) {
			t.Fatalf("error = %v", err)
		}
	})
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestMaterializeReportsArchiveCommandFailure(t *testing.T) {
	if err := materialize(filepath.Join(t.TempDir(), "missing"), "HEAD", t.TempDir()); !errors.Is(err, ErrBuild) {
		t.Fatalf("error = %v", err)
	}
}
