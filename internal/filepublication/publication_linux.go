//go:build linux

package filepublication

import "golang.org/x/sys/unix"

func publishNoReplace(temporary, path string) error {
	return unix.Renameat2(unix.AT_FDCWD, temporary, unix.AT_FDCWD, path, unix.RENAME_NOREPLACE)
}
