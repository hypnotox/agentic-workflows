package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// kindKey maps a singular CLI kind token to its plural config enable-array key.
// invariant: cli-config-kinds
var kindKey = map[string]string{
	"skill":  "skills",
	"agent":  "agents",
	"doc":    "docs",
	"hook":   "hooks",
	"domain": "domains",
}

// kindsOrdered is the display order for `awf list`.
var kindsOrdered = []string{"skill", "agent", "doc", "hook", "domain"}

func unknownKind(kind string) error {
	return fmt.Errorf("unknown kind %q (want: skill, agent, doc, hook, domain)", kind)
}

// enabledNames returns the config enable array for a kind.
func enabledNames(cfg *config.Config, kind string) []string {
	switch kind {
	case "skill":
		return cfg.Skills
	case "agent":
		return cfg.Agents
	case "doc":
		return cfg.Docs
	case "hook":
		return cfg.Hooks
	default: // domain
		return cfg.Domains
	}
}

// catalogNames returns the catalog pool for a catalog-backed kind; the second
// result is false for `domain`, which is freeform (no catalog pool).
func catalogNames(cat *catalog.Catalog, kind string) ([]string, bool) {
	switch kind {
	case "skill":
		return slices.Sorted(maps.Keys(cat.Skills)), true
	case "agent":
		return slices.Sorted(maps.Keys(cat.Agents)), true
	case "doc":
		return slices.Sorted(maps.Keys(cat.Docs)), true
	case "hook":
		return slices.Sorted(slices.Values(cat.Hooks)), true
	default: // domain
		return nil, false
	}
}

