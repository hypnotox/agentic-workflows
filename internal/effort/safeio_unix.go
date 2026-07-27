//go:build linux || darwin

package effort

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func ownerOK(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return !ok || stat.Uid == uint32(os.Geteuid())
}

func validatePathOwner(path string, info os.FileInfo, _ *os.File) error {
	if !ownerOK(info) { // coverage-ignore: exercised only when the test process has privilege to create a foreign-owned fixture
		return safety("foreign-owner", path, nil)
	}
	return nil
}

func platformLstatRegular(path string) (fileIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fileIdentity{}, fmt.Errorf("lstat %s: %w", path, err)
	}
	if err := validateLeaf(path, info); err != nil {
		return fileIdentity{}, err
	}
	if err := validatePathOwner(path, info, nil); err != nil { // coverage-ignore: requires a foreign-owned fixture created by a privileged test process
		return fileIdentity{}, err
	}
	return fileIdentity{info: info}, nil
}

func platformOpenRegularNoFollow(path string, create bool, mode os.FileMode) (*os.File, error) {
	flags := unix.O_RDONLY
	if create {
		flags = unix.O_CREAT | unix.O_RDWR
	}
	fd, err := unix.Open(path, flags|unix.O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, safety("symlink", path, err)
		}
		return nil, fmt.Errorf("open %s without following links: %w", path, err)
	}
	return os.NewFile(uintptr(fd), path), nil
}

func validateOpenedFile(string, *os.File) error { return nil }

func validateLockPermissions(path string, info os.FileInfo) error {
	if info.Mode().Perm() != 0o600 {
		return safety("unsafe-lock", path, fmt.Errorf("mode is %o, want 600", info.Mode().Perm()))
	}
	return nil
}

func lockRepositoryFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX)
}

func unlockRepositoryFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

func openDirectoryForSync(path string) (durableFile, error) { return os.Open(path) }
