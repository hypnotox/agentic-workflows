package adr_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestRenderActiveMDGroupsByStatus is a hermetic unit test using a temp dir.
// invariant: render-active-md
func TestRenderActiveMDGroupsByStatus(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"0001-first-accepted.md": testsupport.ADR("Accepted",
			testsupport.WithDate("2026-06-24"), testsupport.WithTags("tooling"),
			testsupport.WithTitle("0001: First Accepted"), testsupport.WithBody("## Context\nSomething.\n")),
		"0002-a-proposal.md": testsupport.ADR("Proposed",
			testsupport.WithDate("2026-06-24"),
			testsupport.WithTitle("0002: A Proposal"), testsupport.WithBody("## Context\nAnother thing.\n")),
		"0003-already-implemented.md": testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-24"),
			testsupport.WithTitle("0003: Already Implemented"), testsupport.WithBody("## Context\nDone.\n")),
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	got := adr.RenderActiveMD(mustCorpus(t, dir))

	// RenderActiveMD is banner-free (the banner is injected downstream by
	// internal/project, like every other rendered artifact) - content starts
	// directly with the first status heading.
	if !strings.HasPrefix(got, "## ") {
		t.Errorf("expected content to start with a status heading, no embedded banner; got start: %q", got[:min(60, len(got))])
	}

	// Accepted must appear before Proposed.
	acceptedPos := strings.Index(got, "## Accepted")
	implementedPos := strings.Index(got, "## Implemented")
	proposedPos := strings.Index(got, "## Proposed")

	if acceptedPos < 0 {
		t.Error("missing ## Accepted section")
	}
	if implementedPos < 0 {
		t.Error("missing ## Implemented section")
	}
	if proposedPos < 0 {
		t.Error("missing ## Proposed section")
	}
	if acceptedPos > 0 && implementedPos > 0 && acceptedPos > implementedPos {
		t.Errorf("Accepted (%d) should come before Implemented (%d)", acceptedPos, implementedPos)
	}
	if implementedPos > 0 && proposedPos > 0 && implementedPos > proposedPos {
		t.Errorf("Implemented (%d) should come before Proposed (%d)", implementedPos, proposedPos)
	}

	// Each ADR entry should appear under the correct section.
	checkEntry := func(section, title, filename string) {
		t.Helper()
		sectionHeader := "## " + section
		sectionStart := strings.Index(got, sectionHeader)
		if sectionStart < 0 {
			t.Errorf("section %q not found", sectionHeader)
			return
		}
		// Find next section after this one.
		nextSection := strings.Index(got[sectionStart+len(sectionHeader):], "\n## ")
		var sectionBody string
		if nextSection < 0 {
			sectionBody = got[sectionStart:]
		} else {
			sectionBody = got[sectionStart : sectionStart+len(sectionHeader)+nextSection]
		}
		if !strings.Contains(sectionBody, title) {
			t.Errorf("expected %q in section %q; section body:\n%s", title, section, sectionBody)
		}
		if !strings.Contains(sectionBody, filename) {
			t.Errorf("expected filename %q in section %q", filename, section)
		}
	}

	checkEntry("Accepted", "ADR-0001: First Accepted", "0001-first-accepted.md")
	checkEntry("Implemented", "ADR-0003: Already Implemented", "0003-already-implemented.md")
	checkEntry("Proposed", "ADR-0002: A Proposal", "0002-a-proposal.md")
}

// TestRenderActiveMDGroupsSupersededVariants locks the lifecycle convention's
// suffixed status ("Superseded by ADR-NNNN") into one Superseded group: a
// single section header ordered by statusOrder, per-entry full status kept -
// not one alphabetical section per successor.
// invariant: render-active-md
func TestRenderActiveMDGroupsSupersededVariants(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0001-old-one.md": testsupport.ADR("Superseded by ADR-0003",
			testsupport.WithDate("2026-06-24"), testsupport.WithTitle("0001: Old One")),
		"0002-old-two.md": testsupport.ADR("Superseded by ADR-0004",
			testsupport.WithDate("2026-06-24"), testsupport.WithTitle("0002: Old Two")),
		"0003-current.md": testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-24"), testsupport.WithTitle("0003: Current")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	got := adr.RenderActiveMD(mustCorpus(t, dir))
	if strings.Contains(got, "## Superseded by") {
		t.Errorf("per-successor section header rendered instead of one Superseded group:\n%s", got)
	}
	if !strings.Contains(got, "## Superseded\n") {
		t.Errorf("missing single ## Superseded section:\n%s", got)
	}
	for _, entry := range []string{"(Superseded by ADR-0003)", "(Superseded by ADR-0004)"} {
		if !strings.Contains(got, entry) {
			t.Errorf("entry lost its full status %q:\n%s", entry, got)
		}
	}
	if imp, sup := strings.Index(got, "## Implemented"), strings.Index(got, "## Superseded"); imp > sup {
		t.Errorf("Superseded (%d) must sort after Implemented (%d) per statusOrder", sup, imp)
	}
}

