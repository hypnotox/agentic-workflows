package project

import (
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
)

// validateAgainstCatalog checks that every enabled non-local target is in the
// catalog and that its sidecar's section overrides name declared sections.
func validateAgainstCatalog(p renderInputs) error {
	for _, d := range allKindDescriptors() {
		if d.poolNames == nil { // domains: freeform, not catalog-validated
			continue
		}
		if err := checkKindAgainstCatalog(p, d); err != nil {
			return err
		}
	}
	// Domain sidecars are paths-only (ADR-0086 Decision 5): domain rendering
	// passes an empty sidecar and injects its own data map, so authored data:
	// or sections: silently does nothing - and the domain template's own
	// .data.domain reference would mask a data: block
	// from the consumption check.
	for _, name := range p.cfg.Domains {
		sc, err := p.cfg.Sidecar("domains", name)
		if err != nil {
			return err
		}
		if len(sc.Data) > 0 || len(sc.DataDefaults) > 0 || len(sc.Sections) > 0 {
			return fmt.Errorf("domain %q: a domain sidecar is paths-only; nothing reads data:, dataDefaults:, or sections: on it; remove them from .awf/domains/%s.yaml", name, name)
		}
	}
	// agents-doc section overrides against catalog (always-on singleton).
	ad, err := p.cfg.Sidecar("agents-doc", "")
	if err != nil {
		return err
	}
	if len(ad.Paths) > 0 {
		return errors.New("agents-doc: paths: is read only from domain sidecars; remove it from .awf/agents-doc.yaml")
	}
	if err := validateCatalogListData("agents-doc.yaml", ad, projectCatalog(p).Docs["agents-doc"].Data); err != nil {
		return err
	}
	if err := checkSectionsAllowed("agents-doc", "", projectCatalog(p).Docs["agents-doc"].Sections, ad.Sections); err != nil {
		return err
	}
	for _, sg := range plainSingletons(projectCatalog(p)) {
		sc, err := p.cfg.Sidecar(sg.kind, "")
		if err != nil {
			return err
		}
		if len(sc.Paths) > 0 {
			return fmt.Errorf("%s: paths: is read only from domain sidecars; remove it from .awf/%s.yaml", sg.kind, sg.kind)
		}
		if err := validateCatalogListData(sg.kind+".yaml", sc, projectCatalog(p).Docs[sg.kind].Data); err != nil {
			return err
		}
		if err := checkSectionsAllowed(sg.kind, "", sg.sections(projectCatalog(p)), sc.Sections); err != nil {
			return err
		}
	}
	// The config reference's data namespace is injected at generation
	// (ADR-0088): authored data: would be silently overwritten while its key
	// names look consumed, so it is rejected like the domain paths-only rule.
	// The Generated entry left plainSingletons, so its sidecar checks live here.
	cr, err := p.cfg.Sidecar("config-reference", "")
	if err != nil {
		return err
	}
	if len(cr.Data) > 0 || len(cr.DataDefaults) > 0 {
		return errors.New("config-reference: the reference tables are generated; data: and dataDefaults: have no effect; remove them from .awf/config-reference.yaml (sections: remains available)")
	}
	if len(cr.Paths) > 0 {
		return errors.New("config-reference: paths: is read only from domain sidecars; remove it from .awf/config-reference.yaml")
	}
	if err := checkSectionsAllowed("config-reference", "", projectCatalog(p).Docs["config-reference"].Sections, cr.Sections); err != nil {
		return err
	}
	for _, local := range p.cfg.NormalizedLocalDocs() {
		output := config.DocsDir + "/" + local.Name + ".md"
		for name := range projectCatalog(p).Docs {
			if output == docOutPath(p, name) {
				return fmt.Errorf("local document %q collides with standard output %q", local.Name, output)
			}
		}
	}
	return nil
}

// checkKindAgainstCatalog validates every catalog artifact's shaping sidecar.
func checkKindAgainstCatalog(p renderInputs, d kindDescriptor) error {
	for _, name := range d.poolNames(projectCatalog(p)) {
		sc, err := p.cfg.Sidecar(d.Plural, name)
		if err != nil {
			return err
		}
		if len(sc.Paths) > 0 {
			return fmt.Errorf("%s %q: paths: is read only from domain sidecars; remove it from .awf/%s/%s.yaml", d.Singular, name, d.Plural, name)
		}
		if err := validateCatalogListData(d.Plural+"/"+name+".yaml", sc, catalogData(projectCatalog(p), d.Plural, name), glossary.SpecializedListDataKeys(d.Plural, name)...); err != nil {
			return err
		}
		if declared, ok := d.sections(projectCatalog(p), name); ok {
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
