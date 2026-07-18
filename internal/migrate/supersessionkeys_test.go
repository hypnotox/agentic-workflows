package migrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSupersessionFixture lays out a minimal adopted tree with a decisions
// directory, returning the root.
func writeSupersessionFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".awf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("docsDir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	decisions := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(decisions, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(decisions, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestSupersessionKeysMigration covers the generation-12 port: both keys
// stripped, pre-existing item tokens downgraded to refinements, a bookkeeping
// item appended per superseded predecessor, back-pointers backfilled, and the
// suffixed status rewritten bare.
// invariant: upgrade-migrates-supersession-keys
func TestSupersessionKeysMigration(t *testing.T) {
	root := writeSupersessionFixture(t, map[string]string{
		"0001-old.md": "---\nstatus: Superseded by ADR-0002\ndate: 2026-01-01\ntags: [x]\nrelated: []\ndomains: []\nsupersedes: []\nsuperseded_by: \"0002\"\n---\n" +
			"# ADR-0001: Old\n\n## Decision\n\n1. a.\n2. b.\n\n## Invariants\n\n- `invariant: old-slug` - x.\n",
		"0002-new.md": "---\nstatus: Implemented\ndate: 2026-01-02\ntags: [x]\nrelated: []\ndomains: []\nsupersedes: [1]\nsuperseded_by: \"\"\n---\n" +
			"# ADR-0002: New\n\n## Decision\n\n1. Adapts `supersedes: ADR-0003#1` in passing.\n",
		"0003-other.md": "---\nstatus: Implemented\ndate: 2026-01-03\ntags: [x]\nrelated: [2]\ndomains: []\nsupersedes: []\nsuperseded_by: \"\"\n---\n" +
			"# ADR-0003: Other\n\n## Decision\n\n1. c.\n",
	})

	var out bytes.Buffer
	if err := applySupersessionKeys(root, &out); err != nil {
		t.Fatalf("applySupersessionKeys: %v", err)
	}
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	old, newer := read("0001-old.md"), read("0002-new.md")

	// Both keys are gone from every ADR.
	for name, body := range map[string]string{"0001": old, "0002": newer, "0003": read("0003-other.md")} {
		for _, key := range []string{"\nsupersedes:", "\nsuperseded_by:"} {
			if strings.Contains(body, key) {
				t.Errorf("ADR-%s still carries %q", name, strings.TrimSpace(key))
			}
		}
	}

	// A pre-existing item token is downgraded: the mechanical rewrite asserts
	// less, because the corpus is mostly refinements and promoting a genuine
	// retirement back is a reviewable edit while a wrong retirement silently
	// kills an ADR.
	if !strings.Contains(newer, "`refines: ADR-0003#1`") {
		t.Errorf("pre-existing item token was not downgraded to refines:\n%s", newer)
	}

	// The bookkeeping item retires every anchor of the predecessor, using the
	// right key per anchor kind, and lands inside the Decision section.
	for _, want := range []string{
		"`supersedes: ADR-0001#1`", "`supersedes: ADR-0001#2`",
		"`supersedes-invariant: ADR-0001#old-slug`",
	} {
		if !strings.Contains(newer, want) {
			t.Errorf("missing bookkeeping token %s in:\n%s", want, newer)
		}
	}
	// Inside the Decision section: after its heading, and before whatever
	// heading follows it (there is none here, so the item runs to the end).
	decisionStart := strings.Index(newer, "## Decision")
	bookkeeping := strings.Index(newer, "Supersedence bookkeeping")
	if bookkeeping < decisionStart {
		t.Errorf("bookkeeping item landed before the Decision section:\n%s", newer)
	}
	if next := strings.Index(newer[decisionStart+len("## Decision"):], "\n## "); next != -1 &&
		bookkeeping > decisionStart+len("## Decision")+next {
		t.Errorf("bookkeeping item landed outside the Decision section:\n%s", newer)
	}

	// The predecessor gains the back-pointer and a bare status.
	if !strings.Contains(old, "related: [2]") {
		t.Errorf("predecessor did not gain the back-pointer:\n%s", old)
	}
	if !strings.Contains(old, "\nstatus: Superseded\n") {
		t.Errorf("predecessor status was not rewritten bare:\n%s", old)
	}
}

// TestSupersessionKeysOffsetsSurvivePassOne pins the offset bug that shipped
// once: pass 1 shortens the body by three bytes per rewritten token, so an
// append at the parsed DecisionEnd lands early unless the delta is tracked. On
// a token-dense ADR it landed inside the following heading and silently
// destroyed the Invariants section.
// invariant: upgrade-migrates-supersession-keys
func TestSupersessionKeysOffsetsSurvivePassOne(t *testing.T) {
	var tokens []string
	for i := 1; i <= 8; i++ {
		tokens = append(tokens, "`supersedes: ADR-0003#1`")
	}
	root := writeSupersessionFixture(t, map[string]string{
		"0001-old.md": "---\nstatus: Superseded by ADR-0002\ndate: 2026-01-01\ntags: [x]\nrelated: []\ndomains: []\nsupersedes: []\nsuperseded_by: \"0002\"\n---\n" +
			"# ADR-0001: Old\n\n## Decision\n\n1. a.\n",
		// Token-dense carrier: eight rewrites shorten the body by 24 bytes.
		"0002-new.md": "---\nstatus: Implemented\ndate: 2026-01-02\ntags: [x]\nrelated: []\ndomains: []\nsupersedes: [1]\nsuperseded_by: \"\"\n---\n" +
			"# ADR-0002: New\n\n## Decision\n\n1. " + strings.Join(tokens, ", ") + ".\n\n## Invariants\n\n- `invariant: keeper` - x.\n\n## Consequences\n\nc.\n",
		"0003-other.md": "---\nstatus: Implemented\ndate: 2026-01-03\ntags: [x]\nrelated: [2]\ndomains: []\nsupersedes: []\nsuperseded_by: \"\"\n---\n" +
			"# ADR-0003: Other\n\n## Decision\n\n1. c.\n",
	})
	if err := applySupersessionKeys(root, &bytes.Buffer{}); err != nil {
		t.Fatalf("applySupersessionKeys: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0002-new.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	// The heading that follows the Decision section must survive intact: a
	// mangled `## Invariants` silently drops every slug the ADR declares, and
	// surfaces far away as unrelated token-ref drift.
	if !strings.Contains(body, "\n## Invariants\n") {
		t.Errorf("the Invariants heading was corrupted by a stale append offset:\n%s", body)
	}
	if !strings.Contains(body, "- `invariant: keeper` - x.") {
		t.Errorf("the Invariants declaration was destroyed:\n%s", body)
	}
}

// TestSupersessionKeysIsNoOpWithoutKeys pins idempotent re-run safety for the
// two shapes an adopter can present: a tree with no config at all, and a
// corpus that carries neither key.
// invariant: upgrade-migrates-supersession-keys
func TestSupersessionKeysIsNoOpWithoutKeys(t *testing.T) {
	if err := applySupersessionKeys(t.TempDir(), &bytes.Buffer{}); err != nil {
		t.Fatalf("no config should be a no-op, got %v", err)
	}

	clean := "---\nstatus: Implemented\ndate: 2026-01-01\ntags: [x]\nrelated: []\ndomains: []\n---\n# ADR-0001: Clean\n\n## Decision\n\n1. a.\n"
	root := writeSupersessionFixture(t, map[string]string{"0001-clean.md": clean})
	var out bytes.Buffer
	if err := applySupersessionKeys(root, &out); err != nil {
		t.Fatalf("applySupersessionKeys: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("a key-free corpus should report nothing, got %q", out.String())
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-clean.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != clean {
		t.Errorf("a key-free ADR must survive byte-identical:\ngot  %q\nwant %q", b, clean)
	}
}

// TestSupersessionKeysRefusesUnresolvableClaim pins the loud failures: a
// supersedes: entry that is not a number, and one naming an ADR that does not
// exist. A migration that guessed here would silently drop a claim.
// invariant: upgrade-migrates-supersession-keys
func TestSupersessionKeysRefusesUnresolvableClaim(t *testing.T) {
	cases := map[string]string{
		"missing target":    "supersedes: [9]",
		"non-numeric entry": "supersedes: [abc]",
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			root := writeSupersessionFixture(t, map[string]string{
				"0001-a.md": "---\nstatus: Implemented\ndate: 2026-01-01\ntags: [x]\nrelated: []\ndomains: []\n" + line + "\nsuperseded_by: \"\"\n---\n" +
					"# ADR-0001: A\n\n## Decision\n\n1. a.\n",
			})
			if err := applySupersessionKeys(root, &bytes.Buffer{}); err == nil {
				t.Fatal("expected the migration to refuse an unresolvable supersedes: claim")
			}
		})
	}
}

// TestSupersessionKeysEdgeShapes covers the corpus shapes the migration has to
// survive or refuse loudly. Each is a real adopter possibility: an ADR predating
// the Decision-section convention, a related: line that already carries entries,
// and the two structural refusals that must never be guessed past.
// invariant: upgrade-migrates-supersession-keys
func TestSupersessionKeysEdgeShapes(t *testing.T) {
	const fm = "---\nstatus: %s\ndate: 2026-01-01\ntags: [x]\nrelated: %s\ndomains: []\n%s\n---\n"

	t.Run("an ADR with no Decision section is left alone", func(t *testing.T) {
		body := fmt.Sprintf(fm, "Implemented", "[]", `supersedes: []`+"\n"+`superseded_by: ""`) +
			"# ADR-0001: Prose\n\n## Context\n\nNo decision section at all.\n"
		root := writeSupersessionFixture(t, map[string]string{"0001-prose.md": body})
		if err := applySupersessionKeys(root, &bytes.Buffer{}); err != nil {
			t.Fatalf("applySupersessionKeys: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-prose.md"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "supersedes:") {
			t.Errorf("keys should still be stripped from a Decision-less ADR:\n%s", b)
		}
	})

	t.Run("an existing related: list gains the back-pointer, order preserved", func(t *testing.T) {
		root := writeSupersessionFixture(t, map[string]string{
			"0001-old.md": fmt.Sprintf(fm, "Superseded by ADR-0002", "[7, 9]", `supersedes: []`+"\n"+`superseded_by: "0002"`) +
				"# ADR-0001: Old\n\n## Decision\n\n1. a.\n",
			"0002-new.md": fmt.Sprintf(fm, "Implemented", "[]", `supersedes: [1]`+"\n"+`superseded_by: ""`) +
				"# ADR-0002: New\n\n## Decision\n\n1. b.\n",
		})
		if err := applySupersessionKeys(root, &bytes.Buffer{}); err != nil {
			t.Fatalf("applySupersessionKeys: %v", err)
		}
		b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-old.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "related: [7, 9, 2]") {
			t.Errorf("back-pointer must append, preserving existing order:\n%s", b)
		}
	})

	t.Run("a predecessor with no anchors is refused", func(t *testing.T) {
		root := writeSupersessionFixture(t, map[string]string{
			"0001-empty.md": fmt.Sprintf(fm, "Implemented", "[]", `supersedes: []`+"\n"+`superseded_by: ""`) +
				"# ADR-0001: Empty\n\n## Context\n\nNothing to retire.\n",
			"0002-new.md": fmt.Sprintf(fm, "Implemented", "[]", `supersedes: [1]`+"\n"+`superseded_by: ""`) +
				"# ADR-0002: New\n\n## Decision\n\n1. b.\n",
		})
		if err := applySupersessionKeys(root, &bytes.Buffer{}); err == nil {
			t.Fatal("expected a refusal: there is no anchor to write a retirement token for")
		}
	})

	t.Run("a carrier with no Decision section is refused", func(t *testing.T) {
		root := writeSupersessionFixture(t, map[string]string{
			"0001-old.md": fmt.Sprintf(fm, "Implemented", "[]", `supersedes: []`+"\n"+`superseded_by: ""`) +
				"# ADR-0001: Old\n\n## Decision\n\n1. a.\n",
			"0002-new.md": fmt.Sprintf(fm, "Implemented", "[]", `supersedes: [1]`+"\n"+`superseded_by: ""`) +
				"# ADR-0002: New\n\n## Context\n\nNowhere to append the bookkeeping item.\n",
		})
		if err := applySupersessionKeys(root, &bytes.Buffer{}); err == nil {
			t.Fatal("expected a refusal: the bookkeeping item has no Decision section to land in")
		}
	})

	t.Run("a target with no related: line is refused", func(t *testing.T) {
		root := writeSupersessionFixture(t, map[string]string{
			"0001-old.md": "---\nstatus: Implemented\ndate: 2026-01-01\ntags: [x]\ndomains: []\n---\n" +
				"# ADR-0001: Old\n\n## Decision\n\n1. a.\n",
			"0002-new.md": fmt.Sprintf(fm, "Implemented", "[]", `supersedes: [1]`+"\n"+`superseded_by: ""`) +
				"# ADR-0002: New\n\n## Decision\n\n1. b.\n",
		})
		if err := applySupersessionKeys(root, &bytes.Buffer{}); err == nil {
			t.Fatal("expected a refusal rather than a silent body edit where related: is absent")
		}
	})

	t.Run("an unreadable config and a malformed ADR both surface", func(t *testing.T) {
		root := writeSupersessionFixture(t, map[string]string{})
		if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("docsDir: [not, a, string]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := applySupersessionKeys(root, &bytes.Buffer{}); err == nil {
			t.Fatal("expected the config parse error to surface")
		}

		bad := writeSupersessionFixture(t, map[string]string{
			"0001-bad.md": "---\nstatus: [unterminated\n---\n# ADR-0001: Bad\n",
		})
		if err := applySupersessionKeys(bad, &bytes.Buffer{}); err == nil {
			t.Fatal("expected the ADR parse error to surface")
		}
	})
}

// TestSupersessionKeysChainedSupersession pins the chained case: A supersedes
// B, and B supersedes C. B gains its own bookkeeping Decision item, which is a
// NEW anchor of B - so A's tokens, enumerated from the pre-migration parse,
// would leave that anchor unclaimed. B would then be rewritten to bare
// `Superseded` while deriving Partial, and the very next `awf check` would fail
// with adr-coverage-status on a corpus the migration had just produced.
// invariant: upgrade-migrates-supersession-keys
func TestSupersessionKeysChainedSupersession(t *testing.T) {
	const fm = "---\nstatus: %s\ndate: 2026-01-01\ntags: [x]\nrelated: %s\ndomains: []\nsupersedes: %s\nsuperseded_by: %q\n---\n"
	root := writeSupersessionFixture(t, map[string]string{
		"0001-a.md": fmt.Sprintf(fm, "Implemented", "[]", "[2]", "") +
			"# ADR-0001: A\n\n## Decision\n\n1. a.\n",
		"0002-b.md": fmt.Sprintf(fm, "Superseded by ADR-0001", "[]", "[3]", "0001") +
			"# ADR-0002: B\n\n## Decision\n\n1. b.\n",
		"0003-c.md": fmt.Sprintf(fm, "Superseded by ADR-0002", "[]", "[]", "0002") +
			"# ADR-0003: C\n\n## Decision\n\n1. c.\n",
	})
	if err := applySupersessionKeys(root, &bytes.Buffer{}); err != nil {
		t.Fatalf("applySupersessionKeys: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-a.md"))
	if err != nil {
		t.Fatal(err)
	}
	// B ends up with two anchors: its original item 1 and the bookkeeping item 2
	// the migration appends to it. A must retire both, or B is flipped to
	// Superseded while only partially covered.
	for _, want := range []string{"`supersedes: ADR-0002#1`", "`supersedes: ADR-0002#2`"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("ADR-0001 must retire %s, including the anchor the migration itself adds to ADR-0002:\n%s", want, b)
		}
	}

	// The same chain with a slug anchor on the middle ADR: the pending item is
	// slotted with the other items, so the emitted list reads in anchor order
	// rather than trailing the slugs.
	root = writeSupersessionFixture(t, map[string]string{
		"0001-a.md": fmt.Sprintf(fm, "Implemented", "[]", "[2]", "") +
			"# ADR-0001: A\n\n## Decision\n\n1. a.\n",
		"0002-b.md": fmt.Sprintf(fm, "Superseded by ADR-0001", "[]", "[3]", "0001") +
			"# ADR-0002: B\n\n## Decision\n\n1. b.\n\n## Invariants\n\n- `invariant: b-slug` - x.\n",
		"0003-c.md": fmt.Sprintf(fm, "Superseded by ADR-0002", "[]", "[]", "0002") +
			"# ADR-0003: C\n\n## Decision\n\n1. c.\n",
	})
	if err := applySupersessionKeys(root, &bytes.Buffer{}); err != nil {
		t.Fatalf("applySupersessionKeys: %v", err)
	}
	b, err = os.ReadFile(filepath.Join(root, "docs", "decisions", "0001-a.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "`supersedes: ADR-0002#1`, `supersedes: ADR-0002#2`, `supersedes-invariant: ADR-0002#b-slug`"
	if !strings.Contains(string(b), want) {
		t.Errorf("want anchors in order %s, got:\n%s", want, b)
	}
}
