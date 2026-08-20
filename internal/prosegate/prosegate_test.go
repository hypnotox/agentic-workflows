package prosegate

import (
	"slices"
	"strings"
	"testing"
)

func ptr(n int) *int { return &n }

func TestParseCodepoint(t *testing.T) {
	for codepoint, want := range map[string]rune{
		"U+2013": '\u2013',
		"U+2014": '\u2014',
		"U+2018": '\u2018',
		"U+2019": '\u2019',
		"U+201C": '\u201c',
		"U+201D": '\u201d',
		"U+2026": '\u2026',
	} {
		if got, err := ParseCodepoint(codepoint); err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", codepoint, got, err, want)
		}
	}
	for _, s := range []string{"2014", "U+zzzz", "U+0041"} {
		if _, err := ParseCodepoint(s); err == nil {
			t.Errorf("%q: want error, got nil", s)
		}
	}
}

// invariant: tooling/quality-gates:prose-gate-tracked-file-scan (TestScanReportsPunctuationRestraintViolations)
func TestScanReportsPunctuationRestraintViolations(t *testing.T) {
	files := []File{
		{Path: "allowed.md", Bytes: []byte("\u201cquoted\u201d \u2026 ... a \u2014 b \u2014 c\n")},
		{Path: "both.md", Bytes: []byte("a \u2013 b \u2014 c \u2014 d \u2014 e\n\nnext \u2014 paragraph \u2014 stays restrained\n")},
		{Path: "aaa.md", Bytes: []byte("a \u2013 b\n")},
		{Path: "bin.dat", Bytes: []byte("\xff\xfe not utf8 \u2013\n")},
	}
	got, skipped, err := Scan(files, nil)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if want := []string{"bin.dat"}; !slices.Equal(skipped, want) {
		t.Errorf("skipped binary paths: got %v, want %v", skipped, want)
	}
	want := []Finding{
		{Path: "aaa.md", Rune: '\u2013', Count: 1},
		{Path: "both.md", Rune: '\u2013', Count: 1},
		{Path: "both.md", Rune: '\u2014', Count: 3},
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

// invariant: tooling/quality-gates:prose-gate-tracked-file-scan (TestScanSeparatesBlankLineDelimitedParagraphs)
func TestScanSeparatesBlankLineDelimitedParagraphs(t *testing.T) {
	text := "\nfirst \u2014 has \u2014 two\n \t \nsecond \u2014 has \u2014 three \u2014\r\n\r\nthird \u2014 also \u2014 has \u2014 three\n"
	got, _, err := Scan([]File{{Path: "paragraphs.md", Bytes: []byte(text)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Count != 3 || got[1].Count != 3 {
		t.Fatalf("want one finding for each dense paragraph, got %+v", got)
	}
	if first, second := Format(got[0]), Format(got[1]); !strings.Contains(first, "paragraph 2") || !strings.Contains(second, "paragraph 3") {
		t.Fatalf("findings must identify their text blocks, got %q and %q", first, second)
	}
}

// invariant: tooling/quality-gates:prose-gate-tracked-file-scan (TestScanExemptionModes)
func TestScanExemptionModes(t *testing.T) {
	files := []File{{Path: "f.md", Bytes: []byte("a \u2014 b \u2014 c \u2014 d\n")}}
	if got, _, _ := Scan(files, []Exemption{{Path: "f.md", Codepoint: '\u2014'}}); len(got) != 0 {
		t.Errorf("nil-count exemption: want 0 findings, got %+v", got)
	}
	if got, _, _ := Scan(files, []Exemption{{Path: "f.md", Codepoint: '\u2014', Count: ptr(3)}}); len(got) != 0 {
		t.Errorf("matching pin: want 0 findings, got %+v", got)
	}
	got, _, _ := Scan(files, []Exemption{{Path: "f.md", Codepoint: '\u2014', Count: ptr(2)}})
	if len(got) != 1 || got[0].Pinned == nil || *got[0].Pinned != 2 || got[0].Count != 3 {
		t.Fatalf("mismatched pin: want one finding pinned 2 count 3, got %+v", got)
	}
	zero, _, _ := Scan(files, []Exemption{{Path: "f.md", Codepoint: '\u2014', Count: ptr(0)}})
	if len(zero) != 1 || zero[0].Pinned == nil || *zero[0].Pinned != 0 {
		t.Fatalf("zero pin: want one finding pinned 0, got %+v", zero)
	}
	if msg := Format(zero[0]); !strings.Contains(msg, "pins 0") {
		t.Errorf("zero pin message: %q", msg)
	}

	legacy, _, err := Scan(
		[]File{{Path: "legacy.md", Bytes: []byte("\u201chello\u201d \u2026\n")}},
		[]Exemption{{Path: "legacy.md", Codepoint: '\u201c', Count: ptr(99)}},
	)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("retired-codepoint exemptions must remain accepted and inert, got %+v, %v", legacy, err)
	}

	for _, tc := range []struct {
		name  string
		files []File
		ex    Exemption
	}{
		{name: "clean pinned path", files: []File{{Path: "clean.md", Bytes: []byte("clean\n")}}, ex: Exemption{Path: "clean.md", Codepoint: '\u2014', Count: ptr(1)}},
		{name: "missing pinned path", files: nil, ex: Exemption{Path: "missing.md", Codepoint: '\u2014', Count: ptr(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := Scan(tc.files, []Exemption{tc.ex})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Path != tc.ex.Path || got[0].Rune != tc.ex.Codepoint || got[0].Count != 0 || got[0].Pinned == nil || *got[0].Pinned != 1 {
				t.Fatalf("zero-count pin mismatch: got %+v", got)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	plain := Format(Finding{Path: "a.md", Rune: '\u2013', Count: 1})
	if !strings.Contains(plain, "a.md") || !strings.Contains(plain, "en-dash (U+2013)") || !strings.Contains(plain, "1") {
		t.Errorf("plain: %q", plain)
	}
	pinned := Format(Finding{Path: "b.md", Rune: '\u201c', Count: 2, Pinned: ptr(1)})
	if !strings.Contains(pinned, "pins 1") || !strings.Contains(pinned, "2 time") {
		t.Errorf("pinned: %q", pinned)
	}
}
