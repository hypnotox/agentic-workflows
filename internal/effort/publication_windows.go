//go:build windows

package effort

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

const replaceFileWriteThrough = 0x00000001

// publishAtomic uses a write-through no-replace move for creation. ReplaceFileW
// atomically saves the displaced destination before installing the new file;
// an identity mismatch is atomically restored, preserving the raced bytes.
func publishAtomic(tempPath, path string, expected *fileIdentity) error {
	if expected == nil {
		from, err := windows.UTF16PtrFromString(tempPath)
		if err != nil {
			return fmt.Errorf("encode creation source %s: %w", tempPath, err)
		}
		to, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return fmt.Errorf("encode creation destination %s: %w", path, err)
		}
		return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
	}

	displacedPath := tempPath + ".displaced"
	if err := replaceFile(path, tempPath, displacedPath); err != nil {
		return err
	}
	displaced, err := lstatRegular(displacedPath)
	mismatch := unexpectedPublicationIdentity(path, expected, displaced, err)
	if mismatch == nil {
		from, encodeErr := windows.UTF16PtrFromString(displacedPath)
		if encodeErr != nil { // coverage-ignore: displacedPath was derived from an already encoded filesystem path
			return fmt.Errorf("encode displaced effort record %s: %w", displacedPath, encodeErr)
		}
		to, encodeErr := windows.UTF16PtrFromString(tempPath)
		if encodeErr != nil { // coverage-ignore: tempPath was accepted by CreateTemp
			return fmt.Errorf("encode displaced cleanup path %s: %w", tempPath, encodeErr)
		}
		if moveErr := windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH); moveErr != nil {
			return fmt.Errorf("retain displaced effort record %s for cleanup after publishing %s: %w", displacedPath, path, moveErr)
		}
		return nil
	}
	if rollbackErr := replaceFile(path, displacedPath, tempPath); rollbackErr != nil { // coverage-ignore: requires a second namespace race or kernel fault during immediate rollback
		return errors.Join(mismatch, fmt.Errorf("restore unexpected destination at %s after refused publication: %w", path, rollbackErr))
	}
	return mismatch
}

func replaceFile(path, replacement, backup string) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode replaced path %s: %w", path, err)
	}
	replacementPtr, err := windows.UTF16PtrFromString(replacement)
	if err != nil {
		return fmt.Errorf("encode replacement path %s: %w", replacement, err)
	}
	backupPtr, err := windows.UTF16PtrFromString(backup)
	if err != nil {
		return fmt.Errorf("encode displaced path %s: %w", backup, err)
	}
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(replacementPtr)),
		uintptr(unsafe.Pointer(backupPtr)),
		replaceFileWriteThrough,
		0,
		0,
	)
	if result == 0 {
		if callErr == syscall.Errno(0) {
			callErr = syscall.EINVAL
		}
		return fmt.Errorf("ReplaceFileW %s with %s while preserving %s: %w", path, replacement, backup, callErr)
	}
	return nil
}
