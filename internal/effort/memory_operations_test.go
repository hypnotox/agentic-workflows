package effort

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryDisplayDiffUsesPiRows)
func TestMemoryDisplayDiffUsesPiRows(t *testing.T) {
	diff := memoryDiff([]byte("one\ntwo\nthree\n"), []byte("one\nTWO\nthree\n"), 6)
	if diff.Text != " 7 one\n-8 two\n+8 TWO\n 9 three\n" || diff.FirstChangedLine == nil || *diff.FirstChangedLine != 8 || diff.Truncated {
		t.Fatalf("diff = %#v", diff)
	}
	for _, change := range []struct{ before, after string }{
		{"one\n", "one\ntwo\n"},
		{"one\ntwo\n", "one\n"},
		{"a\nold\nz\n", "a\nnew\nz\n"},
		{"a\r\nold\r\nz", "a\r\nnew\r\nz"},
		{"final", "final\n"},
		{"final\n", "final"},
	} {
		got := memoryDiff([]byte(change.before), []byte(change.after), 0)
		if got.Text == "" || got.FirstChangedLine == nil || *got.FirstChangedLine < 1 || strings.Contains(got.Text, "\r") {
			t.Fatalf("display diff=%#v", got)
		}
	}

	// Pi parses each row with /^([+-\s])(\s*\d*)\s(.*)$/, so the omission row is
	// spelled out here rather than read back from the helper under test: a kind
	// byte, an empty right-aligned number field, one space, and an ellipsis.
	for _, want := range []struct {
		width int
		row   string
	}{{1, "   ..."}, {2, "    ..."}, {5, "       ..."}} {
		if got := omissionDisplayRow(want.width); got != want.row || !piDisplayRow.MatchString(got) {
			t.Fatalf("omission row width %d = %q, want %q matching %s", want.width, got, want.row, piDisplayRow)
		}
	}

	before := "old\n" + strings.Repeat("context\n", 12) + "tail old\n"
	after := "new\n" + strings.Repeat("context\n", 12) + "tail new\n"
	separated := memoryDiff([]byte(before), []byte(after), 0)
	// Unjoinable hunks drop only context the fixed window never selected, so the
	// complete diff must not claim the bounding loss truncated reports.
	if strings.Count(separated.Text, "    ...\n") != 1 || separated.Truncated {
		t.Fatalf("separated diff=%#v", separated)
	}

	// The context window is pinned from below as well as above: exactly four
	// unchanged rows surround a lone change in a document with ample context.
	window := strings.Repeat("ctx\n", 20)
	var want strings.Builder
	for line := 17; line <= 20; line++ {
		fmt.Fprintf(&want, " %d ctx\n", line)
	}
	want.WriteString("-21 old\n+21 new\n")
	for line := 22; line <= 25; line++ {
		fmt.Fprintf(&want, " %d ctx\n", line)
	}
	single := memoryDiff([]byte(window+"old\n"+window), []byte(window+"new\n"+window), 0)
	if single.Text != want.String() || single.Truncated {
		t.Fatalf("context window=%q, want %q", single.Text, want.String())
	}
}

// piDisplayRow is Pi's own parseDiffLine grammar, transcribed.
var piDisplayRow = regexp.MustCompile(`^([-+\s])(\s*\d*)\s(.*)$`)

func TestMemoryDisplayDiffBoundsCompleteRows(t *testing.T) {
	long := strings.Repeat("é", maxMemoryDiffBytes)
	diff := memoryDiff([]byte(long+"\n"), []byte("changed "+long+"\n"), 0)
	if !diff.Truncated || len(diff.Text) > maxMemoryDiffBytes || !utf8.ValidString(diff.Text) || !strings.Contains(diff.Text, "-1 [content elided]\n") || !strings.Contains(diff.Text, "+1 [content elided]\n") {
		t.Fatalf("long-line diff=%#v bytes=%d", diff, len(diff.Text))
	}

	rows := make([]displayDiffRow, 12001)
	for i := range rows {
		kind := byte(' ')
		changed := i%2 == 0
		if changed {
			kind = '+'
		}
		rows[i] = displayDiffRow{text: displayRow(kind, i+1, 5, strings.Repeat("x", 16)), changed: changed}
	}
	text, truncated := boundedDisplayRows(rows, 5)
	if !truncated || len(text) > maxMemoryDiffBytes || !utf8.ValidString(text) || !strings.Contains(text, "+    1 ") || !strings.Contains(text, "+12001 ") || !strings.Contains(text, "       ...\n") {
		t.Fatalf("aggregate diff truncated=%t bytes=%d head/tail/omission=%t/%t/%t", truncated, len(text), strings.Contains(text, "+    1 "), strings.Contains(text, "+12001 "), strings.Contains(text, "       ...\n"))
	}
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if line == "       ..." {
			continue
		}
		if len(line) < 8 || (line[0] != '+' && line[0] != '-' && line[0] != ' ') {
			t.Fatalf("incomplete display row %q", line)
		}
	}

	oversizedContext := []displayDiffRow{{text: strings.Repeat("x", maxMemoryDiffBytes+1)}, {text: "+1 changed", changed: true}}
	if got, cut := boundedDisplayRows(oversizedContext, 1); !cut || !strings.Contains(got, "+1 changed") || !strings.Contains(got, "   ...") {
		t.Fatalf("oversized context text=%q truncated=%t", got, cut)
	}
	nearLimit := strings.Repeat("x", maxMemoryDiffBytes-1)
	if _, ok := renderSelectedDisplayRows([]displayDiffRow{{text: nearLimit}, {text: "omitted"}, {text: "selected"}}, []bool{true, false, true}, 1); ok {
		t.Fatal("interior omission overflow accepted")
	}
	if _, ok := renderSelectedDisplayRows([]displayDiffRow{{text: nearLimit}, {text: "omitted"}}, []bool{true, false}, 1); ok {
		t.Fatal("trailing omission overflow accepted")
	}
}

