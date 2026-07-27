// Package contextdelivery enforces the terminal-size boundary for context output.
package contextdelivery

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxDirectBytes = 8192

var (
	tempDir       = os.TempDir
	canonicalPath = func(p string) (string, error) {
		absolute, err := filepath.Abs(p)
		if err != nil { // coverage-ignore: production supplies absolute temp and repository paths, so Abs cannot consult a missing working directory
			return "", err
		}
		return filepath.EvalSymlinks(absolute)
	}
	createTemp = func(dir, pattern string) (spillFile, error) { return os.CreateTemp(dir, pattern) }
	removeFile = os.Remove
)

type spillFile interface {
	io.Writer
	Stat() (os.FileInfo, error)
	Close() error
	Name() string
}

// Deliver writes rendered unchanged through 8192 bytes and otherwise securely
// spills the exact bytes outside repositoryRoot and writes the versioned notice.
func Deliver(rendered []byte, repositoryRoot string, stdout io.Writer) error {
	if len(rendered) <= MaxDirectBytes {
		if err := writeFull(stdout, rendered); err != nil {
			return fmt.Errorf("write context: %w", err)
		}
		return nil
	}
	tmp, err := canonicalPath(tempDir())
	if err != nil {
		return fmt.Errorf("canonicalize temporary directory: %w", err)
	}
	root, err := canonicalPath(repositoryRoot)
	if err != nil {
		return fmt.Errorf("canonicalize repository root: %w", err)
	}
	if strings.ContainsAny(tmp, "\r\n") || strings.ContainsAny(root, "\r\n") || containedBy(root, tmp) {
		return errors.New("temporary directory must be outside the repository")
	}
	f, err := createTemp(tmp, "awf-context-*.txt")
	if err != nil {
		return fmt.Errorf("create context spill: %w", err)
	}
	cleanup := func() { _ = removeFile(f.Name()) }
	fail := func(stage string, err error) error { cleanup(); return fmt.Errorf("%s: %w", stage, err) }
	info, err := f.Stat()
	if err != nil {
		return fail("inspect context spill", err)
	}
	if info.Mode().Perm() != 0o600 {
		return fail("secure context spill", fmt.Errorf("mode is %04o, want 0600", info.Mode().Perm()))
	}
	name, err := canonicalPath(f.Name())
	if err != nil {
		return fail("canonicalize context spill", err)
	}
	if !filepath.IsAbs(name) || strings.ContainsAny(name, "\r\n") || containedBy(root, name) || filepath.Dir(name) != tmp {
		return fail("secure context spill", errors.New("unsafe spill path"))
	}
	if err := writeFull(f, rendered); err != nil {
		return fail("write context spill", err)
	}
	if err := f.Close(); err != nil {
		return fail("close context spill", err)
	}
	first := []byte(fmt.Sprintf("AWF_CONTEXT_SPILL_V1 bytes=%d format=text\n", len(rendered)))
	if err := writeFull(stdout, first); err != nil {
		return fail("write context spill notice first line", err)
	}
	if err := writeFull(stdout, []byte(name)); err != nil {
		return fail("write context spill notice second line", err)
	}
	if err := writeFull(stdout, []byte("\n")); err != nil {
		return fail("write context spill notice final newline", err)
	}
	return nil
}

func containedBy(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}
func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
