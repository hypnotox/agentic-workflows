package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
)

type filesystemProjectReader struct{ root string }

func (r filesystemProjectReader) ReadFile(path string) ([]byte, bool, error) {
	b, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(path)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return slices.Clone(b), true, nil
}
func (r filesystemProjectReader) Entries(prefix string) ([]generatedcheck.TreeEntry, error) {
	var out []generatedcheck.TreeEntry
	base := filepath.Join(r.root, filepath.FromSlash(prefix))
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == base && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return err
		}
		rel, e := filepath.Rel(r.root, p)
		if e != nil { // coverage-ignore: WalkDir supplies paths rooted beneath r.root, so Rel cannot fail on a supported platform
			return e
		}
		out = append(out, generatedcheck.TreeEntry{Path: filepath.ToSlash(rel), Directory: d.IsDir()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enumerate %s: %w", prefix, err)
	}
	slices.SortFunc(out, func(a, b generatedcheck.TreeEntry) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

func (r filesystemProjectReader) Paths(prefix string) ([]string, error) {
	var out []string
	base := filepath.Join(r.root, filepath.FromSlash(prefix))
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == base && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return err
		}
		if !d.IsDir() {
			rel, e := filepath.Rel(r.root, p)
			if e != nil { // coverage-ignore: WalkDir supplies a path under r.root, so Rel cannot fail on a supported platform
				return e
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		subject := prefix
		if subject == "" {
			subject = "project tree"
		}
		return nil, fmt.Errorf("enumerate %s: %w", subject, err)
	}
	slices.Sort(out)
	return out, nil
}
