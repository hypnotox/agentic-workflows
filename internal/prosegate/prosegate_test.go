package prosegate

import (
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
	// two.md carries two distinct runes, so the sort comparator exercises both
	// its path branch (across files) and its rune tie-break (within one file).
	files := []File{
		{Path: "clean.md", Bytes: []byte("plain ascii, nothing banned\n")},
		{Path: "one.md", Bytes: []byte("a \u2014 b\n")},
		{Path: "two.md", Bytes: []byte("a \u2014 b \u2026 c\n")},
		{Path: "bin.dat", Bytes: []byte("\xff\xfe not utf8 \u2014\n")},
		{Path: "exempt.md", Bytes: []byte("\u201c depicted \u201c\n")},
	}
	got, err := Scan(files, []Exemption{{Path: "exempt.md", Codepoint: '\u201c', Count: ptr(2)}})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
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
	files := []File{{Path: "f.md", Bytes: []byte("a \u2014 b \u2014 c\n")}}
	if got, _ := Scan(files, []Exemption{{Path: "f.md", Codepoint: '\u2014'}}); len(got) != 0 {
		t.Errorf("nil-count exemption: want 0 findings, got %+v", got)
	}
	if got, _ := Scan(files, []Exemption{{Path: "f.md", Codepoint: '\u2014', Count: ptr(2)}}); len(got) != 0 {
		t.Errorf("matching pin: want 0 findings, got %+v", got)
	}
	got, _ := Scan(files, []Exemption{{Path: "f.md", Codepoint: '\u2014', Count: ptr(1)}})
	if len(got) != 1 || got[0].Pinned == nil || *got[0].Pinned != 1 || got[0].Count != 2 {
		t.Fatalf("mismatched pin: want one finding pinned 1 count 2, got %+v", got)
	}
	zero, _ := Scan(files, []Exemption{{Path: "f.md", Codepoint: '\u2014', Count: ptr(0)}})
	if len(zero) != 1 || zero[0].Pinned == nil || *zero[0].Pinned != 0 {
		t.Fatalf("zero pin: want one finding pinned 0, got %+v", zero)
	}
	if msg := Format(zero[0]); !strings.Contains(msg, "pins 0") {
		t.Errorf("zero pin message: %q", msg)
	}
}

func TestFormat(t *testing.T) {
	plain := Format(Finding{Path: "a.md", Rune: '\u2014', Count: 3})
	if !strings.Contains(plain, "a.md") || !strings.Contains(plain, "em-dash (U+2014)") || !strings.Contains(plain, "3") {
		t.Errorf("plain: %q", plain)
	}
	pinned := Format(Finding{Path: "b.md", Rune: '\u201c', Count: 2, Pinned: ptr(1)})
	if !strings.Contains(pinned, "pins 1") || !strings.Contains(pinned, "2 time") {
		t.Errorf("pinned: %q", pinned)
	}
}
