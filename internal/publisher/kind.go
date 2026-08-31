package publisher

import (
	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// kindDescriptor is Publisher's compatibility projection of one canonical kind.
type kindDescriptor struct {
	Plural         string
	Singular       string
	freeformDomain bool
	poolNames      func(*catalog.Catalog) []string
	sections       func(*catalog.Catalog, string) ([]string, bool)
	templateID     func(*catalog.Catalog, string) string
}

func publisherKindDescriptor(kind artifactregistry.Kind) kindDescriptor {
	descriptor := kindDescriptor{
		Plural: kind.Plural, Singular: kind.Singular,
		freeformDomain: kind.Cardinality == artifactregistry.CardinalityFreeform,
		sections: func(cat *catalog.Catalog, name string) ([]string, bool) {
			return artifactregistry.Sections(cat, kind.Plural, name)
		},
		templateID: func(cat *catalog.Catalog, name string) string {
			return artifactregistry.TemplateID(cat, kind.Plural, name)
		},
	}
	if kind.Cardinality != artifactregistry.CardinalityFreeform {
		descriptor.poolNames = func(cat *catalog.Catalog) []string {
			names, _ := artifactregistry.Names(cat, kind.Plural)
			return names
		}
	}
	return descriptor
}

func allKindDescriptors() []kindDescriptor {
	kinds := artifactregistry.Kinds()
	out := make([]kindDescriptor, len(kinds))
	for i, kind := range kinds {
		out[i] = publisherKindDescriptor(kind)
	}
	return out
}

// kindDescriptors is a compatibility projection of the canonical registry.
var kindDescriptors = allKindDescriptors()

func descriptorByPlural(kind string) (kindDescriptor, bool) {
	declaration, ok := artifactregistry.KindByPlural(kind)
	if !ok {
		return kindDescriptor{}, false
	}
	return publisherKindDescriptor(declaration), true
}

func mustDescriptor(kind string) kindDescriptor {
	descriptor, _ := descriptorByPlural(kind)
	return descriptor
}
