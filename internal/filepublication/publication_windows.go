//go:build windows

package filepublication

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func publishNoReplace(temporary, path string) error {
	from, err := windows.UTF16PtrFromString(temporary)
	if err != nil {
		return fmt.Errorf("encode publication source %s: %w", temporary, err)
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode publication destination %s: %w", path, err)
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
}
