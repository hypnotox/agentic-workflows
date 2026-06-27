package project

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

// validateAgainstCatalog checks that every enabled non-local target is in the
// catalog and that its sidecar's section overrides name declared sections.
func (p *Project) validateAgainstCatalog() error {
	checkKind := func(kind string, names []string, specs func(string) ([]string, bool)) error {
		for _, name := range names {
			sc, err := p.Cfg.Sidecar(kind, name)
			if err != nil {
				return err
			}
			if sc.Local {
				continue
			}
			declared, ok := specs(name)
			if !ok {
				return fmt.Errorf("%s %q is not in the catalog", strings.TrimSuffix(kind, "s"), name)
			}
			if err := checkSectionsAllowed(kind, name, declared, sc.Sections); err != nil {
				return err
			}
		}
		return nil
	}
	if err := checkKind("skills", p.Cfg.Skills, func(n string) ([]string, bool) {
		s, ok := p.Cat.Skills[n]
		return s.Sections, ok
	}); err != nil {
		return err
	}
	if err := checkKind("agents", p.Cfg.Agents, func(n string) ([]string, bool) {
		a, ok := p.Cat.Agents[n]
		return a.Sections, ok
	}); err != nil {
		return err
	}
	if err := checkKind("docs", p.Cfg.Docs, func(n string) ([]string, bool) {
		d, ok := p.Cat.Docs[n]
		return d.Sections, ok
	}); err != nil {
		return err
	}
	// Hooks against catalog.
	catHooks := make(map[string]bool, len(p.Cat.Hooks))
	for _, h := range p.Cat.Hooks {
		catHooks[h] = true
	}
	for _, h := range p.Cfg.Hooks {
		if !catHooks[h] {
			return fmt.Errorf("hook %q is not in the catalog", h)
		}
	}
	// agents-doc section overrides against catalog (always-on singleton).
	ad, err := p.Cfg.Sidecar("agents-doc", "")
	if err != nil {
		return err
	}
	if !ad.Local {
		if err := checkSectionsAllowed("agents-doc", "", p.Cat.AgentsDoc.Sections, ad.Sections); err != nil {
			return err
		}
	}
	for _, sg := range []struct {
		kind     string
		sections []string
	}{
		{"adr-readme", p.Cat.AdrReadme.Sections},
		{"adr-template", p.Cat.AdrTemplate.Sections},
		{"plans-readme", p.Cat.PlansReadme.Sections},
	} {
		sc, err := p.Cfg.Sidecar(sg.kind, "")
		if err != nil {
			return err
		}
		if !sc.Local {
			if err := checkSectionsAllowed(sg.kind, "", sg.sections, sc.Sections); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkSectionsAllowed verifies that every key in used appears in declared.
// kind and name are used only for error formatting; name may be empty for a
// singleton (e.g. agents-doc).
func checkSectionsAllowed(kind, name string, declared []string, used map[string]config.SectionOverride) error {
	allowed := make(map[string]bool, len(declared))
	for _, s := range declared {
		allowed[s] = true
	}
	label := kind
	if name != "" {
		label = fmt.Sprintf("%s %q", kind, name)
	}
	for sec := range used {
		if !allowed[sec] {
			return fmt.Errorf("%s: unknown section %q (not declared in the catalog)", label, sec)
		}
	}
	return nil
}

// skillFrontmatter is the rendered skill/agent frontmatter contract Claude Code
// requires.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// isSkillOrAgent reports whether a template id renders a skill or agent (the
// outputs that must carry name/description frontmatter).
func isSkillOrAgent(templateID string) bool {
	return strings.HasPrefix(templateID, "skills/") || strings.HasPrefix(templateID, "agents/")
}

// validateFrontmatter checks that content has parseable frontmatter with a
// non-empty name and description.
func validateFrontmatter(content []byte) error {
	var fm skillFrontmatter
	_, found, err := frontmatter.Parse(content, &fm)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("missing frontmatter")
	}
	if strings.TrimSpace(fm.Name) == "" {
		return errors.New("frontmatter name is empty")
	}
	if strings.TrimSpace(fm.Description) == "" {
		return errors.New("frontmatter description is empty")
	}
	return nil
}
