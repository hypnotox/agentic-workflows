package project

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"agentic-workflows/internal/catalog"
	"agentic-workflows/internal/render"
	"agentic-workflows/templates"
)

// ScaffoldConfig generates the bytes of a .claude/awf.yaml that enables every
// skill, agent, and hook in the embedded catalog and pre-populates the vars
// block with the union of all {{ .vars.X }} names referenced by those templates.
// Each var is seeded with an empty string so that strict render (missingkey=zero
// + <no value> check) does not fail on sync.
func ScaffoldConfig(prefix string) ([]byte, error) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		return nil, fmt.Errorf("scaffold: load catalog: %w", err)
	}

	// Collect all referenced var names from every template.
	varSet := map[string]bool{}

	// Skill templates.
	for name := range cat.Skills {
		path := fmt.Sprintf("skills/%s/SKILL.md.tmpl", name)
		if err := collectVars(templates.FS, path, varSet); err != nil {
			return nil, err
		}
	}
	// Agent templates.
	for name := range cat.Agents {
		path := fmt.Sprintf("agents/%s.md.tmpl", name)
		if err := collectVars(templates.FS, path, varSet); err != nil {
			return nil, err
		}
	}
	// Hook templates.
	for _, hook := range cat.Hooks {
		path := fmt.Sprintf("hooks/%s.tmpl", hook)
		if err := collectVars(templates.FS, path, varSet); err != nil {
			return nil, err
		}
	}

	// Build sorted var names.
	varNames := make([]string, 0, len(varSet))
	for v := range varSet {
		varNames = append(varNames, v)
	}
	sort.Strings(varNames)

	// Build sorted skill names.
	skillNames := make([]string, 0, len(cat.Skills))
	for name := range cat.Skills {
		skillNames = append(skillNames, name)
	}
	sort.Strings(skillNames)

	// Build sorted agent names.
	agentNames := make([]string, 0, len(cat.Agents))
	for name := range cat.Agents {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

	// Preserve catalog hook order (already deterministic).
	hookList := make([]string, len(cat.Hooks))
	copy(hookList, cat.Hooks)

	// Emit YAML manually so we control the output format and avoid any
	// round-trip issues with the strict config.Load decoder.
	var b strings.Builder

	b.WriteString("prefix: ")
	b.WriteString(prefix)
	b.WriteString("\n")

	b.WriteString("vars:\n")
	for _, v := range varNames {
		b.WriteString("  ")
		b.WriteString(v)
		b.WriteString(": \"\"\n")
	}

	b.WriteString("skills:\n")
	for _, name := range skillNames {
		b.WriteString("  ")
		b.WriteString(name)
		b.WriteString(": {}\n")
	}

	b.WriteString("agents:\n")
	for _, name := range agentNames {
		b.WriteString("  ")
		b.WriteString(name)
		b.WriteString(": {}\n")
	}

	b.WriteString("hooks:\n")
	for _, hook := range hookList {
		b.WriteString("  - ")
		b.WriteString(hook)
		b.WriteString("\n")
	}

	return []byte(b.String()), nil
}

// collectVars reads the template at path and adds all .vars.X names to varSet.
func collectVars(fsys fs.FS, path string, varSet map[string]bool) error {
	src, err := fs.ReadFile(fsys, path)
	if err != nil {
		return fmt.Errorf("scaffold: read template %s: %w", path, err)
	}
	for _, v := range render.ReferencedVars(string(src)) {
		varSet[v] = true
	}
	return nil
}
