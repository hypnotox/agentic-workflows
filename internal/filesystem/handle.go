// Package filesystem owns root-confined production filesystem access.
package filesystem

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

// Handle provides root-confined filesystem operations.
type Handle struct {
	root *os.Root
}

// ErrIdentityChanged reports that a path no longer names the entry observed by
// the caller. It is stable under wrapping.
var ErrIdentityChanged = errors.New("filesystem: observed identity changed")

// ErrDirectoryNotEmpty reports that expected removal safely preserved a
// directory containing another entry.
var ErrDirectoryNotEmpty = errors.New("filesystem: directory not empty")

// CommittedPublication reports the confined destination and cleanup residue
// carried by a committed exclusive-publication error.
func CommittedPublication(err error) (destination, residue string, committed bool) {
	var cleanup *filepublication.CommittedCleanupError
	if !errors.As(err, &cleanup) {
		return "", "", false
	}
	return cleanup.DestinationPath, cleanup.ResiduePath, true
}

// Open opens root as a root-confined filesystem handle.
func Open(root string) (*Handle, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("filesystem: open root %q: %w", root, err)
	}
	return &Handle{root: r}, nil
}

// Close closes the root-confined filesystem handle.
func (h *Handle) Close() error { return h.root.Close() }

// Walk visits subtree entries with metadata describing each entry itself.
func (h *Handle) Walk(subtree string, visit func(path string, info fs.FileInfo) (bool, error)) error {
	if err := validPath(subtree); err != nil {
		return fmt.Errorf("filesystem: walk %q: %w", subtree, err)
	}
	rootInfo, err := h.root.Lstat(subtree)
	if err != nil {
		return fmt.Errorf("filesystem: walk-info %q: %w", subtree, err)
	}
	if rootInfo.Mode()&fs.ModeSymlink != 0 {
		_, err := visit(subtree, rootInfo)
		if err != nil {
			return fmt.Errorf("filesystem: walk %q: %w", subtree, err)
		}
		return nil
	}
	err = fs.WalkDir(h.root.FS(), subtree, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil { // coverage-ignore: permission controls are execution-identity-dependent; otherwise an underlying WalkDir error requires a concurrent filesystem change
			return fmt.Errorf("filesystem: walk %q: %w", path, walkErr)
		}
		info, err := entry.Info()
		if err != nil { // coverage-ignore: permission controls are execution-identity-dependent; otherwise this requires a concurrent filesystem change
			return fmt.Errorf("filesystem: walk-info %q: %w", path, err)
		}
		descend, err := visit(path, info)
		if err != nil {
			return fmt.Errorf("filesystem: walk %q: %w", path, err)
		}
		if info.IsDir() && !descend {
			return fs.SkipDir
		}
		return nil
	})
	return err
}

// Mkdir creates exactly one directory beneath the selected root and refuses an
// existing entry.
func (h *Handle) Mkdir(path string, mode fs.FileMode) error {
	if err := validPath(path); err != nil {
		return fmt.Errorf("filesystem: mkdir %q: %w", path, err)
	}
	if err := h.root.Mkdir(path, mode); err != nil {
		return fmt.Errorf("filesystem: mkdir %q: %w", path, err)
	}
	return nil
}

// MkdirAll creates path and missing parents beneath the selected root.
func (h *Handle) MkdirAll(path string, mode fs.FileMode) error {
	if err := validPath(path); err != nil {
		return fmt.Errorf("filesystem: mkdir-all %q: %w", path, err)
	}
	if err := h.root.MkdirAll(path, mode); err != nil {
		return fmt.Errorf("filesystem: mkdir-all %q: %w", path, err)
	}
	return nil
}

// Chmod changes path's permission mode beneath the selected root.
func (h *Handle) Chmod(path string, mode fs.FileMode) error {
	if err := validPath(path); err != nil {
		return fmt.Errorf("filesystem: chmod %q: %w", path, err)
	}
	if err := h.root.Chmod(path, mode); err != nil {
		return fmt.Errorf("filesystem: chmod %q: %w", path, err)
	}
	return nil
}

// Publish atomically publishes one complete file without replacement beneath
// the selected root.
func (h *Handle) Publish(path string, contents []byte, mode fs.FileMode) error {
	if err := validPath(path); err != nil {
		return fmt.Errorf("filesystem: publish %q: %w", path, err)
	}
	if err := filepublication.PublishConfined(h.root, path, contents, mode); err != nil {
		return fmt.Errorf("filesystem: publish %q: %w", path, err)
	}
	return nil
}

