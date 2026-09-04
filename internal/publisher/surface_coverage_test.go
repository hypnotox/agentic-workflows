package publisher

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func TestDomainDescriptorProjectsCatalogSections(t *testing.T) {
	descriptor, ok := descriptorByPlural("domains")
	if !ok {
		t.Fatal("domains descriptor is absent")
	}
	sections, present := descriptor.sections(catalog.Standard, "example")
	if present || len(sections) == 0 {
		t.Fatalf("domain sections = %v, present=%t", sections, present)
	}
}
