package snapshot

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type filesystemOps struct {
	walkDir  func(string, fs.WalkDirFunc) error
	rel      func(string, string) (string, error)
	info     func(os.DirEntry) (os.FileInfo, error)
	lstat    func(string) (os.FileInfo, error)
	readlink func(string) (string, error)
	readFile func(string) ([]byte, error)
}

func osFilesystemOps() filesystemOps {
	return filesystemOps{
		walkDir:  filepath.WalkDir,
		rel:      filepath.Rel,
		info:     os.DirEntry.Info,
		lstat:    os.Lstat,
		readlink: os.Readlink,
		readFile: os.ReadFile,
	}
}

// FilesystemTree captures an ordinary directory without requiring Git. It is
// the fallback working universe for repository-state checks before adoption by
// Git. Git metadata and nested checkout roots are never part of that universe.
func FilesystemTree(ctx context.Context, root string) (*Tree, error) {
	return filesystemTree(ctx, root, osFilesystemOps())
}

func filesystemTree(ctx context.Context, root string, ops filesystemOps) (*Tree, error) {
	var files []File
	err := ops.walkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := ops.rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if filepath.Base(path) == ".git" {
				return filepath.SkipDir
			}
			_, markerErr := ops.lstat(filepath.Join(path, ".git"))
			if markerErr == nil {
				return filepath.SkipDir
			}
			if !errors.Is(markerErr, fs.ErrNotExist) {
				return markerErr
			}
			return nil
		}
		info, err := ops.info(entry)
		if err != nil {
			return err
		}
		mode := Regular
		var data []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := ops.readlink(path)
			if err != nil {
				return err
			}
			mode, data = Symlink, []byte(target)
		} else {
			if !info.Mode().IsRegular() {
				return nil
			}
			data, err = ops.readFile(path)
			if err != nil {
				return err
			}
			if info.Mode().Perm()&0o111 != 0 {
				mode = Executable
			}
		}
		files = append(files, File{Path: rel, Mode: mode, Bytes: data})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot filesystem: %w", err)
	}
	return NewTree(files)
}