// referenceBoundedDisplayRows is the per-candidate selection the incremental
// byte accounting replaced: it re-rendered every row for every candidate. It
// stays here as the oracle for byte-identical output.
func referenceBoundedDisplayRows(rows []displayDiffRow, width int) (string, bool) {
	const marker = "[content elided]"
	normalized := append([]displayDiffRow(nil), rows...)
	available := make([]bool, len(rows))
	truncated := false
	for i, row := range normalized {
		available[i] = true
		if len(row.text)+1 <= maxMemoryDiffBytes {
			continue
		}
		truncated = true
		if row.changed {
			normalized[i].text = row.text[:width+2] + marker
		} else {
			available[i] = false
		}
	}
	if text, ok := renderSelectedDisplayRows(normalized, available, width); ok {
		return text, truncated
	}
	selected := make([]bool, len(rows))
	var changed []int
	for i, row := range normalized {
		if row.changed {
			changed = append(changed, i)
		}
	}
	for left, right := 0, len(changed)-1; left <= right; left, right = left+1, right-1 {
		indexes := []int{changed[left]}
		if left != right {
			indexes = append(indexes, changed[right])
		}
		for _, index := range indexes {
			selected[index] = true
			if _, ok := renderSelectedDisplayRows(normalized, selected, width); ok {
				continue
			}
			selected[index] = false
		}
	}
	for distance := 1; distance <= 4; distance++ {
		for _, index := range changed {
			for _, contextIndex := range []int{index - distance, index + distance} {
				if contextIndex < 0 || contextIndex >= len(rows) || selected[contextIndex] || normalized[contextIndex].changed || normalized[contextIndex].text == omissionDisplayRow(width) {
					continue
				}
				selected[contextIndex] = true
				if _, ok := renderSelectedDisplayRows(normalized, selected, width); !ok {
					selected[contextIndex] = false
				}
			}
		}
	}
	text, ok := renderSelectedDisplayRows(normalized, selected, width)
	if !ok || text == "" {
		return omissionDisplayRow(width) + "\n", true
	}
	return text, true
}

func TestBoundedDisplayRowsSelectExactlyThePerCandidateRows(t *testing.T) {
	build := func(count, size int, changedAt func(int) bool, omissionAt func(int) bool, width int) []displayDiffRow {
		rows := make([]displayDiffRow, count)
		for i := range rows {
			switch {
			case omissionAt(i):
				rows[i] = displayDiffRow{text: omissionDisplayRow(width)}
			case changedAt(i):
				rows[i] = displayDiffRow{text: displayRow('+', i+1, width, strings.Repeat("y", size)), changed: true}
			default:
				rows[i] = displayDiffRow{text: displayRow(' ', i+1, width, strings.Repeat("x", size)), changed: false}
			}
		}
		return rows
	}
	never := func(int) bool { return false }
	all := func(int) bool { return true }
	every := func(n int) func(int) bool { return func(i int) bool { return i%n == 0 } }
	for _, shape := range []struct {
		name  string
		rows  []displayDiffRow
		width int
	}{
		{name: "whole-body replacement", rows: build(12001, 16, all, never, 5), width: 5},
		{name: "alternating context", rows: build(12001, 16, every(2), never, 5), width: 5},
		{name: "sparse changes with wide context", rows: build(9000, 24, every(37), never, 4), width: 4},
		{name: "group separators between changes", rows: build(6000, 40, every(11), every(53), 4), width: 4},
		{name: "few wide rows", rows: build(40, 2000, every(3), never, 2), width: 2},
		{name: "everything fits", rows: build(60, 12, every(5), never, 2), width: 2},
		{name: "single change", rows: build(9, 12, func(i int) bool { return i == 4 }, never, 1), width: 1},
		{name: "oversized rows", rows: append(build(30, 8, every(4), never, 2), displayDiffRow{text: strings.Repeat("z", maxMemoryDiffBytes+1), changed: true}, displayDiffRow{text: strings.Repeat("w", maxMemoryDiffBytes+1)}), width: 2},
		{name: "context only", rows: build(20000, 12, never, never, 5), width: 5},
	} {
		t.Run(shape.name, func(t *testing.T) {
			wantText, wantTruncated := referenceBoundedDisplayRows(shape.rows, shape.width)
			gotText, gotTruncated := boundedDisplayRows(shape.rows, shape.width)
			if gotText != wantText || gotTruncated != wantTruncated {
				t.Fatalf("bounded rows truncated=%t/%t bytes=%d/%d first difference at %d", gotTruncated, wantTruncated, len(gotText), len(wantText), commonPrefixLen(gotText, wantText))
			}
		})
	}
}

