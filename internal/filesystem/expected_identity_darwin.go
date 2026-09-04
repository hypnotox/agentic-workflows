//go:build darwin

package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"golang.org/x/sys/unix"
)

// ExpectedIdentity pins one leaf for an expected mutation. The caller must
// Release an identity it abandons; expected-mutation methods consume it.
func (h *Handle) ExpectedIdentity(name string) (*ExpectedIdentity, error) {
	if err := validPath(name); err != nil {
		return nil, fmt.Errorf("filesystem: expected identity %q: %w", name, err)
	}
	parent := path.Dir(name)
	parentRoot, err := h.root.OpenRoot(parent)
	if err != nil {
		return nil, fmt.Errorf("filesystem: expected identity parent %q: %w", name, err)
	}
	defer parentRoot.Close()
	anchor, err := parentRoot.Open(".")
	if err != nil {
		return nil, fmt.Errorf("filesystem: expected identity anchor %q: %w", name, err)
	}
	defer anchor.Close()
	flags := unix.O_RDONLY | unix.O_NONBLOCK | unix.O_NOFOLLOW | unix.O_CLOEXEC
	fd, err := unix.Openat(int(anchor.Fd()), path.Base(name), flags, 0)
	if errors.Is(err, unix.ELOOP) {
		fd, err = unix.Openat(int(anchor.Fd()), path.Base(name), flags|unix.O_SYMLINK, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("filesystem: expected identity %q: %w", name, err)
	}
	file := os.NewFile(uintptr(fd), name)
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("filesystem: expected identity %q: %w", name, err)
	}
	return &ExpectedIdentity{info: info, release: file.Close}, nil
}

var _ fs.FileInfo = (*ExpectedIdentity)(nil)