// Backup reads source once, preserving its permission mode, then exclusively
// publishes a complete sibling backup at source.awf-bak or its first available
// numbered suffix. The supplied confined callbacks keep source access and
// publication policy with the caller while this package owns the shared naming
// and collision protocol.
// Backup copies a source beneath this handle to its first free sibling backup.
func (h *Handle) Backup(source string) (string, error) {
	if err := validPath(source); err != nil {
		return "", fmt.Errorf("filesystem: backup %q: %w", source, err)
	}
	return Backup(source, h.ReadWithMode, h.Publish)
}

func Backup(source string, readWithMode func(string) ([]byte, fs.FileMode, error), publish func(string, []byte, fs.FileMode) error) (string, error) {
	contents, mode, err := readWithMode(source)
	if err != nil {
		return "", err
	}
	for suffix := 0; ; suffix++ {
		destination := source + ".awf-bak"
		if suffix != 0 {
			destination = fmt.Sprintf("%s.%d", destination, suffix)
		}
		if err := publish(destination, contents, mode); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return "", err
		}
		return destination, nil
	}
}

// Replace atomically replaces path with one complete file beneath the selected
// root, preserving the requested final mode. Callers that previously observed
// the destination should use ReplaceExpected.
func (h *Handle) Replace(destination string, contents []byte, mode fs.FileMode) error {
	expected, err := h.root.Lstat(destination)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("filesystem: inspect replacement %q: %w", destination, err)
	}
	return h.ReplaceExpected(destination, expected, contents, mode)
}

// ReplaceExpected publishes only while destination still has expected's entry
// identity. A nil expected identity creates exclusively rather than clobbering.
func (h *Handle) ReplaceExpected(destination string, expected fs.FileInfo, contents []byte, mode fs.FileMode) (returnErr error) {
	if err := validPath(destination); err != nil {
		return fmt.Errorf("filesystem: replace %q: %w", destination, err)
	}
	if expected == nil {
		if err := h.Publish(destination, contents, mode); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return fmt.Errorf("filesystem: replace %q: %w", destination, ErrIdentityChanged)
			}
			return err
		}
		return nil
	}
	if expected.IsDir() {
		return fmt.Errorf("filesystem: replace %q: destination is a directory", destination)
	}
	var temporary string
	var file *os.File
	for range 100 {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil { // coverage-ignore: the operating-system random source is required by the Go runtime and cannot be faulted through this seam
			return fmt.Errorf("filesystem: name replacement temporary for %q: %w", destination, err)
		}
		temporary = path.Join(path.Dir(destination), ".awf-atomic-"+hex.EncodeToString(token[:]))
		var err error
		file, err = h.root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) { // coverage-ignore: a collision requires predicting the cryptographically random temporary name before this operation
			continue // coverage-ignore: a collision requires predicting the cryptographically random temporary name before this operation
		}
		if err != nil {
			return fmt.Errorf("filesystem: create replacement temporary for %q: %w", destination, err)
		}
		break
	}
	if file == nil { // coverage-ignore: exhausting 100 cryptographically random temporary names cannot be induced through the concrete production handle
		return fmt.Errorf("filesystem: create replacement temporary for %q: temporary name collisions exhausted", destination) // coverage-ignore: exhausting 100 cryptographically random temporary names cannot be induced through the concrete production handle
	}
	defer func() {
		if file != nil { // coverage-ignore: this remains non-nil only after a local temporary write storage fault
			returnErr = errors.Join(returnErr, file.Close()) // coverage-ignore: a second close failure after a replacement write failure requires a second storage fault
		}
		if temporary != "" {
			if err := h.root.Remove(temporary); err != nil && !errors.Is(err, fs.ErrNotExist) { // coverage-ignore: the concrete root can fault here only through a concurrent namespace change or storage fault
				returnErr = errors.Join(returnErr, fmt.Errorf("filesystem: remove replacement temporary %q: %w", temporary, err)) // coverage-ignore: the concrete root can fault here only through a concurrent namespace change or storage fault
			}
		}
	}()
	if n, err := file.Write(contents); err != nil { // coverage-ignore: writing a locally-created regular temporary requires a storage fault
		return fmt.Errorf("filesystem: write replacement temporary for %q: %w", destination, err) // coverage-ignore: writing a locally-created regular temporary requires a storage fault
	} else if n != len(contents) { // coverage-ignore: os.File.Write returns a non-nil error when it writes fewer bytes than requested
		return fmt.Errorf("filesystem: write replacement temporary for %q: %w", destination, io.ErrShortWrite) // coverage-ignore: os.File.Write returns a non-nil error when it writes fewer bytes than requested
	}
	if err := file.Chmod(mode); err != nil { // coverage-ignore: chmod of a newly-created regular temporary requires a storage fault
		return fmt.Errorf("filesystem: set replacement mode for %q: %w", destination, err) // coverage-ignore: chmod of a newly-created regular temporary requires a storage fault
	}
	if err := file.Sync(); err != nil { // coverage-ignore: syncing a locally-created regular temporary requires a storage fault
		return fmt.Errorf("filesystem: sync replacement temporary for %q: %w", destination, err) // coverage-ignore: syncing a locally-created regular temporary requires a storage fault
	}
	if err := file.Close(); err != nil { // coverage-ignore: closing a locally-created regular temporary after a successful sync requires a storage fault
		return fmt.Errorf("filesystem: close replacement temporary for %q: %w", destination, err) // coverage-ignore: closing a locally-created regular temporary after a successful sync requires a storage fault
	}
	file = nil
	consumed, err := exchangeExpected(h.root, temporary, destination, expected, false)
	if consumed {
		temporary = ""
	}
	if err != nil {
		return fmt.Errorf("filesystem: replace %q: %w", destination, err)
	}
	return nil
}

