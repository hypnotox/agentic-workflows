package project

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

// validateAgainstCatalog checks that every enabled non-local target is in the
// catalog and that its sidecar's section overrides name declared sections.
func (p *Project) validateAgainstCatalog() error {
	for _, d := range kindDescriptors {
		if d.poolNames == nil { // domains: freeform, not catalog-validated
			continue
		}
		if err := p.checkKindAgainstCatalog(d); err != nil {
			return err
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

// checkKindAgainstCatalog verifies every enabled non-local target of a
// catalog-backed kind is in the catalog and that its sidecar section overrides
// name declared sections.
func (p *Project) checkKindAgainstCatalog(d kindDescriptor) error {
	pool := d.poolNames(p.Cat)
	for _, name := range d.enable(p.Cfg) {
		sc, err := p.Cfg.Sidecar(d.Plural, name)
		if err != nil {
			return err
		}
		if sc.Local {
			continue
		}
		if !slices.Contains(pool, name) {
			return fmt.Errorf("%s %q is not in the catalog", d.Singular, name)
		}
		if declared, ok := d.sections(p.Cat, name); ok {
			if err := checkSectionsAllowed(d.Plural, name, declared, sc.Sections); err != nil {
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
