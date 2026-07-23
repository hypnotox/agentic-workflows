package dashboardruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	pathLstat    = os.Lstat
	pathMkdirAll = os.MkdirAll
)

func ensurePrivateCacheRoot(xdgRoot, cacheRoot string) error {
	if err := ensurePrivateDirectory(xdgRoot); err != nil {
		return err
	}
	current := xdgRoot
	relative, err := filepath.Rel(xdgRoot, cacheRoot)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: cache root escapes XDG cache", ErrUnsafePath)
	}
	for _, component := range splitPath(relative) {
		current = filepath.Join(current, component)
		if err := ensurePrivateDirectory(current); err != nil {
			return err
		}
	}
	return nil
}

func ensurePrivateDirectory(path string) error {
	info, err := pathLstat(path)
	if os.IsNotExist(err) {
		if err := pathMkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("%w: create cache directory: %w", ErrUnsafePath, err)
		}
		info, err = pathLstat(path)
	}
	if err != nil {
		return fmt.Errorf("%w: inspect cache directory: %w", ErrUnsafePath, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 || !ownedByCurrentUser(info) {
		return fmt.Errorf("%w: cache directory %s is not owner-private", ErrUnsafePath, path)
	}
	return nil
}

func splitPath(path string) []string {
	var result []string
	for path != "." && path != "" {
		directory, base := filepath.Split(path)
		if base != "" {
			result = append([]string{base}, result...)
		}
		path = filepath.Clean(directory)
		if path == string(filepath.Separator) {
			break
		}
	}
	return result
}
