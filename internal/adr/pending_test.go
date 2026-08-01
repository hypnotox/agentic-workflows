package adr_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// buildV3Governed assembles a current-state-v3 document carrying an arbitrary
// governed status, State changes block, and Status history: the V2 body shape
// exactly, plus the mandatory slug key and the requested identity heading.
func buildV3Governed(identity, slug, status, stateChanges, history string) string {
	body := build(status, "2026-07-31", oneDecision, stateChanges, history)
	body = strings.Replace(body, "format: current-state-v1\n", "format: current-state-v3\nslug: "+slug+"\n", 1)
	return strings.Replace(body, "# ADR-0137: Test Decision", "# ADR-"+identity+": Test Decision", 1)
}

// buildV3 assembles the scaffold-shaped V3 document: Proposed, no declared
// operations, one history event.
func buildV3(identity, slug string) string {
	return buildV3Governed(identity, slug, "Proposed", "None.", "- 2026-07-31: Proposed")
}

func pendingFixture(slug string) string {
	return buildV3(slug, slug)
}

// v3Changes is the declared operation block the governed V3 fixtures share.
// Three operations leave room for an Implementing history that has applied some
// and still owes others, which is what that status requires.
const v3Changes = "- add `a/b:first`\n- update `a/b:second`\n- remove `a/b:third`"

// v3DigestFor returns the content digest a governed V3 fixture's history stamps
// must repeat. The State changes block is digest-covered, so varying it is how
// these fixtures model an amended body.
func v3DigestFor(t *testing.T, stateChanges string) string {
	t.Helper()
	a, err := adr.ParseV3("scaffold-record.md", []byte(buildV3Governed("scaffold-record", "scaffold-record", "Proposed", stateChanges, "- 2026-07-31: Proposed")))
	if err != nil {
		t.Fatalf("V3 scaffold parse for digest: %v", err)
	}
	return adr.ContentDigest(a.Sections)
}

// parseV3Governed parses a pending V3 record at the given status, declared
// operations, and history.
func parseV3Governed(t *testing.T, status, stateChanges, history string) adr.ADR {
	t.Helper()
	a, err := adr.ParseV3("governed-record.md", []byte(buildV3Governed("governed-record", "governed-record", status, stateChanges, history)))
	if err != nil {
		t.Fatalf("parse V3 at %s: %v", status, err)
	}
	return a
}

// invariant: adr-system/adr-lifecycle:pending-adr-slug-identity
func TestParseV3AcceptsBothIdentityForms(t *testing.T) {
	pending, err := adr.ParseV3("keep-the-corpus.md", []byte(pendingFixture("keep-the-corpus")))
	if err != nil {
		t.Fatalf("pending form: %v", err)
	}
	if pending.Number != "" || pending.Slug != "keep-the-corpus" || !pending.IsPending() {
		t.Fatalf("pending identity = %q/%q pending=%v", pending.Number, pending.Slug, pending.IsPending())
	}
	if pending.Identity() != "keep-the-corpus" || adr.IdentityOrder(pending.Identity()) <= 9999 {
		t.Fatalf("pending identity order = %d", adr.IdentityOrder(pending.Identity()))
	}
	numbered, err := adr.ParseV3("0200-keep-the-corpus.md", []byte(buildV3("0200", "keep-the-corpus")))
	if err != nil {
		t.Fatalf("numbered form: %v", err)
	}
	if numbered.Number != "0200" || numbered.Slug != "keep-the-corpus" || numbered.IsPending() {
		t.Fatalf("numbered identity = %q/%q", numbered.Number, numbered.Slug)
	}
	// The retained slug survives numbering, and identity keys off the number.
	if numbered.Identity() != "0200" || adr.IdentityOrder(numbered.Identity()) != 200 {
		t.Fatalf("numbered identity = %q order %d", numbered.Identity(), adr.IdentityOrder(numbered.Identity()))
	}
	if !numbered.HasV2Semantics() || !numbered.IsV3() || !numbered.IsGoverned() || numbered.IsV2() {
		t.Fatalf("V3 format predicates = %#v", numbered.Format)
	}
}

