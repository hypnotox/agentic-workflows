package migrate

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"gopkg.in/yaml.v3"
)

var selectionRetiredKeys = []string{"skills", "agents", "docs", "targets", "docsDir"}

type dropSelectionOperation struct {
	configEditor configEditor
	walkDir      func(string, fs.WalkDirFunc) error
	readFile     func(string) ([]byte, error)
	removeKey    func([]byte, string) ([]byte, error)
	writeAtomic  func(string, []byte) error
}

func productionDropSelectionOperation() dropSelectionOperation {
	return dropSelectionOperation{
		configEditor: productionConfigEditor(),
		walkDir:      filepath.WalkDir,
		readFile:     os.ReadFile,
		removeKey:    config.RemoveKey,
		writeAtomic:  manifest.WriteFileAtomic,
	}
}

// applyDropSelection ports schema 38 to 39 by removing selection-only config
// keys and project-local sidecar ownership markers.
func applyDropSelection(root string, out *Changes) error {
	return applyDropSelectionWith(root, out, productionDropSelectionOperation())
}

func applyDropSelectionWith(root string, out *Changes, operation dropSelectionOperation) error {
	if err := operation.configEditor.editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		updated := src
		for _, key := range selectionRetiredKeys {
			next, err := operation.removeKey(updated, key)
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(next, updated) {
				planned.Add(fmt.Sprintf("drop-selection: removed %s\n", key))
			}
			updated = next
		}
		return updated, nil
	}); err != nil {
		return err
	}
	return dropSidecarLocalWith(root, out, operation)
}

func dropSidecarLocalWith(root string, out *Changes, operation dropSelectionOperation) error {
	var files []string
	awf := config.RootDir(root)
	err := operation.walkDir(awf, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".yaml") && filepath.Base(path) != "config.yaml" {
			files = append(files, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Preflight all sidecars before writing any one of them.
	for _, path := range files {
		b, err := operation.readFile(path)
		if err != nil {
			return err
		}
		var raw map[string]any
		if err := yaml.Unmarshal(b, &raw); err != nil {
			return err
		}
		if local, ok := raw["local"].(bool); ok && local {
			return fmt.Errorf("sidecar %s has local: true; replace the hand-maintained artifact with a convention part before upgrading", filepath.ToSlash(path))
		}
	}
	for _, path := range files {
		b, err := operation.readFile(path)
		if err != nil {
			return err
		}
		updated, err := operation.removeKey(b, "local")
		if err != nil {
			return err
		}
		if bytes.Equal(updated, b) {
			continue
		}
		if err := operation.writeAtomic(path, updated); err != nil {
			return err
		}
		out.Add("drop-selection: removed local from " + filepath.ToSlash(path))
	}
	return nil
}
