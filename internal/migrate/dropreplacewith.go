package migrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

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
func applyDropReplaceWith(root string) error {
	awfDir := filepath.Join(root, ".claude", "awf")
	sidecars, err := treeSidecars(awfDir)
	if err != nil { // coverage-ignore: treeSidecars only faults on the (ignored) ReadDir error arm
		return err
	}
	for _, sc := range sidecars {
		if err := convertSidecar(awfDir, sc.kind, sc.target, sc.path); err != nil {
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
func convertSidecar(awfDir, kind, target, path string) error {
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
		if err := relocatePart(filepath.Join(awfDir, ov.ReplaceWith), dst); err != nil {
			return err
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return writeSidecarDoc(path, sc.Data, kept, sc.Local, true)
}

func conventionPartPath(awfDir, kind, target, section string) string {
	if kind == "agents-doc" {
		return filepath.Join(awfDir, "parts", "agents-doc", section+".md")
	}
	return filepath.Join(awfDir, kind, "parts", target, section+".md")
}

// relocatePart copies src to dst. A missing src or a dst already holding different
// content fails; a dst identical to src is a no-op (idempotent re-runs).
func relocatePart(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("replaceWith part %s: %w", src, err)
	}
	if existing, err := os.ReadFile(dst); err == nil {
		if bytes.Equal(existing, in) {
			return nil
		}
		return fmt.Errorf("convention part %s already exists with different content", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { // coverage-ignore: dst parent under awfDir is writable when the sidecar was readable
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}
