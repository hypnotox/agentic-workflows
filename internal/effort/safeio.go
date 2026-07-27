package effort

import (
	"errors"
	"fmt"
	"io"
	"os"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type fileIdentity struct{ info os.FileInfo }

func safety(category, path string, err error) error {
	return &awfgit.HardSafetyError{Category: category, Path: path, Err: err}
}

func validateLeaf(path string, info os.FileInfo) error {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return safety("symlink", path, nil)
	case !info.Mode().IsRegular():
		return safety("file-type", path, fmt.Errorf("mode %s is not a regular file", info.Mode()))
	case linkCount(info) != 1:
		return safety("identity", path, fmt.Errorf("regular file has %d links, want 1", linkCount(info)))
	}
	return nil
}

func lstatRegular(path string) (fileIdentity, error) {
	return platformLstatRegular(path)
}

func openRegularNoFollow(path string, create bool, mode os.FileMode) (*os.File, fileIdentity, error) {
	file, err := platformOpenRegularNoFollow(path, create, mode)
	if err != nil {
		return nil, fileIdentity{}, err
	}
	closeOnError := func(err error) (*os.File, fileIdentity, error) {
		_ = file.Close()
		return nil, fileIdentity{}, err
	}
	opened, err := file.Stat()
	if err != nil { // coverage-ignore: the platform opener returned a live descriptor; this branch requires a kernel-level metadata failure
		return closeOnError(fmt.Errorf("inspect opened file %s: %w", path, err))
	}
	if err := validateLeaf(path, opened); err != nil {
		return closeOnError(err)
	}
	if err := validatePathOwner(path, opened, file); err != nil { // coverage-ignore: requires a foreign-owned fixture created by a privileged test process
		return closeOnError(err)
	}
	if err := validateOpenedFile(path, file); err != nil { // coverage-ignore: Unix has no additional handle validation; Windows exercises this branch in platform tests
		return closeOnError(err)
	}
	resident, err := os.Lstat(path)
	if err != nil { // coverage-ignore: the no-follow open just proved this name exists; failure requires a concurrent namespace race
		return closeOnError(fmt.Errorf("re-lstat %s: %w", path, err))
	}
	if err := validateLeaf(path, resident); err != nil { // coverage-ignore: changing the validated leaf between adjacent open and lstat calls requires a concurrent namespace race
		return closeOnError(err)
	}
	if !os.SameFile(opened, resident) { // coverage-ignore: changing identity between adjacent open and lstat calls requires a concurrent namespace race
		return closeOnError(safety("identity", path, errors.New("leaf changed while opening")))
	}
	return file, fileIdentity{info: opened}, nil
}

func readRegularNoFollow(path string) ([]byte, fileIdentity, error) {
	file, identity, err := openRegularNoFollow(path, false, 0)
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
	if err := validatePathOwner(path, info, nil); err != nil { // coverage-ignore: requires a foreign-owned fixture created by a privileged test process
		return err
	}
	return os.ErrExist
}
