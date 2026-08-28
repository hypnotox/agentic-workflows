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
	if err := validateOpenedFile(path, file); err != nil { // coverage-ignore: Linux and Darwin use no additional handle validation, so the defensive error path remains unreachable on every supported runtime
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

func readRegularNoFollow(path string) ([]byte, error) {
	return readRegularNoFollowBounded(path, -1)
}

// readRegularNoFollowBounded preserves the resident no-follow/current-owner
// proof while rejecting an oversized advisory record before decoding it.
func readRegularNoFollowBounded(path string, limit int64) ([]byte, error) {
	raw, _, err := readRegularNoFollowBoundedIdentity(path, limit)
	return raw, err
}

// readRegularNoFollowBoundedIdentity returns content and the identity proven to
// remain resident through the read, so callers can conditionally publish over
// precisely the claim they inspected.
func readRegularNoFollowBoundedIdentity(path string, limit int64) ([]byte, fileIdentity, error) {
	file, identity, err := openRegularNoFollow(path, false, 0)
	if err != nil {
		return nil, fileIdentity{}, err
	}
	var reader io.Reader = file
	if limit >= 0 {
		reader = io.LimitReader(file, limit+1)
	}
	raw, readErr := io.ReadAll(reader)
	closeErr := file.Close()
	if readErr != nil { // coverage-ignore: a validated local regular file read fails only on a kernel or storage fault
		return nil, fileIdentity{}, fmt.Errorf("read %s: %w", path, readErr)
	}
	if limit >= 0 && int64(len(raw)) > limit {
		return nil, fileIdentity{}, fmt.Errorf("read %s: exceeds %d byte bound", path, limit)
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

type publicationIdentityError struct{ err error }

func (e *publicationIdentityError) Error() string { return e.err.Error() }
func (e *publicationIdentityError) Unwrap() error { return e.err }
func publicationIdentityRefusal(err error) error  { return &publicationIdentityError{err: err} }

// residentOwner is the ownership predicate behind ValidateCurrentOwner. It is a
// variable so the foreign-owner refusal is provable without a privileged test
// process able to create a foreign-owned fixture; production always binds the
// platform check. internal/worktree keeps the same seam as managedOwner.
var residentOwner = validatePathOwner

// ValidateCurrentOwner applies the platform's no-follow owner check to an existing path.
func ValidateCurrentOwner(path string, info os.FileInfo) error {
	return residentOwner(path, info, nil)
}
