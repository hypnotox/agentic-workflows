//go:build windows

package dashboardruntime

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

type advisoryLock struct {
	file *os.File
}

func acquireAdvisoryLock(path string) (*advisoryLock, error) {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("%w: lock is not an owner-private regular file", ErrUnsafePath)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: inspect lock: %w", ErrUnsafePath, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: open lock: %w", ErrUnsafePath, err)
	}
	overlapped := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("%w: acquire lock: %w", ErrBuild, err)
	}
	return &advisoryLock{file: file}, nil
}

func (lock *advisoryLock) release() {
	overlapped := new(windows.Overlapped)
	_ = windows.UnlockFileEx(windows.Handle(lock.file.Fd()), 0, 1, 0, overlapped)
	_ = lock.file.Close()
}

func ownedByCurrentUser(os.FileInfo) bool { return true }
