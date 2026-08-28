package testsupport

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TreeSeed is an immutable archived directory tree. Package-specific test
// fixtures capture a fully prepared seed once and clone it for each mutating
// consumer, so no test shares a live root.
type TreeSeed struct {
	archive      []byte
	rootMode     fs.FileMode
	rootModified time.Time
	digest       [sha256.Size]byte
}

// CaptureTree records root as an immutable, process-local seed. Regular files,
// directory modes, executable bits, and symbolic links are preserved.
func CaptureTree(root string) (TreeSeed, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return TreeSeed{}, fmt.Errorf("capture tree seed root: %w", err)
	}
	if !rootInfo.IsDir() {
		return TreeSeed{}, fmt.Errorf("capture tree seed root: %s is not a directory", root)
	}
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		_ = writer.Close()
		return TreeSeed{}, fmt.Errorf("capture tree seed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return TreeSeed{}, fmt.Errorf("capture tree seed: %w", err)
	}
	body := bytes.Clone(archive.Bytes())
	digestInput := append([]byte(fmt.Sprintf("%o\n", rootInfo.Mode().Perm())), body...)
	return TreeSeed{archive: body, rootMode: rootInfo.Mode(), rootModified: rootInfo.ModTime(), digest: sha256.Sum256(digestInput)}, nil
}

// Digest identifies the immutable archived representation.
func (s TreeSeed) Digest() [sha256.Size]byte { return s.digest }

// Clone extracts the seed into a destination that does not yet exist.
func (s TreeSeed) Clone(destination string) error {
	if len(s.archive) == 0 {
		return fmt.Errorf("clone tree seed: empty seed")
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("clone tree seed: destination already exists: %s", destination)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("clone tree seed: inspect destination: %w", err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("clone tree seed: %w", err)
	}
	cleanDestination := filepath.Clean(destination)
	var directories []seedDirectory
	reader := tar.NewReader(bytes.NewReader(s.archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("clone tree seed: %w", err)
		}
		path := filepath.Join(cleanDestination, filepath.FromSlash(header.Name))
		if path == cleanDestination || !strings.HasPrefix(path, cleanDestination+string(filepath.Separator)) {
			return fmt.Errorf("clone tree seed: invalid archived path %q", header.Name)
		}
		mode := fs.FileMode(header.Mode)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return fmt.Errorf("clone tree seed: create directory %s: %w", header.Name, err)
			}
			directories = append(directories, seedDirectory{path: path, mode: mode, modified: header.ModTime})
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("clone tree seed: create parent for %s: %w", header.Name, err)
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
			if err != nil {
				return fmt.Errorf("clone tree seed: create %s: %w", header.Name, err)
			}
			_, copyErr := io.Copy(file, reader)
			closeErr := file.Close()
			if copyErr != nil {
				return fmt.Errorf("clone tree seed: write %s: %w", header.Name, copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("clone tree seed: close %s: %w", header.Name, closeErr)
			}
			if err := os.Chmod(path, mode.Perm()); err != nil {
				return fmt.Errorf("clone tree seed: chmod %s: %w", header.Name, err)
			}
			if err := os.Chtimes(path, header.AccessTime, header.ModTime); err != nil {
				return fmt.Errorf("clone tree seed: set times for %s: %w", header.Name, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("clone tree seed: create symlink parent for %s: %w", header.Name, err)
			}
			if err := os.Symlink(header.Linkname, path); err != nil {
				return fmt.Errorf("clone tree seed: create symlink %s: %w", header.Name, err)
			}
		default:
			return fmt.Errorf("clone tree seed: unsupported entry %q with type %d", header.Name, header.Typeflag)
		}
	}
	for i := len(directories) - 1; i >= 0; i-- {
		directory := directories[i]
		if err := os.Chmod(directory.path, directory.mode.Perm()); err != nil {
			return fmt.Errorf("clone tree seed: chmod directory: %w", err)
		}
		if err := os.Chtimes(directory.path, directory.modified, directory.modified); err != nil {
			return fmt.Errorf("clone tree seed: set directory times: %w", err)
		}
	}
	if err := os.Chmod(destination, s.rootMode.Perm()); err != nil {
		return fmt.Errorf("clone tree seed: chmod root: %w", err)
	}
	if err := os.Chtimes(destination, s.rootModified, s.rootModified); err != nil {
		return fmt.Errorf("clone tree seed: set root times: %w", err)
	}
	return nil
}

type seedDirectory struct {
	path     string
	mode     fs.FileMode
	modified time.Time
}
