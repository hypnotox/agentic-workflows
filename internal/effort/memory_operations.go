package effort

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

const (
	maxMemoryReadLines = 2000
	maxMemoryReadBytes = 50 << 10
	maxMemoryDiffBytes = 50 << 10
	maxMemoryEdits     = 128
)

// MemoryCondition identifies a successful memory operation or a handled refusal.
type MemoryCondition string

const (
	MemoryRead             MemoryCondition = "read"
	MemoryEdited           MemoryCondition = "edited"
	MemoryUpdated          MemoryCondition = "updated"
	MemoryNotOwner         MemoryCondition = "not-owner"
	MemoryMissing          MemoryCondition = "missing"
	MemoryUnsafeActivity   MemoryCondition = "unsafe-activity"
	MemoryInvalid          MemoryCondition = "invalid-memory"
	MemoryUnsafe           MemoryCondition = "unsafe-memory"
	MemoryOffsetOutOfRange MemoryCondition = "offset-out-of-range"
	MemoryNoMatch          MemoryCondition = "no-match"
	MemoryAmbiguousMatch   MemoryCondition = "ambiguous-match"
	MemoryOverlappingEdits MemoryCondition = "overlapping-edits"
	MemoryResultTooLarge   MemoryCondition = "result-too-large"
	MemoryFailure          MemoryCondition = "memory-failure"
)

// MemoryOperationInput is the closed input set accepted by Service.Memory.
type MemoryOperationInput interface{ memoryOperation() }

// MemoryReadInput selects one bounded, one-indexed range from a complete memory document.
type MemoryReadInput struct {
	Slug   string
	Owner  string
	Offset int
	Limit  int
}

// MemoryReplacement is one exact body-only replacement.
type MemoryReplacement struct {
	OldText string
	NewText string
}

// MemoryEditInput carries one atomic exact-replacement batch.
type MemoryEditInput struct {
	Slug  string
	Owner string
	Edits []MemoryReplacement
}

// MemoryUpdateInput carries a structured metadata update and optional advisory owner.
type MemoryUpdateInput struct {
	Slug   string
	Owner  string
	Update MemoryUpdate
}

func (MemoryReadInput) memoryOperation()   {}
func (MemoryEditInput) memoryOperation()   {}
func (MemoryUpdateInput) memoryOperation() {}

// MemoryRange reports the selected complete-document line range.
type MemoryRange struct {
	StartLine   int
	EndLine     int
	TotalLines  int
	NextOffset  *int
	TruncatedBy string
}

// MemoryDiff is a deterministic bounded body-change fact.
type MemoryDiff struct {
	Text             string
	FirstChangedLine *int
	Truncated        bool
}

// MemoryOutcome describes a handled refusal without presentation or transport policy.
type MemoryOutcome struct {
	Category      string
	Condition     string
	ChangedMemory bool
	NextActions   []RecoveryAction
	Cause         string
}

// MemoryEditFact identifies one failed edit and its optional occurrence count.
type MemoryEditFact struct {
	Index       int
	Occurrences int
}

// MemoryOverlapFact identifies the original request indexes of an overlapping pair.
type MemoryOverlapFact struct {
	FirstIndex  int
	SecondIndex int
}

// MemoryOffsetFact reports a syntactically valid offset beyond the document.
type MemoryOffsetFact struct {
	Offset     int
	TotalLines int
}

// MemorySizeFact reports a rejected complete encoded result size.
type MemorySizeFact struct {
	Bytes    int
	MaxBytes int
}

// MemoryOperationResult is the closed protocol-neutral result of a memory operation.
type MemoryOperationResult struct {
	Condition        MemoryCondition
	Memory           *MemoryMetadata
	Content          string
	Range            *MemoryRange
	Offset           *MemoryOffsetFact
	ReplacementCount int
	Diff             *MemoryDiff
	Outcome          *MemoryOutcome
	Edit             *MemoryEditFact
	Overlap          *MemoryOverlapFact
	Size             *MemorySizeFact
}