// Rename moves oldPath to newPath beneath the selected root.
func (h *Handle) Rename(oldPath, newPath string) error {
	if err := validPath(oldPath); err != nil {
		return fmt.Errorf("filesystem: rename %q: %w", oldPath, err)
	}
	if err := validPath(newPath); err != nil {
		return fmt.Errorf("filesystem: rename %q: %w", newPath, err)
	}
	if err := h.root.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("filesystem: rename %q to %q: %w", oldPath, newPath, err)
	}
	return nil
}

// RemoveAll removes path and its descendants beneath the selected root.
func (h *Handle) RemoveAll(path string) error {
	if err := validPath(path); err != nil {
		return fmt.Errorf("filesystem: remove-all %q: %w", path, err)
	}
	if err := h.root.RemoveAll(path); err != nil {
		return fmt.Errorf("filesystem: remove-all %q: %w", path, err)
	}
	return nil
}

// RemoveExpected removes path only while it still has expected's identity.
func (h *Handle) RemoveExpected(destination string, expected fs.FileInfo) (returnErr error) {
	if err := validPath(destination); err != nil {
		return fmt.Errorf("filesystem: remove %q: %w", destination, err)
	}
	if expected == nil {
		return fmt.Errorf("filesystem: remove %q: %w", destination, ErrIdentityChanged)
	}
	if expected.IsDir() {
		directory, err := h.root.Open(destination)
		if err != nil {
			return fmt.Errorf("filesystem: inspect removable directory %q: %w", destination, err)
		}
		_, readErr := directory.Readdirnames(1)
		closeErr := directory.Close()
		if readErr == nil {
			return fmt.Errorf("filesystem: remove %q: %w", destination, ErrDirectoryNotEmpty)
		}
		if !errors.Is(readErr, io.EOF) || closeErr != nil {
			return fmt.Errorf("filesystem: inspect removable directory %q: %w", destination, errors.Join(readErr, closeErr))
		}
	}
	var temporary string
	created := false
	for range 100 {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil { // coverage-ignore: the operating-system random source is required by the Go runtime and cannot be faulted through this seam
			return fmt.Errorf("filesystem: name removal temporary for %q: %w", destination, err)
		}
		temporary = path.Join(path.Dir(destination), ".awf-remove-"+hex.EncodeToString(token[:]))
		if expected.IsDir() {
			returnErr = h.root.Mkdir(temporary, 0o700)
		} else {
			var file *os.File
			file, returnErr = h.root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if returnErr == nil {
				if closeErr := file.Close(); closeErr != nil { // coverage-ignore: closing a newly-created empty regular file requires a storage fault
					removeErr := h.root.Remove(temporary)                                                                              // coverage-ignore: cleanup after a close storage fault requires the same unportable fault source
					return fmt.Errorf("filesystem: close removal temporary for %q: %w", destination, errors.Join(closeErr, removeErr)) // coverage-ignore: closing a newly-created empty regular file requires a storage fault
				}
			}
		}
		if errors.Is(returnErr, fs.ErrExist) { // coverage-ignore: collision requires predicting a cryptographically random temporary
			continue // coverage-ignore: collision requires predicting a cryptographically random temporary
		}
		if returnErr != nil {
			return fmt.Errorf("filesystem: create removal temporary for %q: %w", destination, returnErr)
		}
		created = true
		break
	}
	if !created { // coverage-ignore: exhausting random temporary names is not practically triggerable
		return fmt.Errorf("filesystem: create removal temporary for %q: collisions exhausted", destination) // coverage-ignore: exhausting random temporary names is not practically triggerable
	}
	defer func() {
		if temporary != "" {
			if err := h.root.Remove(temporary); err != nil && !errors.Is(err, fs.ErrNotExist) {
				returnErr = errors.Join(returnErr, fmt.Errorf("filesystem: remove removal temporary %q: %w", temporary, err))
			}
		}
	}()
	consumed, err := exchangeExpected(h.root, temporary, destination, expected, true)
	if consumed {
		temporary = ""
	}
	if err != nil {
		return fmt.Errorf("filesystem: remove %q: %w", destination, err)
	}
	return nil
}

