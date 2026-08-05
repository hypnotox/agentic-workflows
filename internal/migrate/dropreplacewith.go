package migrate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// v1Override / v1Sidecar parse a schema-1 sidecar, which may still carry the
// since-removed replaceWith field (config.SectionOverride no longer does).
type v1Override struct {
	ReplaceWith string `yaml:"replaceWith"`
	Drop        bool   `yaml:"drop"`
}
type v1Sidecar struct {
	Data     map[string]any        `yaml:"data"`
	Sections map[string]v1Override `yaml:"sections"`
	Local    bool                  `yaml:"local"`
}

// applyDropReplaceWith ports schema 1 → 2: every sidecar replaceWith section
// becomes a convention part at the section's conventional path and the field is
// dropped. An occupied destination with differing content, or a missing
// referenced part, fails the upgrade rather than overwriting or losing content.
func applyDropReplaceWith(root string, out *Changes) error {
	return applyDropReplaceWithSidecarWrite(root, out, writeSidecarDoc)
}

func applyDropReplaceWithSidecarWrite(root string, out *Changes, writeSidecar func(string, map[string]any, map[string]any, bool, bool) error) error {
	awfDir := filepath.Join(root, ".claude", "awf")
	sidecars, err := treeSidecars(awfDir)
	if err != nil { // coverage-ignore: treeSidecars only faults on the (ignored) ReadDir error arm
		return err
	}
	for _, sc := range sidecars {
		if err := convertSidecar(awfDir, sc.kind, sc.target, sc.path, out, writeSidecar); err != nil {
			return err
		}
	}
	return nil
}

type sidecarRef struct{ kind, target, path string }

// treeSidecars enumerates per-target sidecars: <awfDir>/<kind>/<name>.yaml for
// kind in {skills,agents,docs,domains} plus the agents-doc singleton.
func treeSidecars(awfDir string) ([]sidecarRef, error) {
	var out []sidecarRef
	if ad := filepath.Join(awfDir, "agents-doc.yaml"); fileExists(ad) {
		out = append(out, sidecarRef{kind: "agents-doc", target: "", path: ad})
	}
	for _, kind := range []string{"skills", "agents", "docs", "domains"} {
		dir := filepath.Join(awfDir, kind)
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil { // coverage-ignore: a present <kind> dir under a readable awfDir does not fault on read
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
				continue
			}
			target := e.Name()[:len(e.Name())-len(".yaml")]
			out = append(out, sidecarRef{kind: kind, target: target, path: filepath.Join(dir, e.Name())})
		}
	}
	return out, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// convertSidecar relocates the sidecar's replaceWith sections to convention parts
// and rewrites the sidecar without them. A sidecar with no replaceWith is untouched.
func convertSidecar(awfDir, kind, target, path string, out *Changes, writeSidecar func(string, map[string]any, map[string]any, bool, bool) error) error {
	b, err := os.ReadFile(path)
	if err != nil { // coverage-ignore: treeSidecars only lists files that stat-exist and stay readable
		return err
	}
	var sc v1Sidecar
	if err := yaml.Unmarshal(b, &sc); err != nil {
		return fmt.Errorf("parse sidecar %s: %w", path, err)
	}
	changed := false
	kept := map[string]any{}
	for sec, ov := range sc.Sections {
		if ov.ReplaceWith == "" {
			if ov.Drop {
				kept[sec] = map[string]any{"drop": true}
			}
			continue
		}
		dst := conventionPartPath(awfDir, kind, target, sec)
		copied, err := relocatePart(filepath.Join(awfDir, ov.ReplaceWith), dst, writeFile)
		if err != nil {
			return err
		}
		if copied {
			out.Add(fmt.Sprintf("drop-replacewith: copied %s to %s", ov.ReplaceWith, treeRelativePath(awfDir, dst)))
		}
		changed = true
	}
	if !changed {
		return nil
	}
	if err := writeSidecar(path, sc.Data, kept, sc.Local, true); err != nil {
		return err
	}
	out.Add("drop-replacewith: rewrote " + treeRelativePath(awfDir, path))
	return nil
}

func treeRelativePath(awfDir, path string) string {
	root := filepath.ToSlash(filepath.Dir(filepath.Dir(awfDir)))
	return strings.TrimPrefix(filepath.ToSlash(path), root+"/")
}

func conventionPartPath(awfDir, kind, target, section string) string {
	if kind == "agents-doc" {
		return filepath.Join(awfDir, "parts", "agents-doc", section+".md")
	}
	return filepath.Join(awfDir, kind, "parts", target, section+".md")
}

// relocatePart copies src to dst and reports whether it wrote the destination.
// A missing src or a dst already holding different content fails; a dst identical
// to src is left untouched for idempotent re-runs.
func relocatePart(src, dst string, write func(string, []byte) error) (bool, error) {
	return relocatePartWithRead(src, dst, os.ReadFile, write)
}

func relocatePartWithRead(src, dst string, read func(string) ([]byte, error), write func(string, []byte) error) (bool, error) {
	in, err := read(src)
	if err != nil {
		return false, fmt.Errorf("replaceWith part %s: %w", src, err)
	}
	if existing, err := read(dst); err == nil {
		if bytes.Equal(existing, in) {
			return false, nil
		}
		return false, fmt.Errorf("convention part %s already exists with different content", dst)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("read destination %s: %w", dst, err)
	}
	if err := write(dst, in); err != nil {
		return false, fmt.Errorf("write destination %s: %w", dst, err)
	}
	return true, nil
}