func memoryRefusal(condition MemoryCondition, observed string, actions ...string) MemoryOperationResult {
	next := make([]RecoveryAction, len(actions))
	for i, action := range actions {
		next[i] = RecoveryAction{Text: action}
	}
	return MemoryOperationResult{Condition: condition, Outcome: &MemoryOutcome{Category: "operation", Condition: observed, NextActions: next}}
}

func memoryFailure(err error) MemoryOperationResult {
	changed := memoryPublicationChanged(err)
	actions := []string{"read the memory to determine whether the change was published", "inspect the memory storage failure"}
	result := memoryRefusal(MemoryFailure, "memory publication cannot be confirmed", actions...)
	result.Outcome.ChangedMemory = changed
	result.Outcome.Cause = boundedMemoryText(err.Error(), maxMemoryDiffBytes)
	return result
}

// Memory performs one typed protocol-neutral memory operation.
func (s *Service) Memory(input MemoryOperationInput) (MemoryOperationResult, error) {
	switch value := input.(type) {
	case MemoryReadInput:
		return s.readMemory(value)
	case MemoryEditInput:
		return s.editMemory(value)
	case MemoryUpdateInput:
		return s.UpdateMemory(value)
	default:
		return MemoryOperationResult{}, errors.New("unsupported memory operation input")
	}
}

func (s *Service) readMemory(input MemoryReadInput) (MemoryOperationResult, error) {
	if input.Offset < 0 || input.Limit < 0 {
		return MemoryOperationResult{}, errors.New("memory read offset and limit must be positive when supplied")
	}
	if input.Offset == 0 {
		input.Offset = 1
	}
	raw, doc, refused, err := s.inspectMemoryOperation(input.Slug, input.Owner)
	if err != nil || refused != nil {
		if refused != nil {
			return *refused, nil
		}
		return MemoryOperationResult{}, err
	}
	if doc.err != nil {
		return memoryRefusal(MemoryInvalid, "the effort memory metadata is invalid", "inspect the effort memory", "repair the effort memory metadata"), nil //nolint:nilerr // malformed managed state is a typed handled refusal
	}
	lines := completeLines(raw)
	total := len(lines)
	if input.Offset > total {
		result := memoryRefusal(MemoryOffsetOutOfRange, "the requested offset is beyond the memory document", "read from an offset within the reported total lines")
		result.Offset = &MemoryOffsetFact{Offset: input.Offset, TotalLines: total}
		return result, nil
	}
	startIndex := input.Offset - 1
	available := total - startIndex
	selectedLines := available
	reason := "none"
	if input.Limit > 0 && input.Limit < selectedLines {
		selectedLines = input.Limit
		reason = "limit"
	}
	if selectedLines > maxMemoryReadLines {
		selectedLines = maxMemoryReadLines
		reason = "lines"
	}
	endIndex := startIndex
	selectedBytes := 0
	for endIndex < startIndex+selectedLines {
		lineBytes := len(lines[endIndex])
		if lineBytes > maxMemoryReadBytes && endIndex == startIndex {
			result := memoryRefusal(MemoryResultTooLarge, "the requested memory line exceeds the read result bound", "edit the memory to split the line", "read again from the same offset after splitting the line")
			result.Size = &MemorySizeFact{Bytes: lineBytes, MaxBytes: maxMemoryReadBytes}
			return result, nil
		}
		if lineBytes > maxMemoryReadBytes-selectedBytes {
			reason = "bytes"
			break
		}
		selectedBytes += lineBytes
		endIndex++
	}
	selected := bytes.Join(lines[startIndex:endIndex], nil)
	end := endIndex
	var next *int
	if endIndex < total {
		value := endIndex + 1
		next = &value
	}
	rangeFact := &MemoryRange{StartLine: input.Offset, EndLine: end, TotalLines: total, NextOffset: next, TruncatedBy: reason}
	metadata := doc.metadata
	return MemoryOperationResult{Condition: MemoryRead, Memory: &metadata, Content: string(selected), Range: rangeFact}, nil
}

