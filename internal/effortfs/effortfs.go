// Package effortfs manages filesystem-resident effort memory beneath a repository root.
package effortfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const memoryName = "memory.md"

// New creates a new active effort and returns the repository-relative path to
// its memory file.
func New(root, slug string) (string, error) {
	if err := validateSlug(slug); err != nil {
		return "", err
	}

	activeRelative := activePath(slug)
	archiveRelative := archivePath(slug)
	if exists, err := pathExists(filepath.Join(root, activeRelative)); err != nil {
		return "", fmt.Errorf("check active effort %q: %w", slug, err)
	} else if exists {
		return "", fmt.Errorf("active effort %q already exists", slug)
	}
	if exists, err := pathExists(filepath.Join(root, archiveRelative)); err != nil {
		return "", fmt.Errorf("check archived effort %q: %w", slug, err)
	} else if exists {
		return "", fmt.Errorf("archived effort %q already exists", slug)
	}

	activeDirectory := filepath.Join(root, activeRelative)
	if err := os.MkdirAll(filepath.Dir(activeDirectory), 0o755); err != nil {
		return "", fmt.Errorf("create active efforts directory: %w", err)
	}
	if err := os.Mkdir(activeDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create active effort %q: %w", slug, err)
	}

	memoryRelative := filepath.Join(activeRelative, memoryName)
	memoryFile, err := os.OpenFile(filepath.Join(root, memoryRelative), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create memory for effort %q: %w", slug, err)
	}
	if _, err := io.WriteString(memoryFile, starter(slug)); err != nil {
		_ = memoryFile.Close()
		return "", fmt.Errorf("write memory for effort %q: %w", slug, err)
	}
	if err := memoryFile.Close(); err != nil {
		return "", fmt.Errorf("close memory for effort %q: %w", slug, err)
	}

	return memoryRelative, nil
}

// List returns the sorted slugs of active efforts. An active effort is a
// directory containing a regular memory.md file.
func List(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, activeRoot()))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("list active efforts: %w", err)
	}

	slugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		memoryPath := filepath.Join(root, activeRoot(), entry.Name(), memoryName)
		info, err := os.Lstat(memoryPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("inspect effort %q memory: %w", entry.Name(), err)
		}
		if info.Mode().IsRegular() {
			slugs = append(slugs, entry.Name())
		}
	}
	sort.Strings(slugs)
	return slugs, nil
}

// Show returns the repository-relative memory path and the memory file's raw
// contents.
func Show(root, slug string) (string, []byte, error) {
	if err := validateSlug(slug); err != nil {
		return "", nil, err
	}

	memoryRelative := filepath.Join(activePath(slug), memoryName)
	body, err := os.ReadFile(filepath.Join(root, memoryRelative))
	if err != nil {
		return "", nil, fmt.Errorf("read memory for effort %q: %w", slug, err)
	}
	return memoryRelative, body, nil
}

// Finish moves an active effort directory into the archive and returns the
// repository-relative archive path.
func Finish(root, slug string) (string, error) {
	if err := validateSlug(slug); err != nil {
		return "", err
	}

	activeRelative := activePath(slug)
	activeDirectory := filepath.Join(root, activeRelative)
	info, err := os.Lstat(activeDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("active effort %q does not exist", slug)
		}
		return "", fmt.Errorf("inspect active effort %q: %w", slug, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("active effort %q is not a directory", slug)
	}

	archiveRelative := archivePath(slug)
	archiveDirectory := filepath.Join(root, archiveRelative)
	if exists, err := pathExists(archiveDirectory); err != nil {
		return "", fmt.Errorf("check archived effort %q: %w", slug, err)
	} else if exists {
		return "", fmt.Errorf("archived effort %q already exists", slug)
	}
	if err := os.MkdirAll(filepath.Dir(archiveDirectory), 0o755); err != nil {
		return "", fmt.Errorf("create effort archive directory: %w", err)
	}
	if err := os.Rename(activeDirectory, archiveDirectory); err != nil {
		return "", fmt.Errorf("archive effort %q: %w", slug, err)
	}
	return archiveRelative, nil
}

func validateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("invalid effort slug: use letters, numbers, hyphens, or underscores")
	}
	for i, char := range slug {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || (i > 0 && (char == '-' || char == '_')) {
			continue
		}
		return fmt.Errorf("invalid effort slug %q: use letters, numbers, hyphens, or underscores", slug)
	}
	return nil
}

func activeRoot() string {
	return filepath.Join(".awf", "efforts")
}

func activePath(slug string) string {
	return filepath.Join(activeRoot(), slug)
}

func archivePath(slug string) string {
	return filepath.Join(".awf", "effort-archive", slug)
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func starter(slug string) string {
	return "# Effort: " + slug + "\n\n" +
		"## Outcome\n\n" +
		"## Current state\n\n" +
		"## Decisions\n\n" +
		"## Next\n"
}
