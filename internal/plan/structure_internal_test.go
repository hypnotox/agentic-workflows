package plan

import (
	"strings"
	"testing"
)

func TestParseTaskRejectsMalformedHeadingAndFieldBoundaries(t *testing.T) {
	cases := []struct {
		lines []string
		want  string
	}{
		{[]string{"### Task invalid"}, "malformed task heading"},
		{[]string{"### Task 1.2: Wrong"}, "task number 1.2, want 1.1"},
		{[]string{"### Task 1.1: Bad", "Kind:batch"}, "malformed field Kind"},
		{[]string{"### Task 1.1: Plain", "plain"}, ""},
	}
	for _, tc := range cases {
		_, _, err := parseTask("fixture.md", tc.lines, 0, 1, 1, false)
		if tc.want == "" && err == nil {
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("parseTask(%q) = %v", tc.lines, err)
		}
	}
}

func TestMarkdownFenceConsumesOpaqueLinesAndQualifiedCloser(t *testing.T) {
	var fence markdownFence
	if fence.consume("plain text") {
		t.Fatal("plain text was opaque")
	}
	if !fence.consume("```go") || fence.marker != '`' {
		t.Fatal("opener was not retained")
	}
	if !fence.consume("## Phase 2: opaque") || fence.marker != '`' {
		t.Fatal("fenced heading escaped")
	}
	if !fence.consume("``` trailing") || fence.marker != '`' {
		t.Fatal("info-bearing closer closed fence")
	}
	if !fence.consume("````") || fence.marker != 0 {
		t.Fatal("long qualified closer did not close")
	}
	if !fence.consume("~~~") || fence.marker != '~' {
		t.Fatal("tilde opener was not retained")
	}
}
