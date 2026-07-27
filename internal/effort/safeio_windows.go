//go:build windows

package effort

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func ownerOK(os.FileInfo) bool { return true }

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
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("encode path %s for no-follow open: %w", path, err)
	}
	access := uint32(windows.GENERIC_READ)
	disposition := uint32(windows.OPEN_EXISTING)
	if create {
		access |= windows.GENERIC_WRITE
		disposition = windows.OPEN_ALWAYS
	}
	handle, err := windows.CreateFile(name, access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, disposition, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s without following reparse points: %w", path, err)
	}
	return os.NewFile(uintptr(handle), path), nil
}

func validateOpenedFile(path string, file *os.File) error {
	handle := windows.Handle(file.Fd())
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return fmt.Errorf("inspect Windows file identity %s: %w", path, err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return safety("symlink", path, errors.New("leaf is a reparse point"))
	}
	if info.NumberOfLinks != 1 {
		return safety("identity", path, fmt.Errorf("regular file has %d links, want 1", info.NumberOfLinks))
	}
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("inspect Windows file owner %s: %w", path, err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return fmt.Errorf("read Windows file owner %s: %w", path, err)
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("open Windows process token for %s: %w", path, err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("read Windows process owner for %s: %w", path, err)
	}
	if !owner.Equals(user.User.Sid) {
		return safety("foreign-owner", path, nil)
	}
	return nil
}

func validateLockPermissions(string, os.FileInfo) error {
	// Windows has no Unix mode bits. The no-follow opener validates that the
	// lock is a regular, single-link file owned by the process user, while the
	// owner-controlled resident directory confines creation and replacement.
	return nil
}

func lockRepositoryFile(file *os.File) error {
	var overlapped windows.Overlapped
	return windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped)
}

func unlockRepositoryFile(file *os.File) error {
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
}

type windowsDirectorySyncFile struct{ *os.File }

func (windowsDirectorySyncFile) Sync() error {
	// Windows publication uses MOVEFILE_WRITE_THROUGH or ReplaceFileW. Those
	// primitives complete the namespace transaction durably; directory handles
	// do not support the Unix fsync contract.
	return nil
}

func openDirectoryForSync(path string) (durableFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return windowsDirectorySyncFile{File: file}, nil
}