// Remove removes path beneath the selected root.
func (h *Handle) Remove(path string) error {
	if err := validPath(path); err != nil {
		return fmt.Errorf("filesystem: remove %q: %w", path, err)
	}
	if err := h.root.Remove(path); err != nil {
		return fmt.Errorf("filesystem: remove %q: %w", path, err)
	}
	return nil
}

// Read reads path beneath the selected root.
func (h *Handle) Read(path string) ([]byte, error) {
	if err := validPath(path); err != nil {
		return nil, fmt.Errorf("filesystem: read %q: %w", path, err)
	}
	b, err := h.root.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: read %q: %w", path, err)
	}
	return b, nil
}

// ReadWithMode reads path and returns its permission mode from one confined open.
func (h *Handle) ReadWithMode(path string) ([]byte, fs.FileMode, error) {
	if err := validPath(path); err != nil {
		return nil, 0, fmt.Errorf("filesystem: read %q: %w", path, err)
	}
	file, err := h.root.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("filesystem: read %q: %w", path, err)
	}
	contents, readErr := io.ReadAll(file)
	info, statErr := file.Stat()
	closeErr := file.Close()
	if readErr != nil {
		return nil, 0, fmt.Errorf("filesystem: read %q: %w", path, errors.Join(readErr, closeErr))
	}
	if statErr != nil { // coverage-ignore: stat on a successfully read confined regular file requires a storage fault
		return nil, 0, fmt.Errorf("filesystem: read %q: %w", path, errors.Join(statErr, closeErr)) // coverage-ignore: stat on a successfully read confined regular file requires a storage fault
	}
	if closeErr != nil { // coverage-ignore: close after a successful confined read requires a storage fault
		return nil, 0, fmt.Errorf("filesystem: read %q: %w", path, closeErr) // coverage-ignore: close after a successful confined read requires a storage fault
	}
	return contents, info.Mode().Perm(), nil
}

// Info returns metadata for path, following a final symbolic link.
func (h *Handle) Info(path string) (fs.FileInfo, error) {
	if err := validPath(path); err != nil {
		return nil, fmt.Errorf("filesystem: info %q: %w", path, err)
	}
	info, err := h.root.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: info %q: %w", path, err)
	}
	return info, nil
}

// LinkInfo returns metadata for path without following a final symbolic link.
func (h *Handle) LinkInfo(path string) (fs.FileInfo, error) {
	if err := validPath(path); err != nil {
		return nil, fmt.Errorf("filesystem: link-info %q: %w", path, err)
	}
	info, err := h.root.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: link-info %q: %w", path, err)
	}
	return info, nil
}

func validPath(path string) error {
	if path == "." || fs.ValidPath(path) {
		return nil
	}
	return errors.New("invalid path")
}
