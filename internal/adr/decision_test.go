package adr

import (
	"errors"
	"strings"
	"testing"
)

func decisionFixture(format, name, slug, decision string) string {
	frontmatter := "---\nformat: " + format + "\nstatus: " + statusProposed + "\ndate: 2026-08-02\n"
	identity := strings.TrimSuffix(name, ".md")
	if slug != "" {
		frontmatter += "slug: " + slug + "\n"
		if Number := FilenameRe.FindStringSubmatch(name); Number != nil {
			identity = Number[1]
		}
	}
	return frontmatter + "---\n# ADR-" + identity + ": Decision fixture\n\n" +
		"## Context\n\nContext.\n\n## Decision\n\n" + decision + "\n\n" +
		"## State changes\n\nNone.\n\n## Consequences\n\nNone.\n\n" +
		"## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-08-02: " + statusProposed + "\n"
}

// invariant: adr-system/adr-lifecycle:decision-item-stable-identity (TestDecisionItemStableIdentity)
func TestDecisionItemStableIdentity(t *testing.T) {
	first := "1. `decision: current-context` First commitment.\n\n   Continuation bytes.\n\n   - nested list\n\n   ```go\n   1. backtick fence stays in the first item\n   ```\n\n"
	second := "2. `decision: supersedes-history` Second commitment.\n\n   ~~~text\n   2. tilde fence stays in the second item\n   ~~~\n"
	v4Bytes := decisionFixture(V4FormatMarker, "pending.md", "pending", first+second)
	before := v4Bytes
	v4, err := ParseV4("pending.md", []byte(v4Bytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(v4.decisions) != 2 || v4.decisions[0].source != first || v4.decisions[1].source != second+"\n\n" {
		t.Fatalf("retained V4 items = %#v", v4.decisions)
	}
	if v4.source != before || v4.decisionStart == 0 || v4.decisionEnd == 0 {
		t.Fatal("V4 source or Decision bounds changed during parsing")
	}
	for selector, want := range map[string]string{"current-context": first, "supersedes-history": second + "\n\n"} {
		item, err := v4.decisionBySelector(selector)
		if err != nil || item.source != want {
			t.Fatalf("identity-only V4 selector %q = %#v, %v", selector, item, err)
		}
	}
	for name, malformed := range map[string]string{
		"missing marker":   strings.Replace(v4Bytes, "`decision: current-context` ", "", 1),
		"malformed marker": strings.Replace(v4Bytes, "current-context", "Current", 1),
		"duplicate marker": strings.Replace(v4Bytes, "supersedes-history", "current-context", 1),
		"empty commitment": strings.Replace(v4Bytes, "First commitment.", "", 1),
	} {
		if _, err := ParseV4("pending.md", []byte(malformed)); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	if _, err := v4.decisionBySelector("#1"); err == nil {
		t.Fatal("V4 ordinal selector was accepted")
	}
	if _, err := v4.decisionBySelector("missing"); err == nil {
		t.Fatal("unknown V4 selector was accepted")
	}
	if v4.source != before {
		t.Fatal("lookup mutated retained source")
	}

	for _, tc := range []struct {
		format, name, slug string
		parse              func(string, []byte) (ADR, error)
	}{
		{V1FormatMarker, "0001-v1.md", "", ParseV1},
		{V2FormatMarker, "0002-v2.md", "", ParseV2},
		{V3FormatMarker, "0003-v3.md", "v3", ParseV3},
	} {
		t.Run(tc.format, func(t *testing.T) {
			doc := decisionFixture(tc.format, tc.name, tc.slug, "1. Legacy ordinal.\n")
			a, err := tc.parse(tc.name, []byte(doc))
			if err != nil || len(a.decisions) != 1 || a.decisions[0].ordinal != 1 || a.decisions[0].source != "1. Legacy ordinal.\n\n\n" {
				t.Fatalf("pre-V4 retention = %#v, %v", a.decisions, err)
			}
			if _, err := a.decisionBySelector("#1"); err == nil {
				t.Fatal("amendable pre-V4 ordinal selector was accepted")
			}
			frozen := a
			frozen.Status = statusImplemented
			item, err := frozen.decisionBySelector("#1")
			if err != nil || item.ordinal != 1 {
				t.Fatalf("frozen #1 = %#v, %v", item, err)
			}
			for _, incompatible := range []string{"1", "#01", "current-context"} {
				if _, err := frozen.decisionBySelector(incompatible); err == nil {
					t.Fatalf("incompatible selector %q was accepted", incompatible)
				}
			}
			if _, err := frozen.decisionBySelector("#2"); err == nil {
				t.Fatal("unknown frozen pre-V4 ordinal was accepted")
			}
		})
	}
}

func TestRetainDecisionItemsIgnoresSectionWithoutBodyNewline(t *testing.T) {
	a := ADR{source: "xDecision", decisionStart: 1, decisionEnd: len("xDecision")}
	retainDecisionItems(&a)
	if a.decisions != nil {
		t.Fatalf("decisions = %#v", a.decisions)
	}
}

func TestDecisionLookupPublicContract(t *testing.T) {
	v4Source := decisionFixture(V4FormatMarker, "pending.md", "pending", "1. `decision: first` First.\n\n2. `decision: second` Second.\n")
	v4, err := ParseV4("pending.md", []byte(v4Source))
	if err != nil {
		t.Fatal(err)
	}
	if got := v4.Decisions(); len(got) != 2 || got[1].Key != "pending:second" {
		t.Fatalf("V4 decisions = %#v", got)
	}
	if got := v4.DecisionSelectors(); strings.Join(got, ",") != "first,second" {
		t.Fatalf("selectors = %v", got)
	}
	item, err := v4.LookupDecision("first")
	if err != nil || item.Key != "pending:first" || !strings.Contains(item.Markdown, "First.") {
		t.Fatalf("LookupDecision = %#v, %v", item, err)
	}
	if _, err := v4.LookupDecision("#1"); err == nil || !strings.Contains(err.Error(), "available: first, second") {
		t.Fatalf("V4 incompatible lookup = %v", err)
	} else {
		var selector *DecisionSelectorError
		if !errors.Is(err, ErrDecisionSelectorIncompatible) || !errors.As(err, &selector) || strings.Join(selector.Available, ",") != "first,second" {
			t.Fatalf("typed V4 selector error = %#v", err)
		}
	}
	if _, err := v4.LookupDecision("missing"); err == nil || !errors.Is(err, ErrDecisionSelectorUnknown) {
		t.Fatalf("unknown selector error = %v", err)
	}

	legacy, err := ParseV1("0001-legacy.md", []byte(decisionFixture(V1FormatMarker, "0001-legacy.md", "", "1. Legacy.\n")))
	if err != nil {
		t.Fatal(err)
	}
	legacy.Status = statusImplemented
	if got := legacy.DecisionSelectors(); strings.Join(got, ",") != "#1" {
		t.Fatalf("legacy selectors = %v", got)
	}
	if item, err := legacy.LookupDecision("#1"); err != nil || item.Key != "0001:#1" {
		t.Fatalf("legacy LookupDecision = %#v, %v", item, err)
	}
	if got := legacy.Decisions(); len(got) != 1 || got[0].Key != "0001:#1" {
		t.Fatalf("legacy decisions = %#v", got)
	}
	amendable := legacy
	amendable.Status = statusProposed
	if got := amendable.Decisions(); got != nil {
		t.Fatalf("amendable legacy decisions = %#v", got)
	}
	if _, err := amendable.LookupDecision("#1"); err == nil || !strings.Contains(err.Error(), "amendable") {
		t.Fatalf("amendable legacy lookup = %v", err)
	}
}
