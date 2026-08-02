package migrate

import (
	"bytes"
	"testing"
)

func TestDecisionItemSlugsMigrationPreservesBytes(t *testing.T) {
	if err := applyDecisionItemSlugs(testContext(t), t.TempDir(), bytes.NewBuffer(nil)); err != nil {
		t.Fatal(err)
	}
	if Current() != 33 {
		t.Fatalf("Current() = %d, want 33", Current())
	}
}
