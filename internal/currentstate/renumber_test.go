package currentstate_test

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
)

// bodied gives a record a distinct canonical body, which is what a slugless
// record is paired on. v2rec leaves Sections nil, so every record built from it
// shares one digest and the ambiguity guard drops them all; a fixture that means
// to exercise pairing has to say what each record contains.
func bodied(a adr.ADR, body string) adr.ADR {
	a.Sections = map[string]string{"Decision": body}
	return a
}

// renumberOurs is the record integration renames: slugless, Implemented, and
// carrying an applied batch, so its pair has to be resolved by every site rather
// than only by the record-level check. The Status history deliberately holds its
// content stamp on a mid-history Amended event with digest-less Applied events
// after it, which is the shape the effort's own record has and the shape that
// kills an implementation reading the last event's stamp.
func renumberOurs(number string) adr.ADR {
	return bodied(v2rec(number, "Implemented",
		[]adr.Operation{op(adr.OpAdd, "d/t:x"), op(adr.OpUpdate, "d/t:y")},
		v2status("Proposed"),
		v2status("Accepted"),
		adr.HistoryEvent{Kind: adr.HistoryAmended, Date: "2026-01-03", Digest: strings.Repeat("a", 64)},
		v2batch(op(adr.OpAdd, "d/t:x"), op(adr.OpUpdate, "d/t:y")),
		v2status("Implemented"),
	), renumberedBody)
}

// renumberedBody is the canonical body the digest pair forms on. A fixture that
// means to test a bound on the digest step gives another record this exact body,
// because sharing it is the only way to make the digest ambiguous.
const renumberedBody = "1. The record integration renames."

// renumberOrigin owns the claim our record revises, so a substitution has a
// Revised-by list to move as well as an Origin.
func renumberOrigin() adr.ADR {
	return bodied(v2rec("0050", "Implemented",
		[]adr.Operation{op(adr.OpAdd, "d/t:y")},
		v2status("Proposed"), v2batch(op(adr.OpAdd, "d/t:y")), v2status("Implemented"),
	), "1. An older unrelated decision.")
}

// renumberTaker is the unrelated record that took the old number meanwhile. It
// is new to the after universe and must land as an ordinary addition.
func renumberTaker() adr.ADR {
	return bodied(v2rec("0194", "Implemented",
		[]adr.Operation{op(adr.OpAdd, "d/t:z")},
		v2status("Proposed"), v2batch(op(adr.OpAdd, "d/t:z")), v2status("Implemented"),
	), "1. Whatever took the number meanwhile.")
}

func renumberBefore() currentstate.Universe {
	return uni([]adr.ADR{renumberOrigin(), renumberOurs("0194")},
		claim("d/t:y", "0050", "0194"), claim("d/t:x", "0194"))
}

func renumberAfter() currentstate.Universe {
	return uni([]adr.ADR{renumberOrigin(), renumberOurs("0202"), renumberTaker()},
		claim("d/t:y", "0050", "0202"), claim("d/t:x", "0202"), claim("d/t:z", "0194"))
}

// invariant: adr-system/adr-lifecycle:renumber-digest-paired
//
// TestRenumberPairsAcrossAnOccupiedNumber is the whole point: the old number is
// occupied on the after side, which is what makes the rename necessary, so the
// pair can only be found by content digest. All four pairing sites are engaged
// at once. The record-level check must not report a deletion or a changed
// number; the batch derivation must see the pair's existing batch as already
// applied rather than as five newly added claims; and the substitution map must
// carry 0194 to 0202 so the moved Origin and Revised-by entries are accepted.
func TestRenumberPairsAcrossAnOccupiedNumber(t *testing.T) {
	if f := currentstate.CheckPair(renumberBefore(), renumberAfter(), currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("expected the renumber to be accepted, got:\n%s", messages(f))
	}
}

