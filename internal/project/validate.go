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
		if err := checkSectionsAllowed("agents-doc", "", p.Cat.Docs["agents-doc"].Sections, ad.Sections); err != nil {
			return err
		}
	}
	for _, sg := range plainSingletons {
		sc, err := p.Cfg.Sidecar(sg.kind, "")
		if err != nil {
			return err
		}
		if !sc.Local {
			if err := checkSectionsAllowed(sg.kind, "", sg.sections(p.Cat), sc.Sections); err != nil {
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
		// Pairing validation (ADR-0050): a reviewing skill may never be enabled
		// without the agent it dispatches. Unlike requiresDoc suppression, this
		// is a hard error — a silently-thinner chain is the failure mode the
		// workflow exists to prevent.
		// invariant: reviewing-skill-agent-pairing
		if d.Plural == "skills" {
			if req := p.Cat.Skills[name].RequiresAgent; req != "" && !slices.Contains(p.Cfg.Agents, req) {
				return fmt.Errorf("skill %q requires agent %q; enable the agent or disable the skill", name, req)
			}
		}
		if declared, ok := d.sections(p.Cat, name); ok {
			if err := checkSectionsAllowed(d.Plural, name, declared, sc.Sections); err != nil {
				return err
			}
		}
	}
	return nil
}

// SkillsRequiringAgent returns the enabled, non-local skills whose catalog
// spec requires agent — exactly the set the pairing validation would fail on
// if the agent left the enable array. `awf remove agent` refuses while it is
// non-empty (ADR-0050).
func (p *Project) SkillsRequiringAgent(agent string) []string {
	var out []string
	for _, name := range p.Cfg.Skills {
		sc, err := p.Cfg.Sidecar("skills", name)
		if err != nil || sc.Local { // err: unreachable — Open pre-validated every enabled sidecar
			continue
		}
		if p.Cat.Skills[name].RequiresAgent == agent {
			out = append(out, name)
		}
	}
	return out
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
