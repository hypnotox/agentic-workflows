//go:build !aix && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris

package filesystem

import "os"

func openExpectedEntry(root *os.Root, path string) (*os.File, error) {
	return root.Open(path)
}
