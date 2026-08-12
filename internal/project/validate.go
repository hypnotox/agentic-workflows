package project

import (
	"errors"
	"fmt"
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
	// passes an empty sidecar and injects its own data map, so authored data:
	// or sections: silently does nothing - and the domain template's own
	// .data.domain reference would mask a data: block
	// from the consumption check.
	for _, name := range p.Cfg.Domains {
		sc, err := p.Cfg.Sidecar("domains", name)
		if err != nil {
			return err
		}
		if len(sc.Data) > 0 || len(sc.DataDefaults) > 0 || len(sc.Sections) > 0 {
			return fmt.Errorf("domain %q: a domain sidecar is paths-only; nothing reads data:, dataDefaults:, or sections: on it; remove them from .awf/domains/%s.yaml", name, name)
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
	if err := validateCatalogListData("agents-doc.yaml", ad, p.Cat.Docs["agents-doc"].Data); err != nil {
		return err
	}
	if err := checkSectionsAllowed("agents-doc", "", p.Cat.Docs["agents-doc"].Sections, ad.Sections); err != nil {
		return err
	}
	for _, sg := range plainSingletons {
		sc, err := p.Cfg.Sidecar(sg.kind, "")
		if err != nil {
			return err
		}
		if len(sc.Paths) > 0 {
			return fmt.Errorf("%s: paths: is read only from domain sidecars; remove it from .awf/%s.yaml", sg.kind, sg.kind)
		}
		if err := validateCatalogListData(sg.kind+".yaml", sc, p.Cat.Docs[sg.kind].Data); err != nil {
			return err
		}
		if err := checkSectionsAllowed(sg.kind, "", sg.sections(p.Cat), sc.Sections); err != nil {
			return err
		}
	}
	// The config reference's data namespace is injected at generation
	// (ADR-0088): authored data: would be silently overwritten while its key
	// names look consumed, so it is rejected like the domain paths-only rule.
	// The Generated entry left plainSingletons, so its sidecar checks live here.
	cr, err := p.Cfg.Sidecar("config-reference", "")
	if err != nil {
		return err
	}
	if len(cr.Data) > 0 || len(cr.DataDefaults) > 0 {
		return errors.New("config-reference: the reference tables are generated; data: and dataDefaults: have no effect; remove them from .awf/config-reference.yaml (sections: remains available)")
	}
	if len(cr.Paths) > 0 {
		return errors.New("config-reference: paths: is read only from domain sidecars; remove it from .awf/config-reference.yaml")
	}
	if err := checkSectionsAllowed("config-reference", "", p.Cat.Docs["config-reference"].Sections, cr.Sections); err != nil {
		return err
	}
	for _, local := range p.Cfg.NormalizedLocalDocs() {
		output := config.DocsDir + "/" + local.Name + ".md"
		for name := range p.Cat.Docs {
			if output == p.docOutPath(name) {
				return fmt.Errorf("local document %q collides with standard output %q", local.Name, output)
			}
		}
	}
	return nil
}

// checkKindAgainstCatalog validates every catalog artifact's shaping sidecar.
func (p *Project) checkKindAgainstCatalog(d kindDescriptor) error {
	for _, name := range d.poolNames(p.Cat) {
		sc, err := p.Cfg.Sidecar(d.Plural, name)
		if err != nil {
			return err
		}
		if len(sc.Paths) > 0 {
			return fmt.Errorf("%s %q: paths: is read only from domain sidecars; remove it from .awf/%s/%s.yaml", d.Singular, name, d.Plural, name)
		}
		if err := validateCatalogListData(d.Plural+"/"+name+".yaml", sc, catalogData(p.Cat, d.Plural, name), specializedListDataKeys(d.Plural, name)...); err != nil {
			return err
		}
		if declared, ok := d.sections(p.Cat, name); ok {
			if err := checkSectionsAllowed(d.Plural, name, declared, sc.Sections); err != nil {
				return err
			}
		}
	}
	return nil
}

func catalogData(cat *catalog.Catalog, kind, name string) map[string]any {
	switch kind {
	case "skills":
		return cat.Skills[name].Data
	case "agents":
		return cat.Agents[name].Data
	case "docs":
		return cat.Docs[name].Data
	default:
		return nil
	}
}

// validateCatalogListData owns the sidecar contract for same-key catalog list
// layering. Differently keyed transforms such as glossary standardTerms/terms
// are intentionally outside this generic path.
func validateCatalogListData(sidecar string, sc config.Sidecar, defaults map[string]any, listLayerExclusions ...string) error {
	excluded := make(map[string]bool, len(listLayerExclusions))
	for _, key := range listLayerExclusions {
		excluded[key] = true
	}
	for key := range sc.DataDefaults {
		if _, ok := defaults[key].([]any); !ok || excluded[key] {
			return fmt.Errorf("%s dataDefaults.%s must name a same-key catalog list default", sidecar, key)
		}
	}
	for key, defaultValue := range defaults {
		if _, ok := defaultValue.([]any); !ok || excluded[key] {
			continue
		}
		value, present := sc.Data[key]
		if !present {
			continue
		}
		if value == nil {
			return fmt.Errorf("%s data.%s must be a list, not null; use dataDefaults.%s: false to suppress the catalog default", sidecar, key, key)
		}
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("%s data.%s must be a list because its catalog default is a list", sidecar, key)
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

// validateCommandWiring binds the always-rendered payloads' gate command at
// sync and check, but not init or the staged index check.
func validateCommandWiring(cfg *config.Config) error {
	if commandVarUnset(cfg, "gateCmd") {
		return errors.New("rendered hook payloads require vars.gateCmd: set it in .awf/config.yaml")
	}
	return nil
}

// commandVarUnset reports whether a command var carries no usable value: the
// key is absent, its value is not a string, or it is blank.
func commandVarUnset(cfg *config.Config, key string) bool {
	s, _ := cfg.Vars[key].(string)
	return strings.TrimSpace(s) == ""
}

// skillFrontmatter is the rendered skill/agent frontmatter contract Claude Code
// requires.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
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

// validateArtifact validates an artifact using its declared encoder, never a
// filename suffix. This keeps policy routing independent of path spelling.
func validateArtifact(content []byte, _ AgentDialect) error {
	return validateFrontmatter(content)
}
