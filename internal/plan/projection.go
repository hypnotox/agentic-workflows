package plan

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// NotFoundError reports an exact plan name or selector that did not resolve.
type NotFoundError struct {
	Kind      string
	Value     string
	Available []string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("plan %s %q not found; available: %s", e.Kind, e.Value, strings.Join(e.Available, ", "))
}

// AmbiguousError reports an exact spelling that names more than one plan.
type AmbiguousError struct {
	Value     string
	Available []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("plan name %q is ambiguous; available: %s", e.Value, strings.Join(e.Available, ", "))
}

// InvalidSelectorError reports a selector outside canonical positive P or P.T
// syntax. Available carries the exact selectors accepted by the selected plan.
type InvalidSelectorError struct {
	Value     string
	Available []string
}

func (e *InvalidSelectorError) Error() string {
	return fmt.Sprintf("plan selector %q must be canonical positive P or P.T; available: %s", e.Value, strings.Join(e.Available, ", "))
}

// Resolve parses the supplied plans directory and selects an exact filename or
// filename stem. It deliberately accepts no path grammar or fuzzy title match.
func Resolve(dir, name string) (Plan, error) {
	plans, err := ParseDir(dir)
	if err != nil {
		return Plan{}, err
	}
	available := availablePlans(plans)
	if !validPlanName(name) {
		return Plan{}, &NotFoundError{Kind: "name", Value: name, Available: available}
	}
	var matches []Plan
	for _, p := range plans {
		if name == p.Filename || name == strings.TrimSuffix(p.Filename, ".md") {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Plan{}, &NotFoundError{Kind: "name", Value: name, Available: available}
	default:
		return Plan{}, &AmbiguousError{Value: name, Available: available}
	}
}

func validPlanName(name string) bool {
	return name != "" && name != "." && name != ".." && !filepath.IsAbs(name) &&
		!windowsAbsRe.MatchString(name) && filepath.Base(name) == name &&
		!strings.ContainsAny(name, `/\\`)
}

func availablePlans(plans []Plan) []string {
	seen := map[string]bool{}
	for _, p := range plans {
		seen[p.Filename] = true
		seen[strings.TrimSuffix(p.Filename, ".md")] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// RenderProjection renders an executable closure selected by canonical P or
// P.T entirely from a validated plan-v1 model.
func RenderProjection(p Plan, selector string) ([]byte, error) {
	available := availableSelectors(p)
	if p.Format != "plan-v1" || p.Preamble == "" || len(p.Phases) == 0 {
		return nil, &Diagnostic{Category: "projection", Path: p.Filename, Detail: "legacy or incomplete plans have no executable projection"}
	}
	phaseNumber, taskNumber, err := parseSelector(selector, available)
	if err != nil {
		return nil, err
	}
	var phase *Phase
	for i := range p.Phases {
		if p.Phases[i].Number == phaseNumber {
			phase = &p.Phases[i]
			break
		}
	}
	if phase == nil {
		return nil, &NotFoundError{Kind: "selector", Value: selector, Available: available}
	}
	var selected []Task
	if taskNumber == 0 {
		selected = phase.Tasks
	} else {
		for _, task := range phase.Tasks {
			if task.Number == taskNumber {
				selected = []Task{task}
				break
			}
		}
		if len(selected) == 0 {
			return nil, &NotFoundError{Kind: "selector", Value: selector, Available: available}
		}
	}

	var b strings.Builder
	b.WriteString(p.Preamble)
	b.WriteString(p.Goal)
	b.WriteString(p.ArchitectureSummary)
	b.WriteString(phase.Prefix)
	for _, task := range selected {
		b.WriteString(task.Content)
	}
	b.WriteString(phase.Close)
	b.WriteString(p.DefinitionOfDone)
	b.WriteString(p.Notes)
	return []byte(b.String()), nil
}

func parseSelector(value string, available []string) (int, int, error) {
	parts := strings.Split(value, ".")
	if len(parts) < 1 || len(parts) > 2 {
		return 0, 0, &InvalidSelectorError{Value: value, Available: available}
	}
	values := [2]int{}
	for i, part := range parts {
		if part == "" || strings.HasPrefix(part, "+") || strings.HasPrefix(part, "0") {
			return 0, 0, &InvalidSelectorError{Value: value, Available: available}
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 {
			return 0, 0, &InvalidSelectorError{Value: value, Available: available}
		}
		values[i] = n
	}
	return values[0], values[1], nil
}

func availableSelectors(p Plan) []string {
	type selector struct {
		phase int
		task  int
		text  string
	}
	var values []selector
	seen := map[string]bool{}
	for _, phase := range p.Phases {
		text := strconv.Itoa(phase.Number)
		if !seen[text] {
			values = append(values, selector{phase: phase.Number, text: text})
			seen[text] = true
		}
		for _, task := range phase.Tasks {
			text = fmt.Sprintf("%d.%d", phase.Number, task.Number)
			if !seen[text] {
				values = append(values, selector{phase: phase.Number, task: task.Number, text: text})
				seen[text] = true
			}
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].phase != values[j].phase {
			return values[i].phase < values[j].phase
		}
		return values[i].task < values[j].task
	})
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = value.text
	}
	return out
}
