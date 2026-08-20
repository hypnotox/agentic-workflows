package project

import (
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// ListDocument renders the fixed catalog and configured domain inventory.
func ListDocumentOperation(p *Project, kindFilter string) (presentation.Document, error) {
	if kindFilter == "target" {
		return listTargetDocument()
	}
	if kindFilter != "" && kindFilter != "domain" && kindFilter != "skill" && kindFilter != "agent" && kindFilter != "doc" {
		return presentation.Document{}, fmt.Errorf("unknown kind %q", kindFilter)
	}
	kinds := Kinds()
	if kindFilter != "" {
		kinds = []string{kindFilter}
	}
	categories := make([]presentation.CollectionCategory, 0, len(kinds))
	for _, kind := range kinds {
		plural, _ := PluralKind(kind)
		var entries []string
		if kind == "domain" {
			entries = slices.Sorted(slices.Values(p.Cfg.Domains))
			if len(entries) == 0 {
				entries = []string{"none"}
			}
		} else {
			entries, _ = CatalogNames(projectCatalog(p), kind)
			for i, name := range entries {
				if sidecar, err := p.Cfg.Sidecar(plural, name); err == nil && (sidecar.Data != nil || sidecar.Sections != nil) {
					entries[i] = name + " (tuned)"
				}
			}
		}
		category, err := listCategory(plural, entries)
		if err != nil {
			return presentation.Document{}, err
		}
		categories = append(categories, category)
	}
	return (presentation.Collection{Status: "artifact inventory", Categories: categories}).Document()
}

func listTargetDocument() (presentation.Document, error) {
	// KnownTargets is the fixed compile-time pair "claude" and "pi", both safe
	// presentation literals; listCategory cannot reject this closed set.
	category, _ := listCategory("targets", KnownTargets())
	return (presentation.Collection{Status: "artifact inventory", Categories: []presentation.CollectionCategory{category}}).Document()
}

func listCategory(label string, entries []string) (presentation.CollectionCategory, error) {
	values := make([]presentation.Value, 0, len(entries))
	for _, entry := range entries {
		value, err := presentation.Literal(entry)
		if err != nil {
			return presentation.CollectionCategory{}, err
		}
		values = append(values, value)
	}
	return presentation.CollectionCategory{Label: label, Values: values}, nil
}
