//go:build linux

package effort

import (
	"os"
	"path/filepath"
	"testing"
)

var _ func(os.FileInfo) uint64 = linkCount

func TestLinuxLinkCountConvertsPlatformWidth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")
	if err := os.WriteFile(path, []byte("record"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := path + ".link"
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	got := linkCount(info)
	if got != 2 {
		t.Fatalf("link count = %d, want 2", got)
	}
}
