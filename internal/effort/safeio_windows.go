//go:build windows

package effort

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

type windowsSecurityAPI struct {
	information func(windows.Handle) (uint32, uint32, error)
	ownerSID    func(windows.Handle) (string, error)
	currentSID  func() (string, error)
}

var nativeWindowsSecurityAPI = windowsSecurityAPI{
	information: func(handle windows.Handle) (uint32, uint32, error) {
		var info windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
			return 0, 0, err
		}
		return info.FileAttributes, info.NumberOfLinks, nil
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

func linkCount(os.FileInfo) uint64 { return 1 }

func platformLstatRegular(path string) (fileIdentity, error) {
	file, identity, err := openRegularNoFollow(path, false, 0)
	if err != nil {
		return fileIdentity{}, err
	}
	if err := file.Close(); err != nil {
		return fileIdentity{}, fmt.Errorf("close %s after identity inspection: %w", path, err)
	}
	return identity, nil
}

func platformOpenRegularNoFollow(path string, create bool, _ os.FileMode) (*os.File, error) {
	access := uint32(windows.GENERIC_READ)
	disposition := uint32(windows.OPEN_EXISTING)
	if create {
		access |= windows.GENERIC_WRITE
		disposition = windows.OPEN_ALWAYS
	}
	return openWindowsPathNoFollow(path, access, disposition, false)
}

func openWindowsPathNoFollow(path string, access, disposition uint32, directory bool) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("encode path %s for no-follow open: %w", path, err)
	}
	flags := uint32(windows.FILE_ATTRIBUTE_NORMAL | windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if directory {
		flags |= windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	handle, err := windows.CreateFile(name, access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, disposition, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s without following reparse points: %w", path, err)
	}
	return os.NewFile(uintptr(handle), path), nil
}

func validateWindowsOwner(path string, handle windows.Handle, api windowsSecurityAPI) error {
	owner, err := api.ownerSID(handle)
	if err != nil {
		return fmt.Errorf("inspect Windows owner %s: %w", path, err)
	}
	current, err := api.currentSID()
	if err != nil {
		return fmt.Errorf("read Windows process owner for %s: %w", path, err)
	}
	if owner != current {
		return safety("foreign-owner", path, nil)
	}
	return nil
}

func validatePathOwner(path string, info os.FileInfo, opened *os.File) error {
	file := opened
	closeFile := false
	if file == nil {
		var err error
		file, err = openWindowsPathNoFollow(path, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES, windows.OPEN_EXISTING, info.IsDir())
		if err != nil {
			return err
		}
		closeFile = true
	}
	if closeFile {
		defer file.Close()
	}
	attributes, _, err := nativeWindowsSecurityAPI.information(windows.Handle(file.Fd()))
	if err != nil {
		return fmt.Errorf("inspect Windows file identity %s: %w", path, err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return safety("symlink", path, errors.New("path is a reparse point"))
	}
	return validateWindowsOwner(path, windows.Handle(file.Fd()), nativeWindowsSecurityAPI)
}

func validateOpenedFile(path string, file *os.File) error {
	attributes, links, err := nativeWindowsSecurityAPI.information(windows.Handle(file.Fd()))
	if err != nil {
		return fmt.Errorf("inspect Windows file identity %s: %w", path, err)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return safety("symlink", path, errors.New("leaf is a reparse point"))
	}
	if links != 1 {
		return safety("identity", path, fmt.Errorf("regular file has %d links, want 1", links))
	}
	return nil
}

type windowsDirectorySyncFile struct{ *os.File }

func (windowsDirectorySyncFile) Sync() error {
	// Windows documents no directory-handle equivalent of Unix fsync. Atomic
	// publication flushes the published file handle before this bookkeeping step.
	return nil
}

func openDirectoryForSync(path string) (durableFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return windowsDirectorySyncFile{File: file}, nil
}
