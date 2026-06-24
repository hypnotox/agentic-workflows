// cmd/awf/list_add.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"agentic-workflows/internal/config"
	"agentic-workflows/internal/project"
)

// skillState returns the display state of a skill: "local", "tuned", "enabled", or "available".
// enabled is true when the skill appears in the project config.
func skillState(sc config.SkillConfig, enabled bool) string {
	if !enabled {
		return "available"
	}
	switch {
	case sc.Local:
		return "local"
	case sc.Data != nil || sc.Sections != nil:
		return "tuned"
	default:
		return "enabled"
	}
}

func runList(root string) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(p.Cat.Skills))
	for n := range p.Cat.Skills {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		sc, ok := p.Cfg.Skills[n]
		fmt.Printf("  %-24s %s\n", n, skillState(sc, ok))
	}
	return nil
}

func runAdd(root, skill string) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if _, ok := p.Cat.Skills[skill]; !ok {
		return fmt.Errorf("%q is not a catalog skill (run: awf list)", skill)
	}
	if _, ok := p.Cfg.Skills[skill]; ok {
		return fmt.Errorf("%q already enabled", skill)
	}
	cfgPath := filepath.Join(root, ".claude", "awf.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	updated, err := appendSkill(string(b), skill)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, []byte(updated), 0o644); err != nil {
		return err
	}
	return runSync(root)
}

// appendSkill inserts "  <skill>: {}" immediately after the "skills:" line.
// Handles the "skills: {}" empty-map form by converting it to a block mapping.
func appendSkill(src, skill string) (string, error) {
	lines := splitLines(src)
	for i, l := range lines {
		if l == "skills: {}" {
			lines[i] = "skills:"
			rest := append([]string{"  " + skill + ": {}"}, lines[i+1:]...)
			return joinLines(append(lines[:i+1], rest...)), nil
		}
		if l == "skills:" {
			rest := append([]string{"  " + skill + ": {}"}, lines[i+1:]...)
			return joinLines(append(lines[:i+1], rest...)), nil
		}
	}
	return "", fmt.Errorf("no skills: key in awf.yaml")
}

func joinLines(ls []string) string {
	out := ""
	for i, l := range ls {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

func splitLines(s string) []string {
	var out, cur = []string{}, ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	return append(out, cur)
}
