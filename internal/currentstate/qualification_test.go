package currentstate_test

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
)

func TestQualifyIncomingExactAndEvilMerge(t *testing.T) {
	v2 := adr.ADR{Number: "0200", Format: adr.CurrentStateV2, Status: "Implemented"}
	body := []byte("---\nformat: current-state-v2\nstatus: Implemented\ndate: 2026-01-01\n---\n# ADR-0200: Old\n")
	result := currentstate.Universe{ADRs: []adr.ADR{v2}, Sources: map[string][]byte{"0200": body}}
	incoming := currentstate.Universe{ADRs: []adr.ADR{v2}, Sources: map[string][]byte{"0200": append([]byte(nil), body...)}}
	got := currentstate.QualifyIncoming(currentstate.Universe{}, result, []currentstate.Universe{incoming}, adr.CurrentStateV3)
	if len(got) != 1 || !got[0].Qualified || got[0].Introduction.Identity != "0200" {
		t.Fatalf("exact qualification = %#v", got)
	}
	incoming.Sources["0200"] = append(incoming.Sources["0200"], []byte("evil\n")...)
	got = currentstate.QualifyIncoming(currentstate.Universe{}, result, []currentstate.Universe{incoming}, adr.CurrentStateV3)
	if len(got) != 1 || got[0].Qualified {
		t.Fatalf("evil qualification = %#v", got)
	}
}

func TestQualifyIncomingRequiresSourcesAndRejectsSameIdentityEdits(t *testing.T) {
	record := adr.ADR{Number: "0200", Format: adr.CurrentStateV2}
	result := currentstate.Universe{ADRs: []adr.ADR{record}, Sources: map[string][]byte{"0200": []byte("result")}}
	incoming := currentstate.Universe{ADRs: []adr.ADR{record}}
	got := currentstate.QualifyIncoming(currentstate.Universe{}, result, []currentstate.Universe{incoming}, adr.CurrentStateV3)
	if len(got) != 1 || got[0].Qualified {
		t.Fatalf("missing source qualification = %#v", got)
	}
	incoming.Sources = map[string][]byte{"0200": []byte("parent")}
	got = currentstate.QualifyIncoming(currentstate.Universe{}, result, []currentstate.Universe{incoming}, adr.CurrentStateV3)
	if len(got) != 1 || got[0].Qualified {
		t.Fatalf("same-identity edit qualification = %#v", got)
	}
}

func TestQualifyIncomingAllowsOnlyDeterministicRenumberSubstitutions(t *testing.T) {
	sections := map[string]string{"Context": "same", "Decision": "same"}
	parentRecord := adr.ADR{Number: "0200", Filename: "0200-old.md", Format: adr.CurrentStateV2, Status: "Implemented", Sections: sections}
	resultRecord := adr.ADR{Number: "0201", Filename: "0201-old.md", Format: adr.CurrentStateV2, Status: "Implemented", Sections: sections}
	parentBody := []byte("# ADR-0200: Old\nOrigin: ADR-0200\nRevised-by: ADR-0200\n")
	resultBody := []byte("# ADR-0201: Old\nOrigin: ADR-0201\nRevised-by: ADR-0201\n")
	result := currentstate.Universe{ADRs: []adr.ADR{resultRecord}, Sources: map[string][]byte{"0201": resultBody}}
	incoming := currentstate.Universe{ADRs: []adr.ADR{parentRecord}, Sources: map[string][]byte{"0200": parentBody}}
	got := currentstate.QualifyIncoming(currentstate.Universe{}, result, []currentstate.Universe{incoming}, adr.CurrentStateV3)
	if len(got) != 1 || !got[0].Qualified {
		t.Fatalf("renumber qualification = %#v", got)
	}
	resultRecord.Filename = "0201-unrelated.md"
	result.ADRs = []adr.ADR{resultRecord}
	got = currentstate.QualifyIncoming(currentstate.Universe{}, result, []currentstate.Universe{incoming}, adr.CurrentStateV3)
	if len(got) != 1 || got[0].Qualified {
		t.Fatalf("unrelated filename qualification = %#v", got)
	}
	legacyParent := adr.ADR{Number: "0100", Filename: "0100-old.md", Format: adr.Legacy, Sections: sections}
	legacyResult := adr.ADR{Number: "0101", Filename: "0101-old.md", Format: adr.Legacy, Sections: sections}
	incoming = currentstate.Universe{ADRs: []adr.ADR{legacyParent}, Sources: map[string][]byte{"0100": []byte("# ADR-0100: Old\nOrigin: ADR-0100\n")}}
	result = currentstate.Universe{ADRs: []adr.ADR{legacyResult}, Sources: map[string][]byte{"0101": []byte("# ADR-0101: Old\nOrigin: ADR-0101\n")}}
	got = currentstate.QualifyIncoming(currentstate.Universe{}, result, []currentstate.Universe{incoming}, adr.CurrentStateV3)
	if len(got) != 1 || got[0].Qualified {
		t.Fatalf("legacy provenance substitution qualified = %#v", got)
	}
}
