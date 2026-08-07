package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

const approvalPath = config.DirName + "/current-state-migration.yaml"

// attestationTree is upgrade's structural view of its root-confined tree.
type attestationTree interface {
	Walk(string, func(string, fs.FileInfo) (bool, error)) error
	Read(string) ([]byte, error)
	Info(string) (fs.FileInfo, error)
	LinkInfo(string) (fs.FileInfo, error)
}

type digestRecord struct {
	path    string
	mode    uint32
	content []byte
}

// treeDigest recomputes the sealed current-state attestation tree digest.
func treeDigest(root string, tree attestationTree) (string, error) {
	bytes, err := tree.Read(config.DirName + "/config.yaml")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("not an awf project (run `awf init`): %w", err)
		}
		return "", fmt.Errorf("read config: %w", err)
	}
	cfg, err := config.Parse(config.RootDir(root), bytes)
	if err != nil {
		return "", err
	}
	universe := map[string]bool{config.DirName + "/config.yaml": true, approvalPath: true}
	for _, sub := range []string{config.DirName + "/domains", config.DirName + "/topics"} {
		if err := collectUnder(tree, sub, universe); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	decisions := config.DocsDir + "/decisions"
	if err := collectADRs(tree, decisions, universe); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if cfg.CurrentState != nil {
		if err := collectMarkerSources(tree, cfg.CurrentState.Sources, universe); err != nil {
			return "", err
		}
	}
	var records []digestRecord
	for path := range universe {
		content, err := tree.Read(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", err
		}
		info, err := tree.Info(path)
		if err != nil {
			return "", err
		}
		records = append(records, digestRecord{path, uint32(info.Mode().Perm()), content})
	}
	slices.SortFunc(records, func(a, b digestRecord) int { return strings.Compare(a.path, b.path) })
	h := sha256.New()
	for _, rec := range records {
		fmt.Fprintf(h, "%s\x00%o\x00%d\x00", rec.path, rec.mode, len(rec.content))
		h.Write(rec.content)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func collectUnder(tree attestationTree, sub string, universe map[string]bool) error {
	return tree.Walk(sub, func(path string, info fs.FileInfo) (bool, error) {
		if !info.Mode().IsRegular() {
			return info.IsDir(), nil
		}
		universe[path] = true
		return false, nil
	})
}

func collectADRs(tree attestationTree, decisions string, universe map[string]bool) error {
	return tree.Walk(decisions, func(path string, info fs.FileInfo) (bool, error) {
		if info.IsDir() {
			return true, nil
		}
		if adr.FileIdentity(path[strings.LastIndex(path, "/")+1:]) != "" {
			universe[path] = true
		}
		return false, nil
	})
}

func prunedMarkerDirectory(path string) bool {
	switch path[strings.LastIndex(path, "/")+1:] {
	case ".git", "vendor", "node_modules":
		return true
	}
	return false
}

func collectMarkerSources(tree attestationTree, sources []config.CurrentStateSource, universe map[string]bool) error {
	return tree.Walk(".", func(path string, info fs.FileInfo) (bool, error) {
		if info.IsDir() {
			if path != "." && prunedMarkerDirectory(path) {
				return false, nil
			}
			if path != "." {
				if _, err := tree.LinkInfo(path + "/.git"); err == nil {
					return false, nil
				} else if !errors.Is(err, fs.ErrNotExist) {
					return false, err
				}
				if _, err := tree.Info(path + "/" + config.DirName); err == nil {
					return false, nil
				} else if !errors.Is(err, fs.ErrNotExist) {
					return false, err
				}
			}
			return true, nil
		}
		for _, src := range sources {
			for _, glob := range src.Globs {
				if pathglob.Match(glob, path) {
					universe[path] = true
					return false, nil
				}
			}
		}
		return false, nil
	})
}
