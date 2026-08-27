//go:build linux

package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkInfoDoesNotRetainDescriptorsAndExpectedIdentityReleaseIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "entry"), []byte("entry"), 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	before, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for range 256 {
		if _, err := h.LinkInfo("entry"); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) > len(before)+1 {
		t.Fatalf("LinkInfo retained descriptors: before %d, after %d", len(before), len(after))
	}
	identity, err := h.ExpectedIdentity("entry")
	if err != nil {
		t.Fatal(err)
	}
	retained, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) <= len(before) {
		t.Fatalf("ExpectedIdentity did not retain a descriptor: before %d, while held %d", len(before), len(retained))
	}
	if err := identity.Release(); err != nil {
		t.Fatal(err)
	}
	if identity.SameFile(mustLinkInfo(t, h, "entry")) {
		t.Fatal("released identity remained valid")
	}
	if err := identity.Release(); err != nil {
		t.Fatalf("second identity release: %v", err)
	}
	released, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(released) > len(before)+1 {
		t.Fatalf("released identity retained descriptor: before %d, after %d", len(before), len(released))
	}
}

func mustLinkInfo(t *testing.T, h *Handle, name string) os.FileInfo {
	t.Helper()
	info, err := h.LinkInfo(name)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
