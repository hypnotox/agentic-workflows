//go:build windows

package git

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

type windowsComponentAPI struct {
	attributes func(windows.Handle) (uint32, error)
	ownerSID   func(windows.Handle) (string, error)
	currentSID func() (string, error)
}

var nativeWindowsComponentAPI = windowsComponentAPI{
	attributes: func(handle windows.Handle) (uint32, error) {
		var info windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
			return 0, err
		}
		return info.FileAttributes, nil
	},
	ownerSID: func(handle windows.Handle) (string, error) {
		descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
		if err != nil {
			return "", err
		}
		owner, _, err := descriptor.Owner()
		if err != nil {
			return "", err
		}
		return owner.String(), nil
	},
	currentSID: func() (string, error) {
		token, err := windows.OpenCurrentProcessToken()
		if err != nil {
			return "", err
		}
		defer token.Close()
		user, err := token.GetTokenUser()
		if err != nil {
			return "", err
		}
		return user.User.Sid.String(), nil
	},
}

// ownedByCurrentUser deliberately rejects Windows FileInfo metadata because it
// carries no SID. Windows production ownership checks use security descriptors
// on no-follow handles through platformLstatComponent.
func ownedByCurrentUser(os.FileInfo) bool { return false }

func validateWindowsComponent(path string, handle windows.Handle, requireOwner bool, api windowsComponentAPI) error {
	attributes, err := api.attributes(handle)
	if err != nil {
		return fmt.Errorf("inspect Windows component %s: %w", path, err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return &HardSafetyError{Category: "symlink", Path: path, Err: errors.New("component is a reparse point")}
	}
	if !requireOwner {
		return nil
	}
	owner, err := api.ownerSID(handle)
	if err != nil {
		return fmt.Errorf("inspect Windows owner %s: %w", path, err)
	}
	current, err := api.currentSID()
	if err != nil {
		return fmt.Errorf("read Windows process owner for %s: %w", path, err)
	}
	if owner != current {
		return &HardSafetyError{Category: "foreign-owner", Path: path}
	}
	return nil
}

func platformLstatComponent(path string, requireOwner bool) (os.FileInfo, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("encode Windows component %s: %w", path, err)
	}
	handle, err := windows.CreateFile(name, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, fmt.Errorf("open Windows component %s without following reparse points: %w", path, err)
	}
	file := os.NewFile(uintptr(handle), path)
	defer file.Close()
	if err := validateWindowsComponent(path, handle, requireOwner, nativeWindowsComponentAPI); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened Windows component %s: %w", path, err)
	}
	return info, nil
}
