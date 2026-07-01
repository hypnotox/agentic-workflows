package migrate

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// applyTreeLayout ports a pre-ADR-0009 monolithic .claude/awf.yaml into the
// .claude/awf/ tree: a skeleton config.yaml (flat enable arrays + skeleton
// fields), per-target sidecars for everything non-prose, every replaceWith part
// copied to its convention path, and the agents-doc prose re-modelled into
// convention parts. Idempotent: a no-op (nil) when .claude/awf.yaml is absent.
func applyTreeLayout(root string) error {
	claudeDir := filepath.Join(root, ".claude")
	legacyPath := filepath.Join(claudeDir, "awf.yaml")
	if _, err := os.Stat(legacyPath); errors.Is(err, os.ErrNotExist) {
		return nil // already ported (or never legacy)
	}
	lc, err := readLegacy(legacyPath)
	if err != nil {
		return err
	}
	awfDir := filepath.Join(claudeDir, "awf")

	// config.yaml skeleton — only fields the new config.Config accepts.
	skeleton := map[string]any{"prefix": lc.Prefix}
	if lc.DocsDir != "" && lc.DocsDir != "docs" {
		skeleton["docsDir"] = lc.DocsDir // carry a non-default docs root through (ADR-0009 Decision 2)
	}
	if len(lc.Vars) > 0 {
		skeleton["vars"] = lc.Vars
	}
	if lc.Invariants != nil {
		skeleton["invariants"] = lc.Invariants
	}
	skeleton["skills"] = slices.Sorted(maps.Keys(lc.Skills))
	skeleton["agents"] = slices.Sorted(maps.Keys(lc.Agents))
	skeleton["docs"] = slices.Sorted(maps.Keys(lc.Docs))
	if len(lc.Hooks) > 0 {
		skeleton["hooks"] = lc.Hooks
	}
	if err := writeYAML(filepath.Join(awfDir, "config.yaml"), skeleton); err != nil {
		return err
	}

	// Per-kind sidecars + convention parts.
	for _, kv := range []struct {
		kind string
		set  map[string]legacySidecar
	}{
		{"skills", lc.Skills}, {"agents", lc.Agents}, {"docs", lc.Docs},
	} {
		for _, name := range slices.Sorted(maps.Keys(kv.set)) {
			if err := portSidecar(awfDir, kv.kind, name, kv.set[name]); err != nil {
				return err
			}
		}
	}

	// agents-doc: prose → convention parts, the rest → agents-doc.yaml sidecar.
	if lc.AgentsDoc != nil {
		if err := portAgentsDoc(awfDir, *lc.AgentsDoc); err != nil {
			return err
		}
	}

	// Remove the legacy single-file layout.
	if err := os.Remove(legacyPath); err != nil { // coverage-ignore: post-port removal of the legacy file we just stat'd and read; fails only on a permission fault that root bypasses
		return err
	}
	if err := os.Remove(filepath.Join(claudeDir, "awf.lock")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// portSidecar writes the sidecar for one target and copies its replaceWith
// sections out to convention parts (.claude/awf/<kind>/parts/<name>/<sec>.md),
// dropping those sections from the sidecar (the convention binds them). drop
// overrides and data/local stay in the sidecar.
func portSidecar(awfDir, kind, name string, sc legacySidecar) error {
	kept, err := portSectionOverrides(sc.Sections, awfDir, func(sec string) string {
		return filepath.Join(awfDir, kind, "parts", name, sec+".md")
	})
	if err != nil {
		return err
	}
	return writeSidecarDoc(filepath.Join(awfDir, kind, name+".yaml"), sc.Data, kept, sc.Local, false)
}

// portSectionOverrides walks a legacy sidecar's section overrides: each replaceWith
// section is copied out to the convention part at dst(sec); each drop is preserved
// in the returned kept map. The shared body of portSidecar and portAgentsDoc.
func portSectionOverrides(sections map[string]legacySectionOverride, awfDir string, dst func(sec string) string) (map[string]any, error) {
	kept := map[string]any{}
	for _, sec := range slices.Sorted(maps.Keys(sections)) {
		ov := sections[sec]
		if ov.ReplaceWith != "" {
			if err := copyPart(filepath.Join(awfDir, ov.ReplaceWith), dst(sec)); err != nil {
				return nil, err
			}
			continue
		}
		if ov.Drop {
			kept[sec] = map[string]any{"drop": true}
		}
	}
	return kept, nil
}

// portAgentsDoc re-models the agents-doc singleton: the ownership/identity
// scalars become convention parts (with their `## ` headings, so the rendered
// AGENTS.md stays byte-identical — the section body the template emits includes
// the heading), explicit replaceWith sections become convention parts under
// parts/agents-doc/, and the remaining data/drops land in agents-doc.yaml.
func portAgentsDoc(awfDir string, ad legacySidecar) error {
	data := map[string]any{}
	for k, v := range ad.Data {
		data[k] = v
	}
	heads := map[string]string{
		"you-and-this-project": "## You and this project",
		"identity":             "## Identity",
	}
	for _, p := range []struct{ key, section string }{
		{"ownership", "you-and-this-project"}, {"identity", "identity"},
	} {
		v, ok := data[p.key]
		if !ok {
			continue
		}
		delete(data, p.key)
		dst := filepath.Join(awfDir, "parts", "agents-doc", p.section+".md")
		body := heads[p.section] + "\n\n" + fmt.Sprint(v)
		if err := writeFile(dst, []byte(body)); err != nil {
			return err
		}
	}
	kept, err := portSectionOverrides(ad.Sections, awfDir, func(sec string) string {
		return filepath.Join(awfDir, "parts", "agents-doc", sec+".md")
	})
	if err != nil {
		return err
	}
	return writeSidecarDoc(filepath.Join(awfDir, "agents-doc.yaml"), data, kept, ad.Local, false)
}

// copyPart reads a legacy part body and writes it verbatim to its convention
// path, then removes the legacy source so the old flat parts/ dir is cleaned up.
func copyPart(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read part %s: %w", src, err)
	}
	if err := writeFile(dst, b); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil && !errors.Is(err, os.ErrNotExist) { // coverage-ignore: removal of the legacy part src we just read and copied; a non-NotExist error needs a permission fault that root bypasses
		return err
	}
	return nil
}

// writeSidecarDoc assembles a sidecar doc {data, sections, local} and writes it to
// path. When the doc is empty it removes path (removeIfEmpty: in-place schema
// conversion) or is a no-op (a fresh port to a new tree, where path does not exist).
func writeSidecarDoc(path string, data, sections map[string]any, local, removeIfEmpty bool) error {
	doc := map[string]any{}
	if len(data) > 0 {
		doc["data"] = data
	}
	if len(sections) > 0 {
		doc["sections"] = sections
	}
	if local {
		doc["local"] = true
	}
	if len(doc) == 0 {
		if removeIfEmpty {
			return os.Remove(path)
		}
		return nil
	}
	return writeYAML(path, doc)
}

func writeYAML(path string, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil { // coverage-ignore: v is always a skeleton map or sidecar values decoded from YAML; yaml.Marshal has no unsupported type to fail on
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return writeFile(path, b)
}

func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
