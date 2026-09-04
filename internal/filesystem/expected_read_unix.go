//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package filesystem

import (
	"os"
	"syscall"
)

// openExpectedEntry opens a leaf without following it and without blocking on
// a concurrently installed special file. The caller verifies retained identity
// and entry type through the returned descriptor before reading.
func openExpectedEntry(root *os.Root, path string) (*os.File, error) {
	return root.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
