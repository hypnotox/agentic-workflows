package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// skillState returns the display state of a skill: "local", "tuned", "enabled",
// or "available". enabled is true when the skill appears in the config enable
// array; sc is its loaded sidecar (zero when none).
func skillState(sc config.Sidecar, enabled bool) string {
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

func runList(root string, stdout io.Writer) error {
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
		enabled := slices.Contains(p.Cfg.Skills, n)
		var sc config.Sidecar
		if enabled {
			if sc, err = p.Cfg.Sidecar("skills", n); err != nil { // coverage-ignore: project.Open validates sidecars, so a malformed one fails Open before this read
				return err
			}
		}
		fmt.Fprintf(stdout, "  %-24s %s\n", n, skillState(sc, enabled))
	}
	return nil
}

func runAdd(root, skill string, stdout io.Writer) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if _, ok := p.Cat.Skills[skill]; !ok {
		return fmt.Errorf("%q is not a catalog skill (run: awf list)", skill)
	}
	if slices.Contains(p.Cfg.Skills, skill) {
		return fmt.Errorf("%q already enabled", skill)
	}
	cfgPath := filepath.Join(root, ".awf", "config.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil { // coverage-ignore: config.yaml was just read by project.Open; a re-read cannot fail without a race
		return err
	}
	updated, err := appendSkill(string(b), skill)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, []byte(updated), 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault that root bypasses
		return err
	}
	return runSync(root, stdout)
}

// appendSkill adds "- <skill>" under the "skills:" key in config.yaml, handling
// the empty-array forms "skills: []" and a bare "skills:" line.
func appendSkill(src, skill string) (string, error) {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if l == "skills: []" {
			lines[i] = "skills:"
			rest := append([]string{"  - " + skill}, lines[i+1:]...)
			return strings.Join(append(lines[:i+1], rest...), "\n"), nil
		}
		if l == "skills:" {
			rest := append([]string{"  - " + skill}, lines[i+1:]...)
			return strings.Join(append(lines[:i+1], rest...), "\n"), nil
		}
	}
	return "", errors.New("no skills: key in config.yaml")
}
