package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
	skeleton["skills"] = sortedSidecarNames(lc.Skills)
	skeleton["agents"] = sortedSidecarNames(lc.Agents)
	skeleton["docs"] = sortedSidecarNames(lc.Docs)
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
		for _, name := range sortedSidecarNames(kv.set) {
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
	keptSections := map[string]any{}
	for _, sec := range sortedOverrideNames(sc.Sections) {
		ov := sc.Sections[sec]
		if ov.ReplaceWith != "" {
			dst := filepath.Join(awfDir, kind, "parts", name, sec+".md")
			if err := copyPart(filepath.Join(awfDir, ov.ReplaceWith), dst); err != nil {
				return err
			}
			continue
		}
		if ov.Drop {
			keptSections[sec] = map[string]any{"drop": true}
		}
	}
	sidecar := map[string]any{}
	if len(sc.Data) > 0 {
		sidecar["data"] = sc.Data
	}
	if len(keptSections) > 0 {
		sidecar["sections"] = keptSections
	}
	if sc.Local {
		sidecar["local"] = true
	}
	if len(sidecar) == 0 {
		return nil // nothing to override → no sidecar (enabled by config.yaml array)
	}
	return writeYAML(filepath.Join(awfDir, kind, name+".yaml"), sidecar)
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
	keptSections := map[string]any{}
	for _, sec := range sortedOverrideNames(ad.Sections) {
		ov := ad.Sections[sec]
		if ov.ReplaceWith != "" {
			dst := filepath.Join(awfDir, "parts", "agents-doc", sec+".md")
			if err := copyPart(filepath.Join(awfDir, ov.ReplaceWith), dst); err != nil {
				return err
			}
			continue
		}
		if ov.Drop {
			keptSections[sec] = map[string]any{"drop": true}
		}
	}
	sidecar := map[string]any{}
	if len(data) > 0 {
		sidecar["data"] = data
	}
	if len(keptSections) > 0 {
		sidecar["sections"] = keptSections
	}
	if ad.Local {
		sidecar["local"] = true
	}
	if len(sidecar) == 0 {
		return nil
	}
	return writeYAML(filepath.Join(awfDir, "agents-doc.yaml"), sidecar)
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

func sortedSidecarNames(m map[string]legacySidecar) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedOverrideNames(m map[string]legacySectionOverride) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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
