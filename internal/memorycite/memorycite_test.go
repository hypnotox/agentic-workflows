package memorycite

import (
	"slices"
	"strings"
	"testing"
)

func ptr(n int) *int { return &n }

func owned(slug string) string { return dir + slug + "/" + memoryBase }

func TestScanTextDiscriminatesConcreteOwnedMemoryPaths(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		want []string
	}{
		{"slash path", owned("real-effort"), []string{"real-effort/memory.md"}},
		{"backslash path", strings.ReplaceAll(owned("real-effort"), "/", `\`), []string{"real-effort/memory.md"}},
		{"relative path", "../../" + owned("real-effort"), []string{"real-effort/memory.md"}},
		{"prose", "see " + owned("real-effort") + " for details", []string{"real-effort/memory.md"}},
		{"link", "[checkpoint](" + owned("real-effort") + ")", []string{"real-effort/memory.md"}},
		{"code", "`" + owned("real-effort") + "`", []string{"real-effort/memory.md"}},
		{"bare efforts directory", "see " + dir + " for residents", nil},
		{"placeholder", dir + "<effort-slug>/" + memoryBase, nil},
		{"placeholder mid slug", dir + "effort-<slug>/" + memoryBase, nil},
		{"wrong basename", dir + "real-effort/notes.md", nil},
		{"invalid slug", dir + "Real_Effort/" + memoryBase, nil},
		{"overlong slug", owned(strings.Repeat("a", 64)), nil},
		{"leading hyphen", owned("-effort"), nil},
		{"trailing hyphen", owned("effort-"), nil},
		{"double hyphen", owned("effort--name"), nil},
		{"basename prefix", owned("real-effort") + ".bak", nil},
		{"two references", owned("first") + " and " + owned("second"), []string{"first/memory.md", "second/memory.md"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := ScanText("f.md", []byte(test.text))
			if len(got) != len(test.want) {
				t.Fatalf("%q: got %d reference(s) %v, want %d", test.text, len(got), got, len(test.want))
			}
			for index, reference := range got {
				if reference.Segment != test.want[index] || reference.Path != "f.md" || reference.Line != 1 {
					t.Errorf("reference %d = %#v, want segment %q at f.md:1", index, reference, test.want[index])
				}
			}
		})
	}
}

func TestScanTextNumbersLines(t *testing.T) {
	text := "clean\n" + owned("one") + "\nalso clean\n" + owned("two") + "\n"
	got := ScanText("plan.md", []byte(text))
	wantLines := []int{2, 4}
	if len(got) != 2 {
		t.Fatalf("got %d reference(s) %v, want 2", len(got), got)
	}
	for index, reference := range got {
		if reference.Line != wantLines[index] {
			t.Errorf("reference %d: line %d, want %d", index, reference.Line, wantLines[index])
		}
	}
}

func TestScanAppliesExemptions(t *testing.T) {
	files := []File{
		{Path: "a.md", Bytes: []byte(owned("one") + "\n")},
		{Path: "clean.md", Bytes: []byte("nothing here\n")},
		{Path: "any.md", Bytes: []byte(owned("x") + "\n" + owned("y") + "\n")},
		{Path: "pinned.md", Bytes: []byte(owned("z") + "\n")},
		{Path: "mismatch.md", Bytes: []byte(owned("p") + "\n" + owned("q") + "\n")},
	}
	exemptions := []Exemption{
		{Path: "any.md"},
		{Path: "pinned.md", Count: ptr(1)},
		{Path: "mismatch.md", Count: ptr(1)},
		{Path: "gone.md", Count: ptr(2)},
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
	for index, finding := range got {
		expected := want[index]
		if finding.Path != expected.Path || !slices.Equal(finding.Lines, expected.Lines) {
			t.Errorf("finding %d: got %s %v, want %s %v", index, finding.Path, finding.Lines, expected.Path, expected.Lines)
		}
		switch {
		case (finding.Pinned == nil) != (expected.Pinned == nil):
			t.Errorf("finding %d: pin presence mismatch", index)
		case finding.Pinned != nil && *finding.Pinned != *expected.Pinned:
			t.Errorf("finding %d: pin %d, want %d", index, *finding.Pinned, *expected.Pinned)
		}
	}
}

func TestFormatNamesOwnedMemoryRepair(t *testing.T) {
	plain := Format(Finding{Path: "docs/plans/p.md", Lines: []int{3, 9}})
	for _, want := range []string{"docs/plans/p.md", "2 effort-owned memory citation(s)", "3, 9", ".awf/efforts/", "placeholder"} {
		if !strings.Contains(plain, want) {
			t.Errorf("unpinned format missing %q: %s", want, plain)
		}
	}
	pinned := Format(Finding{Path: "docs/plans/p.md", Lines: []int{3}, Pinned: ptr(2)})
	for _, want := range []string{"docs/plans/p.md", "1 effort-owned memory citation(s)", "pins 2"} {
		if !strings.Contains(pinned, want) {
			t.Errorf("pinned format missing %q: %s", want, pinned)
		}
	}
}
