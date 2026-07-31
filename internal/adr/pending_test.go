package adr_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// buildV3 assembles a current-state-v3 document: the V2 body exactly, plus the
// mandatory slug key and the requested identity heading.
func buildV3(identity, slug string) string {
	body := build("Proposed", "2026-07-31", oneDecision, "None.", "- 2026-07-31: Proposed")
	body = strings.Replace(body, "format: current-state-v1\n", "format: current-state-v3\nslug: "+slug+"\n", 1)
	return strings.Replace(body, "# ADR-0137: Test Decision", "# ADR-"+identity+": Test Decision", 1)
}

func pendingFixture(slug string) string {
	return buildV3(slug, slug)
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

	stray := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(stray, "notes.md"), "# Just notes\n")
	if _, err := adr.ParseDir(stray); err == nil || !strings.Contains(err.Error(), "notes.md: not an ADR record") {
		t.Fatalf("stray file = %v", err)
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
	// refusal path (ADR-0194 item 12).
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
