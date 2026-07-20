package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// approvalPath is the migration approval file the seal covers by digest and the
// final upgrade deletes. The permanent runtime neither parses nor claims it; the
// upgrade consumption path names it only to recompute the sealed digest and to
// journal its one deletion.
const approvalPath = config.DirName + "/current-state-migration.yaml"

// digestRecord is one (slash-relative path, permission mode, content) triple in
// the attestation tree digest.
type digestRecord struct {
	path    string
	mode    uint32
	content []byte
}

// treeDigest recomputes the sealed current-state attestation tree digest over
// the current on-disk tree. The tree is already the post-normalization prepared
// result when the final upgrade runs (HEAD equals the sealed PreparedHead), so
// the digest reads current bytes and modes with no mutation overlay and no
// cross-schema conversion. The universe is exactly the config file, every domain
// sidecar, every topic input, every ADR, every file matched by the current
// config's marker source globs, and the required migration approval file; no
// other path enters it. The record encoding matches the bridge seal byte for
// byte, so an unchanged prepared tree reproduces the sealed digest.
func treeDigest(root string) (string, error) {
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return "", err
	}
	universe := map[string]bool{
		config.DirName + "/config.yaml": true,
		approvalPath:                    true,
	}
	for _, sub := range []string{config.DirName + "/domains", config.DirName + "/topics"} {
		if err := collectUnder(root, sub, universe); err != nil { // coverage-ignore: only a permission or concurrent-removal fault under an existing subtree reaches here
			return "", err
		}
	}
	decisions := strings.TrimRight(cfg.DocsDir, "/") + "/decisions"
	if err := collectADRs(root, decisions, universe); err != nil { // coverage-ignore: only a permission or concurrent-removal fault under the decisions subtree reaches here
		return "", err
	}
	if cfg.CurrentState != nil {
		if err := collectMarkerSources(root, cfg.CurrentState.Sources, universe); err != nil { // coverage-ignore: only a permission or concurrent-removal fault under the scanned tree reaches here
			return "", err
		}
	}
	var records []digestRecord
	for path := range universe {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err // coverage-ignore: a universe path collected from disk reads cleanly unless a concurrent removal races
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil { // coverage-ignore: ReadFile just succeeded for this same path; failure requires a concurrent filesystem race
			return "", err
		}
		records = append(records, digestRecord{path: path, mode: uint32(info.Mode().Perm()), content: content})
	}
	slices.SortFunc(records, func(a, b digestRecord) int { return strings.Compare(a.path, b.path) })
	h := sha256.New()
	for _, rec := range records {
		fmt.Fprintf(h, "%s\x00%o\x00%d\x00", rec.path, rec.mode, len(rec.content))
		h.Write(rec.content)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// collectUnder adds every regular file below the sub subtree of root to the
// universe. A missing subtree contributes nothing.
func collectUnder(root, sub string, universe map[string]bool) error {
	return filepath.WalkDir(filepath.Join(root, filepath.FromSlash(sub)), func(path string, de fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil { // coverage-ignore: after the optional missing root, only a permission or concurrent-removal fault reaches here
			return err
		}
		if de.IsDir() {
			return nil
		}
		info, err := de.Info()
		if err != nil { // coverage-ignore: WalkDir just returned this entry; failure requires a concurrent filesystem race
			return err
		}
		if !info.Mode().IsRegular() { // coverage-ignore: a non-regular file in the authored .awf domain/topic subtree is not a case a unit fixture creates
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		universe[filepath.ToSlash(rel)] = true
		return nil
	})
}

// collectADRs adds every ADR-named file under the decisions subtree to the
// universe, so generated indexes such as INDEX.md are excluded.
func collectADRs(root, decisions string, universe map[string]bool) error {
	return filepath.WalkDir(filepath.Join(root, filepath.FromSlash(decisions)), func(path string, de fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil { // coverage-ignore: after the optional missing root, only a permission or concurrent-removal fault reaches here
			return err
		}
		if de.IsDir() || !adr.FilenameRe.MatchString(de.Name()) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		universe[filepath.ToSlash(rel)] = true
		return nil
	})
}

// collectMarkerSources adds every regular file matched by any current marker
// source glob to the universe, skipping nested projects and dependency trees the
// same way the marker scan does.
func collectMarkerSources(root string, sources []config.CurrentStateSource, universe map[string]bool) error {
	return filepath.WalkDir(root, func(path string, de fs.DirEntry, err error) error {
		if err != nil { // coverage-ignore: requires a permission fault or concurrent source-tree removal
			return err
		}
		if de.IsDir() {
			if path != root && (de.Name() == ".git" || de.Name() == "vendor" || de.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if path != root {
				if _, statErr := os.Stat(filepath.Join(path, config.DirName)); statErr == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		relSlash := filepath.ToSlash(rel)
		for _, src := range sources {
			for _, glob := range src.Globs {
				if pathglob.Match(glob, relSlash) {
					universe[relSlash] = true
					return nil
				}
			}
		}
		return nil
	})
}
