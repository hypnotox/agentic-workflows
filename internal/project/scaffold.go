package project

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// ScaffoldConfig generates the bytes of a .awf/config.yaml that enables
// every skill, agent, and hook in the embedded catalog (as flat name arrays) and
// pre-populates the vars block with the union of all {{ .vars.X }} names
// referenced by those templates. Each var is seeded with an empty string so that
// strict render (missingkey=zero + <no value> check) does not fail on sync.
func ScaffoldConfig(prefix string) ([]byte, error) {
	cat, err := catalog.Load(templates.FS)
	if err != nil { // coverage-ignore: catalog.Load over the embedded templates.FS cannot fail at runtime
		return nil, fmt.Errorf("scaffold: load catalog: %w", err)
	}

	// Collect all referenced var names from every template.
	varSet := map[string]bool{}

	// Skill templates.
	for name := range cat.Skills {
		path := fmt.Sprintf("skills/%s/SKILL.md.tmpl", name)
		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog skill name has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	// Agent templates.
	for name := range cat.Agents {
		path := fmt.Sprintf("agents/%s.md.tmpl", name)
		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog agent name has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	// Hook templates.
	for _, hook := range cat.Hooks {
		path := fmt.Sprintf("hooks/%s.tmpl", hook)
		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog hook name has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}

	// Build sorted var names.
	varNames := sortedKeys(varSet)

	// Build sorted skill names.
	skillNames := sortedKeys(cat.Skills)

	// Build sorted agent names.
	agentNames := sortedKeys(cat.Agents)

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
		b.WriteString("  - ")
		b.WriteString(name)
		b.WriteString("\n")
	}

	b.WriteString("agents:\n")
	for _, name := range agentNames {
		b.WriteString("  - ")
		b.WriteString(name)
		b.WriteString("\n")
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
