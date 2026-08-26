package plan

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

// SetNowForTest overrides the now seam for a test and returns the previous
// value, so the caller can restore it. It lives in an in-package _test.go file
// (package plan) so the external plan_test package can reach it without the seam
// shipping in the production binary (mirrors adr.SetNowForTest, ADR-0063).
func SetNowForTest(fn func() time.Time) (prev func() time.Time) {
	prev = now
	now = fn
	return prev
}

// NewFile exists only in plan's test binary to preserve external package test
// coverage while production callers must provide the confined capability.
func NewFile(dir, title string) (returnPath string, returnErr error) {
	files, err := filesystem.Open(dir)
	if err != nil {
		return "", err
	}
	defer files.Close()
	lease, err := filesystem.AcquireTrackedLease(context.Background(), dir)
	if err != nil {
		return "", err
	}
	defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
	path, err := NewFileLeased(dir, lease, files, filepath.ToSlash("."), title)
	if err != nil {
		slug, slugErr := slugify(strings.TrimSpace(title))
		if slugErr != nil {
			return "", err
		}
		rel := now().Format("2006-01-02") + "-" + slug + ".md"
		return "", errors.New(strings.ReplaceAll(err.Error(), rel, filepath.Join(dir, rel)))
	}
	return filepath.Join(dir, filepath.FromSlash(path)), nil
}
