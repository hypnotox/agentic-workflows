package project

import (
	"strings"
	"testing"
)

// invariant: config/migrations-and-locks:schema-min-version
func TestSchemaMinimumVersionAuthority(t *testing.T) {
	for schema, minimum := range minVersionBySchema {
		if err := ValidateSchemaMinimumVersion(schema, minimum); err != nil {
			t.Errorf("schema %d at minimum %s: %v", schema, minimum, err)
		}
	}
	if got := minVersionBySchema[20]; got != "0.24.0" {
		t.Fatalf("generation-20 minimum version = %q, want 0.24.0", got)
	}
	if err := ValidateSchemaMinimumVersion(20, "0.23.0"); err == nil || !strings.Contains(err.Error(), "requires awf 0.24.0") {
		t.Fatalf("generation-20 older binary error = %v", err)
	}
	if err := ValidateSchemaMinimumVersion(21, Version); err == nil || !strings.Contains(err.Error(), "no minimum") {
		t.Fatalf("unmapped schema error = %v", err)
	}
}