// TestRenderActiveMDSupersedence covers the ADR-0120 item 10 rendering: a
// full chain, an item annotation, and a slug annotation each render under
// ## Supersedence, and a supersession-free corpus renders neither subsection.
// invariant: active-md-supersedence-rendering
// invariant: active-md-annotates-superseded-anchors
func TestRenderActiveMDSupersedence(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0001-old.md": testsupport.ADR("Superseded by ADR-0002",
			testsupport.WithSupersededBy("0002"), testsupport.WithTitle("0001: Old")),
		"0002-new.md": testsupport.ADR("Implemented", testsupport.WithSupersedes(1),
			testsupport.WithTitle("0002: New")),
		"0003-target.md": testsupport.ADR("Accepted", testsupport.WithRelated(4, 5),
			testsupport.WithTitle("0003: Target"),
			testsupport.WithBody("## Decision\n\n1. a.\n2. b.\n\n## Invariants\n\n- `invariant: some-slug` - x.\n")),
		"0004-citer.md": testsupport.ADR("Implemented", testsupport.WithTitle("0004: Citer"),
			testsupport.WithBody("## Decision\n\n1. Overrides `supersedes: ADR-0003#2` and `supersedes-invariant: ADR-0003#some-slug`.\n")),
		"0005-refiner.md": testsupport.ADR("Implemented", testsupport.WithTitle("0005: Refiner"),
			testsupport.WithBody("## Decision\n\n1. Adapts `refines: ADR-0003#1`.\n")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := adr.RenderActiveMD(mustCorpus(t, dir))
	for _, want := range []string{
		"## Supersedence",
		"### Chains\n\n- ADR-0001 superseded by ADR-0002\n",
		// A refinement reads "refined by" and a retirement "superseded by": the
		// reader must be able to tell an adapted decision from a replaced one
		// (ADR-0128 item 2).
		"### Superseded anchors on live ADRs\n\n- ADR-0003: item 1 refined by ADR-0005; item 2 superseded by ADR-0004; slug `some-slug` superseded by ADR-0004\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}

	// A supersession-free corpus renders neither subsection.
	plain := t.TempDir()
	if err := os.WriteFile(filepath.Join(plain, "0001-a.md"),
		[]byte(testsupport.ADR("Accepted", testsupport.WithTitle("0001: A"))), 0o644); err != nil {
		t.Fatal(err)
	}
	got = adr.RenderActiveMD(mustCorpus(t, plain))
	if strings.Contains(got, "Supersedence") || strings.Contains(got, "###") {
		t.Errorf("supersession-free corpus must not render the section:\n%s", got)
	}
}

// invariant: render-active-md
func TestRenderActiveMDPlaceholderWhenNoADRs(t *testing.T) {
	dir := t.TempDir()
	// A non-ADR markdown file must not count.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := adr.RenderActiveMD(mustCorpus(t, dir))
	if !strings.Contains(got, "No decisions recorded yet") {
		t.Errorf("expected placeholder index for an ADR-less dir, got:\n%s", got)
	}
}

func TestParseDirExtractsStatusAndTitle(t *testing.T) {
	dir := t.TempDir()
	content := testsupport.ADR("Accepted", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0007: Example Title"), testsupport.WithBody("## Context\nx\n"))
	if err := os.WriteFile(filepath.Join(dir, "0007-example.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-ADR file must be skipped.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	got := adrs[0]
	if got.Number != "0007" || got.Status != "Accepted" || got.Title != "ADR-0007: Example Title" || got.Filename != "0007-example.md" {
		t.Errorf("unexpected ADR: %+v", got)
	}
}

// TestParseDirExtractsTagsAndRelated confirms the revived tags:/related:
// frontmatter is lifted into adr.ADR (previously parsed past and dropped).
func TestParseDirExtractsTagsAndRelated(t *testing.T) {
	dir := t.TempDir()
	content := testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
		testsupport.WithTags("context", "config"), testsupport.WithRelated(1, 92),
		testsupport.WithTitle("0007: Tagged"), testsupport.WithBody("## Context\nx\n"))
	if err := os.WriteFile(filepath.Join(dir, "0007-tagged.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	got := adrs[0]
	if len(got.Tags) != 2 || got.Tags[0] != "context" || got.Tags[1] != "config" {
		t.Errorf("tags: got %#v", got.Tags)
	}
	if len(got.Related) != 2 || got.Related[0] != 1 || got.Related[1] != 92 {
		t.Errorf("related: got %#v", got.Related)
	}
}

// parseOne writes content as a single ADR fixture and returns its parse.
func parseOne(t *testing.T, content string) adr.ADR {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0001-fixture.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	return adrs[0]
}

// TestSupersessionRefExtraction covers the ADR-0120 token grammar: both keys
// extracted from the Decision section with the kind named by the key, tokens
// elsewhere inert, and the anchor shape rules (no leading-zero items, no
// uppercase slugs).
func TestSupersessionRefExtraction(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []adr.SupersessionRef
	}{
		{
			name: "both kinds extracted",
			body: "## Decision\n\n1. Overrides `supersedes: ADR-0116#2`.\n2. Retires `supersedes-invariant: ADR-0031#retired-slug`.\n",
			want: []adr.SupersessionRef{{Target: "0116", Item: 2, Relation: adr.Retires, CarrierItem: 1}, {Target: "0031", Slug: "retired-slug", Relation: adr.Retires, CarrierItem: 2}},
		},
		{
			name: "token outside the Decision section is inert",
			body: "## Context\n\nMentions `supersedes: ADR-0116#2` in passing.\n\n## Decision\n\n1. No tokens.\n",
			want: nil,
		},
		{
			name: "leading-zero item and uppercase slug do not match",
			body: "## Decision\n\n1. `supersedes: ADR-0116#03` and `supersedes-invariant: ADR-0031#Bad-Slug`.\n",
			want: nil,
		},
		{
			name: "fenced tokens are inert while real tokens remain visible",
			body: "## Decision\n\n```\n`supersedes: ADR-0999#7`\n## Fake\n1. Fake.\n```\n\n~~~\n`supersedes-invariant: ADR-0998#fake`\n~~~\n\n1. Real `supersedes: ADR-0116#2`.\n2. Real `supersedes-invariant: ADR-0031#retired-slug`.\n",
			want: []adr.SupersessionRef{{Target: "0116", Item: 2, Relation: adr.Retires, CarrierItem: 1}, {Target: "0031", Slug: "retired-slug", Relation: adr.Retires, CarrierItem: 2}},
		},
		{
			name: "faux fence closer leaves fenced ADR syntax inert",
			body: "## Decision\n\n```\n`supersedes: ADR-0999#7`\n``` not-a-closer\n## Fake\n`supersedes-invariant: ADR-0998#fake`\n```\n\n1. Real `supersedes: ADR-0116#2`.\n2. Real `supersedes-invariant: ADR-0031#retired-slug`.\n",
			want: []adr.SupersessionRef{{Target: "0116", Item: 2, Relation: adr.Retires, CarrierItem: 1}, {Target: "0031", Slug: "retired-slug", Relation: adr.Retires, CarrierItem: 2}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := parseOne(t, testsupport.ADR("Implemented", testsupport.WithTitle("0001: Fixture"), testsupport.WithBody(tc.body)))
			if !reflect.DeepEqual(a.Refs, tc.want) {
				t.Errorf("Refs: got %#v, want %#v", a.Refs, tc.want)
			}
		})
	}
}

// TestDecisionItems covers column-0 item enumeration: indented sub-lists
// skipped, multi-digit items matched, and fenced syntax ignored.
func TestDecisionItems(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []int
	}{
		{"indented sub-item not enumerated", "## Decision\n\n1. First.\n   1. Sub-item.\n2. Second.\n", []int{1, 2}},
		{"multi-digit item enumerated", "## Decision\n\n13. Thirteenth.\n", []int{13}},
		{"fenced column-0 numbered line is inert", "## Decision\n\n1. Example:\n\n```\n1. fenced line\n```\n\n2. Real.\n", []int{1, 2}},
		{"tilde fenced column-0 numbered line is inert", "## Decision\n\n~~~\n1. fenced line\n~~~\n\n1. Real.\n", []int{1}},
		{"no items", "## Decision\n\nProse only.\n", nil},
		{"final item without trailing newline", "## Decision\n\n1. Final.", []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := parseOne(t, testsupport.ADR("Implemented", testsupport.WithTitle("0001: Fixture"), testsupport.WithBody(tc.body)))
			if got := a.DecisionItems(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DecisionItems: got %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestSupersessionIndex covers the render view's derivation: chains sorted by
// predecessor, refs into missing or non-live targets dropped, and the override
// order (items by number, then slugs by slug, ties by successor).
func TestDecisionSectionOffsetsIgnoreFencedHeadings(t *testing.T) {
	body := "## Decision\n\n1. Real.\n\n```\n## Fake\n```\n\n2. Still real.\n\n## Consequences\n\nx\n"
	a := parseOne(t, testsupport.ADR("Implemented", testsupport.WithTitle("0001: Fixture"), testsupport.WithBody(body)))
	if got, want := a.DecisionEnd-a.DecisionStart, len("## Decision\n\n1. Real.\n\n```\n## Fake\n```\n\n2. Still real.\n\n"); got != want {
		t.Errorf("Decision section length = %d, want %d", got, want)
	}
}

// TestSupersessionModel covers ADR-0129's derived model: anchors as nodes,
// claims as edges carrying relation and rationale site, chains rendered
// one-to-many, and annotations scoped to live targets.
// invariant: supersession-model-anchor-nodes
func TestSupersessionModel(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		// Two chains, written successor-first to exercise the chain sort.
		"0001-old.md": testsupport.ADR("Superseded by ADR-0004", testsupport.WithSupersededBy("0004"), testsupport.WithTitle("0001: Old")),
		"0002-target.md": testsupport.ADR("Implemented", testsupport.WithTitle("0002: Target"),
			testsupport.WithBody("## Decision\n\n1. a.\n2. b.\n\n## Invariants\n\n- `invariant: s-one` - x.\n- `invariant: s-two` - y.\n")),
		"0003-elder.md": testsupport.ADR("Superseded by ADR-0005", testsupport.WithSupersededBy("0005"), testsupport.WithTitle("0003: Elder")),
		// 0004 carries every ref shape: items out of order, two slugs out of
		// order, a ref into a Superseded target, a dangling ref, and it claims
		// chain 0001.
		"0004-citer.md": testsupport.ADR("Implemented", testsupport.WithSupersedes(1), testsupport.WithTitle("0004: Citer"),
			testsupport.WithBody("## Decision\n\n1. `supersedes: ADR-0002#2`, `supersedes: ADR-0002#1`, "+
				"`supersedes-invariant: ADR-0002#s-two`, `supersedes-invariant: ADR-0002#s-one`, "+
				"`supersedes: ADR-0003#1`, `supersedes: ADR-0042#1`.\n")),
		// 0005 claims chain 0003 and re-claims 0002 item 1.
		"0005-later.md": testsupport.ADR("Accepted", testsupport.WithSupersedes(3), testsupport.WithTitle("0005: Later"),
			testsupport.WithBody("## Decision\n\n1. `supersedes: ADR-0002#1`.\n2. `refines: ADR-0002#2`.\n")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := mustCorpus(t, dir)

	// Nodes: items and slugs are independently claimable anchors, observable
	// through the claims recorded against each. 0002 carries two items and two
	// slugs, and every one of them is claimed.
	wantAnchors := []string{"ADR-0002#1", "ADR-0002#2", "ADR-0002#s-one", "ADR-0002#s-two"}
	seen := map[string]bool{}
	for _, claim := range c.ClaimsOn("0002") {
		seen[claim.Anchor.String()] = true
	}
	for _, want := range wantAnchors {
		if !seen[want] {
			t.Errorf("anchor %s carries no claim; items and slugs must both be nodes", want)
		}
	}

	// Edges carry the rationale site: 0005's refinement sits in its item 2.
	var refine adr.Claim
	for _, claim := range c.ClaimsOn("0002") {
		if claim.Carrier == "0005" && claim.Relation == adr.Refines {
			refine = claim
		}
	}
	if refine.CarrierItem != 2 {
		t.Errorf("refinement CarrierItem = %d, want 2 (the item whose prose justifies it)", refine.CarrierItem)
	}

	// 0002 is fully covered: every item and slug carries a retirement from an
	// Implemented carrier. 0004's own anchors are untouched, so it stays Live.
	if got := c.State("0002"); got != adr.StateCovered {
		t.Errorf("0002 state = %q, want Covered", got)
	}
	if got := c.State("0004"); got != adr.StateLive {
		t.Errorf("0004 state = %q, want Live", got)
	}

	// Chains merge the derived coverage with the transitional frontmatter
	// pairs, one-to-many.
	want := []adr.Chain{
		{Predecessor: "0001", Successors: []string{"0004"}},
		{Predecessor: "0002", Successors: []string{"0004"}},
		{Predecessor: "0003", Successors: []string{"0005"}},
	}
	if got := c.Chains(); !reflect.DeepEqual(got, want) {
		t.Errorf("chains:\ngot  %#v\nwant %#v", got, want)
	}

	// A Covered ADR drops out of the per-anchor annotations: the chains
	// subsection already names its retirers.
	for _, claim := range c.AnnotatedAnchors() {
		if claim.Anchor.ADR == "0002" {
			t.Errorf("covered ADR-0002 must not carry per-anchor annotations: %#v", claim)
		}
	}
}

// TestSupersessionStates covers the three derived states including both
// residuals, and the zero-anchor case that must not be vacuously Covered.
// invariant: supersession-model-derives-state
func TestSupersessionStates(t *testing.T) {
	const twoItems = "## Decision\n\n1. a.\n2. b.\n"
	cases := []struct {
		name    string
		carrier string
		want    adr.State
	}{
		{
			name:    "no claims at all is Live",
			carrier: testsupport.ADR("Implemented", testsupport.WithTitle("0002: C"), testsupport.WithBody("## Decision\n\n1. x.\n")),
			want:    adr.StateLive,
		},
		{
			name:    "claims on every anchor from an Implemented carrier is Covered",
			carrier: testsupport.ADR("Implemented", testsupport.WithTitle("0002: C"), testsupport.WithBody("## Decision\n\n1. `supersedes: ADR-0001#1`, `supersedes: ADR-0001#2`.\n")),
			want:    adr.StateCovered,
		},
		{
			name:    "some anchors retired is Partial",
			carrier: testsupport.ADR("Implemented", testsupport.WithTitle("0002: C"), testsupport.WithBody("## Decision\n\n1. `supersedes: ADR-0001#1`.\n")),
			want:    adr.StatePartial,
		},
		{
			// Residual one: refinements never count toward coverage, so an ADR
			// whose every item is merely adapted is still Live.
			name:    "every anchor refined is Live, never Covered",
			carrier: testsupport.ADR("Implemented", testsupport.WithTitle("0002: C"), testsupport.WithBody("## Decision\n\n1. `refines: ADR-0001#1`, `refines: ADR-0001#2`.\n")),
			want:    adr.StateLive,
		},
		{
			// Residual two: a Proposed successor must not kill its predecessor.
			name:    "retirements from a non-Implemented carrier do not count",
			carrier: testsupport.ADR("Proposed", testsupport.WithTitle("0002: C"), testsupport.WithBody("## Decision\n\n1. `supersedes: ADR-0001#1`, `supersedes: ADR-0001#2`.\n")),
			want:    adr.StateLive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(dir, "0001-target.md"),
				testsupport.ADR("Accepted", testsupport.WithTitle("0001: Target"),
					testsupport.WithRelated(2), testsupport.WithBody(twoItems)))
			testsupport.WriteFile(t, filepath.Join(dir, "0002-carrier.md"), tc.carrier)
			if got := mustCorpus(t, dir).State("0001"); got != tc.want {
				t.Errorf("state = %q, want %q", got, tc.want)
			}
		})
	}

	// A zero-anchor ADR is Live, not vacuously Covered: "every anchor retired"
	// is trivially true of an empty set, and believing that would retire an ADR
	// nobody superseded.
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-empty.md"),
		"---\nstatus: Accepted\n---\n# ADR-0001: Empty\n\n## Context\n\nNo decision section.\n")
	if got := mustCorpus(t, dir).State("0001"); got != adr.StateLive {
		t.Errorf("zero-anchor state = %q, want Live", got)
	}
}

// TestParseDirExtractsSupersedes confirms `supersedes:` frontmatter round-trips
// into adr.ADR (full-supersession claims, ADR-0120).
func TestParseDirExtractsSupersedes(t *testing.T) {
	content := "---\nstatus: Implemented\nsupersedes: [31]\n---\n# ADR-0120: T\n\n## Decision\n\n1. x.\n"
	a := parseOne(t, content)
	if !reflect.DeepEqual(a.Supersedes, []int{31}) {
		t.Errorf("Supersedes: got %#v, want [31]", a.Supersedes)
	}
}

// TestParseDirGlobError exercises the glob-pattern failure path: a directory
// whose name contains an unterminated "[" yields an ErrBadPattern from Glob.
func TestParseDirGlobError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad[")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.ParseDir(dir); err == nil {
		t.Fatal("expected glob error for malformed pattern, got nil")
	}
}

// TestParseDirReadError exercises the os.ReadFile failure path: a directory
// squatting on a path that matches the ADR filename pattern cannot be read as a
// file (fails for all users, including root).
func TestParseDirReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "0001-squatter.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.ParseDir(dir); err == nil {
		t.Fatal("expected read error for directory in file's place, got nil")
	}
}

// TestParseDirParseError exercises the parse failure path: malformed YAML
// frontmatter in an ADR file makes parse return an error.
func TestParseDirParseError(t *testing.T) {
	dir := t.TempDir()
	content := "---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n"
	if err := os.WriteFile(filepath.Join(dir, "0001-broken.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.ParseDir(dir); err == nil {
		t.Fatal("expected parse error for malformed frontmatter, got nil")
	}
}

// TestLoadCorpusParseError ensures the construction seam propagates a parse
// error rather than yielding a partial corpus. The render entry points take a
// Corpus (ADR-0130 item 1), so this is the one place a malformed ADR can
// surface: they no longer parse and so no longer fail.
func TestLoadCorpusParseError(t *testing.T) {
	dir := t.TempDir()
	content := "---\nstatus: [unterminated\n---\n# ADR-0001: Broken\n"
	if err := os.WriteFile(filepath.Join(dir, "0001-broken.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := adr.LoadCorpus(dir)
	if err == nil {
		t.Fatal("expected error from malformed frontmatter, got nil")
	}
	if len(got.All()) != 0 {
		t.Errorf("expected an empty corpus on error, got %d ADR(s)", len(got.All()))
	}
}

// TestRenderActiveMDSortsWithinStatusAndOrdersExtra covers two paths: sorting
// multiple ADRs within one status group (by number), and appending a status not
// present in statusOrder (sorted after the known statuses).
func TestRenderActiveMDSortsWithinStatusAndOrdersExtra(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0002-second-accepted.md": testsupport.ADR("Accepted", testsupport.WithTitle("0002: Second Accepted")),
		"0001-first-accepted.md":  testsupport.ADR("Accepted", testsupport.WithTitle("0001: First Accepted")),
		"0003-draft-status.md":    testsupport.ADR("Draft", testsupport.WithTitle("0003: Draft Status")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	got := adr.RenderActiveMD(mustCorpus(t, dir))

	// Within the Accepted group, 0001 must be listed before 0002.
	first := strings.Index(got, "ADR-0001: First Accepted")
	second := strings.Index(got, "ADR-0002: Second Accepted")
	if first < 0 || second < 0 {
		t.Fatalf("missing accepted entries; got:\n%s", got)
	}
	if first > second {
		t.Errorf("0001 (%d) should be listed before 0002 (%d)", first, second)
	}

	// The unknown "Draft" status must appear as its own section, after the
	// known statuses.
	acceptedPos := strings.Index(got, "## Accepted")
	draftPos := strings.Index(got, "## Draft")
	if draftPos < 0 {
		t.Fatalf("missing ## Draft section; got:\n%s", got)
	}
	if acceptedPos > draftPos {
		t.Errorf("Accepted (%d) should come before extra status Draft (%d)", acceptedPos, draftPos)
	}
}

// invariant: adr-sections-parsed
func TestParseDirExtractsSections(t *testing.T) {
	dir := t.TempDir()
	content := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("0009: S"), testsupport.WithBody("## Context\nctx body\n## Invariants\n- `invariant: example-slug` - a thing.\n## Consequences\ncons\n"))
	if err := os.WriteFile(filepath.Join(dir, "0009-s.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	inv := adrs[0].Sections["Invariants"]
	if !strings.Contains(inv, "invariant: example-slug") {
		t.Errorf("Invariants section missing tag; got: %q", inv)
	}
	if !strings.Contains(adrs[0].Sections["Context"], "ctx body") {
		t.Errorf("Context section wrong: %q", adrs[0].Sections["Context"])
	}
}

// adrTemplateFixture mirrors the shape of the rendered docs/decisions/template.md
// (marker comments, frontmatter placeholders, title heading) without the
// Invariants section's backtick-heavy prose, which NewFile never touches.
const adrTemplateFixture = `<!-- GENERATED by awf -- do not edit -->
<!-- awf:edit frontmatter -- default -->
---
status: Proposed
date: YYYY-MM-DD
supersedes: []
superseded_by: ""
tags: []
related: []
domains: []
---
# ADR-NNNN: Title


<!-- awf:edit body -- default -->
## Context

Fixture body.
`

func writeTemplateFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(adrTemplateFixture), 0o644); err != nil {
		t.Fatal(err)
	}
}

// swapNow overrides the adr package's now seam for the duration of a test.
func swapNow(t *testing.T, fn func() time.Time) {
	t.Helper()
	orig := adr.SetNowForTest(fn)
	t.Cleanup(func() { adr.SetNowForTest(orig) })
}

func fixedNow() time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) }

func TestNextNumberEmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := adr.NextNumber(dir)
	if err != nil {
		t.Fatalf("NextNumber: %v", err)
	}
	if got != "0001" {
		t.Errorf("NextNumber(empty) = %q, want 0001", got)
	}
}

func TestNextNumberSkipsGapToMaxPlusOne(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"0001-first.md", "0003-third.md"} {
		content := testsupport.ADR("Accepted", testsupport.WithTitle("title"))
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := adr.NextNumber(dir)
	if err != nil {
		t.Fatalf("NextNumber: %v", err)
	}
	// invariant: adr-new-sequential-numbering
	if got != "0004" {
		t.Errorf("NextNumber = %q, want 0004", got)
	}
}

func TestNextNumberPropagatesParseDirError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad[")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NextNumber(dir); err == nil {
		t.Fatal("expected glob error to propagate")
	}
}

func TestNewFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir)
	swapNow(t, fixedNow)

	path, err := adr.NewFile(dir, "My Cool Title")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	want := filepath.Join(dir, "0001-my-cool-title.md")
	if path != want {
		t.Errorf("NewFile path = %q, want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	body := string(got)
	// invariant: adr-new-strips-markers
	if strings.Contains(body, "GENERATED by awf") || strings.Contains(body, "awf:edit") {
		t.Errorf("marker comment survived: %q", body)
	}
	// invariant: adr-new-heading-matches-file
	if !strings.Contains(body, "# ADR-0001: My Cool Title") {
		t.Errorf("heading not filled in: %q", body)
	}
	if !strings.Contains(body, "date: 2026-07-01") {
		t.Errorf("date not filled in: %q", body)
	}
}

// TestNewFileSequentialCallsGetDifferentNumbers documents why NewFile's
// overwrite guard can never fire from repeated same-process calls: NextNumber
// always returns one more than every existing NNNN-*.md file, so a second call
// with the same title lands at the next number instead of colliding.
func TestNewFileSequentialCallsGetDifferentNumbers(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFixture(t, dir)
	swapNow(t, fixedNow)
	first, err := adr.NewFile(dir, "Same Title")
	if err != nil {
		t.Fatalf("first NewFile: %v", err)
	}
	second, err := adr.NewFile(dir, "Same Title")
	if err != nil {
		t.Fatalf("second NewFile: %v", err)
	}
	if first == second {
		t.Fatalf("expected distinct paths, both were %q", first)
	}
	wantFirst := filepath.Join(dir, "0001-same-title.md")
	wantSecond := filepath.Join(dir, "0002-same-title.md")
	if first != wantFirst || second != wantSecond {
		t.Errorf("got (%q, %q), want (%q, %q)", first, second, wantFirst, wantSecond)
	}
}

func TestNewFilePropagatesNextNumberError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bad[")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NewFile(dir, "Some Title"); err == nil {
		t.Fatal("expected NextNumber's glob error to propagate")
	}
}

