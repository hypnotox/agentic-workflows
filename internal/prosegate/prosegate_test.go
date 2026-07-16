package prosegate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ptr(n int) *int { return &n }

func TestParseCodepoint(t *testing.T) {
	if r, err := ParseCodepoint("U+2014"); err != nil || r != '\u2014' {
		t.Fatalf("U+2014: got %q, %v", r, err)
	}
	for _, s := range []string{"2014", "U+zzzz", "U+0041"} {
		if _, err := ParseCodepoint(s); err == nil {
			t.Errorf("%q: want error, got nil", s)
		}
	}
}

// invariant: prose-gate-tracked-file-scan
func TestScanReportsBannedRunesOutsideExemptions(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// two.md carries two distinct runes, so the sort comparator exercises both
	// its path branch (across files) and its rune tie-break (within one file).
	write("clean.md", "plain ascii, nothing banned\n")
	write("one.md", "a \u2014 b\n")
	write("two.md", "a \u2014 b \u2026 c\n")
	write("bin.dat", "\xff\xfe not utf8 \u2014\n")
	write("exempt.md", "\u201c depicted \u201c\n")

	paths := []string{"clean.md", "one.md", "two.md", "bin.dat", "exempt.md"}
	got, err := Scan(root, paths, []Exemption{
		{Path: "exempt.md", Codepoint: '\u201c', Count: ptr(2)},
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	// Expected, sorted: one.md em, two.md em, two.md ellipsis. bin.dat is skipped
	// (not UTF-8); exempt.md's two curly quotes match the pin.
	want := []Finding{
		{Path: "one.md", Rune: '\u2014', Count: 1},
		{Path: "two.md", Rune: '\u2014', Count: 1},
		{Path: "two.md", Rune: '\u2026', Count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d findings, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Path != want[i].Path || got[i].Rune != want[i].Rune || got[i].Count != want[i].Count {
			t.Errorf("finding %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestScanExemptionModes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.md"), []byte("a \u2014 b \u2014 c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// nil Count permits any number.
	if got, _ := Scan(root, []string{"f.md"}, []Exemption{{Path: "f.md", Codepoint: '\u2014'}}); len(got) != 0 {
		t.Errorf("nil-count exemption: want 0 findings, got %+v", got)
	}
	// matching pinned Count permits.
	if got, _ := Scan(root, []string{"f.md"}, []Exemption{{Path: "f.md", Codepoint: '\u2014', Count: ptr(2)}}); len(got) != 0 {
		t.Errorf("matching pin: want 0 findings, got %+v", got)
	}
	// mismatched pinned Count reports, carrying the pin.
	got, _ := Scan(root, []string{"f.md"}, []Exemption{{Path: "f.md", Codepoint: '\u2014', Count: ptr(1)}})
	if len(got) != 1 || got[0].Pinned != 1 || got[0].Count != 2 {
		t.Fatalf("mismatched pin: want one finding pinned 1 count 2, got %+v", got)
	}
}

func TestScanUnreadablePathIsError(t *testing.T) {
	root := t.TempDir()
	if _, err := Scan(root, []string{"does-not-exist.md"}, nil); err == nil {
		t.Fatal("want error for an unreadable path, got nil")
	}
}

func TestFormat(t *testing.T) {
	plain := Format(Finding{Path: "a.md", Rune: '\u2014', Count: 3})
	if !strings.Contains(plain, "a.md") || !strings.Contains(plain, "em-dash (U+2014)") || !strings.Contains(plain, "3") {
		t.Errorf("plain: %q", plain)
	}
	pinned := Format(Finding{Path: "b.md", Rune: '\u201c', Count: 2, Pinned: 1})
	if !strings.Contains(pinned, "pins 1") || !strings.Contains(pinned, "2 time") {
		t.Errorf("pinned: %q", pinned)
	}
}
