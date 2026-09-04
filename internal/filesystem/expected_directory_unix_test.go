//go:build linux || darwin

package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestExpectedEmptyDirectoryRemovalDoesNotBlockOnSpecialFileReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "owned")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	h, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	expected, err := h.ExpectedIdentity("owned")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() { result <- h.RemoveExpectedEmptyDirectory("owned", expected, 0o755) }()
	select {
	case err := <-result:
		if !errors.Is(err, ErrIdentityChanged) {
			t.Fatalf("special-file replacement error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected directory removal blocked on special-file replacement")
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("special-file replacement was not preserved: info=%v err=%v", info, err)
	}
}
