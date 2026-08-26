//go:build linux

package filepublication

import (
	"os"

	"golang.org/x/sys/unix"
)

func publishNoReplace(temporary, path string) error {
	return unix.Renameat2(unix.AT_FDCWD, temporary, unix.AT_FDCWD, path, unix.RENAME_NOREPLACE)
}

func publishNoReplaceAt(anchor *os.File, temporary, path string) error {
	return unix.Renameat2(int(anchor.Fd()), temporary, int(anchor.Fd()), path, unix.RENAME_NOREPLACE)
}