// invariant: adr-system/adr-lifecycle:renumber-digest-paired
//
// TestRenumberPairsWhenTheOldNumberIsVacant is the same rename with nothing
// occupying the number it leaves. It is the only shape that exercises the
// deletion sweep over the before universe: while the old number is occupied,
// that sweep finds a record at the old key whether or not it resolves aliases,
// and reports no deletion either way.
func TestRenumberPairsWhenTheOldNumberIsVacant(t *testing.T) {
	before := uni([]adr.ADR{renumberOrigin(), renumberOurs("0194")},
		claim("d/t:y", "0050", "0194"), claim("d/t:x", "0194"))
	after := uni([]adr.ADR{renumberOrigin(), renumberOurs("0202")},
		claim("d/t:y", "0050", "0202"), claim("d/t:x", "0202"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("expected a rename into a vacant number to be accepted, got:\n%s", messages(f))
	}
}

// invariant: adr-system/adr-lifecycle:renumber-digest-paired
//
// TestRenumberRequiresByteIdenticalHistory pins the one thing a digest pair adds
// beyond the ordinary rules. The pair forms, because the canonical body is what
// it keys on and Status history sits outside the digest, so nothing else would
// notice that the rename also advanced the record's history.
func TestRenumberRequiresByteIdenticalHistory(t *testing.T) {
	moved := renumberOurs("0202")
	moved.History = append(append([]adr.HistoryEvent(nil), moved.History...), v2status("Implemented"))
	after := currentstate.Universe{
		ADRs:   []adr.ADR{renumberOrigin(), moved, renumberTaker()},
		Topics: renumberAfter().Topics,
	}
	f := currentstate.CheckPair(renumberBefore(), after, currentstate.AuthoredCommit)
	if !strings.Contains(messages(f), "violates the renumbering rule") {
		t.Fatalf("expected the renumbering rule to reject a moved history, got:\n%s", messages(f))
	}
}

// invariant: adr-system/adr-lifecycle:renumber-digest-paired
//
// TestRenumberRefusedWhenDigestAmbiguous holds the fail-closed edge. A body
// carried by more than one slugless record on a side names no single record, so
// no pair forms and the rename is refused rather than guessed. What refusal
// looks like here is worth stating: the old number is still occupied, so the
// resolution falls back to the number and our record is read against the
// stranger that took it, which is exactly the mispair digest-first exists to
// avoid. Its signature is the renamed record re-adding a claim that already
// exists, which can only be reported when it has no before side of its own.
// The twin is placed last on the before side and first on the after side on
// purpose. Without the guard, each side would simply keep whichever record it
// saw first, and the two sides would name different records: the rename would
// then be paired across to the twin, and the twin itself would read as deleted.
// Ordering the twin identically on both sides hides that, because the sides
// agree by accident.
func TestRenumberRefusedWhenDigestAmbiguous(t *testing.T) {
	twin := bodied(v2rec("0060", "Implemented", nil, v2status("Proposed")), renumberedBody)
	before := currentstate.Universe{
		ADRs:   append(append([]adr.ADR(nil), renumberBefore().ADRs...), twin),
		Topics: renumberBefore().Topics,
	}
	after := currentstate.Universe{
		ADRs:   append([]adr.ADR{twin}, renumberAfter().ADRs...),
		Topics: renumberAfter().Topics,
	}
	f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit)
	if !strings.Contains(messages(f), "ADR-0202 adds claim d/t:x, which already existed") {
		t.Fatalf("expected an ambiguous digest to withhold the pair, got:\n%s", messages(f))
	}
	if strings.Contains(messages(f), "0060") {
		t.Fatalf("an ambiguous digest must not drag an unrelated record into a pair, got:\n%s", messages(f))
	}
}

