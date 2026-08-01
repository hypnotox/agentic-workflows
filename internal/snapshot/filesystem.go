package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// FilesystemTree captures an ordinary directory without requiring Git. It is
// the fallback working universe for repository-state checks before adoption by
// Git; Git metadata is never part of that universe.
func FilesystemTree(ctx context.Context, root string) (*Tree, error) {
	var files []File
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil { // coverage-ignore: WalkDir yields descendants of the same absolute root
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil { // coverage-ignore: WalkDir just observed the entry; only a concurrent mutation can remove it
			return err
		}
		mode := Regular
		var data []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil { // coverage-ignore: Info just identified the symlink; only a concurrent mutation can break the read
				return err
			}
			mode, data = Symlink, []byte(target)
		} else {
			if !info.Mode().IsRegular() {
				return nil
			}
			data, err = os.ReadFile(path)
			if err != nil { // coverage-ignore: Info just identified a readable regular entry; failure requires permission denial or a concurrent mutation
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
