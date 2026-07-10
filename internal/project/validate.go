package project

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
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
	// Domain sidecars are paths-only (ADR-0086 Decision 5): domain rendering
	// passes an empty sidecar and injects its own data map, so an authored
	// data:, sections:, or local: entry silently does nothing — and the
	// domain template's own .data.domain reference would mask a data: block
	// from the consumption check.
	for _, name := range p.Cfg.Domains {
		sc, err := p.Cfg.Sidecar("domains", name)
		if err != nil {
			return err
		}
		if len(sc.Data) > 0 || len(sc.Sections) > 0 || sc.Local {
			return fmt.Errorf("domain %q: a domain sidecar is paths-only — nothing reads data:, sections:, or local: on it; remove them from .awf/domains/%s.yaml", name, name)
		}
	}
	// agents-doc section overrides against catalog (always-on singleton).
	ad, err := p.Cfg.Sidecar("agents-doc", "")
	if err != nil {
		return err
	}
	if len(ad.Paths) > 0 {
		return errors.New("agents-doc: paths: is read only from domain sidecars; remove it from .awf/agents-doc.yaml")
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
		if len(sc.Paths) > 0 {
			return fmt.Errorf("%s: paths: is read only from domain sidecars; remove it from .awf/%s.yaml", sg.kind, sg.kind)
		}
		if !sc.Local {
			if err := checkSectionsAllowed(sg.kind, "", sg.sections(p.Cat), sc.Sections); err != nil {
				return err
			}
		}
	}
	// The config reference's data namespace is injected at generation
	// (ADR-0088): authored data: would be silently overwritten while its key
	// names look consumed, so it is rejected like the domain paths-only rule.
	// The Generated entry left plainSingletons, so its sidecar checks live here.
	// invariant: config-reference-data-rejected
	cr, err := p.Cfg.Sidecar("config-reference", "")
	if err != nil {
		return err
	}
	if len(cr.Data) > 0 {
		return errors.New("config-reference: the reference tables are generated — data: has no effect; remove it from .awf/config-reference.yaml (sections:/local: remain available)")
	}
	if len(cr.Paths) > 0 {
		return errors.New("config-reference: paths: is read only from domain sidecars; remove it from .awf/config-reference.yaml")
	}
	if !cr.Local {
		if err := checkSectionsAllowed("config-reference", "", p.Cat.Docs["config-reference"].Sections, cr.Sections); err != nil {
			return err
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
		// Inert-field rejection (ADR-0086 Decision 5): paths: is read only from
		// domain sidecars (ADR-0077), so on any other kind it is configuration
		// that silently does nothing. Checked before the local: skip — a local
		// sidecar cannot carry it either.
		// invariant: inert-sidecar-field-rejected
		if len(sc.Paths) > 0 {
			return fmt.Errorf("%s %q: paths: is read only from domain sidecars; remove it from .awf/%s/%s.yaml", d.Singular, name, d.Plural, name)
		}
		if sc.Local {
			continue
		}
		if !slices.Contains(pool, name) {
			return fmt.Errorf("%s %q is not in the catalog", d.Singular, name)
		}
		// Closure validation (ADR-0081): every enabled, non-local artifact's
		// direct catalog requirements are enabled — transitive closure follows
		// by induction. Generalizes the ADR-0050 RequiresAgent pairing (that
		// edge is now one case of the same loop); a silently-thinner chain is
		// the failure mode the workflow exists to prevent.
		// invariant: reviewing-skill-agent-pairing
		// invariant: enabled-set-closed
		if d.Plural == "skills" || d.Plural == "agents" {
			if err := p.checkNodeRequirements(catalog.Node{Kind: d.Singular, Name: name}); err != nil {
				return err
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

// checkNodeRequirements fails when any of n's direct catalog requirements is
// not enabled, with a repair hint naming the exact edit and awf upgrade as
// the pre-migration recovery path (ADR-0081 Decision 3).
func (p *Project) checkNodeRequirements(n catalog.Node) error {
	for _, r := range catalog.RequiresOf(p.Cat, n) {
		if !p.nodeEnabled(r) {
			return fmt.Errorf("%s %q requires %s %q; add it to %s: in .awf/config.yaml (or run `awf upgrade` after a binary upgrade), or remove the %s",
				n.Kind, n.Name, r.Kind, r.Name, r.Kind+"s", n.Kind)
		}
	}
	return nil
}

// nodeEnabled reports whether n appears in its kind's config enable array.
func (p *Project) nodeEnabled(n catalog.Node) bool {
	switch n.Kind {
	case "skill":
		return slices.Contains(p.Cfg.Skills, n.Name)
	case "agent":
		return slices.Contains(p.Cfg.Agents, n.Name)
	case "doc":
		return slices.Contains(p.Cfg.Docs, n.Name)
	}
	return false
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