// invariant: adr-system/adr-lifecycle:renumber-digest-paired
//
// TestAmendedRecordStillPairsOnItsNumber is the ordinary commit, and the case
// digest-first must not disturb. Amending a record moves its digest, so neither
// side finds a partner and both fall back to the number. The pair is then an
// ordinary one, and the ordinary rules judge it: here the amendment carries no
// operation, so it is reported as an unmatched change rather than as a deletion
// plus an addition.
func TestAmendedRecordStillPairsOnItsNumber(t *testing.T) {
	amendable := bodied(v2rec("0194", "Accepted",
		[]adr.Operation{op(adr.OpAdd, "d/t:x"), op(adr.OpUpdate, "d/t:y")},
		v2status("Proposed"),
		adr.HistoryEvent{Kind: adr.HistoryStatus, Date: "2026-01-02", Status: "Accepted", Digest: "old-digest"},
	), "1. Before the amendment.")
	amended := bodied(amendable, "1. After the amendment, at the same number.")
	amended.History = append(append([]adr.HistoryEvent(nil), amendable.History...),
		adr.HistoryEvent{Kind: adr.HistoryAmended, Date: "2026-01-03", Digest: "new-digest"})

	before := uni([]adr.ADR{renumberOrigin(), amendable}, claim("d/t:y", "0050"))
	after := uni([]adr.ADR{renumberOrigin(), amended}, claim("d/t:y", "0050"))
	f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit)
	if strings.Contains(messages(f), "was deleted across this transition") {
		t.Fatalf("an amended record must still pair on its number, got:\n%s", messages(f))
	}
	if len(f) != 0 {
		t.Fatalf("expected an amendment alone to be accepted, got:\n%s", messages(f))
	}
}

// invariant: adr-system/adr-lifecycle:renumber-digest-paired
//
// TestUnchangedSluglessRecordsPairOnTheirNumbers pins that the digest step
// re-keys nothing when no number moves. Every record here matches itself by
// digest at the key it already holds, so the resolution must leave all of them
// on their numbers and the transition must read exactly as it did before
// digest-first existed.
func TestUnchangedSluglessRecordsPairOnTheirNumbers(t *testing.T) {
	advancing := bodied(v2rec("0070", "Accepted",
		[]adr.Operation{op(adr.OpAdd, "d/t:w"), op(adr.OpUpdate, "d/t:y")},
		v2status("Proposed"), v2status("Accepted"),
	), "1. A record advancing its status.")
	implementing := advancing
	implementing.Status = "Implementing"
	implementing.History = append(append([]adr.HistoryEvent(nil), advancing.History...),
		v2status("Implementing"), v2batch(op(adr.OpAdd, "d/t:w")))

	before := uni([]adr.ADR{renumberOrigin(), advancing}, claim("d/t:y", "0050"))
	after := uni([]adr.ADR{renumberOrigin(), implementing},
		claim("d/t:y", "0050"), claim("d/t:w", "0070"))
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("expected an ordinary status advance to be accepted, got:\n%s", messages(f))
	}
}

// invariant: adr-system/adr-lifecycle:renumber-digest-paired
//
// TestRenumberIgnoresASlugCarryingTwin bounds the digest step to records
// carrying no slug. A slug-carrying record always pairs on that slug and never
// reaches the digest step, so it must not enter the digest index either: if it
// did, a pending record that happens to share the renamed record's canonical
// body would make the digest ambiguous on both sides, the guard would withhold
// the alias, and the rename would be refused. That is the integration deadlock
// this rule exists to remove, reintroduced by an unrelated record's body.
func TestRenumberIgnoresASlugCarryingTwin(t *testing.T) {
	twin := bodied(v2rec("", "Proposed", nil, v2status("Proposed")), renumberedBody)
	twin.Slug = "an-unrelated-pending-record"
	twin.Format = adr.CurrentStateV3
	before, after := renumberBefore(), renumberAfter()
	before.ADRs = append(before.ADRs, twin)
	after.ADRs = append(after.ADRs, twin)
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("a slug-carrying twin must not make the digest ambiguous, got:\n%s", messages(f))
	}
}

// invariant: adr-system/adr-lifecycle:renumber-digest-paired
//
// TestRenumberIgnoresAnUngovernedTwin is the same bound on the other half of
// the condition. A legacy record predates the governed grammar entirely and
// declares no operations, so it takes part in no pairing; a body it happens to
// share with the renamed record must not withhold the alias either.
func TestRenumberIgnoresAnUngovernedTwin(t *testing.T) {
	twin := bodied(adr.ADR{Number: "0003", Format: adr.Legacy, Status: "Accepted"}, renumberedBody)
	before, after := renumberBefore(), renumberAfter()
	before.ADRs = append(before.ADRs, twin)
	after.ADRs = append(after.ADRs, twin)
	if f := currentstate.CheckPair(before, after, currentstate.AuthoredCommit); len(f) != 0 {
		t.Fatalf("an ungoverned twin must not make the digest ambiguous, got:\n%s", messages(f))
	}
}
