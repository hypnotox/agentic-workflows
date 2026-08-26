//go:build windows

package filesystem

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const volumeNameGUID = 0x1

// CanonicalRoot returns an opened directory's final GUID-volume path so drive,
// volume, symlink, and case aliases select one advisory-lock identity.
func CanonicalRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
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
	resolved := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(handle, &resolved[0], uint32(len(resolved)), volumeNameGUID)
	if err != nil {
		return "", fmt.Errorf("resolve final path: %w", err)
	}
	if n >= uint32(len(resolved)) {
		resolved = make([]uint16, n+1)
		n, err = windows.GetFinalPathNameByHandle(handle, &resolved[0], uint32(len(resolved)), volumeNameGUID)
		if err != nil {
			return "", fmt.Errorf("resolve final path: %w", err)
		}
	}
	return strings.ToLower(windows.UTF16ToString(resolved[:n])), nil
}
