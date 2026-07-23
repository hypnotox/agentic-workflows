//go:build !windows

package dashboardruntime

import (
	"fmt"
	"os"
	"syscall"
)

var (
	lockLstat    = os.Lstat
	lockOpenFile = os.OpenFile
	lockFlock    = syscall.Flock
)

type advisoryLock struct {
	file *os.File
}

func acquireAdvisoryLock(path string) (*advisoryLock, error) {
	if info, err := lockLstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 || !ownedByCurrentUser(info) {
			return nil, fmt.Errorf("%w: lock is not an owner-private regular file", ErrUnsafePath)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: inspect lock: %w", ErrUnsafePath, err)
	}
	file, err := lockOpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%w: open lock: %w", ErrUnsafePath, err)
	}
	if err := lockFlock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("%w: acquire lock: %w", ErrBuild, err)
	}
	return &advisoryLock{file: file}, nil
}

func (lock *advisoryLock) release() {
	_ = syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	_ = lock.file.Close()
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}
