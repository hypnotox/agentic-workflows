//go:build darwin

package filepublication

import "golang.org/x/sys/unix"

func publishNoReplace(temporary, path string) error {
	return unix.RenamexNp(temporary, path, unix.RENAME_EXCL)
}
