//go:build windows

package adr

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const volumeNameGUID = 0x1

// canonicalDecisionsDirectory uses the opened directory's final GUID-volume path so drive,
// volume, symlink, and case aliases select one advisory-lock identity.
func canonicalDecisionsDirectory(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("make absolute: %w", err)
	}
	handle, err := windows.CreateFile(windows.StringToUTF16Ptr(absolute), windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", fmt.Errorf("open directory: %w", err)
	}
	defer windows.CloseHandle(handle)
	path := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(handle, &path[0], uint32(len(path)), volumeNameGUID)
	if err != nil {
		return "", fmt.Errorf("resolve final path: %w", err)
	}
	if n >= uint32(len(path)) {
		path = make([]uint16, n+1)
		n, err = windows.GetFinalPathNameByHandle(handle, &path[0], uint32(len(path)), volumeNameGUID)
		if err != nil {
			return "", fmt.Errorf("resolve final path: %w", err)
		}
	}
	return strings.ToLower(windows.UTF16ToString(path[:n])), nil
}
