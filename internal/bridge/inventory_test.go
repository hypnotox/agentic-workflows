package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

func writeBridgeADR(t *testing.T, dir, name, status, invariants, decision, history string) {
	t.Helper()
	body := "---\nstatus: " + status + "\ndate: 2026-07-19\n---\n# ADR-" + name[:4] + ": Test\n\n## Decision\n\n" + decision + "\n\n## Invariants\n\n" + invariants + "\n" + history
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
func bridgeCorpus(t *testing.T, dir string) adr.Corpus {
	t.Helper()
	c, e := adr.LoadCorpus(dir)
	if e != nil {
		t.Fatal(e)
	}
	return c
}

func inventoryEntry(inv Inventory, key string) (LegacyInvariant, bool) {
	for _, entry := range inv.Entries {
		if string(entry.Key) == key {
			return entry, true
		}
	}
	return LegacyInvariant{}, false
}

func TestBuildInventory(t *testing.T) {
	dir := t.TempDir()
	writeBridgeADR(t, dir, "0001-one.md", "Implemented", "- `invariant: backed` - x.\n- `unbacked-invariant: manual` - y.", "1. Declare.", "")
	writeBridgeADR(t, dir, "0002-two.md", "Superseded", "- `invariant: retired` - x.", "1. Declare.", "")
	writeBridgeADR(t, dir, "0003-carrier.md", "Implemented", "", "1. retire `supersedes-invariant: ADR-0002#retired`.", "\n## Migration history\n\n- 2026-07-19: retired invariant `ADR-0002#retired`; basis: encoded\n")
	writeBridgeADR(t, dir, "0004-proposed.md", "Proposed", "- `invariant: ignored` - x.", "1. inactive `supersedes-invariant: ADR-0001#backed`.", "")
	inv, err := BuildInventory(bridgeCorpus(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Entries) != 3 {
		t.Fatalf("%#v", inv)
	}
	backed, _ := inventoryEntry(inv, "ADR-0001#backed")
	manual, _ := inventoryEntry(inv, "ADR-0001#manual")
	retired, _ := inventoryEntry(inv, "ADR-0002#retired")
	if backed.Backing != "test" || manual.Backing != "unbacked" || !backed.Active || retired.Active || retired.Carrier != "0003" || retired.CarrierDecisionItem != 1 || retired.History == nil {
		t.Fatalf("facts: %#v %#v %#v", backed, manual, retired)
	}
	if _, ok := inventoryEntry(inv, "ADR-0004#ignored"); ok {
		t.Fatal("inactive declaration entered inventory")
	}
}

func TestBuildInventoryMigrationRetirement(t *testing.T) {
	dir := t.TempDir()
	writeBridgeADR(t, dir, "0001-one.md", "Implemented", "- `invariant: old` - x.", "1. x.", "\n## Migration history\n\n- 2026-07-19: retired invariant `ADR-0001#old`; basis: migration; rationale: reviewed retirement\n")
	inv, err := BuildInventory(bridgeCorpus(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if inv.Entries[0].Active || inv.Entries[0].History.Basis != "migration" {
		t.Fatalf("%#v", inv)
	}
}

func TestBuildInventoryRefusals(t *testing.T) {
	cases := []struct{ name, first, second, want string }{
		{"duplicate", "- `invariant: x` - x.\n- `invariant: x` - x.", "", "duplicate"},
		{"unknown-history", "- `invariant: x` - x.", "\n## Migration history\n\n- 2026-07-19: retired invariant `ADR-0009#x`; basis: migration; rationale: gone\n", "undeclared"},
		{"date-mismatch", "- `invariant: x` - x.", "\n## Migration history\n\n- 2026-07-20: retired invariant `ADR-0001#x`; basis: migration; rationale: gone\n", "carrier"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeBridgeADR(t, dir, "0001-one.md", "Implemented", tc.first, "1. x.", tc.second)
			_, err := BuildInventory(bridgeCorpus(t, dir))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%v", err)
			}
		})
	}
	dir := t.TempDir()
	writeBridgeADR(t, dir, "0001-one.md", "Implemented", "- `invariant: x` - x.", "1. x.", "\n## Migration history\n\n- 2026-07-19: retired invariant `ADR-0001#x`; basis: migration; rationale: gone\n")
	writeBridgeADR(t, dir, "0002-two.md", "Implemented", "", "1. x.", "\n## Migration history\n\n- 2026-07-19: retired invariant `ADR-0001#x`; basis: migration; rationale: also gone\n")
	if _, err := BuildInventory(bridgeCorpus(t, dir)); err == nil || !strings.Contains(err.Error(), "duplicate migration") {
		t.Fatalf("%v", err)
	}
}