func (s *Service) editMemory(input MemoryEditInput) (MemoryOperationResult, error) {
	if len(input.Edits) < 1 || len(input.Edits) > maxMemoryEdits {
		return MemoryOperationResult{}, fmt.Errorf("memory edit requires 1 through %d replacements", maxMemoryEdits)
	}
	for i, edit := range input.Edits {
		if edit.OldText == "" || len(edit.OldText) > maxMemoryBytes || len(edit.NewText) > maxMemoryBytes || !utf8.ValidString(edit.OldText) || !utf8.ValidString(edit.NewText) {
			return MemoryOperationResult{}, fmt.Errorf("memory edit %d strings must be valid UTF-8, oldText must be nonempty, and each string must be at most 1 MiB", i)
		}
	}
	_, doc, refused, err := s.inspectMemoryOperation(input.Slug, input.Owner)
	if err != nil || refused != nil {
		if refused != nil {
			return *refused, nil
		}
		return MemoryOperationResult{}, err
	}
	if doc.err != nil {
		return memoryRefusal(MemoryInvalid, "the effort memory metadata is invalid", "inspect the effort memory", "repair the effort memory metadata"), nil //nolint:nilerr // malformed managed state is a typed handled refusal
	}
	type match struct{ start, end, index int }
	matches := make([]match, 0, len(input.Edits))
	for i, edit := range input.Edits {
		positions := overlappingIndexes(doc.body, []byte(edit.OldText))
		if len(positions) == 0 {
			result := memoryRefusal(MemoryNoMatch, "an exact edit text is absent from the original memory body", "read the current memory body", "submit exact text from the current body")
			result.Edit = &MemoryEditFact{Index: i}
			return result, nil
		}
		if len(positions) > 1 {
			result := memoryRefusal(MemoryAmbiguousMatch, "an exact edit text occurs more than once in the original memory body", "include more surrounding text to make the edit unique")
			result.Edit = &MemoryEditFact{Index: i, Occurrences: len(positions)}
			return result, nil
		}
		matches = append(matches, match{start: positions[0], end: positions[0] + len(edit.OldText), index: i})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].start == matches[j].start {
			return matches[i].index < matches[j].index
		}
		return matches[i].start < matches[j].start
	})
	for i := 1; i < len(matches); i++ {
		if matches[i].start < matches[i-1].end {
			first, second := matches[i-1].index, matches[i].index
			if first > second {
				first, second = second, first
			}
			result := memoryRefusal(MemoryOverlappingEdits, "two requested edits overlap in the original memory body", "submit a batch with disjoint original regions")
			result.Overlap = &MemoryOverlapFact{FirstIndex: first, SecondIndex: second}
			return result, nil
		}
	}
	newBody := make([]byte, 0, len(doc.body))
	cursor := 0
	for _, found := range matches {
		newBody = append(newBody, doc.body[cursor:found.start]...)
		newBody = append(newBody, input.Edits[found.index].NewText...)
		cursor = found.end
	}
	newBody = append(newBody, doc.body[cursor:]...)
	metadata := doc.metadata
	metadata.Effort = input.Slug
	metadata.Updated = formatMemoryTime(s.now())
	encoded, err := encodeMemory(metadata, newBody)
	if err != nil { // coverage-ignore: fixed scalar-only metadata was validated before encoding
		return MemoryOperationResult{}, fmt.Errorf("encode edited memory: %w", err)
	}
	if len(encoded) > maxMemoryBytes {
		result := memoryRefusal(MemoryResultTooLarge, "the edited memory would exceed the resident size bound", "reduce the replacement size", "split the intended content across durable artifacts")
		result.Size = &MemorySizeFact{Bytes: len(encoded), MaxBytes: maxMemoryBytes}
		return result, nil
	}
	diff := memoryDiff(doc.body, newBody)
	if err := s.store.replaceMemory(s.paths.memoryFile(input.Slug), encoded); err != nil {
		return memoryFailure(err), nil
	}
	return MemoryOperationResult{Condition: MemoryEdited, Memory: &metadata, ReplacementCount: len(input.Edits), Diff: &diff}, nil
}