func commonPrefixLen(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryEditDiffOfALargeResidentStaysWellInsideTheClientTimeout)
func TestMemoryEditDiffOfALargeResidentStaysWellInsideTheClientTimeout(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "wide-edit", Title: "Wide edit"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("wide-edit")
	// Short lines maximise the display-row count a near-1-MiB resident yields,
	// and the diff is computed before publication, so a per-candidate re-render
	// makes this in-bounds edit impossible rather than merely slow.
	body := strings.Repeat("a\n", 500000)
	writeMemoryFixture(t, path, "wide-edit", []byte(body))
	start := time.Now()
	got, err := service.Memory(MemoryEditInput{Slug: "wide-edit", Edits: []MemoryReplacement{{OldText: body, NewText: strings.Repeat("b\n", 500000)}}})
	elapsed := time.Since(start)
	if err != nil || got.Condition != MemoryEdited || !got.Diff.Truncated || len(got.Diff.Text) > maxMemoryDiffBytes {
		t.Fatalf("wide edit=%#v err=%v", got, err)
	}
	// The generated Pi client abandons a memory invocation after 15 seconds.
	if elapsed > 5*time.Second {
		t.Fatalf("bounded display rows took %s for %d lines", elapsed, 500000)
	}
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryPreviewDoesNotPublishOrClock)
func TestMemoryPreviewDoesNotPublishOrClock(t *testing.T) {
	root := initEffortRepo(t)
	calls := 0
	service := openTestService(t, root, func(deps *Dependencies) { deps.Clock = func() time.Time { calls++; return time.Now().UTC() } })
	if _, err := service.New(testContext(t), NewInput{Slug: "preview", Title: "Preview"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("preview")
	writeMemoryFixture(t, path, "preview", []byte("old body\n"))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edit, err := service.Memory(MemoryEditInput{Slug: "preview", Edits: []MemoryReplacement{{OldText: "old", NewText: "new"}}, Preview: true})
	if err != nil || edit.Condition != MemoryPreviewed || edit.Memory != nil || edit.ReplacementCount != 1 || edit.Diff == nil || edit.Diff.Text == "" {
		t.Fatalf("edit preview=%#v err=%v", edit, err)
	}
	phase, next := "next phase", "next action"
	update, err := service.Memory(MemoryUpdateInput{Slug: "preview", Update: MemoryUpdate{Phase: &phase, Next: &next}, Preview: true})
	if err != nil || update.Condition != MemoryPreviewed || update.Memory != nil || update.ReplacementCount != 0 || update.Diff == nil || !strings.Contains(update.Diff.Text, "phase:") || strings.Contains(update.Diff.Text, "+5 updated:") || strings.Contains(update.Diff.Text, "-5 updated:") {
		t.Fatalf("update preview=%#v err=%v", update, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || calls != 1 {
		t.Fatalf("preview published=%t clock calls=%d", !bytes.Equal(before, after), calls)
	}
	// previewUpdateOf runs one update preview against exact resident bytes
	// through the entrypoint Pi uses, so the preview under test is the shipped
	// one and its read-only promise is checked on every case.
	previewUpdateOf := func(raw []byte, update MemoryUpdate) MemoryDiff {
		t.Helper()
		if writeErr := os.WriteFile(path, raw, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		result, previewErr := service.Memory(MemoryUpdateInput{Slug: "preview", Update: update, Preview: true})
		if previewErr != nil || result.Condition != MemoryPreviewed || result.Diff == nil {
			t.Fatalf("preview=%#v err=%v", result, previewErr)
		}
		unchanged, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(raw, unchanged) {
			t.Fatalf("preview published bytes err=%v", readErr)
		}
		return *result.Diff
	}

	currentPhase := beforePhase(before)
	noChange := previewUpdateOf(before, MemoryUpdate{Phase: &currentPhase})
	if noChange.Text != "" || noChange.FirstChangedLine != nil {
		t.Fatalf("no change diff=%#v", noChange)
	}
	canonicalWithMetadataLikeBody := []byte("---\neffort: preview\nphase: old phase\nnext: old next\nupdated: \"2026-08-06T12:00:00Z\"\n---\nphase: body phase\nnext: body next\n")
	bodySafe := previewUpdateOf(canonicalWithMetadataLikeBody, MemoryUpdate{Phase: &phase, Next: &next})
	if !strings.Contains(bodySafe.Text, "+3 phase: next phase") || !strings.Contains(bodySafe.Text, "+4 next: next action") || strings.Contains(bodySafe.Text, "+7 phase: next phase") || strings.Contains(bodySafe.Text, "+8 next: next action") {
		t.Fatalf("preview rewrote body metadata lookalikes: %#v", bodySafe)
	}

	// inspectCanonical constrains duplicates and unknown keys but never key
	// order, and safe repair inserts an absent key, so a preview located by
	// fixed line index silently shows nothing for either shape.
	reordered := []byte("---\neffort: preview\nupdated: \"2026-08-06T12:00:00Z\"\nphase: old phase\nnext: old next\n---\nbody\n")
	shape := previewUpdateOf(reordered, MemoryUpdate{Phase: &phase, Next: &next})
	if !strings.Contains(shape.Text, "+3 phase: next phase") || !strings.Contains(shape.Text, "+4 next: next action") || strings.Contains(changedDiffRows(shape.Text), "updated:") {
		t.Fatalf("reordered-key preview=%#v", shape)
	}
	absentPhase := []byte("---\neffort: preview\nnext: old next\nupdated: \"2026-08-06T12:00:00Z\"\n---\nbody\n")
	shape = previewUpdateOf(absentPhase, MemoryUpdate{Phase: &phase})
	if !strings.Contains(shape.Text, "+3 phase: next phase") || strings.Contains(changedDiffRows(shape.Text), "updated:") {
		t.Fatalf("absent-phase repair preview=%#v", shape)
	}
	// Inspection carries no value for an updated key it cannot read, so a preview
	// encoded from the inspected value writes the empty string over whatever the
	// resident holds. Safe repair still previews these residents, and none of
	// them may show an updated row: the key is the preview's one omission.
	for _, resident := range []string{
		"---\neffort: preview\nphase: old phase\nnext: old next\n---\nbody\n",
		"---\neffort: preview\nphase: old phase\nnext: old next\nupdated: 2026-08-06T12:00:00Z\n---\nbody\n",
		"---\neffort: preview\nphase: old phase\nnext: old next\nupdated: 12345\n---\nbody\n",
	} {
		shape = previewUpdateOf([]byte(resident), MemoryUpdate{Phase: &phase})
		if !strings.Contains(shape.Text, "+3 phase: next phase") || strings.Contains(changedDiffRows(shape.Text), "updated:") {
			t.Fatalf("unreadable-updated preview of %q = %#v", resident, shape)
		}
	}

	// A previewed metadata line must carry the quoting publication applies, or
	// the reader is shown a line the resident will never contain.
	quoted := "weird: value"
	shape = previewUpdateOf(before, MemoryUpdate{Phase: &quoted})
	published, err := service.Memory(MemoryUpdateInput{Slug: "preview", Update: MemoryUpdate{Phase: &quoted}})
	if err != nil || published.Condition != MemoryUpdated {
		t.Fatalf("quoted publication=%#v err=%v", published, err)
	}
	quotedRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shape.Text, "+3 phase: 'weird: value'") || !strings.Contains(string(quotedRaw), "phase: 'weird: value'\n") {
		t.Fatalf("quoted preview=%q published=%q", shape.Text, quotedRaw)
	}

	legacyRaw := []byte("Effort: preview\nPhase: old phase\nNext: old next\nUpdated: Not yet updated.\n\nold body")
	legacy := previewUpdateOf(legacyRaw, MemoryUpdate{Next: &next})
	if legacy.FirstChangedLine == nil || *legacy.FirstChangedLine != 3 || !strings.Contains(legacy.Text, "-3 Next: old next") || !strings.Contains(legacy.Text, "+3 Next: next action") || strings.Contains(legacy.Text, "+4 Updated:") || strings.Contains(legacy.Text, "-4 Updated:") {
		t.Fatalf("legacy preview=%#v", legacy)
	}
	legacyPhase := previewUpdateOf(legacyRaw, MemoryUpdate{Phase: &phase})
	if !strings.Contains(legacyPhase.Text, "-2 Phase: old phase") || !strings.Contains(legacyPhase.Text, "+2 Phase: next phase") || strings.Contains(changedDiffRows(legacyPhase.Text), "Next:") {
		t.Fatalf("legacy phase preview=%#v", legacyPhase)
	}
	if err := os.WriteFile(path, legacyRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	legacyEdit, err := service.Memory(MemoryEditInput{Slug: "preview", Edits: []MemoryReplacement{{OldText: "old", NewText: "new"}}, Preview: true})
	if err != nil || legacyEdit.Diff == nil || legacyEdit.Diff.FirstChangedLine == nil || *legacyEdit.Diff.FirstChangedLine != 6 {
		t.Fatalf("legacy edit preview=%#v err=%v", legacyEdit, err)
	}
	publishedEdit, err := service.Memory(MemoryEditInput{Slug: "preview", Edits: []MemoryReplacement{{OldText: "old", NewText: "new"}}})
	if err != nil || publishedEdit.Diff == nil || publishedEdit.Diff.FirstChangedLine == nil || *publishedEdit.Diff.FirstChangedLine != 7 {
		t.Fatalf("canonical edit result=%#v err=%v", publishedEdit, err)
	}

	largeBody := []byte("OLD" + strings.Repeat("x", maxMemoryBytes-200))
	writeMemoryFixture(t, path, "preview", largeBody)
	largeReplacement := strings.Repeat("y", 1000)
	largePreview, err := service.Memory(MemoryEditInput{Slug: "preview", Edits: []MemoryReplacement{{OldText: "OLD", NewText: largeReplacement}}, Preview: true})
	if err != nil || largePreview.Condition != MemoryPreviewed {
		t.Fatalf("oversized result preview=%#v err=%v", largePreview, err)
	}
	largeResult, err := service.Memory(MemoryEditInput{Slug: "preview", Edits: []MemoryReplacement{{OldText: "OLD", NewText: largeReplacement}}})
	if err != nil || largeResult.Condition != MemoryResultTooLarge {
		t.Fatalf("oversized normal result=%#v err=%v", largeResult, err)
	}

	invalidRaw := []byte("---\neffort: preview\nphase: \"\"\nnext: \"\"\nupdated: \"2026-08-06T12:00:00Z\"\n---\nbody")
	if err := os.WriteFile(path, invalidRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := service.Memory(MemoryUpdateInput{Slug: "preview", Update: MemoryUpdate{Phase: &phase}, Preview: true})
	if err != nil || got.Condition != MemoryInvalid {
		t.Fatalf("unsafe repair preview=%#v err=%v", got, err)
	}
	beforeRepair, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	clockBeforeRepair := calls
	repairPreview, err := service.Memory(MemoryUpdateInput{Slug: "preview", Update: MemoryUpdate{Phase: &phase, Next: &next}, Preview: true})
	afterRepairPreview, readErr := os.ReadFile(path)
	if err != nil || readErr != nil || repairPreview.Condition != MemoryPreviewed || !bytes.Equal(beforeRepair, afterRepairPreview) || calls != clockBeforeRepair || !strings.Contains(repairPreview.Diff.Text, "+3 phase: next phase") || !strings.Contains(repairPreview.Diff.Text, "+4 next: next action") {
		t.Fatalf("safe repair preview=%#v err=%v readErr=%v published=%t clocks=%d", repairPreview, err, readErr, !bytes.Equal(beforeRepair, afterRepairPreview), calls-clockBeforeRepair)
	}
	repaired, err := service.Memory(MemoryUpdateInput{Slug: "preview", Update: MemoryUpdate{Phase: &phase, Next: &next}})
	if err != nil || repaired.Condition != MemoryUpdated || !strings.Contains(repaired.Diff.Text, "+3 phase: next phase") || !strings.Contains(repaired.Diff.Text, "+4 next: next action") || !strings.Contains(repaired.Diff.Text, "updated:") {
		t.Fatalf("safe repair result=%#v diff=%q err=%v", repaired, repaired.Diff.Text, err)
	}
}

// changedDiffRows keeps only the added and removed rows, so an assertion about
// a rewritten line cannot be satisfied by an unchanged context row.
func changedDiffRows(text string) string {
	var rows []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			rows = append(rows, line)
		}
	}
	return strings.Join(rows, "\n")
}

func beforePhase(raw []byte) string {
	for _, rawLine := range diffLines(raw) {
		line := displayLine(rawLine)
		if strings.HasPrefix(line, "phase: ") {
			return strings.TrimPrefix(line, "phase: ")
		}
	}
	return ""
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryOperationsReadEditAndUpdateCanonicalResident)
func TestMemoryOperationsReadEditAndUpdateCanonicalResident(t *testing.T) {
	root := initEffortRepo(t)
	now := time.Date(2026, 8, 5, 14, 15, 16, 123456789, time.UTC)
	clockCalls := 0
	service := openTestService(t, root, func(deps *Dependencies) {
		deps.Clock = func() time.Time {
			clockCalls++
			return now.Add(time.Duration(clockCalls-1) * time.Second)
		}
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "memory-ops", Title: "Memory operations"}); err != nil {
		t.Fatal(err)
	}
	clockCalls = 0
	path := service.paths.memoryFile("memory-ops")
	body := []byte("alpha one\r\nbeta two\r\nunrelated bytes")
	writeMemoryFixture(t, path, "memory-ops", body)

	read, err := service.Memory(MemoryReadInput{Slug: "memory-ops", Offset: 7, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if read.Condition != MemoryRead || read.Content != "alpha one\r\nbeta two\r\n" || read.Range.StartLine != 7 || read.Range.EndLine != 8 || read.Range.TotalLines != 9 || read.Range.NextOffset == nil || *read.Range.NextOffset != 9 || read.Range.TruncatedBy != "limit" {
		t.Fatalf("read = %#v", read)
	}
	if read.Memory.Effort != "memory-ops" || read.Memory.Phase != "phase stays" || read.Memory.Next != "next stays" {
		t.Fatalf("read memory = %#v", read.Memory)
	}

	edited, err := service.Memory(MemoryEditInput{Slug: "memory-ops", Edits: []MemoryReplacement{
		{OldText: "beta two", NewText: "gamma two"},
		{OldText: "alpha", NewText: "beta"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Condition != MemoryEdited || edited.ReplacementCount != 2 || edited.Memory.Updated != now.Format(time.RFC3339Nano) || edited.Diff.FirstChangedLine == nil || *edited.Diff.FirstChangedLine != 7 || edited.Diff.Truncated {
		t.Fatalf("edited = %#v", edited)
	}
	if clockCalls != 1 {
		t.Fatalf("edit clock calls = %d", clockCalls)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	metadata, gotBody, err := readMemoryMetadata(raw, "memory-ops")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Phase != "phase stays" || metadata.Next != "next stays" || string(gotBody) != "beta one\r\ngamma two\r\nunrelated bytes" {
		t.Fatalf("metadata=%#v body=%q", metadata, gotBody)
	}

	phase, next := "repaired phase", "repaired next"
	updateTime := now.Add(time.Second)
	updated, err := service.Memory(MemoryUpdateInput{Slug: "memory-ops", Update: MemoryUpdate{Phase: &phase, Next: &next}})
	if err != nil || updated.Condition != MemoryUpdated || updated.Memory.Phase != phase || updated.Memory.Next != next || updated.Memory.Updated != updateTime.Format(time.RFC3339Nano) || updated.Diff == nil || !strings.Contains(updated.Diff.Text, "-5 updated:") || !strings.Contains(updated.Diff.Text, "+5 updated:") || !strings.Contains(updated.Diff.Text, updateTime.Format(time.RFC3339Nano)) {
		t.Fatalf("updated=%#v diff=%q err=%v", updated, updated.Diff.Text, err)
	}
	if clockCalls != 2 {
		t.Fatalf("total clock calls = %d", clockCalls)
	}
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryReadPaginationBoundariesAndLegacyInput)
func TestMemoryReadPaginationBoundariesAndLegacyInput(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "pagination", Title: "Pagination"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("pagination")

	legacy := []byte("Effort: pagination\nPhase: legacy phase\nNext: legacy next\nUpdated: Not yet updated.\n\nlast line has no newline")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := service.Memory(MemoryReadInput{Slug: "pagination"})
	if err != nil || got.Condition != MemoryRead || got.Content != string(legacy) || got.Range.TotalLines != 6 || got.Range.EndLine != 6 || got.Range.NextOffset != nil || got.Range.TruncatedBy != "none" || got.Memory.Updated != notYetUpdated {
		t.Fatalf("legacy read=%#v err=%v", got, err)
	}

	lines := strings.Repeat("x\n", maxMemoryReadLines+1)
	writeMemoryFixture(t, path, "pagination", []byte(lines))
	got, err = service.Memory(MemoryReadInput{Slug: "pagination", Offset: 7})
	if err != nil || got.Range.TruncatedBy != "lines" || got.Range.EndLine != 2006 || got.Range.NextOffset == nil || *got.Range.NextOffset != 2007 {
		t.Fatalf("line truncation=%#v err=%v", got, err)
	}

	complete := strings.Repeat("a", 30000) + "\n" + strings.Repeat("b", 22000) + "\nsecond\n"
	writeMemoryFixture(t, path, "pagination", []byte(complete))
	got, err = service.Memory(MemoryReadInput{Slug: "pagination", Offset: 7})
	if err != nil || got.Condition != MemoryRead || got.Range.TruncatedBy != "bytes" || len(got.Content) != 30001 || got.Range.EndLine != 7 || got.Range.NextOffset == nil || *got.Range.NextOffset != 8 {
		t.Fatalf("complete-line byte truncation=%#v bytes=%d err=%v", got, len(got.Content), err)
	}

	largeLine := strings.Repeat("é", 25601) + "\nsecond\n"
	writeMemoryFixture(t, path, "pagination", []byte(largeLine))
	got, err = service.Memory(MemoryReadInput{Slug: "pagination", Offset: 7})
	if err != nil || got.Condition != MemoryResultTooLarge || got.Size == nil || got.Size.Bytes != 51203 || got.Size.MaxBytes != 51200 {
		t.Fatalf("unpageable line=%#v err=%v", got, err)
	}

	got, err = service.Memory(MemoryReadInput{Slug: "pagination", Offset: 7, Limit: math.MaxInt})
	if err != nil || got.Condition != MemoryResultTooLarge {
		t.Fatalf("max-int limit=%#v err=%v", got, err)
	}

	total := 8
	got, err = service.Memory(MemoryReadInput{Slug: "pagination", Offset: total + 1})
	if err != nil || got.Condition != MemoryOffsetOutOfRange || got.Offset.Offset != total+1 || got.Offset.TotalLines != total || got.Outcome == nil || got.Outcome.Category != "operation" || got.Outcome.ChangedMemory {
		t.Fatalf("offset refusal=%#v err=%v", got, err)
	}
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryEditOriginalBodyMatchingAndAtomicRefusals)
func TestMemoryEditOriginalBodyMatchingAndAtomicRefusals(t *testing.T) {
	root := initEffortRepo(t)
	now := time.Date(2026, 8, 6, 7, 8, 9, 0, time.UTC)
	clockCalls := 0
	service := openTestService(t, root, func(deps *Dependencies) {
		deps.Clock = func() time.Time { clockCalls++; return now }
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "exact-edits", Title: "Exact edits"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("exact-edits")

	cases := []struct {
		name      string
		body      string
		edits     []MemoryReplacement
		condition MemoryCondition
		index     int
		count     int
		pair      [2]int
	}{
		{name: "no-match", body: "one", edits: []MemoryReplacement{{OldText: "absent", NewText: "x"}}, condition: MemoryNoMatch},
		{name: "ambiguous-overlapping-occurrences", body: "aaa", edits: []MemoryReplacement{{OldText: "aa", NewText: "x"}}, condition: MemoryAmbiguousMatch, count: 2},
		{name: "nested", body: "abcdef", edits: []MemoryReplacement{{OldText: "bcde", NewText: "x"}, {OldText: "cd", NewText: "y"}}, condition: MemoryOverlappingEdits, pair: [2]int{0, 1}},
		{name: "stable-original-indexes", body: "abcdef", edits: []MemoryReplacement{{OldText: "cde", NewText: "x"}, {OldText: "abc", NewText: "y"}}, condition: MemoryOverlappingEdits, pair: [2]int{0, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeMemoryFixture(t, path, "exact-edits", []byte(tc.body))
			before, _ := os.ReadFile(path)
			got, err := service.Memory(MemoryEditInput{Slug: "exact-edits", Edits: tc.edits})
			if err != nil || got.Condition != tc.condition || got.Outcome == nil || got.Outcome.ChangedMemory {
				t.Fatalf("result=%#v err=%v", got, err)
			}
			if tc.condition == MemoryNoMatch || tc.condition == MemoryAmbiguousMatch {
				if got.Edit.Index != tc.index || got.Edit.Occurrences != tc.count {
					t.Fatalf("edit fact=%#v", got.Edit)
				}
			}
			if tc.condition == MemoryOverlappingEdits && (got.Overlap.FirstIndex != tc.pair[0] || got.Overlap.SecondIndex != tc.pair[1]) {
				t.Fatalf("overlap=%#v", got.Overlap)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("refused batch changed memory")
			}
		})
	}

	writeMemoryFixture(t, path, "exact-edits", []byte("first old\nsecond old\n"))
	got, err := service.Memory(MemoryEditInput{Slug: "exact-edits", Edits: []MemoryReplacement{
		{OldText: "second old", NewText: "created old first"},
		{OldText: "first old", NewText: "changed"},
	}})
	if err != nil || got.Condition != MemoryEdited {
		t.Fatalf("original evaluation=%#v err=%v", got, err)
	}

	writeMemoryFixture(t, path, "exact-edits", []byte("unchanged"))
	clockCalls = 0
	got, err = service.Memory(MemoryEditInput{Slug: "exact-edits", Edits: []MemoryReplacement{{OldText: "unchanged", NewText: "unchanged"}}})
	if err != nil || got.Condition != MemoryEdited || got.Diff.FirstChangedLine != nil || got.Diff.Text != "" || got.Memory.Updated != now.Format(time.RFC3339Nano) || clockCalls != 1 {
		t.Fatalf("no-op=%#v clock calls=%d err=%v", got, clockCalls, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted, _, err := readMemoryMetadata(raw, "exact-edits")
	if err != nil || persisted.Updated != now.Format(time.RFC3339Nano) {
		t.Fatalf("no-op persisted metadata=%#v err=%v", persisted, err)
	}
}

// invariant: tooling/effort-management:effort-record-authority (TestMemoryOperationOwnerInspectionDoesNotRequireValidMutableMetadata)
func TestMemoryOperationOwnerInspectionDoesNotRequireValidMutableMetadata(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "owner-check", Title: "Owner check"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("owner-check")

	absentResident, err := service.Memory(MemoryReadInput{Slug: "absent-owner-check", Owner: testIDA})
	if err != nil || absentResident.Condition != MemoryMissing {
		t.Fatalf("absent effort resident=%#v err=%v", absentResident, err)
	}
	missing, err := service.Memory(MemoryReadInput{Slug: "owner-check", Owner: testIDA})
	if err != nil || missing.Condition != MemoryMissing {
		t.Fatalf("missing activity=%#v err=%v", missing, err)
	}
	if attached := service.AttachActivity("owner-check", testIDA); attached.Condition != ActivityAttached {
		t.Fatalf("attach=%#v", attached)
	}
	if err := os.WriteFile(path, []byte("---\neffort: owner-check\nphase: \"\"\nnext: \"\"\nupdated: bad\n---\nbody"), 0o600); err != nil {
		t.Fatal(err)
	}
	phase, next := "fixed", "fixed next"
	repaired, err := service.Memory(MemoryUpdateInput{Slug: "owner-check", Owner: testIDA, Update: MemoryUpdate{Phase: &phase, Next: &next}})
	if err != nil || repaired.Condition != MemoryUpdated {
		t.Fatalf("owner repair=%#v err=%v", repaired, err)
	}

	wrong, err := service.Memory(MemoryReadInput{Slug: "owner-check", Owner: testIDB})
	if err != nil || wrong.Condition != MemoryNotOwner {
		t.Fatalf("wrong owner=%#v err=%v", wrong, err)
	}
	activity := service.paths.activityFile("owner-check")
	if err := os.WriteFile(activity, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsafe, err := service.Memory(MemoryReadInput{Slug: "owner-check", Owner: testIDA})
	if err != nil || unsafe.Condition != MemoryUnsafeActivity {
		t.Fatalf("unsafe activity=%#v err=%v", unsafe, err)
	}

	// A staged activity-shaped resident cannot be inspected ahead of effort
	// resident validation: both absent state and an unsafe extra leaf win.
	state := service.paths.stateFile("owner-check")
	if err := os.Remove(state); err != nil {
		t.Fatal(err)
	}
	unsafe, err = service.Memory(MemoryReadInput{Slug: "owner-check", Owner: testIDA})
	if err != nil || unsafe.Condition != MemoryMissing {
		t.Fatalf("absent state before unsafe activity=%#v err=%v", unsafe, err)
	}
	if err := os.WriteFile(state, []byte(`{"schemaVersion":2,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"owner-check","title":"Owner check","createdAt":"2026-08-05T14:15:16.123456789Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(filepath.Dir(path), "foreign")
	if err := os.WriteFile(foreign, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsafe, err = service.Memory(MemoryReadInput{Slug: "owner-check", Owner: testIDA})
	if err != nil || unsafe.Condition != MemoryUnsafe {
		t.Fatalf("unsafe effort resident before unsafe activity=%#v err=%v", unsafe, err)
	}
}

func TestMemoryOperationResidentSafetyAndInvalidMemoryConditions(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	missing, err := service.Memory(MemoryReadInput{Slug: "absent"})
	if err != nil || missing.Condition != MemoryMissing {
		t.Fatalf("missing effort=%#v err=%v", missing, err)
	}
	if _, err := service.New(testContext(t), NewInput{Slug: "resident-cases", Title: "Resident cases"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("resident-cases")
	if err := os.WriteFile(path, []byte("malformed"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid, err := service.Memory(MemoryReadInput{Slug: "resident-cases"})
	if err != nil || invalid.Condition != MemoryInvalid {
		t.Fatalf("invalid memory=%#v err=%v", invalid, err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	unsafe, err := service.Memory(MemoryReadInput{Slug: "resident-cases"})
	if err != nil || unsafe.Condition != MemoryUnsafe {
		t.Fatalf("unsafe memory=%#v err=%v", unsafe, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	writeMemoryFixture(t, path, "resident-cases", []byte("body"))
	foreign := filepath.Join(filepath.Dir(path), "foreign")
	if err := os.WriteFile(foreign, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsafe, err = service.Memory(MemoryReadInput{Slug: "resident-cases"})
	if err != nil || unsafe.Condition != MemoryUnsafe {
		t.Fatalf("unsafe effort=%#v err=%v", unsafe, err)
	}
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryEditResultBoundsAndDiffBounds)
func TestMemoryEditResultBoundsAndDiffBounds(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "bounded-edit", Title: "Bounded edit"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("bounded-edit")
	writeMemoryFixture(t, path, "bounded-edit", []byte("old"))
	before, _ := os.ReadFile(path)
	tooLarge := strings.Repeat("x", 1048576)
	got, err := service.Memory(MemoryEditInput{Slug: "bounded-edit", Edits: []MemoryReplacement{{OldText: "old", NewText: tooLarge}}})
	if err != nil || got.Condition != MemoryResultTooLarge || got.Size.Bytes <= got.Size.MaxBytes || got.Size.MaxBytes != 1048576 {
		t.Fatalf("large result=%#v err=%v", got, err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("oversized result changed memory")
	}

	largeBody := strings.Repeat("a", 25600) + "OLD" + strings.Repeat("b", 25600)
	writeMemoryFixture(t, path, "bounded-edit", []byte(largeBody))
	got, err = service.Memory(MemoryEditInput{Slug: "bounded-edit", Edits: []MemoryReplacement{{OldText: "OLD", NewText: "NEW"}}})
	if err != nil || got.Condition != MemoryEdited || !got.Diff.Truncated || len(got.Diff.Text) > 51200 {
		t.Fatalf("bounded diff=%#v err=%v", got, err)
	}

	// One removed and one added row, each a kind byte, a one-digit line number,
	// a space, its content, and a terminator. These literal fixture sizes make
	// the complete diff exactly 50 KiB, then one byte over it, without relying
	// on the production bound.
	exactDiffBody := strings.Repeat("a", 25593) + "OLD"
	writeMemoryFixture(t, path, "bounded-edit", []byte(exactDiffBody))
	got, err = service.Memory(MemoryEditInput{Slug: "bounded-edit", Edits: []MemoryReplacement{{OldText: "OLD", NewText: "NEW"}}})
	if err != nil || got.Condition != MemoryEdited || got.Diff.Truncated || len(got.Diff.Text) != 51200 {
		t.Fatalf("exactly bounded contextual diff=%#v bytes=%d err=%v", got, len(got.Diff.Text), err)
	}

	// One byte over, so the added row is dropped for an omission row and the
	// result reports the bounding loss.
	overDiffBody := strings.Repeat("b", 25595) + "X"
	writeMemoryFixture(t, path, "bounded-edit", []byte(overDiffBody))
	got, err = service.Memory(MemoryEditInput{Slug: "bounded-edit", Edits: []MemoryReplacement{{OldText: "X", NewText: "XY"}}})
	if err != nil || got.Condition != MemoryEdited || !got.Diff.Truncated || len(got.Diff.Text) > 51200 || !strings.HasPrefix(got.Diff.Text, "-7 b") || !strings.HasSuffix(got.Diff.Text, "\n   ...\n") {
		t.Fatalf("over-bound contextual diff truncated=%t bytes=%d err=%v", got.Diff.Truncated, len(got.Diff.Text), err)
	}
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryPublicationFailuresReportUncertaintyWithoutRetry)
func TestMemoryPublicationFailuresReportUncertaintyWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		stage   string
		changed bool
	}{
		{stage: "memory-update.write"},
		{stage: "memory-update.fsync"},
		{stage: "memory-update.rename"},
		{stage: "memory-update.directory-fsync", changed: true},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openTestService(t, root, func(deps *Dependencies) {
				deps.Fault = func(stage string) error {
					if stage == tc.stage {
						if tc.changed {
							return errors.New(strings.Repeat("é", maxMemoryDiffBytes) + string([]byte{0xff}))
						}
						return errors.New("publication fault")
					}
					return nil
				}
			})
			if _, err := service.New(testContext(t), NewInput{Slug: "publication", Title: "Publication"}); err != nil {
				t.Fatal(err)
			}
			path := service.paths.memoryFile("publication")
			writeMemoryFixture(t, path, "publication", []byte("old"))
			before, _ := os.ReadFile(path)
			got, err := service.Memory(MemoryEditInput{Slug: "publication", Edits: []MemoryReplacement{{OldText: "old", NewText: "new"}}})
			if err != nil || got.Condition != MemoryFailure || got.Outcome.ChangedMemory != tc.changed || got.Outcome.Cause == "" || len(got.Outcome.Cause) > maxMemoryDiffBytes || !utf8.ValidString(got.Outcome.Cause) || len(got.Outcome.NextActions) == 0 || !strings.HasPrefix(got.Outcome.NextActions[0].Text, "read the memory") {
				t.Fatalf("failure=%#v err=%v", got, err)
			}
			after, _ := os.ReadFile(path)
			if bytes.Equal(before, after) != !tc.changed {
				t.Fatalf("changed bytes=%t want=%t", !bytes.Equal(before, after), tc.changed)
			}
		})
	}
}

func TestMemoryOperationPerEntrypointRefusalAndUpdateBoundaries(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	phase := "phase"
	if _, err := service.Memory(MemoryEditInput{Slug: "bad_slug", Edits: []MemoryReplacement{{OldText: "x", NewText: "y"}}}); err == nil {
		t.Fatal("edit accepted invalid slug")
	}
	if got, err := service.Memory(MemoryEditInput{Slug: "absent", Edits: []MemoryReplacement{{OldText: "x", NewText: "y"}}}); err != nil || got.Condition != MemoryMissing {
		t.Fatalf("edit missing=%#v err=%v", got, err)
	}
	if _, err := service.Memory(MemoryUpdateInput{Slug: "bad_slug", Update: MemoryUpdate{Phase: &phase}}); err == nil {
		t.Fatal("update accepted invalid slug")
	}
	if got, err := service.Memory(MemoryUpdateInput{Slug: "absent", Update: MemoryUpdate{Phase: &phase}}); err != nil || got.Condition != MemoryMissing {
		t.Fatalf("update missing=%#v err=%v", got, err)
	}
	if _, err := service.New(testContext(t), NewInput{Slug: "entrypoints", Title: "Entrypoints"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("entrypoints")
	if err := os.WriteFile(path, []byte("malformed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Memory(MemoryEditInput{Slug: "entrypoints", Edits: []MemoryReplacement{{OldText: "x", NewText: "y"}}}); err != nil || got.Condition != MemoryInvalid {
		t.Fatalf("edit invalid=%#v err=%v", got, err)
	}
	writeMemoryFixture(t, path, "entrypoints", []byte("abcdef"))
	if got, err := service.Memory(MemoryEditInput{Slug: "entrypoints", Edits: []MemoryReplacement{{OldText: "abc", NewText: "x"}, {OldText: "ab", NewText: "y"}}}); err != nil || got.Condition != MemoryOverlappingEdits {
		t.Fatalf("same-start overlap=%#v err=%v", got, err)
	}

	if err := os.WriteFile(path, []byte("---\neffort: entrypoints\nphase: \"\"\nnext: valid\nupdated: bad\n---\nbody"), 0o600); err != nil {
		t.Fatal(err)
	}
	next := "replacement next"
	if got, err := service.Memory(MemoryUpdateInput{Slug: "entrypoints", Update: MemoryUpdate{Next: &next}}); err != nil || got.Condition != MemoryInvalid {
		t.Fatalf("partial repair=%#v err=%v", got, err)
	}

	metadata := MemoryMetadata{Effort: "entrypoints", Phase: "p", Next: "n", Updated: "2026-08-05T12:00:00Z"}
	header, err := encodeMemory(metadata, nil)
	if err != nil {
		t.Fatal(err)
	}
	exact := append([]byte{}, header...)
	exact = append(exact, bytes.Repeat([]byte("x"), maxMemoryBytes-len(header))...)
	if err := os.WriteFile(path, exact, 0o600); err != nil {
		t.Fatal(err)
	}
	longPhase := strings.Repeat("p", 500)
	if got, err := service.Memory(MemoryUpdateInput{Slug: "entrypoints", Update: MemoryUpdate{Phase: &longPhase}}); err != nil || got.Condition != MemoryResultTooLarge || got.Size.Bytes <= maxMemoryBytes {
		t.Fatalf("oversized update=%#v err=%v", got, err)
	}
	if typed, err := updateMemoryForTest(service, "entrypoints", MemoryUpdate{Phase: &longPhase}); err != nil || typed.Condition != MemoryResultTooLarge || typed.Size == nil {
		t.Fatalf("typed oversized update=%#v err=%v", typed, err)
	}

	faultService := openTestService(t, root, func(deps *Dependencies) {
		deps.Fault = func(stage string) error {
			if stage == "memory-update.write" {
				return errors.New("update fault")
			}
			return nil
		}
	})
	writeMemoryFixture(t, path, "entrypoints", []byte("body"))
	if got, err := faultService.Memory(MemoryUpdateInput{Slug: "entrypoints", Update: MemoryUpdate{Phase: &phase}}); err != nil || got.Condition != MemoryFailure {
		t.Fatalf("update failure=%#v err=%v", got, err)
	}
	cause := errors.New("cause")
	if !errors.Is(&memoryPublicationError{Changed: true, Err: cause}, cause) {
		t.Fatal("publication error lost cause")
	}
}

func TestMemoryOperationInputValidation(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.Memory(nil); err == nil {
		t.Fatal("unsupported operation accepted")
	}
	if _, err := service.Memory(MemoryReadInput{Slug: "bad_slug"}); err == nil {
		t.Fatal("invalid slug accepted")
	}
	if _, err := service.Memory(MemoryReadInput{Slug: "valid", Owner: "NOT-A-UUID"}); err == nil {
		t.Fatal("invalid owner accepted")
	}
	if _, err := service.Memory(MemoryReadInput{Slug: "valid", Offset: -1}); err == nil {
		t.Fatal("negative offset accepted")
	}
	if _, err := service.Memory(MemoryReadInput{Slug: "valid", Limit: -1}); err == nil {
		t.Fatal("negative limit accepted")
	}
	for _, edits := range [][]MemoryReplacement{
		nil,
		make([]MemoryReplacement, 129),
		{{OldText: "", NewText: "x"}},
		{{OldText: "x", NewText: string([]byte{0xff})}},
		{{OldText: strings.Repeat("x", 1048577)}},
		{{OldText: "x", NewText: strings.Repeat("x", 1048577)}},
	} {
		if _, err := service.Memory(MemoryEditInput{Slug: "valid", Edits: edits}); err == nil {
			t.Fatalf("invalid edits accepted: count=%d", len(edits))
		}
	}
	if _, err := service.Memory(MemoryUpdateInput{Slug: "valid"}); err == nil {
		t.Fatal("empty update accepted")
	}
	blank := " "
	if _, err := service.Memory(MemoryUpdateInput{Slug: "valid", Update: MemoryUpdate{Phase: &blank}}); err == nil {
		t.Fatal("invalid phase accepted")
	}
	if _, err := service.Memory(MemoryUpdateInput{Slug: "valid", Update: MemoryUpdate{Next: &blank}}); err == nil {
		t.Fatal("invalid next accepted")
	}
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryLiteralPaginationAndSequentialReconstruction)
func TestMemoryLiteralPaginationAndSequentialReconstruction(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "literal-pages", Title: "Literal pages"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("literal-pages")

	exactBytes := strings.Repeat("x", 51199) + "\n" + "next\n"
	writeMemoryFixture(t, path, "literal-pages", []byte(exactBytes))
	accepted, err := service.Memory(MemoryReadInput{Slug: "literal-pages", Offset: 7})
	if err != nil || accepted.Condition != MemoryRead || len(accepted.Content) != 51200 || accepted.Range.TruncatedBy != "bytes" || accepted.Range.NextOffset == nil || *accepted.Range.NextOffset != 8 {
		t.Fatalf("literal 50-KiB acceptance=%#v err=%v", accepted, err)
	}
	tooLarge := strings.Repeat("x", 51200) + "\n"
	writeMemoryFixture(t, path, "literal-pages", []byte(tooLarge))
	refused, err := service.Memory(MemoryReadInput{Slug: "literal-pages", Offset: 7})
	if err != nil || refused.Condition != MemoryResultTooLarge || refused.Size == nil || refused.Size.Bytes != 51201 || refused.Size.MaxBytes != 51200 {
		t.Fatalf("literal 50-KiB refusal=%#v err=%v", refused, err)
	}

	exactLines := strings.Repeat("z\n", 2000)
	writeMemoryFixture(t, path, "literal-pages", []byte(exactLines))
	accepted, err = service.Memory(MemoryReadInput{Slug: "literal-pages", Offset: 7})
	if err != nil || accepted.Condition != MemoryRead || accepted.Range.EndLine != 2006 || accepted.Range.NextOffset != nil || accepted.Range.TruncatedBy != "none" || strings.Count(accepted.Content, "\n") != 2000 {
		t.Fatalf("literal 2,000-line acceptance=%#v err=%v", accepted, err)
	}
	excessLines := exactLines + "last\n"
	writeMemoryFixture(t, path, "literal-pages", []byte(excessLines))
	accepted, err = service.Memory(MemoryReadInput{Slug: "literal-pages", Offset: 7})
	if err != nil || accepted.Range.TruncatedBy != "lines" || accepted.Range.NextOffset == nil || *accepted.Range.NextOffset != 2007 || strings.Count(accepted.Content, "\n") != 2000 {
		t.Fatalf("literal 2,001-line pagination=%#v err=%v", accepted, err)
	}

	var body strings.Builder
	for i := range 4500 {
		fmt.Fprintf(&body, "line-%04d payload\n", i)
	}
	writeMemoryFixture(t, path, "literal-pages", []byte(body.String()))
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var reconstructed strings.Builder
	offset := 1
	for {
		page, pageErr := service.Memory(MemoryReadInput{Slug: "literal-pages", Offset: offset})
		if pageErr != nil || page.Condition != MemoryRead {
			t.Fatalf("page offset=%d result=%#v err=%v", offset, page, pageErr)
		}
		reconstructed.WriteString(page.Content)
		if page.Range.NextOffset == nil {
			break
		}
		if *page.Range.NextOffset <= offset {
			t.Fatalf("continuation did not advance: %d -> %d", offset, *page.Range.NextOffset)
		}
		offset = *page.Range.NextOffset
	}
	if reconstructed.String() != string(want) {
		t.Fatal("sequential pages lost or duplicated document bytes")
	}
}

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryEditLiteralOneMiBAnd128Boundaries)
func TestMemoryEditLiteralOneMiBAnd128Boundaries(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "literal-edits", Title: "Literal edits"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("literal-edits")
	writeMemoryFixture(t, path, "literal-edits", []byte("x"))

	exact := strings.Repeat("n", 1048576)
	result, err := service.Memory(MemoryEditInput{Slug: "literal-edits", Edits: []MemoryReplacement{{OldText: "x", NewText: exact}}})
	if err != nil || result.Condition != MemoryResultTooLarge || result.Size == nil || result.Size.MaxBytes != 1048576 {
		t.Fatalf("literal one-MiB string acceptance reached result bound as %#v err=%v", result, err)
	}
	if _, err := service.Memory(MemoryEditInput{Slug: "literal-edits", Edits: []MemoryReplacement{{OldText: "x", NewText: strings.Repeat("n", 1048577)}}}); err == nil {
		t.Fatal("literal one-MiB-plus-one string was accepted")
	}

	body := make([]string, 128)
	edits := make([]MemoryReplacement, 128)
	for i := range body {
		body[i] = fmt.Sprintf("old-%03d", i)
		edits[i] = MemoryReplacement{OldText: body[i], NewText: fmt.Sprintf("new-%03d", i)}
	}
	writeMemoryFixture(t, path, "literal-edits", []byte(strings.Join(body, "\n")))
	result, err = service.Memory(MemoryEditInput{Slug: "literal-edits", Edits: edits})
	if err != nil || result.Condition != MemoryEdited || result.ReplacementCount != 128 {
		t.Fatalf("literal 128-edit acceptance=%#v err=%v", result, err)
	}
	if _, err := service.Memory(MemoryEditInput{Slug: "literal-edits", Edits: make([]MemoryReplacement, 129)}); err == nil {
		t.Fatal("literal 129-edit batch was accepted")
	}
}

func writeMemoryFixture(t *testing.T, path, slug string, body []byte) {
	t.Helper()
	metadata := MemoryMetadata{Effort: slug, Phase: "phase stays", Next: "next stays", Updated: "2026-08-05T12:00:00Z"}
	raw, err := encodeMemory(metadata, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
