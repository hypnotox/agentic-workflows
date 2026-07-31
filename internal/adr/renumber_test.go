package adr_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestRenumberPendingRewritesNameAndHeadingOnly pins numbering's record-level
// effect surface: the file takes the NNNN-<slug>.md name, the heading takes the
// number, and every other byte - the retained slug key, the frontmatter, the
// body, the Status history - is identical to what the pending file held. The
// original path is gone, so nothing can resolve the record twice.
func TestRenumberPendingRewritesNameAndHeadingOnly(t *testing.T) {
	dir := t.TempDir()
	before := pendingFixture("numbering-target")
	testsupport.WriteFile(t, filepath.Join(dir, "numbering-target.md"), before)

	if err := adr.RenumberPending(dir, "numbering-target", 217); err != nil {
		t.Fatalf("RenumberPending: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "numbering-target.md")); !os.IsNotExist(err) {
		t.Fatalf("pending file survived numbering: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "0217-numbering-target.md"))
	if err != nil {
		t.Fatalf("read numbered record: %v", err)
	}
	want := strings.Replace(before, "# ADR-numbering-target:", "# ADR-0217:", 1)
	if string(got) != want {
		t.Errorf("numbering changed more than the heading:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(string(got), "slug: numbering-target\n") {
		t.Error("the retained slug key must survive numbering")
	}
}

// TestRenumberPendingRewritesOnlyTheRecordsOwnHeading pins the rewrite to the
// first match. A record about ADR numbering quotes the pending heading form in
// its body readily, and a quote is prose rather than identity: the digest
// covers those bytes, so rewriting every match would silently amend a record
// while numbering claims to touch nothing but the name and the heading. The
// file mode carries across too, since numbering is conceptually a rename.
func TestRenumberPendingRewritesOnlyTheRecordsOwnHeading(t *testing.T) {
	dir := t.TempDir()
	pending := filepath.Join(dir, "quoting.md")
	quote := "# ADR-quoting: A decision\n"
	before := pendingFixture("quoting") + "\nThe pending heading form reads:\n\n```\n" + quote + "```\n"
	testsupport.WriteFile(t, pending, before)
	if err := os.Chmod(pending, 0o640); err != nil {
		t.Fatal(err)
	}

	if err := adr.RenumberPending(dir, "quoting", 218); err != nil {
		t.Fatalf("RenumberPending: %v", err)
	}
	numbered := filepath.Join(dir, "0218-quoting.md")
	got, err := os.ReadFile(numbered)
	if err != nil {
		t.Fatalf("read numbered record: %v", err)
	}
	if want := strings.Replace(before, "# ADR-quoting:", "# ADR-0218:", 1); string(got) != want {
		t.Errorf("a body quote of the heading was rewritten:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(string(got), quote) {
		t.Error("the quoted heading in the body must survive numbering verbatim")
	}
	info, err := os.Stat(numbered)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o640 {
		t.Errorf("numbering changed the file mode to %v", perm)
	}
}

// TestRenumberPendingRefusals covers the two ways the rewrite seam can be
// pointed at something it cannot renumber: a slug with no pending file, and a
// file whose heading does not carry the slug identity form.
func TestRenumberPendingRefusals(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "headless.md"),
		strings.Replace(pendingFixture("headless"), "# ADR-headless: Test Decision", "# Some other heading", 1))

	if err := adr.RenumberPending(dir, "absent", 1); err == nil || !strings.Contains(err.Error(), "read pending record absent") {
		t.Errorf("absent slug error = %v", err)
	}
	if err := adr.RenumberPending(dir, "headless", 1); err == nil || !strings.Contains(err.Error(), "has no \"# ADR-headless:\" heading") {
		t.Errorf("heading-less record error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "0001-headless.md")); !os.IsNotExist(err) {
		t.Error("a refused renumber must write nothing")
	}
}

// TestHistoriesEqualComparesEveryEventField proves the byte-equality comparison
// the numbering transition takes: a pair whose history matches field for field
// is equal, and a change to any one recorded field is not. Reusing an
// append-tolerant variant here would have accepted the truncation case.
func TestHistoriesEqualComparesEveryEventField(t *testing.T) {
	base := adr.ADR{History: []adr.StatusEntry{
		{Kind: adr.HistoryStatus, Date: "2026-07-31", Status: "Implementing", Digest: "sha256:a"},
		{Kind: adr.HistoryApplied, Date: "2026-07-31", Operations: []adr.Operation{{Verb: adr.OpAdd, ID: "d/t:x"}}},
	}}
	same := adr.ADR{History: append([]adr.StatusEntry(nil), base.History...)}
	if !adr.HistoriesEqual(base, same) {
		t.Fatal("identical histories must compare equal")
	}
	for name, mutate := range map[string]func(h []adr.StatusEntry){
		"kind":       func(h []adr.StatusEntry) { h[1].Kind = adr.HistoryAmended },
		"date":       func(h []adr.StatusEntry) { h[0].Date = "2026-08-01" },
		"status":     func(h []adr.StatusEntry) { h[0].Status = "Implemented" },
		"digest":     func(h []adr.StatusEntry) { h[0].Digest = "sha256:b" },
		"operations": func(h []adr.StatusEntry) { h[1].Operations = []adr.Operation{{Verb: adr.OpRemove, ID: "d/t:x"}} },
	} {
		history := append([]adr.StatusEntry(nil), base.History[0], base.History[1])
		mutate(history)
		if adr.HistoriesEqual(base, adr.ADR{History: history}) {
			t.Errorf("a changed %s must not compare equal", name)
		}
	}
	if adr.HistoriesEqual(base, adr.ADR{History: base.History[:1]}) {
		t.Error("a truncated history must not compare equal")
	}
	if adr.HistoriesEqual(base, adr.ADR{History: append(append([]adr.StatusEntry(nil), base.History...), adr.StatusEntry{Kind: adr.HistoryStatus, Date: "2026-08-01", Status: "Implemented"})}) {
		t.Error("an appended event must not compare equal")
	}
}
