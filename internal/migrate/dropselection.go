package migrate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"gopkg.in/yaml.v3"
)

var selectionRetiredKeys = []string{"skills", "agents", "docs", "targets", "docsDir"}

type dropSelectionOperation struct {
	configEditor configEditor
	readDir      func(string) ([]os.DirEntry, error)
	walkDir      func(string, fs.WalkDirFunc) error
	readFile     func(string) ([]byte, error)
	stat         func(string) (fs.FileInfo, error)
	removeKey    func([]byte, string) ([]byte, error)
	writeAtomic  func(string, []byte, os.FileMode) error
}

type selectionSidecarEdit struct {
	path, source string
	bytes        []byte
	mode         os.FileMode
}

func productionDropSelectionOperation() dropSelectionOperation {
	return dropSelectionOperation{
		configEditor: productionConfigEditor(),
		readDir:      os.ReadDir,
		walkDir:      filepath.WalkDir,
		readFile:     os.ReadFile,
		stat:         os.Stat,
		removeKey:    config.RemoveKey,
		writeAtomic:  manifest.WriteFileAtomicMode,
	}
}

// applyDropSelection ports schema 38 to 39 by removing selection-only config
// keys and project-local sidecar ownership markers.
func applyDropSelection(root string, out *Changes) error {
	return applyDropSelectionWith(root, out, productionDropSelectionOperation())
}

func applyDropSelectionWith(root string, out *Changes, operation dropSelectionOperation) error {
	// A local sidecar refusal must leave config.yaml untouched too.
	edits, err := preflightSidecarLocal(root, operation)
	if err != nil {
		return err
	}
	if err := operation.configEditor.editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		return removeSelectionConfig(src, planned, operation.removeKey)
	}); err != nil {
		return err
	}
	return writeSidecarLocal(edits, out, operation)
}

func removeSelectionConfig(src []byte, planned *Changes, removeKey func([]byte, string) ([]byte, error)) ([]byte, error) {
	updated := src
	for _, key := range selectionRetiredKeys {
		next, err := removeKey(updated, key)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(next, updated) {
			planned.Add(fmt.Sprintf("drop-selection: removed %s\n", key))
		}
		updated = next
	}
	return updated, nil
}

// selectionSidecarPaths is the frozen schema-39 artifact-sidecar surface. It
// deliberately does not discover arbitrary YAML below .awf: resident state,
// topics, parts, and checkout trees are not artifact sidecars.
func selectionSidecarPaths(root string, operation dropSelectionOperation) ([]string, error) {
	awf := config.RootDir(root)
	var paths []string
	for _, kind := range catalog.SingletonKinds() {
		path := filepath.Join(awf, kind+".yaml")
		if _, err := operation.stat(path); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("stat sidecar %s: %w", filepath.ToSlash(path), err)
		}
		paths = append(paths, path)
	}
	for _, kind := range []string{"skills", "agents"} {
		dir := filepath.Join(awf, kind)
		entries, err := operation.readDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read sidecar directory %s: %w", filepath.ToSlash(dir), err)
		}
		for _, entry := range entries {
			name := entry.Name()
			stem := strings.TrimSuffix(name, ".yaml")
			if entry.IsDir() || stem == name || config.ValidateArtifactName(kind, stem) != nil {
				continue
			}
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	docs := filepath.Join(awf, "docs")
	err := operation.walkDir(docs, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk sidecar %s: %w", filepath.ToSlash(path), err)
		}
		if entry.IsDir() {
			if _, statErr := operation.stat(filepath.Join(path, ".git")); statErr == nil {
				return fs.SkipDir
			} else if !errors.Is(statErr, fs.ErrNotExist) {
				return fmt.Errorf("stat checkout marker %s: %w", filepath.ToSlash(filepath.Join(path, ".git")), statErr)
			}
			return nil
		}
		rel := strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(docs)+"/")
		if strings.HasSuffix(rel, ".yaml") && validHistoricalDocName(strings.TrimSuffix(rel, ".yaml")) {
			paths = append(paths, path)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return paths, nil
	}
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func validHistoricalDocName(name string) bool {
	if name == "" || strings.Contains(name, "..") || strings.HasSuffix(name, ".md") {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || config.ValidateArtifactName("doc", segment) != nil {
			return false
		}
		valid := false
		for _, r := range segment {
			valid = valid || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		}
		if !valid {
			return false
		}
	}
	return true
}

func preflightSidecarLocal(root string, operation dropSelectionOperation) ([]selectionSidecarEdit, error) {
	paths, err := selectionSidecarPaths(root, operation)
	if err != nil {
		return nil, err
	}
	edits := make([]selectionSidecarEdit, 0, len(paths))
	for _, path := range paths {
		source, err := operation.readFile(path)
		if err != nil {
			return nil, fmt.Errorf("read sidecar %s: %w", filepath.ToSlash(path), err)
		}
		info, err := operation.stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat sidecar %s: %w", filepath.ToSlash(path), err)
		}
		var raw map[string]any
		if err := yaml.Unmarshal(source, &raw); err != nil {
			return nil, fmt.Errorf("parse sidecar %s: %w", filepath.ToSlash(path), err)
		}
		if local, ok := raw["local"].(bool); ok && local {
			return nil, fmt.Errorf("sidecar %s has local: true; replace the hand-maintained artifact with a convention part before upgrading", filepath.ToSlash(path))
		}
		edits = append(edits, selectionSidecarEdit{path: path, source: filepath.ToSlash(path), bytes: source, mode: info.Mode().Perm()})
	}
	return edits, nil
}

func writeSidecarLocal(edits []selectionSidecarEdit, out *Changes, operation dropSelectionOperation) error {
	for _, edit := range edits {
		updated, err := operation.removeKey(edit.bytes, "local")
		if err != nil {
			return fmt.Errorf("remove local from sidecar %s: %w", edit.source, err)
		}
		if bytes.Equal(updated, edit.bytes) {
			continue
		}
		if err := operation.writeAtomic(edit.path, updated, edit.mode); err != nil {
			return fmt.Errorf("write sidecar %s: %w", edit.source, err)
		}
		out.Add("drop-selection: removed local from " + edit.source)
	}
	return nil
}
