package effort

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type fileIdentity struct{ info os.FileInfo }

func safety(category, path string, err error) error {
	return &awfgit.HardSafetyError{Category: category, Path: path, Err: err}
}

func ownerOK(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return !ok || stat.Uid == uint32(os.Geteuid())
}

func validateLeaf(path string, info os.FileInfo) error {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return safety("symlink", path, nil)
	case !info.Mode().IsRegular():
		return safety("file-type", path, fmt.Errorf("mode %s is not a regular file", info.Mode()))
	case !ownerOK(info): // coverage-ignore: exercised only when the test process has privilege to create a foreign-owned fixture
		return safety("foreign-owner", path, nil)
	case linkCount(info) != 1:
		return safety("identity", path, fmt.Errorf("regular file has %d links, want 1", linkCount(info)))
	}
	return nil
}

func linkCount(info os.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Nlink
	}
	return 1 // coverage-ignore: this package's flock and O_NOFOLLOW implementation targets platforms whose os.FileInfo carries syscall.Stat_t
}

func lstatRegular(path string) (fileIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fileIdentity{}, fmt.Errorf("lstat %s: %w", path, err)
	}
	if err := validateLeaf(path, info); err != nil {
		return fileIdentity{}, err
	}
	return fileIdentity{info: info}, nil
}

func openRegularNoFollow(path string, flags int, mode os.FileMode) (*os.File, fileIdentity, error) {
	fd, err := syscall.Open(path, flags|syscall.O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return nil, fileIdentity{}, safety("symlink", path, err)
		}
		return nil, fileIdentity{}, fmt.Errorf("open %s without following links: %w", path, err)
	}
	file := os.NewFile(uintptr(fd), path)
	closeOnError := func(err error) (*os.File, fileIdentity, error) {
		_ = file.Close()
		return nil, fileIdentity{}, err
	}
	opened, err := file.Stat()
	if err != nil { // coverage-ignore: syscall.Open returned a live descriptor; this branch requires a kernel-level fstat failure
		return closeOnError(fmt.Errorf("fstat %s: %w", path, err))
	}
	if err := validateLeaf(path, opened); err != nil {
		return closeOnError(err)
	}
	resident, err := os.Lstat(path)
	if err != nil { // coverage-ignore: the no-follow open just proved this name exists; failure requires a concurrent namespace race
		return closeOnError(fmt.Errorf("re-lstat %s: %w", path, err))
	}
	if err := validateLeaf(path, resident); err != nil { // coverage-ignore: changing the validated leaf type or owner between adjacent open and lstat calls requires a concurrent namespace race
		return closeOnError(err)
	}
	if !os.SameFile(opened, resident) { // coverage-ignore: changing identity between adjacent open and lstat calls requires a concurrent namespace race
		return closeOnError(safety("identity", path, errors.New("leaf changed while opening")))
	}
	return file, fileIdentity{info: opened}, nil
}

func readRegularNoFollow(path string) ([]byte, fileIdentity, error) {
	file, identity, err := openRegularNoFollow(path, syscall.O_RDONLY, 0)
	if err != nil {
		return nil, fileIdentity{}, err
	}
	raw, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil { // coverage-ignore: a validated local regular file read fails only on a kernel or storage fault
		return nil, fileIdentity{}, fmt.Errorf("read %s: %w", path, readErr)
	}
	if closeErr != nil { // coverage-ignore: closing a read-only descriptor after successful ReadAll has no userspace failure trigger
		return nil, fileIdentity{}, fmt.Errorf("close %s after read: %w", path, closeErr)
	}
	resident, err := lstatRegular(path)
	if err != nil { // coverage-ignore: the leaf was validated immediately before reading; failure requires a concurrent namespace race
		return nil, fileIdentity{}, err
	}
	if !os.SameFile(identity.info, resident.info) { // coverage-ignore: replacing the leaf during a bounded read requires a concurrent namespace race
		return nil, fileIdentity{}, safety("identity", path, errors.New("leaf changed while reading"))
	}
	return raw, identity, nil
}

func requireIdentity(path string, expected fileIdentity) error {
	current, err := lstatRegular(path)
	if err != nil { // coverage-ignore: callers retain a prior identity; failure here requires a concurrent namespace race
		return err
	}
	if !os.SameFile(expected.info, current.info) {
		return safety("identity", path, errors.New("leaf was replaced before publication"))
	}
	return nil
}

func requireAbsent(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil { // coverage-ignore: local lstat reports either a resident inode or os.ErrNotExist absent a kernel fault
		return fmt.Errorf("lstat destination %s: %w", path, err)
	}
	if err := validateLeaf(path, info); err != nil {
		return err
	}
	return os.ErrExist
}