func TestNewFileMissingTemplate(t *testing.T) {
	dir := t.TempDir()
	if _, err := adr.NewFile(dir, "No Template Here"); err == nil {
		t.Fatal("expected error for missing template.md")
	}
}

func TestNewFileEmptySlug(t *testing.T) {
	dir := t.TempDir()
	// No template.md needed - slugify errors before the file is ever read.
	if _, err := adr.NewFile(dir, "!!!"); err == nil {
		t.Fatal("expected slugify error for an all-punctuation title")
	}
}

func TestNewFileMissingDatePlaceholder(t *testing.T) {
	dir := t.TempDir()
	broken := strings.Replace(adrTemplateFixture, "date: YYYY-MM-DD\n", "", 1)
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NewFile(dir, "Some Title"); err == nil {
		t.Fatal("expected error for missing date placeholder")
	}
}

func TestNewFileMissingTitlePlaceholder(t *testing.T) {
	dir := t.TempDir()
	broken := strings.Replace(adrTemplateFixture, "# ADR-NNNN: Title\n", "", 1)
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := adr.NewFile(dir, "Some Title"); err == nil {
		t.Fatal("expected error for missing title placeholder")
	}
}

// TestStatusLiteralsOwnedByADRPackage enforces ADR-0130 item 3 mechanically:
// the ADR status vocabulary is compared in internal/adr and nowhere else, so
// every other consumer asks a predicate. This is what stops the three-way "is
// live" and five-way "is superseded" divergences the ADR was written to end.
//
// The scan is scoped to comparisons against an ADR-typed .Status field. Local
// status variables (internal/audit's `st`) are out of scope until Phase 3 of
// the coverage-derived-supersession plan gives audit parsed records; that task
// widens this scan to cover them.
//
// internal/project/context.go carries the single enumerated exception: ADR-0129
// item 4 keeps its Tier-2 exclusion on a direct prefix test.
// invariant: corpus-owns-status-literals
func TestStatusLiteralsOwnedByADRPackage(t *testing.T) {
	// Two shapes: a comparison against an ADR's .Status field, and a comparison
	// of any local against a status literal. internal/audit used to hold the
	// second shape on a local `st`; Phase 3 of the coverage-derived-supersession
	// plan gave it parsed records, so the scan covers both from here on.
	statusCmp := regexp.MustCompile(
		`\.Status\s*[!=]=\s*"` +
			`|HasPrefix\([^)]*\.Status\s*,\s*"` +
			`|[!=]=\s*"(Accepted|Implemented|Proposed|Superseded)"`)

	seen, transitional := 0, 0
	root := filepath.Join("..", "..", "internal")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// internal/adr owns the literals; the predicates themselves live there.
		if strings.HasPrefix(path, filepath.Join(root, "adr")+string(filepath.Separator)) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		seen++
		isContext := path == filepath.Join(root, "project", "context.go")
		for i, line := range strings.Split(string(data), "\n") {
			if !statusCmp.MatchString(line) {
				continue
			}
			// ADR-0129 item 4: awf context's Tier-2 exclusion is the one
			// consumer that keeps a direct status test.
			if isContext && strings.Contains(line, "tier1[a.Number]") {
				continue
			}
			// Transitional: the suffixed/scalar symmetry half asserts the very
			// encoding ADR-0128 removes, so it cannot be phrased as a predicate.
			// Task 6.2 of the coverage-derived-supersession plan deletes it; the
			// count guard below then fails and this branch comes out with it.
			if strings.Contains(line, `"Superseded by ADR-"+a.SupersededBy`) {
				transitional++
				continue
			}
			t.Errorf("%s:%d compares an ADR status literal directly - use an adr.ADR predicate (ADR-0130 item 3):\n\t%s",
				path, i+1, strings.TrimSpace(line))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Guard against a vacuous pass if internal/ is ever relocated.
	if seen < 10 {
		t.Fatalf("inspected only %d non-test source file(s) under internal/; the scan is not reaching the tree", seen)
	}
	// The transitional exemption is expected to be needed exactly once until the
	// schema removal deletes the symmetry check. When that lands, this fails and
	// the exemption branch above must be deleted rather than re-tuned.
	if transitional != 1 {
		t.Fatalf("transitional suffixed-status exemption matched %d line(s), want 1 - if the symmetry check is gone, delete the exemption branch", transitional)
	}
}

// mustCorpus loads a decisions directory into the view for a render test.
// RenderActiveMD and RenderDomainIndex take a Corpus rather than a directory
// (ADR-0130 item 1), so the parse is the caller's, made once.
func mustCorpus(t *testing.T, dir string) adr.Corpus {
	t.Helper()
	c, err := adr.LoadCorpus(dir)
	if err != nil {
		t.Fatalf("LoadCorpus(%s): %v", dir, err)
	}
	return c
}
