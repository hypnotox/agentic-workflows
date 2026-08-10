//go:build windows

package filepublication

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type windowsPublicationAPI struct {
	move func(string, string, uint32) error
}

var nativeWindowsPublicationAPI = windowsPublicationAPI{
	move: func(fromPath, toPath string, flags uint32) error {
		from, err := windows.UTF16PtrFromString(fromPath)
		if err != nil {
			return fmt.Errorf("encode publication source %s: %w", fromPath, err)
		}
		to, err := windows.UTF16PtrFromString(toPath)
		if err != nil {
			return fmt.Errorf("encode publication destination %s: %w", toPath, err)
		}
		return windows.MoveFileEx(from, to, flags)
	},
}

func publishNoReplace(temporary, path string) error {
	return publishNoReplaceWindows(temporary, path, nativeWindowsPublicationAPI)
}

func publishNoReplaceWindows(temporary, path string, api windowsPublicationAPI) error {
	return api.move(temporary, path, windows.MOVEFILE_WRITE_THROUGH)
}
