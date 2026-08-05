package effort

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestMemoryOperationsReadEditAndUpdateCanonicalResident)
func TestMemoryOperationsReadEditAndUpdateCanonicalResident(t *testing.T) {
	root := initEffortRepo(t)
	now := time.Date(2026, 8, 5, 14, 15, 16, 123456789, time.UTC)
	clockCalls := 0
	service := openTestService(t, root, func(deps *Dependencies) {
		deps.Clock = func() time.Time { clockCalls++; return now }
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
	updated, err := service.Memory(MemoryUpdateInput{Slug: "memory-ops", Update: MemoryUpdate{Phase: &phase, Next: &next}})
	if err != nil || updated.Condition != MemoryUpdated || updated.Memory.Phase != phase || updated.Memory.Next != next || updated.Memory.Updated != now.Format(time.RFC3339Nano) {
		t.Fatalf("updated=%#v err=%v", updated, err)
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
	service := openTestService(t, root, nil)
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
	got, err = service.Memory(MemoryEditInput{Slug: "exact-edits", Edits: []MemoryReplacement{{OldText: "unchanged", NewText: "unchanged"}}})
	if err != nil || got.Condition != MemoryEdited || got.Diff.FirstChangedLine != nil || got.Diff.Text != "" {
		t.Fatalf("no-op=%#v err=%v", got, err)
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
	if err != nil || got.Condition != MemoryEdited || !got.Diff.Truncated || len(got.Diff.Text) != 51200 {
		t.Fatalf("bounded diff=%#v err=%v", got, err)
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