func runAdd(root, kind, name string, stdout io.Writer) error {
	key, ok := kindKey[kind]
	if !ok {
		return unknownKind(kind)
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if pool, catalogBacked := catalogNames(p.Cat, kind); catalogBacked {
		if !slices.Contains(pool, name) {
			return fmt.Errorf("%q is not a catalog %s (run: awf list %s)", name, kind, kind)
		}
	} else if err := config.ValidateDomainName(name); err != nil {
		return err
	}
	if slices.Contains(enabledNames(p.Cfg, kind), name) {
		return fmt.Errorf("%s %q already enabled", kind, name)
	}
	if err := rewriteConfig(root, key, name, true); err != nil { // coverage-ignore: rewriteConfig only fails on faults its own ignored branches cover; unreachable after the validations above
		return err
	}
	// Doc-gated skill: warn when its required doc is not enabled, since it would
	// otherwise render nothing (inv: doc-gated-skill-suppressed).
	if kind == "skill" {
		if req := p.Cat.Skills[name].RequiresDoc; req != "" && !slices.Contains(p.Cfg.Docs, req) {
			fmt.Fprintf(stdout, "note: skill %q requires the %q doc, which is not enabled — it will not render until you run `awf add doc %s`\n", name, req, req)
		}
	}
	return runSync(root, stdout)
}

func runRemove(root, kind, name string, stdout io.Writer) error {
	key, ok := kindKey[kind]
	if !ok {
		return unknownKind(kind)
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if !slices.Contains(enabledNames(p.Cfg, kind), name) {
		return fmt.Errorf("%s %q is not enabled", kind, name)
	}
	if err := rewriteConfig(root, key, name, false); err != nil { // coverage-ignore: rewriteConfig only fails on faults its own ignored branches cover; unreachable after the validations above
		return err
	}
	if hasSidecarOrParts(root, key, name) {
		fmt.Fprintf(stdout, "note: %s %q still has a sidecar or convention parts under .awf/ — now orphaned (awf check will flag them); delete them or re-add to keep them\n", kind, name)
	}
	return runSync(root, stdout)
}

// rewriteConfig edits the enable array for key in .awf/config.yaml (adding or
// removing name) and writes it back.
func rewriteConfig(root, key, name string, add bool) error {
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil { // coverage-ignore: config.yaml was just read by project.Open; a re-read cannot fail without a race
		return err
	}
	updated, err := editArray(string(b), key, name, add)
	if err != nil { // coverage-ignore: add/remove validate membership against the live config before editing, so the array branch they hit always exists
		return err
	}
	if err := os.WriteFile(cfgPath, []byte(updated), 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault that root bypasses
		return err
	}
	return nil
}

// hasSidecarOrParts reports whether an orphaned sidecar (<key>/<name>.yaml) or
// convention-parts dir (<key>/parts/<name>) for the target exists under .awf/.
func hasSidecarOrParts(root, key, name string) bool {
	awf := filepath.Join(root, ".awf")
	for _, p := range []string{
		filepath.Join(awf, key, name+".yaml"),
		filepath.Join(awf, key, "parts", name),
	} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// editArray adds or removes "  - name" under the "key:" block of a config.yaml
// source, scoped to that key's block so a name shared across kinds is touched in
// the right array only. It handles the key with items, the "key: []" and bare
// "key:" forms, and — for add — the key being absent entirely.
func editArray(src, key, name string, add bool) (string, error) {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		switch l {
		case key + ": []":
			if !add {
				return "", fmt.Errorf("%s has no %q entry", key, name)
			}
			lines[i] = key + ":"
			return strings.Join(slices.Insert(lines, i+1, "  - "+name), "\n"), nil
		case key + ":":
			if add {
				return strings.Join(slices.Insert(lines, i+1, "  - "+name), "\n"), nil
			}
			// Scan only this key's block (indented items) so a same-named entry under
			// another kind is left untouched. invariant: remove-block-scoped
			for j := i + 1; j < len(lines) && strings.HasPrefix(lines[j], "  "); j++ {
				if lines[j] == "  - "+name {
					return strings.Join(slices.Delete(lines, j, j+1), "\n"), nil
				}
			}
			return "", fmt.Errorf("%s has no %q entry", key, name)
		}
	}
	if !add {
		return "", fmt.Errorf("no %s: key in config.yaml", key)
	}
	// Key absent: append a new block before any trailing empty line.
	block := []string{key + ":", "  - " + name}
	if n := len(lines); n > 0 && lines[n-1] == "" {
		return strings.Join(slices.Insert(lines, n-1, block...), "\n"), nil
	}
	return strings.Join(append(lines, block...), "\n"), nil // coverage-ignore: a config.yaml read from disk always ends in a newline, so Split leaves a trailing "" and the branch above is taken
}

func runList(root, kindFilter string, stdout io.Writer) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	kinds := kindsOrdered
	if kindFilter != "" {
		if _, ok := kindKey[kindFilter]; !ok {
			return unknownKind(kindFilter)
		}
		kinds = []string{kindFilter}
	}
	for _, kind := range kinds {
		fmt.Fprintf(stdout, "%s:\n", kindKey[kind])
		pool, catalogBacked := catalogNames(p.Cat, kind)
		if !catalogBacked { // domains: configured set only
			for _, n := range slices.Sorted(slices.Values(p.Cfg.Domains)) {
				fmt.Fprintf(stdout, "  %-28s %s\n", n, "configured")
			}
			continue
		}
		for _, n := range pool {
			fmt.Fprintf(stdout, "  %-28s %s\n", n, targetState(p, kind, n))
		}
	}
	return nil
}

// targetState returns the display state of a catalog-backed target: "available"
// when not enabled, else "local"/"tuned"/"enabled" from its sidecar (hooks carry
// no sidecar, so they are only "enabled"/"available").
func targetState(p *project.Project, kind, name string) string {
	if !slices.Contains(enabledNames(p.Cfg, kind), name) {
		return "available"
	}
	if kind != "hook" {
		sc, _ := p.Cfg.Sidecar(kindKey[kind], name)
		switch {
		case sc.Local:
			return "local"
		case sc.Data != nil || sc.Sections != nil:
			return "tuned"
		}
	}
	return "enabled"
}