// UpdateMemory applies one typed structured metadata update.
func (s *Service) UpdateMemory(input MemoryUpdateInput) (MemoryOperationResult, error) {
	if err := validateMemoryUpdate(input.Update); err != nil {
		return MemoryOperationResult{}, err
	}
	_, doc, refused, err := s.inspectMemoryOperation(input.Slug, input.Owner)
	if err != nil || refused != nil {
		if refused != nil {
			return *refused, nil
		}
		return MemoryOperationResult{}, err
	}
	metadata, encoded, invalid := s.prepareMemoryUpdate(input.Slug, doc, input.Update)
	if invalid != nil {
		result := memoryRefusal(MemoryInvalid, "the effort memory metadata cannot be safely repaired by this update", "inspect the effort memory metadata", invalid.NextAction)
		return result, nil //nolint:nilerr // unsafe repair is a typed handled refusal
	}
	if len(encoded) > maxMemoryBytes {
		result := memoryRefusal(MemoryResultTooLarge, "the updated memory would exceed the resident size bound", "reduce the replacement metadata size")
		result.Size = &MemorySizeFact{Bytes: len(encoded), MaxBytes: maxMemoryBytes}
		return result, nil
	}
	if err := s.store.replaceMemory(s.paths.memoryFile(input.Slug), encoded); err != nil {
		return memoryFailure(err), nil
	}
	return MemoryOperationResult{Condition: MemoryUpdated, Memory: &metadata}, nil
}

func validateMemoryUpdate(update MemoryUpdate) error {
	if update.Phase == nil && update.Next == nil {
		return errors.New("memory update requires at least one replacement field")
	}
	if update.Phase != nil {
		if err := validateMemoryMutable(*update.Phase); err != nil {
			return fmt.Errorf("invalid memory phase: %w", err)
		}
	}
	if update.Next != nil {
		if err := validateMemoryMutable(*update.Next); err != nil {
			return fmt.Errorf("invalid memory next: %w", err)
		}
	}
	return nil
}

func (s *Service) inspectMemoryOperation(slug, owner string) ([]byte, memoryDocument, *MemoryOperationResult, error) {
	if err := validateSlug(slug); err != nil {
		return nil, memoryDocument{}, nil, invalidSlugRefusal(slug, err)
	}
	if owner != "" && !uuidV4Pattern.MatchString(owner) {
		return nil, memoryDocument{}, nil, errors.New("memory owner must be a lowercase UUIDv4")
	}
	if _, err := s.store.loadDirectory(s.paths.effort(slug), slug, false); err != nil {
		condition := MemoryUnsafe
		observed := "the effort resident cannot be safely used"
		actions := []string{"inspect the effort resident", "repair the effort resident"}
		if errors.Is(err, os.ErrNotExist) {
			condition = MemoryMissing
			observed = "the requested effort is absent"
			actions = []string{"list the active efforts", "use an active effort slug"}
		}
		result := memoryRefusal(condition, observed, actions...)
		return nil, memoryDocument{}, &result, nil
	}
	if owner != "" {
		activity, _, err := s.activityCurrentIdentity(slug)
		if err != nil {
			result := memoryRefusal(MemoryUnsafeActivity, "the activity resident cannot be safely used", "inspect the activity resident", "attach again after repairing the activity resident")
			return nil, memoryDocument{}, &result, nil //nolint:nilerr // unsafe managed state is a typed handled refusal
		}
		if activity == nil {
			result := memoryRefusal(MemoryMissing, "the requested activity is absent", "attach the session owner to the effort")
			return nil, memoryDocument{}, &result, nil
		}
		if activity.Owner != owner {
			result := memoryRefusal(MemoryNotOwner, "the activity resident names a different owner", "clear the stale local association", "attach explicitly to take over")
			return nil, memoryDocument{}, &result, nil
		}
	}
	path := s.paths.memoryFile(slug)
	raw, err := readRegularNoFollowBounded(path, maxMemoryBytes)
	if err != nil {
		condition := MemoryInvalid
		observed := "the effort memory cannot be read"
		var unsafe *awfgit.HardSafetyError
		if errors.As(err, &unsafe) {
			condition = MemoryUnsafe
			observed = "the memory resident cannot be safely used"
		}
		result := memoryRefusal(condition, observed, "inspect the effort memory resident", "repair the effort memory resident")
		return nil, memoryDocument{}, &result, nil
	}
	return raw, inspectMemory(raw, slug), nil, nil
}

