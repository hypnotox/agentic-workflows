package bridge

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

type InvariantKey string

type Retirement struct {
	Carrier, CarrierPath string
	DecisionItem         int
}

type LegacyInvariant struct {
	Key                  InvariantKey `json:"-"`
	Declarer, Slug       string
	DeclarerPath         string
	Backing              string
	Carrier, CarrierPath string
	CarrierDecisionItem  int
	Active               bool
	History              *MigrationHistoryEntry
}

type Inventory struct{ Entries []LegacyInvariant }

func BuildInventory(corpus adr.Corpus) (Inventory, error) {
	effective := map[string]Retirement{}
	for _, a := range corpus.All() {
		if !a.IsLegacyShipped() {
			continue
		}
		for _, ref := range corpus.RefsOf(a.Number) {
			if ref.Relation == adr.Retires && ref.Slug != "" {
				key := "ADR-" + ref.Target + "#" + ref.Slug
				effective[key] = Retirement{Carrier: a.Number, CarrierPath: a.Path, DecisionItem: ref.CarrierItem}
			}
		}
	}
	var out Inventory
	declared := map[string]bool{}
	for _, a := range corpus.All() {
		if !a.IsLegacyShipped() {
			continue
		}
		for _, decl := range a.InvariantDecls() {
			key := "ADR-" + a.Number + "#" + decl.Slug
			if declared[key] {
				return Inventory{}, fmt.Errorf("duplicate legacy invariant anchor %s", key)
			}
			declared[key] = true
			backing := "test"
			if decl.Unbacked {
				backing = "unbacked"
			}
			r, retired := effective[key]
			out.Entries = append(out.Entries, LegacyInvariant{Key: InvariantKey(key), Declarer: a.Number, DeclarerPath: a.Path, Slug: decl.Slug, Backing: backing, Carrier: r.Carrier, CarrierPath: r.CarrierPath, CarrierDecisionItem: r.DecisionItem, Active: !retired})
		}
	}
	historyByKey := map[string]MigrationHistoryEntry{}
	for _, a := range corpus.All() {
		raw, err := rawADR(corpus, a.Number)
		if err != nil { // coverage-ignore: the corpus was loaded from this same file moments earlier; failure requires a concurrent filesystem race
			return Inventory{}, err
		}
		entries, err := ParseMigrationHistory(raw, a.Number, effective)
		if err != nil {
			return Inventory{}, err
		}
		for _, entry := range entries {
			if entry.Date != a.Date {
				return Inventory{}, fmt.Errorf("migration history for %s uses %s; carrier ADR-%s date is %s", entry.Key, entry.Date, a.Number, a.Date)
			}
			if _, ok := declared[entry.Key]; !ok {
				return Inventory{}, fmt.Errorf("migration history names undeclared invariant %s", entry.Key)
			}
			if prior, duplicate := historyByKey[entry.Key]; duplicate {
				return Inventory{}, fmt.Errorf("duplicate migration history for %s in ADR-%s and ADR-%s", entry.Key, prior.ADR, entry.ADR)
			}
			historyByKey[entry.Key] = entry
		}
	}
	for n := range out.Entries {
		key := string(out.Entries[n].Key)
		if history, ok := historyByKey[key]; ok {
			copy := history
			out.Entries[n].History = &copy
			if history.Basis == "migration" {
				out.Entries[n].Active = false
			}
		}
	}
	slices.SortFunc(out.Entries, func(a, b LegacyInvariant) int { return strings.Compare(string(a.Key), string(b.Key)) })
	return out, nil
}
