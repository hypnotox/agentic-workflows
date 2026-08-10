//go:build windows

package effort

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

type windowsPublicationAPI struct {
	move    func(string, string, uint32) error
	replace func(string, string, string, uint32) error
	inspect func(string) (fileIdentity, error)
	same    func(fileIdentity, fileIdentity) bool
	flush   func(string, fileIdentity) error
}

var nativeWindowsPublicationAPI = windowsPublicationAPI{
	move: func(fromPath, toPath string, flags uint32) error {
		from, err := windows.UTF16PtrFromString(fromPath)
		if err != nil {
			return fmt.Errorf("encode move source %s: %w", fromPath, err)
		}
		to, err := windows.UTF16PtrFromString(toPath)
		if err != nil {
			return fmt.Errorf("encode move destination %s: %w", toPath, err)
		}
		return windows.MoveFileEx(from, to, flags)
	},
	replace: replaceFile,
	inspect: lstatRegular,
	same: func(left, right fileIdentity) bool {
		return os.SameFile(left.info, right.info)
	},
	flush: flushPublishedWindowsFile,
}

func moveDirectoryNoReplace(fromPath, toPath string) error {
	from, err := windows.UTF16PtrFromString(fromPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(toPath)
	if err != nil {
		return err
	}
	// Without MOVEFILE_REPLACE_EXISTING, a collision is a refusal. WRITE_THROUGH
	// is Windows' documented completion boundary for this namespace move.
	return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
}

// publishAtomic uses MoveFileEx with its documented write-through flag for
// creation. ReplaceFileW accepts only its documented merge-error flags, so
// replacement uses zero flags and then reopens and flushes the published file.
// Windows documents no directory fsync primitive; the namespace operation is
// atomic, while the explicit file flush is the available durability boundary.
func publishAtomic(tempPath, path string, expected *fileIdentity) error {
	return publishAtomicWindows(tempPath, path, expected, nativeWindowsPublicationAPI)
}

func publishAtomicWindows(tempPath, path string, expected *fileIdentity, api windowsPublicationAPI) error {
	published, err := api.inspect(tempPath)
	if err != nil {
		return fmt.Errorf("inspect Windows publication source %s: %w", tempPath, err)
	}
	if expected == nil {
		if err := api.move(tempPath, path, windows.MOVEFILE_WRITE_THROUGH); err != nil {
			return err
		}
		if err := api.flush(path, published); err != nil {
			return fmt.Errorf("flush created file %s after atomic publication: %w", path, err)
		}
		return nil
	}

	displacedPath := tempPath + ".displaced"
	if err := api.replace(path, tempPath, displacedPath, 0); err != nil {
		return err
	}
	displaced, inspectErr := api.inspect(displacedPath)
	mismatch := unexpectedWindowsPublicationIdentity(path, expected, displaced, inspectErr, api.same)
	if mismatch == nil {
		if err := api.move(displacedPath, tempPath, windows.MOVEFILE_WRITE_THROUGH); err != nil {
			return fmt.Errorf("retain displaced effort record %s for cleanup after publishing %s: %w", displacedPath, path, err)
		}
		if err := api.flush(path, published); err != nil {
			return fmt.Errorf("flush replaced file %s after atomic publication: %w", path, err)
		}
		return nil
	}
	if rollbackErr := api.replace(path, displacedPath, tempPath, 0); rollbackErr != nil { // coverage-ignore: requires a second namespace race or kernel fault during immediate rollback
		return errors.Join(mismatch, fmt.Errorf("restore unexpected destination at %s after refused publication: %w", path, rollbackErr))
	}
	if inspectErr == nil {
		if flushErr := api.flush(path, displaced); flushErr != nil {
			return errors.Join(mismatch, fmt.Errorf("flush restored unexpected destination %s: %w", path, flushErr))
		}
	}
	return publicationIdentityRefusal(mismatch)
}

// removeAtomic replaces the expected resident with a disposable sibling, then
// marks the opened replacement file for deletion. Deletion follows the handle,
// so a later name replacement leaves a successor intact.
func removeAtomic(tempPath, path string, expected *fileIdentity) error {
	displacedPath := tempPath + ".displaced"
	if err := replaceFile(path, tempPath, displacedPath, 0); err != nil {
		return err
	}
	displaced, err := lstatRegular(displacedPath)
	if mismatch := unexpectedWindowsPublicationIdentity(path, expected, displaced, err, func(left, right fileIdentity) bool { return os.SameFile(left.info, right.info) }); mismatch != nil {
		if rollbackErr := replaceFile(path, displacedPath, tempPath, 0); rollbackErr != nil { // coverage-ignore: requires a second namespace race or kernel fault during immediate rollback
			return errors.Join(publicationIdentityRefusal(mismatch), fmt.Errorf("restore unexpected destination at %s after refused removal: %w", path, rollbackErr))
		}
		return publicationIdentityRefusal(mismatch)
	}
	file, identity, err := openRegularNoFollow(path, false, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	published, err := lstatRegular(path)
	if err != nil || !os.SameFile(identity.info, published.info) {
		if err == nil {
			err = safety("identity", path, errors.New("replacement changed before removal"))
		}
		return publicationIdentityRefusal(err)
	}
	deleteOnClose := byte(1)
	if err := windows.SetFileInformationByHandle(windows.Handle(file.Fd()), windows.FileDispositionInfo, &deleteOnClose, 1); err != nil {
		return err
	}
	return os.Remove(displacedPath)
}

func unexpectedWindowsPublicationIdentity(path string, expected *fileIdentity, displaced fileIdentity, inspectErr error, same func(fileIdentity, fileIdentity) bool) error {
	if inspectErr == nil && same(*expected, displaced) {
		return nil
	}
	if inspectErr != nil {
		return inspectErr
	}
	return safety("identity", path, errors.New("destination changed before atomic publication"))
}

func flushPublishedWindowsFile(path string, expected fileIdentity) error {
	file, err := openWindowsPathNoFollow(path, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.OPEN_EXISTING, false)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect reopened published file %s: %w", path, err)
	}
	if err := validateLeaf(path, info); err != nil {
		return err
	}
	if err := validatePathOwner(path, info, file); err != nil {
		return err
	}
	if err := validateOpenedFile(path, file); err != nil {
		return err
	}
	if !os.SameFile(expected.info, info) {
		return safety("identity", path, errors.New("published file changed before durability flush"))
	}
	if err := windows.FlushFileBuffers(windows.Handle(file.Fd())); err != nil {
		return err
	}
	return nil
}

func replaceFile(path, replacement, backup string, flags uint32) error {
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
		uintptr(flags),
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
