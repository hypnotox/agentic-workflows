package memorycite

import (
	"slices"
	"strings"
	"testing"
)

func ptr(n int) *int { return &n }

// Every fixture is built from the dir constant rather than written out, for the
// reason the package doc-comment gives: a test line writing a concrete name
// right after the prefix would carry the shape the detector flags.
func TestScanTextDiscriminatesConcreteSegments(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want []string // expected segments, in order
	}{
		{"concrete file flags", dir + "effort.md", []string{"effort.md"}},
		{"placeholder segment passes", dir + "<effort-slug>.md", nil},
		{"placeholder mid-segment passes", dir + "eff<x>.md", nil},
		{"bare directory before a space passes", "see " + dir + " for the file", nil},
		{"bare directory before a backtick passes", "`" + dir + "` holds it", nil},
		{"bare directory before a backslash passes", dir + `\` + "`", nil},
		{"bare directory at end of input passes", "it lives in " + dir, nil},
		{"ignore file passes", dir + ignoreFile, nil},
		{"ignore file before a double quote passes", `"` + dir + ignoreFile + `"`, nil},
		{"ignore file before a single quote passes", "'" + dir + ignoreFile + "'", nil},
		{"ignore-file prefix is not an exact match", dir + ".gitignored.md", []string{".gitignored.md"}},
		{"nested path flags its first segment", dir + "nested/file.awf-bak", []string{"nested"}},
		{
			"two on one line, left to right",
			dir + "first.md and " + dir + "second.md",
			[]string{"first.md", "second.md"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ScanText("f.md", []byte(tc.text))
			if len(got) != len(tc.want) {
				t.Fatalf("%q: got %d reference(s) %v, want %d", tc.text, len(got), got, len(tc.want))
			}
			for i, ref := range got {
				if ref.Segment != tc.want[i] {
					t.Errorf("reference %d: segment %q, want %q", i, ref.Segment, tc.want[i])
				}
				if ref.Path != "f.md" || ref.Line != 1 {
					t.Errorf("reference %d: got %s:%d, want f.md:1", i, ref.Path, ref.Line)
				}
			}
		})
	}
}

func TestScanTextNumbersLines(t *testing.T) {
	text := "clean\n" + dir + "one.md\nalso clean\n" + dir + "two.md\n"
	got := ScanText("plan.md", []byte(text))
	wantLines := []int{2, 4}
	if len(got) != 2 {
		t.Fatalf("got %d reference(s) %v, want 2", len(got), got)
	}
	for i, ref := range got {
		if ref.Line != wantLines[i] {
			t.Errorf("reference %d: line %d, want %d", i, ref.Line, wantLines[i])
		}
	}
}

func TestScanAppliesExemptions(t *testing.T) {
	files := []File{
		{Path: "a.md", Bytes: []byte(dir + "one.md\n")},
		{Path: "clean.md", Bytes: []byte("nothing here\n")},
		{Path: "any.md", Bytes: []byte(dir + "x.md\n" + dir + "y.md\n")},
		{Path: "pinned.md", Bytes: []byte(dir + "z.md\n")},
		{Path: "mismatch.md", Bytes: []byte(dir + "p.md\n" + dir + "q.md\n")},
	}
	exemptions := []Exemption{
		{Path: "any.md"},                     // nil count: suppressed at any count
		{Path: "pinned.md", Count: ptr(1)},   // matching pin: suppressed
		{Path: "mismatch.md", Count: ptr(1)}, // pin below the actual count: reported
		{Path: "gone.md", Count: ptr(2)},     // pinned but the citations are gone
	}
	got := Scan(files, exemptions)
	want := []Finding{
		{Path: "a.md", Lines: []int{1}},
		{Path: "gone.md", Pinned: ptr(2)},
		{Path: "mismatch.md", Lines: []int{1, 2}, Pinned: ptr(1)},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d finding(s) %v, want %d", len(got), got, len(want))
	}
	for i, f := range got {
		w := want[i]
		if f.Path != w.Path || !slices.Equal(f.Lines, w.Lines) {
			t.Errorf("finding %d: got %s %v, want %s %v", i, f.Path, f.Lines, w.Path, w.Lines)
		}
		switch {
		case (f.Pinned == nil) != (w.Pinned == nil):
			t.Errorf("finding %d: pin presence mismatch, got %v", i, f.Pinned)
		case f.Pinned != nil && *f.Pinned != *w.Pinned:
			t.Errorf("finding %d: pin %d, want %d", i, *f.Pinned, *w.Pinned)
		}
	}
}

func TestFormat(t *testing.T) {
	plain := Format(Finding{Path: "docs/plans/p.md", Lines: []int{3, 9}})
	for _, want := range []string{"docs/plans/p.md", "2 working-memory citation(s)", "3, 9", "placeholder"} {
		if !strings.Contains(plain, want) {
			t.Errorf("unpinned format missing %q: %s", want, plain)
		}
	}
	pinned := Format(Finding{Path: "docs/plans/p.md", Lines: []int{3}, Pinned: ptr(2)})
	for _, want := range []string{"docs/plans/p.md", "1 working-memory citation(s)", "pins 2"} {
		if !strings.Contains(pinned, want) {
			t.Errorf("pinned format missing %q: %s", want, pinned)
		}
	}
}
