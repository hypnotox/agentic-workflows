//go:build windows

package filepublication

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func publishNoReplace(temporary, path string) error {
	return publishNoReplaceWindows(temporary, path, moveWindowsFile)
}

func publishNoReplaceWindows(temporary, path string, move func(string, string, uint32) error) error {
	return move(temporary, path, windows.MOVEFILE_WRITE_THROUGH)
}

func publishNoReplaceAt(anchor *os.File, temporary, path string) error {
	rootName, err := finalPathByHandle(windows.Handle(anchor.Fd()))
	if err != nil {
		return err
	}
	return publishNoReplace(
		filepath.Join(rootName, filepath.FromSlash(temporary)),
		filepath.Join(rootName, filepath.FromSlash(path)),
	)
}

func finalPathByHandle(handle windows.Handle) (string, error) {
	const volumeNameGUID = 0x1
	resolved := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(handle, &resolved[0], uint32(len(resolved)), volumeNameGUID)
	if err != nil {
		return "", fmt.Errorf("resolve publication parent: %w", err)
	}
	if n >= uint32(len(resolved)) {
		resolved = make([]uint16, n+1)
		n, err = windows.GetFinalPathNameByHandle(handle, &resolved[0], uint32(len(resolved)), volumeNameGUID)
		if err != nil {
			return "", fmt.Errorf("resolve publication parent: %w", err)
		}
	}
	return windows.UTF16ToString(resolved[:n]), nil
}

func moveWindowsFile(fromPath, toPath string, flags uint32) error {
	from, err := windows.UTF16PtrFromString(fromPath)
	if err != nil {
		return fmt.Errorf("encode publication source %s: %w", fromPath, err)
	}
	to, err := windows.UTF16PtrFromString(toPath)
	if err != nil {
		return fmt.Errorf("encode publication destination %s: %w", toPath, err)
	}
	return windows.MoveFileEx(from, to, flags)
}
