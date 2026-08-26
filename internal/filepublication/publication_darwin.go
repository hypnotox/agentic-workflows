//go:build darwin

package filepublication

import (
	"os"

	"golang.org/x/sys/unix"
)

func publishNoReplace(temporary, path string) error {
	return unix.RenamexNp(temporary, path, unix.RENAME_EXCL)
}

func publishNoReplaceAt(anchor *os.File, temporary, path string) error {
	return unix.RenameatxNp(int(anchor.Fd()), temporary, int(anchor.Fd()), path, unix.RENAME_EXCL)
}