// invariant: adr-system/adr-lifecycle:adr-slug-frontmatter-mandatory
func TestParseV3RejectsMalformedSlugIdentity(t *testing.T) {
	for _, tc := range []struct{ name, file, body, want string }{
		{"missing slug", "no-slug.md",
			strings.Replace(pendingFixture("no-slug"), "slug: no-slug\n", "", 1),
			"frontmatter slug is required"},
		{"empty slug", "empty.md",
			strings.Replace(pendingFixture("empty"), "slug: empty\n", "slug: \"\"\n", 1),
			"frontmatter slug is required"},
		{"non-canonical slug", "shouty.md",
			strings.Replace(pendingFixture("shouty"), "slug: shouty", "slug: Shouty Slug", 1),
			"is not in slug form"},
		{"unslugifiable slug", "punct.md",
			strings.Replace(pendingFixture("punct"), "slug: punct", `slug: "!!!"`, 1),
			"is not in slug form"},
		{"filename disagrees with slug", "other-name.md", pendingFixture("declared"), "does not carry the frontmatter slug"},
		{"numbered filename disagrees", "0200-other.md", buildV3("0200", "declared"), "does not carry the frontmatter slug"},
		{"pending heading carries a number", "headed.md",
			strings.Replace(pendingFixture("headed"), "# ADR-headed:", "# ADR-0200:", 1),
			"heading must be"},
		{"numbered heading carries the slug", "0200-headed.md",
			strings.Replace(buildV3("0200", "headed"), "# ADR-0200:", "# ADR-headed:", 1),
			"heading must be"},
		{"heading has no title", "titleless.md",
			strings.Replace(pendingFixture("titleless"), "# ADR-titleless: Test Decision", "# ADR-titleless: ", 1),
			"heading must be"},
		{"body is not V3", "wrong-format.md",
			strings.Replace(pendingFixture("wrong-format"), "current-state-v3", "current-state-v2", 1),
			`frontmatter format must be "current-state-v3"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := adr.ParseV3(tc.file, []byte(tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
	// V1 and V2 stay slug-rejecting: the narrower closed frontmatter has no
	// slug field, so the key is an unknown-field rejection there.
	v2 := strings.Replace(buildV3("0200", "x"), "current-state-v3", "current-state-v2", 1)
	if _, err := adr.ParseV2("0200-x.md", []byte(v2)); err == nil || !strings.Contains(err.Error(), "slug") {
		t.Fatalf("V2 with a slug key = %v", err)
	}
}

// invariant: adr-system/adr-lifecycle:adr-status-enum-and-matrix
func TestParseRecordRoutesPendingAndV3Cutoff(t *testing.T) {
	boundaries := adr.FormatBoundaries{V1From: 100, V2From: 150, V3From: 200}
	v3, err := adr.ParseRecord("0200-cutoff.md", []byte(buildV3("0200", "cutoff")), boundaries)
	if err != nil || !v3.IsV3() {
		t.Fatalf("numbered V3 = %#v err=%v", v3.Format, err)
	}
	v2, err := adr.ParseRecord("0199-below.md", []byte(strings.Replace(buildV3("0199", "below"), "format: current-state-v3\nslug: below\n", "format: current-state-v2\n", 1)), boundaries)
	if err != nil || !v2.IsV2() {
		t.Fatalf("record below the V3 cutoff = %#v err=%v", v2.Format, err)
	}
	// A numberless file routes by its declared marker, not by any cutoff.
	pending, err := adr.ParseRecord("cutoff-free.md", []byte(pendingFixture("cutoff-free")), adr.FormatBoundaries{})
	if err != nil || !pending.IsV3() || pending.Number != "" {
		t.Fatalf("pending routing = %#v err=%v", pending, err)
	}
	for _, tc := range []struct{ name, body string }{
		{"no frontmatter", "# Just notes\n"},
		{"unparseable frontmatter", "---\nstatus: [bad\n---\n# Notes\n"},
		{"non-governed frontmatter", "---\nstatus: Accepted\n---\n# Notes\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := adr.ParseRecord("notes.md", []byte(tc.body), boundaries); err == nil || !strings.Contains(err.Error(), "not an ADR record") {
				t.Fatalf("err = %v, want the not-an-ADR-record refusal", err)
			}
		})
	}
}

// invariant: adr-system/adr-lifecycle:corpus-single-identity-key
func TestParseDirRefusesStraysAndCarriesPendingRecords(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-numbered.md"), "---\nstatus: Accepted\n---\n# ADR-0001: Numbered\n")
	testsupport.WriteFile(t, filepath.Join(dir, "pending-one.md"), pendingFixture("pending-one"))
	for _, reserved := range []string{"README.md", "INDEX.md", "template.md"} {
		testsupport.WriteFile(t, filepath.Join(dir, reserved), "# reserved\n")
	}
	records, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %#v", records)
	}
	corpus, err := adr.NewCorpus(records)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := corpus.BySlug("absent"); ok {
		t.Fatal("absent slug resolved")
	}
	if got, ok := corpus.BySlug("pending-one"); !ok || got.Filename != "pending-one.md" {
		t.Fatalf("BySlug = %#v ok=%v", got, ok)
	}
	if got, ok := corpus.ByIdentity("pending-one"); !ok || got.Slug != "pending-one" {
		t.Fatalf("ByIdentity(slug) = %#v ok=%v", got, ok)
	}
	if got, ok := corpus.ByIdentity("0001"); !ok || got.Number != "0001" {
		t.Fatalf("ByIdentity(number) = %#v ok=%v", got, ok)
	}
	// The pending record holds no number, so it neither raises the next number
	// nor the next identity.
	if next, err := adr.NextNumber(dir); err != nil || next != "0002" {
		t.Fatalf("NextNumber = %q err=%v", next, err)
	}
	if next, err := corpus.NextIdentity(); err != nil || next != 2 {
		t.Fatalf("NextIdentity = %d err=%v", next, err)
	}
	// A pending record is outside the brownfield identity set the boundary seals.
	cutoff, gaps, err := adr.AdoptionBoundary(dir)
	if err != nil || cutoff != 2 || len(gaps) != 0 {
		t.Fatalf("AdoptionBoundary = %d %v %v", cutoff, gaps, err)
	}
	// A corpus of nothing but pending records leaves that identity set empty,
	// which yields the same boundary an empty corpus does rather than indexing
	// past the end of it.
	onlyPending := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(onlyPending, "pending-two.md"), pendingFixture("pending-two"))
	cutoff, gaps, err = adr.AdoptionBoundary(onlyPending)
	if err != nil || cutoff != 1 || len(gaps) != 0 {
		t.Fatalf("pending-only AdoptionBoundary = %d %v %v", cutoff, gaps, err)
	}

	stray := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(stray, "notes.md"), "# Just notes\n")
	if _, err := adr.ParseDir(stray); err == nil || !strings.Contains(err.Error(), "notes.md: not an ADR record") {
		t.Fatalf("stray file = %v", err)
	}
}

// A V3 record carries V2 semantics, so the governed content rules must bind it
// exactly as they bind a V2 record. The fixture is a parsed V3 document with a
// real governed history on purpose: a V3 record that never leaves Proposed
// cannot tell these predicates apart from the V1 fallbacks beside them, which
// is what makes this claim's backing real rather than nominal.
// invariant: adr-system/adr-lifecycle:adr-amendable-until-terminal
func TestV3ContentAmendsUntilTerminalStatus(t *testing.T) {
	digest := v3DigestFor(t, v3Changes)
	proposed := "- 2026-07-31: Proposed"
	accepted := proposed + "\n- 2026-07-31: Accepted; content-sha256: " + digest
	implementing := accepted + "\n- 2026-07-31: Implementing; content-sha256: " + digest +
		"\n- 2026-07-31: Applied; operations: add `a/b:first`"
	allApplied := implementing + "\n- 2026-07-31: Applied; operations: update `a/b:second`, remove `a/b:third`"
	for _, tc := range []struct {
		name, status, history string
		amendable             bool
	}{
		{"proposed", "Proposed", proposed, true},
		{"accepted", "Accepted", accepted, true},
		{"implementing", "Implementing", implementing, true},
		{"implemented", "Implemented", allApplied + "\n- 2026-07-31: Implemented; content-sha256: " + digest, false},
		{"abandoned", "Abandoned", accepted + "\n- 2026-07-31: Abandoned; content-sha256: " + digest + "; rationale: stopped; safely", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := parseV3Governed(t, tc.status, v3Changes, tc.history)
			if record.IsContentAmendable() != tc.amendable {
				t.Fatalf("IsContentAmendable = %v, want %v", record.IsContentAmendable(), tc.amendable)
			}
			// FrozenContentEqual reads the same freeze point: past it a changed
			// body is refused, before it the amendment is allowed.
			changed := record
			changed.Sections = map[string]string{"Decision": "1. Something else entirely"}
			if adr.FrozenContentEqual(record, changed) != tc.amendable {
				t.Fatalf("FrozenContentEqual = %v, want %v", adr.FrozenContentEqual(record, changed), tc.amendable)
			}
		})
	}
}

// invariant: adr-system/adr-lifecycle:applied-history-events-append-only
func TestV3HistoryIsPrefixAppendOnly(t *testing.T) {
	digest := v3DigestFor(t, v3Changes)
	implementing := "- 2026-07-31: Proposed\n- 2026-07-31: Accepted; content-sha256: " + digest +
		"\n- 2026-07-31: Implementing; content-sha256: " + digest
	first := "\n- 2026-07-31: Applied; operations: add `a/b:first`"
	second := "\n- 2026-07-31: Applied; operations: update `a/b:second`"
	before := parseV3Governed(t, "Implementing", v3Changes, implementing+first)
	if !adr.HistoryTransitionValid(before, parseV3Governed(t, "Implementing", v3Changes, implementing+first+second)) {
		t.Fatal("appending one Applied batch while Implementing must be legal")
	}
	// Mutating a retained event and deleting one are both refused: the before
	// history must survive as an exact prefix.
	if adr.HistoryTransitionValid(before, parseV3Governed(t, "Implementing", v3Changes, implementing+second)) {
		t.Fatal("rewriting the retained Applied event must be refused")
	}
	truncated := before
	truncated.History = before.History[:len(before.History)-1]
	if adr.HistoryTransitionValid(before, truncated) {
		t.Fatal("deleting the retained Applied event must be refused")
	}
	// An Amended event is the one other same-status append the format allows.
	// The amendment changes the body, so the after record declares a wider State
	// changes block and stamps that body's digest, while the retained Accepted
	// event keeps the original stamp.
	acceptedOnly := "- 2026-07-31: Proposed\n- 2026-07-31: Accepted; content-sha256: " + digest
	wide := v3Changes + "\n- add `a/b:fourth`"
	amendable := parseV3Governed(t, "Accepted", v3Changes, acceptedOnly)
	amended := parseV3Governed(t, "Accepted", wide, acceptedOnly+"\n- 2026-07-31: Amended; content-sha256: "+v3DigestFor(t, wide))
	if !adr.HistoryTransitionValid(amendable, amended) {
		t.Fatal("appending one Amended event while Accepted must be legal")
	}
}

// invariant: adr-system/adr-lifecycle:adr-status-enum-and-matrix
func TestV3FollowsTheGovernedStatusMatrix(t *testing.T) {
	// Implementing exists only in the governed matrix, so routing a V3 record to
	// the V1 matrix would refuse both of these edges.
	if !adr.TransitionLegal("Accepted", "Implementing", adr.CurrentStateV3) {
		t.Fatal("Accepted -> Implementing must be legal for V3")
	}
	if !adr.TransitionLegal("Implementing", "Implemented", adr.CurrentStateV3) {
		t.Fatal("Implementing -> Implemented must be legal for V3")
	}
	if adr.TransitionLegal("Implemented", "Accepted", adr.CurrentStateV3) {
		t.Fatal("no edge leaves a terminal status")
	}
	// The governed history grammar parses too: a V3 Applied event resolves its
	// operations against the declared State changes block, which the V1 history
	// parser does not do.
	digest := v3DigestFor(t, v3Changes)
	record := parseV3Governed(t, "Implementing", v3Changes, "- 2026-07-31: Proposed\n- 2026-07-31: Accepted; content-sha256: "+digest+
		"\n- 2026-07-31: Implementing; content-sha256: "+digest+"\n- 2026-07-31: Applied; operations: add `a/b:first`")
	applied := record.History[len(record.History)-1]
	if applied.Kind != adr.HistoryApplied || len(applied.Operations) != 1 || applied.Operations[0].ID != "a/b:first" {
		t.Fatalf("Applied event = %#v", applied)
	}
}

// invariant: adr-system/adr-lifecycle:corpus-single-identity-key
func TestNewCorpusReportsDuplicateIdentitiesAndStillPopulates(t *testing.T) {
	records := []adr.ADR{
		{Number: "0001", Filename: "0001-a.md"},
		{Number: "0001", Filename: "0001-b.md"},
		{Number: "0001", Filename: "0001-c.md"},
		{Slug: "shared", Filename: "shared.md"},
		{Number: "0002", Slug: "shared", Filename: "0002-shared.md"},
	}
	corpus, err := adr.NewCorpus(records)
	var duplicate *adr.DuplicateIdentityError
	if !errors.As(err, &duplicate) {
		t.Fatalf("err = %v, want *adr.DuplicateIdentityError", err)
	}
	if len(duplicate.Numbers) != 1 || duplicate.Numbers[0] != "0001" || len(duplicate.Slugs) != 1 || duplicate.Slugs[0] != "shared" {
		t.Fatalf("duplicate = %#v", duplicate)
	}
	if msg := duplicate.Error(); !strings.Contains(msg, `ADR number 0001 is declared by more than one file`) ||
		!strings.Contains(msg, `ADR slug "shared" is declared by more than one file`) {
		t.Fatalf("message = %q", msg)
	}
	// The corpus is still populated, last-wins, for the numbering command's
	// refusal path (ADR-0202 item 12).
	if got, ok := corpus.ByNumber("0001"); !ok || got.Filename != "0001-c.md" {
		t.Fatalf("last-wins number = %#v ok=%v", got, ok)
	}
	if got, ok := corpus.BySlug("shared"); !ok || got.Filename != "0002-shared.md" {
		t.Fatalf("last-wins slug = %#v ok=%v", got, ok)
	}
}

// invariant: adr-system/adr-lifecycle:pending-adr-slug-identity
func TestFileIdentityClassifiesDecisionBasenames(t *testing.T) {
	for _, tc := range []struct{ base, want string }{
		{"0194-slug-identified.md", "0194"},
		{"slug-identified.md", "slug-identified"},
		{"README.md", ""},
		{"INDEX.md", ""},
		{"template.md", ""},
		{"diagram.png", ""},
	} {
		if got := adr.FileIdentity(tc.base); got != tc.want {
			t.Fatalf("FileIdentity(%q) = %q, want %q", tc.base, got, tc.want)
		}
	}
	if !adr.IsReservedBasename("README.md") || adr.IsReservedBasename("0001-x.md") {
		t.Fatal("reserved basename set")
	}
}

// invariant: adr-system/adr-lifecycle:pending-adr-slug-identity
func TestRenderIndexMDSortsNumberedBeforePending(t *testing.T) {
	corpus, err := adr.NewCorpus([]adr.ADR{
		{Slug: "zebra", Title: "ADR-zebra: Zebra", Filename: "zebra.md", Status: "Proposed"},
		{Number: "0002", Title: "ADR-0002: Two", Filename: "0002-two.md", Status: "Proposed"},
		{Slug: "apple", Title: "ADR-apple: Apple", Filename: "apple.md", Status: "Proposed"},
		{Number: "0001", Title: "ADR-0001: One", Filename: "0001-one.md", Status: "Proposed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := adr.RenderIndexMD(corpus)
	want := "## In flight\n\n" +
		"- [ADR-0001: One](0001-one.md) (Proposed)\n" +
		"- [ADR-0002: Two](0002-two.md) (Proposed)\n" +
		"- [ADR-apple: Apple](apple.md) (Proposed)\n" +
		"- [ADR-zebra: Zebra](zebra.md) (Proposed)\n"
	if !strings.HasPrefix(got, want) {
		t.Fatalf("index =\n%s\nwant prefix\n%s", got, want)
	}
}

// The numbered identity form is exactly four digits. That width is the boundary
// IsSlugIdentity, IdentityOrder, and Corpus.ByIdentity all key on, so a digit
// string of any other width has to fall on the slug side and rank with the
// pending records. Nothing sanctioned mints one - the scaffold refuses an
// all-digit title slug - but a hand-authored record can carry it, and widening
// the form would silently move it between identity classes. The rank is compared
// against an ordinary slug rather than a literal so the case pins the grouping,
// not the constant.
// invariant: adr-system/adr-lifecycle:pending-adr-slug-identity
func TestNumberedIdentityFormIsExactlyFourDigits(t *testing.T) {
	slugRank := adr.IdentityOrder("an-ordinary-slug")
	for _, ref := range []string{"1", "123", "12345", "000000"} {
		if !adr.IsSlugIdentity(ref) {
			t.Errorf("%q is not the four-digit numbered form and must read as a slug identity", ref)
		}
		if got := adr.IdentityOrder(ref); got != slugRank {
			t.Errorf("%q ranks %d, but every slug identity ranks %d", ref, got, slugRank)
		}
	}
	for _, ref := range []string{"0001", "0200", "9999"} {
		if adr.IsSlugIdentity(ref) {
			t.Errorf("%q is the numbered identity form", ref)
		}
		if adr.IdentityOrder(ref) >= slugRank {
			t.Errorf("%q must rank below every pending identity", ref)
		}
	}
}
