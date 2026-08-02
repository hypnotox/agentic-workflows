package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// Diagnostic describes a stable plan parse, resolution, or projection failure.
// Path names the affected plan when one has been selected.
type Diagnostic struct {
	Category string
	Path     string
	Detail   string
}

func (d *Diagnostic) Error() string {
	if d.Path == "" {
		return fmt.Sprintf("plan %s: %s", d.Category, d.Detail)
	}
	return fmt.Sprintf("plan %s at %s: %s", d.Category, d.Path, d.Detail)
}

// DiagnosticsError retains every independently malformed plan in one directory
// parse while allowing valid siblings to remain available to project checks.
type DiagnosticsError struct {
	Diagnostics []*Diagnostic
}

func (e *DiagnosticsError) Error() string {
	if len(e.Diagnostics) == 1 {
		return e.Diagnostics[0].Error()
	}
	return fmt.Sprintf("%d plan diagnostics (first: %s)", len(e.Diagnostics), e.Diagnostics[0])
}

func (e *DiagnosticsError) Unwrap() []error {
	out := make([]error, len(e.Diagnostics))
	for i, diagnostic := range e.Diagnostics {
		out[i] = diagnostic
	}
	return out
}

// ExecutionMode identifies the transaction owner declared by a plan-v1 phase.
type ExecutionMode string

const (
	ExecutionInline         ExecutionMode = "inline"
	ExecutionSubagentDriven ExecutionMode = "subagent-driven"
)

// TaskKind identifies the optional specialized form of a plan-v1 task.
type TaskKind string

const (
	TaskImplementation TaskKind = ""
	TaskSpike          TaskKind = "spike"
	TaskBatch          TaskKind = "batch"
)

// TaskLatitude identifies the optional exact authoring form of a plan-v1 task.
type TaskLatitude string

const (
	TaskQualifying TaskLatitude = ""
	TaskExact      TaskLatitude = "exact"
)

// PathKind identifies how a Paths entry is interpreted.
type PathKind string

const (
	PathLiteral  PathKind = "literal"
	PathGlob     PathKind = "glob"
	PathPathspec PathKind = "pathspec"
)

// PathEntry is one validated Paths entry. Authored retains the decoded JSON
// string. Value is the prefix-free literal/glob value or the complete Git
// pathspec payload, which consumers must pass byte-for-byte.
type PathEntry struct {
	Kind     PathKind
	Authored string
	Value    string
}

// TaskFields is the typed field vocabulary directly beneath a task heading.
type TaskFields struct {
	Kind           TaskKind
	Latitude       TaskLatitude
	Question       string
	Paths          []PathEntry
	Representative string
	Edge           string
	PostCheck      string
}

// Phase is one ordered executable phase in a plan-v1 document. Prefix retains
// the phase heading, spacing, and execution-mode declaration exactly.
type Phase struct {
	Number        int
	Title         string
	Prefix        string
	ExecutionMode ExecutionMode
	Tasks         []Task
	Close         string
}

// Task is one ordered task in a plan-v1 phase. Content retains its complete
// authored block so projection never reparses or reconstructs Markdown.
type Task struct {
	Phase   int
	Number  int
	Title   string
	Fields  TaskFields
	Content string
}

