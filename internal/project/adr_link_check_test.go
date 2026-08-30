package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// A dangling ADR related: number yields adr-related-link drift; a resolving one
// yields none. Unconditional (no glossary configured here).
func TestCheckADRRelatedLinks(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\n")
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/0001-a.md"),
		testsupport.ADR("Accepted", testsupport.WithDate("2026-07-13"),
			testsupport.WithRelated(1, 42), testsupport.WithTitle("0001: A"),
			testsupport.WithBody("## Context\nx\n")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift := checkADRRelatedLinks(mustDeriveCorpus(t, p))
	if len(drift) != 1 || drift[0].Kind != "adr-related-link" || !strings.Contains(drift[0].Detail, "0042") {
		t.Fatalf("want one adr-related-link(0042) drift, got %#v", drift)
	}
}

// Every clause of the slug is exercised: a descending array reports
// adr-related-order and an ascending one does not; the finding names the FIRST
// descent; and resolution stays independent of ordering, so a descending array
// still has every entry checked against the corpus. The last two are what the
// separate-loops implementation exists for - a merged loop that aborts the
// resolution scan at the first descent, or a missing break, each passes a test
// that only checks the simple case.
func TestCheckADRRelatedAscending(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars: {}\ndomains: []\n")
	write := func(name, title string, related ...int) {
		opts := []testsupport.ADROption{testsupport.WithDate("2026-07-13"),
			testsupport.WithTitle(title), testsupport.WithBody("## Context\nx\n")}
		if len(related) > 0 {
			opts = append(opts, testsupport.WithRelated(related...))
		}
		testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/"+name),
			testsupport.ADR("Accepted", opts...))
	}
	write("0001-a.md", "0001: A", 42, 2)        // one descent, every entry resolves
	write("0002-b.md", "0002: B", 1, 42)        // ascending
	write("0003-c.md", "0003: C", 42, 2, 77)    // descent AND a dangling entry after it
	write("0004-d.md", "0004: D", 42, 2, 43, 3) // two descents; only the first is reported
	write("0042-e.md", "0042: E")
	write("0043-f.md", "0043: F")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	drift := checkADRRelatedLinks(mustDeriveCorpus(t, p))
	kinds := map[string][]string{}
	for _, d := range drift {
		kinds[d.Kind] = append(kinds[d.Kind], d.Detail)
	}
	order, link := kinds["adr-related-order"], kinds["adr-related-link"]

	// 0002 ascends and must contribute nothing; 0001, 0003 and 0004 each
	// contribute exactly one ordering finding.
	if len(order) != 3 {
		t.Fatalf("want three adr-related-order findings (0001, 0003, 0004), got %d: %q", len(order), order)
	}
	joined := strings.Join(order, "\n")
	if strings.Contains(joined, "ADR-0002") {
		t.Errorf("an ascending array must not be reported, got %q", joined)
	}
	for _, want := range []string{"ADR-0001", "ADR-0003", "ADR-0004"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing ordering finding for %s in %q", want, joined)
		}
	}

	// The dangling entry sits AFTER the descent in 0003, so a loop that stops
	// scanning at the first descent would never reach it.
	if len(link) != 1 || !strings.Contains(link[0], "ADR-0077") {
		t.Errorf("a descent must not suppress the dangling-link scan for later entries, got %q", link)
	}

	// 0004 descends twice; exactly one finding, naming the first descent.
	var d4 string
	for _, d := range order {
		if strings.Contains(d, "ADR-0004") {
			d4 = d
		}
	}
	if !strings.Contains(d4, "descends at 2 after 42") {
		t.Errorf("finding must name the first descent (2 after 42), got %q", d4)
	}
	if strings.Contains(d4, "after 43") {
		t.Errorf("only the first descent is reported, got %q", d4)
	}
}
