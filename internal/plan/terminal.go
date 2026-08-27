package plan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var terminalRangeRe = regexp.MustCompile(`^[0-9a-f]{40}\.\.[0-9a-f]{40}$`)

// TerminalReconciliation is the parsed historical evidence recorded when a
// Proposed plan becomes Implemented. It deliberately records what landed,
// rather than a comparison with planned choreography.
type TerminalReconciliation struct {
	ImplementationRange string
	TouchedPaths        []string
	MaterialDeviations  []string
}

// ImplementationEndpoints returns the already grammar-validated immutable
// base and head identifiers in the terminal implementation range.
func (r TerminalReconciliation) ImplementationEndpoints() (base, head string) {
	return r.ImplementationRange[:40], r.ImplementationRange[42:]
}

// ParseTerminalReconciliation parses the optional, exact Notes subsection.
// An absent subsection is ordinary while a plan remains Proposed; terminal
// transition validation requires it.
func ParseTerminalReconciliation(notes string) (*TerminalReconciliation, error) {
	lines := strings.Split(notes, "\n")
	start := -1
	var fence markdownFence
	for i, line := range lines {
		if fence.consume(line) {
			continue
		}
		if line == "### Terminal reconciliation" {
			if start >= 0 {
				return nil, fmt.Errorf("duplicate Terminal reconciliation")
			}
			start = i
		}
	}
	if start < 0 {
		return nil, nil
	}
	if start+3 >= len(lines) || !strings.HasPrefix(lines[start+1], "Implementation range: ") || lines[start+2] != "Touched paths:" {
		return nil, fmt.Errorf("Terminal reconciliation requires implementation range and touched paths")
	}
	r := &TerminalReconciliation{ImplementationRange: strings.TrimSpace(strings.TrimPrefix(lines[start+1], "Implementation range: "))}
	if !terminalRangeRe.MatchString(r.ImplementationRange) {
		return nil, fmt.Errorf("Terminal reconciliation implementation range must be two lowercase 40-hex commit IDs")
	}
	i := start + 3
	var err error
	r.TouchedPaths, i, err = terminalList(lines, i, "Material deviations:", false)
	if err != nil {
		return nil, err
	}
	r.MaterialDeviations, i, err = terminalList(lines, i, "", true)
	if err != nil {
		return nil, err
	}
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return nil, fmt.Errorf("Terminal reconciliation has trailing content")
		}
	}
	return r, nil
}

func terminalList(lines []string, i int, boundary string, allowNone bool) ([]string, int, error) {
	var values []string
	seen := map[string]bool{}
	for ; i < len(lines); i++ {
		line := lines[i]
		if boundary != "" && line == boundary {
			if len(values) == 0 {
				return nil, i, fmt.Errorf("terminal reconciliation has empty list before %s", boundary)
			}
			return values, i + 1, nil
		}
		if boundary == "" && strings.TrimSpace(line) == "" {
			break
		}
		if !strings.HasPrefix(line, "- ") {
			return nil, i, fmt.Errorf("Terminal reconciliation requires list entries")
		}
		value := strings.TrimPrefix(line, "- ")
		if boundary != "" {
			path, err := strconv.Unquote(value)
			if err != nil || strconv.Quote(path) != value {
				return nil, i, fmt.Errorf("Terminal reconciliation touched paths must use canonical quoted strings")
			}
			value = path
		} else {
			value = strings.TrimSpace(value)
		}
		if value == "" || seen[value] {
			return nil, i, fmt.Errorf("Terminal reconciliation has empty or duplicate list entry")
		}
		seen[value] = true
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil, i, fmt.Errorf("Terminal reconciliation has empty list")
	}
	if allowNone && len(values) == 1 && values[0] == "none" {
		return values, i, nil
	}
	for _, value := range values {
		if value == "none" {
			return nil, i, fmt.Errorf("Terminal reconciliation mixes none with material deviations")
		}
	}
	return values, i, nil
}