func (s *Service) prepareMemoryUpdate(slug string, doc memoryDocument, update MemoryUpdate) (MemoryMetadata, []byte, *invalidMemoryUpdate) {
	if !doc.boundary || doc.identity != slug {
		return MemoryMetadata{}, nil, &invalidMemoryUpdate{NextAction: "repair memory.md manually with a matching bounded canonical or legacy identity"}
	}
	if doc.err != nil {
		if doc.invalid["phase"] && update.Phase == nil || doc.invalid["next"] && update.Next == nil || len(doc.invalid) == 0 {
			return MemoryMetadata{}, nil, &invalidMemoryUpdate{NextAction: memoryUpdateCommand(slug, doc.invalid)}
		}
	}
	metadata := doc.metadata
	metadata.Effort = slug
	if update.Phase != nil {
		metadata.Phase = *update.Phase
	}
	if update.Next != nil {
		metadata.Next = *update.Next
	}
	metadata.Updated = formatMemoryTime(s.now())
	if validateMemoryMutable(metadata.Phase) != nil || validateMemoryMutable(metadata.Next) != nil { // coverage-ignore: supplied fields are validated and every unrepaired invalid field returned above
		return MemoryMetadata{}, nil, &invalidMemoryUpdate{NextAction: memoryUpdateCommand(slug, doc.invalid)}
	}
	encoded, err := encodeMemory(metadata, doc.body)
	if err != nil { // coverage-ignore: fixed scalar-only metadata was validated before encoding
		return MemoryMetadata{}, nil, &invalidMemoryUpdate{NextAction: "inspect the effort memory metadata"}
	}
	return metadata, encoded, nil
}

func completeLines(raw []byte) [][]byte {
	lines := bytes.SplitAfter(raw, []byte("\n"))
	if len(lines) > 1 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func boundedMemoryText(value string, limit int) string {
	value = strings.ToValidUTF8(value, "?")
	if len(value) <= limit {
		return value
	}
	return string(validUTF8Prefix([]byte(value), limit))
}

func validUTF8Prefix(raw []byte, limit int) []byte {
	end := limit
	for end > 0 && !utf8.Valid(raw[:end]) {
		end--
	}
	return raw[:end]
}

func overlappingIndexes(body, old []byte) []int {
	var indexes []int
	for offset := 0; offset+len(old) <= len(body); {
		found := bytes.Index(body[offset:], old)
		if found < 0 {
			break
		}
		at := offset + found
		indexes = append(indexes, at)
		offset = at + 1
	}
	return indexes
}

func memoryDiff(before, after []byte) MemoryDiff {
	if bytes.Equal(before, after) {
		return MemoryDiff{}
	}
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	// Canonical memory has six header lines, so the first body line is line 7.
	line := 7 + bytes.Count(before[:prefix], []byte("\n"))
	text := append([]byte("before:\n"), before...)
	text = append(text, []byte("\nafter:\n")...)
	text = append(text, after...)
	truncated := len(text) > maxMemoryDiffBytes
	if truncated {
		text = validUTF8Prefix(text, maxMemoryDiffBytes)
	}
	return MemoryDiff{Text: string(text), FirstChangedLine: &line, Truncated: truncated}
}
