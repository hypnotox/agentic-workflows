package effort

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pmezard/go-difflib/difflib"

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
	MemoryPreviewed        MemoryCondition = "previewed"
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
	Slug    string
	Owner   string
	Edits   []MemoryReplacement
	Preview bool
}

// MemoryUpdateInput carries a structured metadata update and optional advisory owner.
type MemoryUpdateInput struct {
	Slug    string
	Owner   string
	Update  MemoryUpdate
	Preview bool
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
	if input.Preview {
		bodyOffset := bytes.Count(raw[:len(raw)-len(doc.body)], []byte("\n"))
		diff := memoryDiff(doc.body, newBody, bodyOffset)
		return MemoryOperationResult{Condition: MemoryPreviewed, ReplacementCount: len(input.Edits), Diff: &diff}, nil
	}
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
	diff := memoryDiff(doc.body, newBody, 6)
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
	raw, doc, refused, err := s.inspectMemoryOperation(input.Slug, input.Owner)
	if err != nil || refused != nil {
		if refused != nil {
			return *refused, nil
		}
		return MemoryOperationResult{}, err
	}
	if input.Preview {
		if invalid := invalidMemoryUpdateFor(input.Slug, doc, input.Update); invalid != nil {
			result := memoryRefusal(MemoryInvalid, "the effort memory metadata cannot be safely repaired by this update", "inspect the effort memory metadata", invalid.NextAction)
			return result, nil
		}
		diff := updatePreviewDiff(raw, input.Update, doc.metadata, doc.legacy)
		return MemoryOperationResult{Condition: MemoryPreviewed, Diff: &diff}, nil
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
	diff := memoryDiff(raw, encoded, 0)
	return MemoryOperationResult{Condition: MemoryUpdated, Memory: &metadata, Diff: &diff}, nil
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

func invalidMemoryUpdateFor(slug string, doc memoryDocument, update MemoryUpdate) *invalidMemoryUpdate {
	if !doc.boundary || doc.identity != slug {
		return &invalidMemoryUpdate{NextAction: "repair memory.md manually with a matching bounded canonical or legacy identity"}
	}
	if doc.err != nil && (doc.invalid["phase"] && update.Phase == nil || doc.invalid["next"] && update.Next == nil || len(doc.invalid) == 0) {
		return &invalidMemoryUpdate{NextAction: memoryUpdateCommand(slug, doc.invalid)}
	}
	return nil
}

func (s *Service) prepareMemoryUpdate(slug string, doc memoryDocument, update MemoryUpdate) (MemoryMetadata, []byte, *invalidMemoryUpdate) {
	if invalid := invalidMemoryUpdateFor(slug, doc, update); invalid != nil {
		return MemoryMetadata{}, nil, invalid
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

type displayDiffRow struct {
	text    string
	changed bool
}

func memoryDiff(before, after []byte, lineOffset int) MemoryDiff {
	if bytes.Equal(before, after) {
		return MemoryDiff{}
	}
	oldLines, newLines := diffLines(before), diffLines(after)
	groups := difflib.NewMatcher(oldLines, newLines).GetGroupedOpCodes(4)
	width := len(strconv.Itoa(max(len(oldLines), len(newLines)) + lineOffset))
	var rows []displayDiffRow
	first := 0
	contextOmitted := false
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			rows = append(rows, displayDiffRow{text: omissionDisplayRow(width)})
			contextOmitted = true
		}
		for _, code := range group {
			switch code.Tag {
			case 'e':
				for i := code.I1; i < code.I2; i++ {
					rows = append(rows, displayDiffRow{text: displayRow(' ', i+lineOffset+1, width, displayLine(oldLines[i]))})
				}
			case 'd':
				for i := code.I1; i < code.I2; i++ {
					n := i + lineOffset + 1
					if first == 0 {
						first = n
					}
					rows = append(rows, displayDiffRow{text: displayRow('-', n, width, displayLine(oldLines[i])), changed: true})
				}
			case 'i':
				for i := code.J1; i < code.J2; i++ {
					n := i + lineOffset + 1
					if first == 0 {
						first = n
					}
					rows = append(rows, displayDiffRow{text: displayRow('+', n, width, displayLine(newLines[i])), changed: true})
				}
			case 'r':
				for i := code.I1; i < code.I2; i++ {
					n := i + lineOffset + 1
					if first == 0 {
						first = n
					}
					rows = append(rows, displayDiffRow{text: displayRow('-', n, width, displayLine(oldLines[i])), changed: true})
				}
				for i := code.J1; i < code.J2; i++ {
					n := i + lineOffset + 1
					rows = append(rows, displayDiffRow{text: displayRow('+', n, width, displayLine(newLines[i])), changed: true})
				}
			}
		}
	}
	text, truncated := boundedDisplayRows(rows, width)
	return MemoryDiff{Text: text, FirstChangedLine: &first, Truncated: truncated || contextOmitted}
}

func diffLines(raw []byte) []string {
	lines := completeLines(raw)
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = string(line)
	}
	return out
}

func displayLine(line string) string {
	text := strings.TrimSuffix(line, "\n")
	if len(line) > 0 && line[len(line)-1] == '\n' {
		text = strings.TrimSuffix(text, "\r")
	}
	return text
}

func displayRow(kind byte, line, width int, content string) string {
	return fmt.Sprintf("%c%*d %s", kind, width, line, content)
}

func omissionDisplayRow(width int) string {
	return " " + strings.Repeat(" ", width) + " ..."
}

func boundedDisplayRows(rows []displayDiffRow, width int) (string, bool) {
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
	if !ok || text == "" { // coverage-ignore: one elided changed-row prefix and omission rows fit far below the fixed 50-KiB bound
		return omissionDisplayRow(width) + "\n", true
	}
	return text, true
}

func renderSelectedDisplayRows(rows []displayDiffRow, selected []bool, width int) (string, bool) {
	out := make([]byte, 0, min(maxMemoryDiffBytes, len(rows)*32))
	omitted := false
	for index, row := range rows {
		if !selected[index] {
			omitted = true
			continue
		}
		if omitted {
			omission := omissionDisplayRow(width) + "\n"
			if len(out)+len(omission) > maxMemoryDiffBytes {
				return "", false
			}
			out = append(out, omission...)
			omitted = false
		}
		if len(out)+len(row.text)+1 > maxMemoryDiffBytes {
			return "", false
		}
		out = append(out, row.text...)
		out = append(out, '\n')
	}
	if omitted {
		omission := omissionDisplayRow(width) + "\n"
		if len(out)+len(omission) > maxMemoryDiffBytes {
			return "", false
		}
		out = append(out, omission...)
	}
	return string(out), true
}

func updatePreviewDiff(raw []byte, update MemoryUpdate, metadata MemoryMetadata, legacy bool) MemoryDiff {
	phaseKey, nextKey := "phase:", "next:"
	if legacy {
		phaseKey, nextKey = "Phase:", "Next:"
	}
	phaseLine, nextLine := 2, 3
	if legacy {
		phaseLine, nextLine = 1, 2
	}
	lines := completeLines(raw)
	after := make([]byte, 0, len(raw))
	for index, line := range lines {
		text := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
		ending := string(line[len(text):])
		switch {
		case index == phaseLine && update.Phase != nil && strings.HasPrefix(text, phaseKey) && *update.Phase != metadata.Phase:
			text = phaseKey + " " + *update.Phase
		case index == nextLine && update.Next != nil && strings.HasPrefix(text, nextKey) && *update.Next != metadata.Next:
			text = nextKey + " " + *update.Next
		}
		after = append(after, text...)
		after = append(after, ending...)
	}
	return memoryDiff(raw, after, 0)
}
