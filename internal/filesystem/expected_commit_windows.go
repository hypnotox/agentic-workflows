//go:build windows

package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"golang.org/x/sys/windows"
)

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func exchangeExpected(root *os.Root, temporary, destination string, expected fs.FileInfo, remove bool) (bool, error) {
	anchor, err := root.Open(".")
	if err != nil {
		return false, fmt.Errorf("filesystem: open atomic root: %w", err)
	}
	defer anchor.Close()
	return exchangeExpectedAnchored(root, anchor, temporary, destination, expected, remove)
}

func exchangeExpectedAnchored(root *os.Root, anchor *os.File, temporary, destination string, expected fs.FileInfo, remove bool) (bool, error) {
	rootName, err := finalWindowsPath(windows.Handle(anchor.Fd()))
	if err != nil {
		return false, err
	}
	temporaryAbs := filepath.Join(rootName, filepath.FromSlash(temporary))
	destinationAbs := filepath.Join(rootName, filepath.FromSlash(destination))
	if remove {
		return removeExpectedWindows(root, temporary, temporaryAbs, destination, destinationAbs, expected)
	}
	displaced := temporary + ".displaced"
	displacedAbs := filepath.Join(rootName, filepath.FromSlash(displaced))
	if err := replaceWindowsFile(destinationAbs, temporaryAbs, displacedAbs); err != nil {
		return false, fmt.Errorf("filesystem: exchange expected entry %q: %w", destination, err)
	}
	displacedInfo, inspectErr := root.Lstat(displaced)
	if inspectErr != nil || !os.SameFile(expected, displacedInfo) {
		mismatch := error(ErrIdentityChanged)
		if inspectErr != nil {
			mismatch = errors.Join(mismatch, inspectErr)
		}
		if rollbackErr := replaceWindowsFile(destinationAbs, displacedAbs, temporaryAbs); rollbackErr != nil {
			cause := errors.Join(mismatch, fmt.Errorf("restore unexpected entry at %q: %w", destination, rollbackErr))
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: displaced, Cause: cause}
		}
		return false, mismatch
	}
	if err := root.Remove(displaced); err != nil {
		return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: displaced, Cause: err}
	}
	return true, nil
}

func removeExpectedWindows(root *os.Root, temporary, temporaryAbs, destination, destinationAbs string, expected fs.FileInfo) (bool, error) {
	if err := root.Remove(temporary); err != nil {
		return false, fmt.Errorf("filesystem: remove atomic marker %q: %w", temporary, err)
	}
	if err := filepublication.MoveNoReplace(destinationAbs, temporaryAbs); err != nil {
		return true, fmt.Errorf("filesystem: move expected entry %q aside: %w", destination, err)
	}
	displaced, inspectErr := root.Lstat(temporary)
	if inspectErr != nil || !os.SameFile(expected, displaced) {
		mismatch := error(ErrIdentityChanged)
		if inspectErr != nil {
			mismatch = errors.Join(mismatch, inspectErr)
		}
		if rollbackErr := filepublication.MoveNoReplace(temporaryAbs, destinationAbs); rollbackErr != nil {
			cause := errors.Join(mismatch, fmt.Errorf("restore unexpected entry at %q: %w", destination, rollbackErr))
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
		}
		return true, mismatch
	}
	if err := root.Remove(temporary); err != nil {
		return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: err}
	}
	return true, nil
}

func finalWindowsPath(handle windows.Handle) (string, error) {
	resolved := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(handle, &resolved[0], uint32(len(resolved)), volumeNameGUID)
	if err != nil {
		return "", fmt.Errorf("resolve atomic parent: %w", err)
	}
	if n >= uint32(len(resolved)) {
		resolved = make([]uint16, n+1)
		n, err = windows.GetFinalPathNameByHandle(handle, &resolved[0], uint32(len(resolved)), volumeNameGUID)
		if err != nil {
			return "", fmt.Errorf("resolve atomic parent: %w", err)
		}
	}
	return windows.UTF16ToString(resolved[:n]), nil
}

func replaceWindowsFile(destination, replacement, displaced string) error {
	destinationPtr, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	replacementPtr, err := windows.UTF16PtrFromString(replacement)
	if err != nil {
		return err
	}
	displacedPtr, err := windows.UTF16PtrFromString(displaced)
	if err != nil {
		return err
	}
	result, _, callErr := replaceFileW.Call(uintptr(unsafe.Pointer(destinationPtr)), uintptr(unsafe.Pointer(replacementPtr)), uintptr(unsafe.Pointer(displacedPtr)), 0, 0, 0)
	if result == 0 {
		if callErr == syscall.Errno(0) {
			callErr = syscall.EINVAL
		}
		return callErr
	}
	return nil
}
