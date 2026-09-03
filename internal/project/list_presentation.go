package project

import (
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// ListDocument renders the fixed catalog and configured domain inventory.
func listDocument(cfg *config.Config, cat *catalog.Catalog, kindFilter string) (presentation.Document, error) {
	if kindFilter == "target" {
		return listTargetDocument()
	}
	kinds := Kinds()
	if kindFilter != "" {
		if _, ok := PluralKind(kindFilter); !ok {
			return presentation.Document{}, fmt.Errorf("unknown kind %q", kindFilter)
		}
		kinds = []string{kindFilter}
	}
	categories := make([]presentation.CollectionCategory, 0, len(kinds))
	for _, kind := range kinds {
		plural, ok := PluralKind(kind)
		if !ok {
			return presentation.Document{}, fmt.Errorf("unknown kind %q", kind)
		}
		var entries []string
		if kind == "domain" {
			entries = slices.Sorted(slices.Values(cfg.Domains))
			if len(entries) == 0 {
				entries = []string{"none"}
			}
		} else {
			entries, _ = CatalogNames(cat, kind)
			for i, name := range entries {
				if sidecar, err := cfg.Sidecar(plural, name); err == nil && (sidecar.Data != nil || sidecar.Sections != nil) {
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