var (
	phaseHeadingRe = regexp.MustCompile(`^## Phase ([1-9][0-9]*): (.+)$`)
	taskHeadingRe  = regexp.MustCompile(`^### Task ([1-9][0-9]*)\.([1-9][0-9]*): (.+)$`)
	fieldLineRe    = regexp.MustCompile(`^([A-Za-z][A-Za-z-]*):(.*)$`)
	windowsAbsRe   = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

func structuralError(path, category, detail string) error {
	return &Diagnostic{Category: category, Path: path, Detail: detail}
}

func parsePlanV1(path string, source, body string, p *Plan) error {
	p.Source = []byte(source)
	p.Format = "plan-v1"
	lines := splitLines(body)
	if err := rejectRetiredSections(path, lines); err != nil {
		return err
	}
	if len(lines) == 0 || !strings.HasPrefix(lineText(lines[0]), "# Plan: ") || strings.TrimSpace(strings.TrimPrefix(lineText(lines[0]), "# Plan: ")) == "" {
		return structuralError(path, "structure", "expected # Plan: title")
	}
	p.Title = strings.TrimSpace(strings.TrimPrefix(lineText(lines[0]), "# Plan: "))

	idx := skipBlank(lines, 1)
	if idx >= len(lines) || lineText(lines[idx]) != "## Goal" {
		return structuralError(path, "structure", "expected ## Goal after title")
	}
	goalEnd, err := findRequiredBoundary(path, lines, idx+1, "Goal", func(s string) bool { return s == "## Architecture summary" })
	if err != nil {
		return err
	}
	if !sectionBodyNonempty(lines, idx+1, goalEnd) {
		return structuralError(path, "structure", "Goal must be nonempty")
	}
	p.Preamble = source[:len(source)-len(body)] + strings.Join(lines[:idx], "")
	p.Goal = strings.Join(lines[idx:goalEnd], "")

	idx = goalEnd
	archEnd, err := findRequiredBoundary(path, lines, idx+1, "Architecture summary", func(s string) bool { return strings.HasPrefix(s, "## Phase ") })
	if err != nil {
		return err
	}
	if !sectionBodyNonempty(lines, idx+1, archEnd) {
		return structuralError(path, "structure", "Architecture summary must be nonempty")
	}
	p.ArchitectureSummary = strings.Join(lines[idx:archEnd], "")
	idx = archEnd

	for idx < len(lines) && strings.HasPrefix(lineText(lines[idx]), "## Phase ") {
		ph, next, err := parsePhase(path, lines, idx, len(p.Phases)+1)
		if err != nil {
			return err
		}
		p.Phases = append(p.Phases, ph)
		idx = next
	}
	if len(p.Phases) == 0 { // coverage-ignore: the required phase boundary either enters the loop or already returned a diagnostic
		return structuralError(path, "structure", "expected one or more phases")
	}
	if idx >= len(lines) || lineText(lines[idx]) != "## Definition of done" {
		return structuralError(path, "structure", "expected ## Definition of done after final phase")
	}
	dodEnd := len(lines)
	var dodFence markdownFence
	for i := idx + 1; i < len(lines); i++ {
		text := lineText(lines[i])
		if dodFence.consume(text) {
			continue
		}
		if text == "## Notes" {
			dodEnd = i
			break
		}
		if isReservedTopHeading(text) {
			return structuralError(path, "structure", "unexpected top-level section before Notes")
		}
	}
	if !hasPlainBullet(lines[idx+1 : dodEnd]) {
		return structuralError(path, "structure", "Definition of done requires a nonempty plain bullet")
	}
	if hasCheckbox(lines[idx+1 : dodEnd]) {
		return structuralError(path, "structure", "Definition of done uses plain bullets, not checkboxes")
	}
	p.DefinitionOfDone = strings.Join(lines[idx:dodEnd], "")
	idx = dodEnd
	if idx < len(lines) {
		if lineText(lines[idx]) != "## Notes" { // coverage-ignore: dodEnd moves below len only when this exact heading is found
			return structuralError(path, "structure", "unexpected top-level content")
		}
		p.Notes = strings.Join(lines[idx:], "")
		idx = len(lines)
	}
	if idx != len(lines) { // coverage-ignore: the optional Notes branch consumes the entire remaining document
		return structuralError(path, "structure", "unexpected top-level content")
	}
	for _, ph := range p.Phases {
		for _, task := range ph.Tasks {
			if task.Fields.Kind == TaskSpike && !sectionBodyNonempty(splitLines(p.Notes), 1, len(splitLines(p.Notes))) {
				return structuralError(path, "relationship", "spike requires nonempty Notes")
			}
		}
	}
	return nil
}

func splitLines(s string) []string {
	lines := strings.SplitAfter(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func lineText(line string) string { return strings.TrimRight(line, "\r\n") }

func skipBlank(lines []string, start int) int {
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	return start
}

func sectionBodyNonempty(lines []string, start, end int) bool {
	if start > end || start < 0 || end > len(lines) { // coverage-ignore: parser-owned boundaries are constructed within the same line slice
		return false
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "")) != ""
}

// markdownFence tracks CommonMark-style backtick and tilde fences for the
// structural scanner. A closing run must use the opening marker, be at least
// as long, and carry no info string.
type markdownFence struct {
	marker byte
	length int
}

// consume reports whether line is structurally opaque, including the opener,
// every line inside the fence, and a qualifying closer.
func (f *markdownFence) consume(line string) bool {
	marker, length, ok := structuralFenceMarker(line)
	if f.marker == 0 {
		if !ok {
			return false
		}
		f.marker, f.length = marker, length
		return true
	}
	if ok && marker == f.marker && length >= f.length && structuralFenceCloser(line, length) {
		f.marker, f.length = 0, 0
	}
	return true
}

func structuralFenceMarker(line string) (byte, int, bool) {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent > 3 || indent == len(line) {
		return 0, 0, false
	}
	marker := line[indent]
	if marker != '`' && marker != '~' {
		return 0, 0, false
	}
	length := 0
	for indent+length < len(line) && line[indent+length] == marker {
		length++
	}
	return marker, length, length >= 3
}

func structuralFenceCloser(line string, markerLength int) bool {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	return strings.TrimSpace(line[indent+markerLength:]) == ""
}

func findRequiredBoundary(path string, lines []string, start int, section string, expected func(string) bool) (int, error) {
	var fence markdownFence
	for i := start; i < len(lines); i++ {
		text := lineText(lines[i])
		if fence.consume(text) {
			continue
		}
		if expected(text) {
			return i, nil
		}
		if isReservedTopHeading(text) {
			return 0, structuralError(path, "structure", "unexpected top-level section after "+section)
		}
	}
	return 0, structuralError(path, "structure", "missing section after "+section)
}

func isReservedTopHeading(s string) bool {
	return s == "## Goal" || s == "## Architecture summary" || strings.HasPrefix(s, "## Phase ") ||
		s == "## Definition of done" || s == "## Notes" || s == "## File structure" || s == "## Verification"
}

func rejectRetiredSections(path string, lines []string) error {
	var fence markdownFence
	for _, line := range lines {
		text := lineText(line)
		if fence.consume(text) {
			continue
		}
		if text == "## File structure" || text == "## Verification" {
			return structuralError(path, "structure", "File structure and Verification are not plan-v1 sections")
		}
	}
	return nil
}

func hasPlainBullet(lines []string) bool {
	var fence markdownFence
	for _, line := range lines {
		text := lineText(line)
		if fence.consume(text) {
			continue
		}
		if len(text) < 3 || !strings.ContainsRune("-*+", rune(text[0])) || text[1] != ' ' {
			continue
		}
		body := strings.TrimSpace(text[2:])
		if body != "" && !strings.HasPrefix(strings.ToLower(body), "[ ]") && !strings.HasPrefix(strings.ToLower(body), "[x]") {
			return true
		}
	}
	return false
}

func hasCheckbox(lines []string) bool {
	var fence markdownFence
	for _, line := range lines {
		text := strings.ToLower(strings.TrimSpace(lineText(line)))
		if fence.consume(text) {
			continue
		}
		if len(text) >= 5 && strings.ContainsRune("-*+", rune(text[0])) && (strings.HasPrefix(text[1:], " [ ]") || strings.HasPrefix(text[1:], " [x]")) {
			return true
		}
	}
	return false
}

func forbiddenTaskTitle(title string) bool {
	for _, prefix := range []string{"optional ", "optional:", "optional-", "conditional ", "conditional:", "conditional-"} {
		if strings.HasPrefix(title, prefix) {
			return true
		}
	}
	for _, suffix := range []string{
		" (optional)", " (conditional)", " [optional]", " [conditional]",
		" if needed", " if required", " if applicable",
		" when needed", " when required", " when applicable",
		" as needed", " where required", " where applicable",
	} {
		if strings.HasSuffix(title, suffix) {
			return true
		}
	}
	return false
}

func parsePhase(path string, lines []string, start, want int) (Phase, int, error) {
	m := phaseHeadingRe.FindStringSubmatch(lineText(lines[start]))
	if m == nil {
		return Phase{}, start, structuralError(path, "structure", "malformed phase heading")
	}
	n, _ := parsePositive(m[1])
	if n != want {
		return Phase{}, start, structuralError(path, "numbering", fmt.Sprintf("phase number %d, want %d", n, want))
	}
	ph := Phase{Number: n, Title: m[2]}
	i := skipBlank(lines, start+1)
	if i >= len(lines) {
		return ph, start, structuralError(path, "structure", fmt.Sprintf("phase %d requires an execution mode", n))
	}
	switch lineText(lines[i]) {
	case "**Execution mode: inline.**":
		ph.ExecutionMode = ExecutionInline
	case "**Execution mode: subagent-driven.**":
		ph.ExecutionMode = ExecutionSubagentDriven
	default:
		return ph, start, structuralError(path, "structure", fmt.Sprintf("phase %d requires exact execution mode", n))
	}
	i++
	firstTask := skipBlank(lines, i)
	if firstTask >= len(lines) || !strings.HasPrefix(lineText(lines[firstTask]), "### Task ") {
		if firstTask < len(lines) && hasCheckbox(lines[firstTask:firstTask+1]) {
			return ph, start, structuralError(path, "structure", "task checkboxes are not plan-v1 declarations")
		}
		return ph, start, structuralError(path, "structure", fmt.Sprintf("phase %d requires one or more tasks", n))
	}
	ph.Prefix = strings.Join(lines[start:firstTask], "")
	i = firstTask
	for i < len(lines) && strings.HasPrefix(lineText(lines[i]), "### Task ") {
		task, next, err := parseTask(path, lines, i, n, len(ph.Tasks)+1)
		if err != nil {
			return ph, start, err
		}
		ph.Tasks = append(ph.Tasks, task)
		i = next
	}
	if i >= len(lines) || lineText(lines[i]) != "### Phase close" {
		return ph, start, structuralError(path, "structure", fmt.Sprintf("phase %d requires one final Phase close", n))
	}
	end := len(lines)
	var closeFence markdownFence
	for j := i + 1; j < len(lines); j++ {
		text := lineText(lines[j])
		if closeFence.consume(text) {
			continue
		}
		if strings.HasPrefix(text, "## Phase ") || text == "## Definition of done" {
			end = j
			break
		}
		if strings.HasPrefix(text, "### Task ") || text == "### Phase close" {
			return ph, start, structuralError(path, "phase-close", fmt.Sprintf("Phase close must be the final child of phase %d", n))
		}
		if isReservedTopHeading(text) {
			return ph, start, structuralError(path, "structure", "unexpected top-level section after Phase close")
		}
	}
	ph.Close = strings.Join(lines[i:end], "")
	phaseContent := strings.Join(lines[start:end], "")
	if countExecutionModeDeclarations(phaseContent) != 1 {
		return ph, start, structuralError(path, "structure", fmt.Sprintf("phase %d requires exactly one execution-mode declaration", n))
	}
	if commitFenceSwallowsPlanBoundary(ph.Close) || countCompleteCommitFences(ph.Close) != 1 || countCompleteCommitFences(phaseContent) != 1 {
		return ph, start, structuralError(path, "phase-close", fmt.Sprintf("phase %d requires exactly one non-ignored commit fence in Phase close", n))
	}
	if len(ph.Tasks) == 1 && ph.Tasks[0].Fields.Kind == TaskSpike {
		return ph, start, structuralError(path, "relationship", "spike cannot constitute a phase alone")
	}
	return ph, end, nil
}

func parseTask(path string, lines []string, start, phase, want int) (Task, int, error) {
	m := taskHeadingRe.FindStringSubmatch(lineText(lines[start]))
	if m == nil {
		return Task{}, start, structuralError(path, "structure", "malformed task heading")
	}
	pnum, _ := parsePositive(m[1])
	num, _ := parsePositive(m[2])
	if pnum != phase || num != want {
		return Task{}, start, structuralError(path, "numbering", fmt.Sprintf("task number %d.%d, want %d.%d", pnum, num, phase, want))
	}
	task := Task{Phase: pnum, Number: num, Title: m[3]}
	i := start + 1
	seen := map[string]bool{}
	for i < len(lines) {
		name, value, field, malformed := parseField(lineText(lines[i]))
		if malformed {
			return task, start, structuralError(path, "field", fmt.Sprintf("task %d.%d has malformed field %s", phase, num, name))
		}
		if !field {
			break
		}
		if !knownField(name) {
			return task, start, structuralError(path, "field", fmt.Sprintf("task %d.%d has unknown field %s", phase, num, name))
		}
		if seen[name] {
			return task, start, structuralError(path, "field", fmt.Sprintf("task %d.%d duplicates field %s", phase, num, name))
		}
		if value == "" {
			return task, start, structuralError(path, "field", fmt.Sprintf("task %d.%d field %s must be nonempty", phase, num, name))
		}
		seen[name] = true
		if err := assignTaskField(path, &task.Fields, name, value); err != nil {
			return task, start, err
		}
		i++
	}
	end := len(lines)
	var taskFence markdownFence
	for j := i; j < len(lines); j++ {
		text := lineText(lines[j])
		if taskFence.consume(text) {
			continue
		}
		if strings.HasPrefix(text, "### Task ") || text == "### Phase close" || strings.HasPrefix(text, "## Phase ") || text == "## Definition of done" {
			end = j
			break
		}
		if isReservedTopHeading(text) {
			return task, start, structuralError(path, "structure", "unexpected top-level section inside task")
		}
	}
	var fieldFence markdownFence
	for _, line := range lines[i:end] {
		text := lineText(line)
		if fieldFence.consume(text) {
			continue
		}
		name, _, field, _ := parseField(text)
		if field && knownField(name) {
			return task, start, structuralError(path, "field", fmt.Sprintf("task %d.%d field %s is not contiguous below its heading", phase, num, name))
		}
	}
	task.Content = strings.Join(lines[start:end], "")
	if hasCheckbox(lines[start:end]) {
		return task, start, structuralError(path, "structure", "task checkboxes are not plan-v1 declarations")
	}
	lowerTitle := strings.ToLower(strings.TrimSpace(task.Title))
	if forbiddenTaskTitle(lowerTitle) {
		return task, start, structuralError(path, "structure", "conditional and optional task declarations are forbidden")
	}
	if err := validateTask(path, task, strings.Join(lines[i:end], "")); err != nil {
		return task, start, err
	}
	return task, end, nil
}

func parsePositive(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parseField(line string) (name, value string, field, malformed bool) {
	m := fieldLineRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false, false
	}
	name = m[1]
	rest := m[2]
	if rest == "" {
		return name, "", true, false
	}
	if !strings.HasPrefix(rest, " ") {
		return name, "", false, knownField(name)
	}
	return name, strings.TrimSpace(strings.TrimPrefix(rest, " ")), true, false
}

func knownField(name string) bool {
	switch name {
	case "Kind", "Latitude", "Question", "Paths", "Representative", "Edge", "Post-check":
		return true
	default:
		return false
	}
}

func assignTaskField(path string, fields *TaskFields, name, value string) error {
	switch name {
	case "Kind":
		fields.Kind = TaskKind(value)
	case "Latitude":
		fields.Latitude = TaskLatitude(value)
	case "Question":
		fields.Question = value
	case "Paths":
		entries, err := parsePathEntries(value)
		if err != nil {
			return structuralError(path, "paths", err.Error())
		}
		fields.Paths = entries
	case "Representative":
		fields.Representative = value
	case "Edge":
		fields.Edge = value
	case "Post-check":
		fields.PostCheck = value
	}
	return nil
}

func validateTask(path string, task Task, body string) error {
	f := task.Fields
	if f.Kind != TaskImplementation && f.Kind != TaskSpike && f.Kind != TaskBatch {
		return structuralError(path, "field", fmt.Sprintf("task %d.%d Kind must be spike or batch", task.Phase, task.Number))
	}
	if f.Latitude != TaskQualifying && f.Latitude != TaskExact {
		return structuralError(path, "field", fmt.Sprintf("task %d.%d Latitude must be exact", task.Phase, task.Number))
	}
	if f.Kind == TaskSpike {
		if f.Question == "" {
			return structuralError(path, "relationship", fmt.Sprintf("spike %d.%d requires Question", task.Phase, task.Number))
		}
		if len(f.Paths) > 0 || f.Representative != "" || f.Edge != "" || f.PostCheck != "" {
			return structuralError(path, "relationship", fmt.Sprintf("spike %d.%d forbids batch fields", task.Phase, task.Number))
		}
		if strings.TrimSpace(body) != "" {
			return structuralError(path, "relationship", fmt.Sprintf("spike %d.%d has no prose body", task.Phase, task.Number))
		}
	} else if f.Question != "" {
		return structuralError(path, "relationship", fmt.Sprintf("task %d.%d Question requires Kind: spike", task.Phase, task.Number))
	}
	if f.Kind == TaskBatch {
		if len(f.Paths) == 0 || f.Representative == "" || f.Edge == "" || f.PostCheck == "" {
			return structuralError(path, "relationship", fmt.Sprintf("batch %d.%d requires Paths, Representative, Edge, and Post-check", task.Phase, task.Number))
		}
	} else if f.Representative != "" || f.Edge != "" {
		return structuralError(path, "relationship", fmt.Sprintf("task %d.%d Representative and Edge require Kind: batch", task.Phase, task.Number))
	}
	needsPostCheck := f.Kind == TaskBatch
	for _, entry := range f.Paths {
		if entry.Kind == PathGlob || entry.Kind == PathPathspec {
			needsPostCheck = true
		}
	}
	if needsPostCheck && f.PostCheck == "" {
		return structuralError(path, "relationship", fmt.Sprintf("task %d.%d requires Post-check for batch, glob, or pathspec scope", task.Phase, task.Number))
	}
	return nil
}

func parsePathEntries(raw string) ([]PathEntry, error) {
	var values []any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, errors.New("paths must be a nonempty JSON array of strings")
	}
	if len(values) == 0 {
		return nil, errors.New("paths must be a nonempty JSON array of strings")
	}
	entries := make([]PathEntry, 0, len(values))
	seen := map[string]bool{}
	for _, item := range values {
		value, ok := item.(string)
		if !ok || value == "" {
			return nil, errors.New("paths entries must be nonempty strings")
		}
		if seen[value] {
			return nil, errors.New("paths entries must be unique after JSON decoding")
		}
		seen[value] = true
		entry, err := parsePathEntry(value)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func parsePathEntry(authored string) (PathEntry, error) {
	switch {
	case strings.HasPrefix(authored, "glob:"):
		value := strings.TrimPrefix(authored, "glob:")
		if !confinedPath(value) {
			return PathEntry{}, errors.New("glob path escapes repository")
		}
		if err := pathglob.Validate(value); err != nil {
			return PathEntry{}, err
		}
		return PathEntry{Kind: PathGlob, Authored: authored, Value: value}, nil
	case strings.HasPrefix(authored, "pathspec:"):
		value := strings.TrimPrefix(authored, "pathspec:")
		if _, err := pathspecPathPortion(value); err != nil {
			return PathEntry{}, err
		}
		return PathEntry{Kind: PathPathspec, Authored: authored, Value: value}, nil
	default:
		if strings.ContainsAny(authored, "*?[") || strings.HasPrefix(authored, ":") {
			return PathEntry{}, errors.New("literal path contains glob or Git pathspec magic syntax")
		}
		if !confinedPath(authored) {
			return PathEntry{}, errors.New("literal path escapes repository")
		}
		return PathEntry{Kind: PathLiteral, Authored: authored, Value: authored}, nil
	}
}

func confinedPath(value string) bool {
	if value == "" || filepath.IsAbs(value) || windowsAbsRe.MatchString(value) || strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\\`) {
		return false
	}
	for _, component := range strings.Split(strings.ReplaceAll(value, `\`, "/"), "/") {
		if component == ".." {
			return false
		}
	}
	return true
}

func pathspecPathPortion(value string) (string, error) {
	if value == "" {
		return "", errors.New("empty pathspec")
	}
	path := value
	switch {
	case strings.HasPrefix(value, ":("):
		end := strings.Index(value[2:], ")")
		if end < 0 {
			return "", errors.New("pathspec magic prefix is missing terminator")
		}
		end += 2
		words := strings.Split(value[2:end], ",")
		if len(words) == 0 || words[0] == "" {
			return "", errors.New("unrecognized or malformed pathspec magic prefix")
		}
		seen := map[string]bool{}
		for _, word := range words {
			key := word
			if strings.HasPrefix(word, "attr:") {
				key = "attr"
			}
			if !recognizedPathspecMagic(word) || seen[key] {
				return "", errors.New("unrecognized or malformed pathspec magic prefix")
			}
			seen[key] = true
		}
		if seen["glob"] && seen["literal"] {
			return "", errors.New("pathspec magic glob and literal are incompatible")
		}
		path = value[end+1:]
	case strings.HasPrefix(value, ":"):
		signature := value[1:]
		i := 0
		seen := map[byte]bool{}
		for i < len(signature) && strings.ContainsRune("/!^", rune(signature[i])) {
			if seen[signature[i]] {
				return "", errors.New("unrecognized or malformed pathspec magic prefix")
			}
			seen[signature[i]] = true
			i++
		}
		if i == 0 || (seen['!'] && seen['^']) {
			return "", errors.New("unrecognized or malformed pathspec magic prefix")
		}
		if i < len(signature) && signature[i] == ':' {
			i++
		}
		path = signature[i:]
	}
	if !confinedPath(path) {
		return "", errors.New("pathspec path escapes repository")
	}
	return path, nil
}

func recognizedPathspecMagic(word string) bool {
	switch word {
	case "top", "literal", "glob", "icase", "exclude":
		return true
	default:
		return strings.HasPrefix(word, "attr:") && strings.TrimSpace(strings.TrimPrefix(word, "attr:")) != ""
	}
}

func countExecutionModeDeclarations(content string) int {
	var fence markdownFence
	count := 0
	for _, line := range strings.Split(content, "\n") {
		if fence.consume(line) {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "**Execution mode:") {
			count++
		}
	}
	return count
}

func countCompleteCommitFences(content string) int {
	var fence markdownFence
	counted := false
	count := 0
	for _, line := range strings.Split(content, "\n") {
		marker, length, ok := structuralFenceMarker(line)
		if fence.marker == 0 {
			if !ok {
				continue
			}
			fence.marker, fence.length = marker, length
			indent := len(line) - len(strings.TrimLeft(line, " "))
			counted = marker == '`' && isCommitInfo(line[indent+length:])
			continue
		}
		if ok && marker == fence.marker && length >= fence.length && structuralFenceCloser(line, length) {
			if counted {
				count++
			}
			fence.marker, fence.length = 0, 0
			counted = false
		}
	}
	return count
}

// commitFenceSwallowsPlanBoundary catches an unclosed exact commit fence before
// it can make a later phase or Definition-of-done heading structurally opaque.
// Other fenced examples retain ordinary Markdown opacity.
func commitFenceSwallowsPlanBoundary(content string) bool {
	var fence markdownFence
	commitFence := false
	for _, line := range strings.Split(content, "\n") {
		marker, length, ok := structuralFenceMarker(line)
		if fence.marker == 0 {
			if !ok {
				continue
			}
			fence.marker, fence.length = marker, length
			indent := len(line) - len(strings.TrimLeft(line, " "))
			commitFence = marker == '`' && isCommitInfo(line[indent+length:])
			continue
		}
		if ok && marker == fence.marker && length >= fence.length && structuralFenceCloser(line, length) {
			fence.marker, fence.length = 0, 0
			commitFence = false
			continue
		}
		if commitFence && (strings.HasPrefix(line, "## Phase ") || line == "## Definition of done") {
			return true
		}
	}
	return false
}
