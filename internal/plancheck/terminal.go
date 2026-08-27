package plancheck

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/plan"
)

// TerminalTransition validates the selected before/after plan evidence. An
// already Implemented body is history. A Proposed-to-Implemented transition
// must reconcile the actual selected changed paths and material deviations in
// its parsed Notes; it never trusts prose markers detached from that evidence.
func TerminalTransition(before, after []plan.Plan, changed []string) error {
	old := make(map[string]plan.Plan, len(before))
	for _, p := range before {
		old[p.Filename] = p
	}
	for _, next := range after {
		prior, exists := old[next.Filename]
		if !exists {
			continue
		}
		if prior.IsImplemented() && !bytes.Equal(planBody(prior.Source), planBody(next.Source)) {
			return fmt.Errorf("%s: Implemented plan body changed", next.Path)
		}
		if !prior.IsProposed() || !next.IsImplemented() {
			continue
		}
		if err := reconcileTerminal(next, changed); err != nil {
			return err
		}
	}
	return nil
}

func planBody(source []byte) []byte {
	if !bytes.HasPrefix(source, []byte("---\n")) {
		return source
	}
	if end := bytes.Index(source[4:], []byte("\n---\n")); end >= 0 {
		return source[end+9:]
	}
	return source
}

func reconcileTerminal(p plan.Plan, changed []string) error {
	if len(changed) == 0 {
		return fmt.Errorf("%s: terminal transition has unavailable touched-path evidence", p.Path)
	}
	var outside []string
	for _, changedPath := range changed {
		if changedPath == p.Path {
			continue
		}
		if !plannedPath(p, changedPath) {
			outside = append(outside, changedPath)
		}
	}
	notes := strings.ToLower(p.Notes)
	if !strings.Contains(notes, "touched paths:") {
		return fmt.Errorf("%s: terminal transition lacks touched-path reconciliation", p.Path)
	}
	if !strings.Contains(notes, "material deviations:") {
		return fmt.Errorf("%s: terminal transition lacks material-deviation reconciliation", p.Path)
	}
	for _, changedPath := range changed {
		if changedPath != p.Path && !strings.Contains(p.Notes, changedPath) {
			return fmt.Errorf("%s: terminal reconciliation omits touched path %q", p.Path, changedPath)
		}
	}
	for _, changedPath := range outside {
		if !strings.Contains(p.Notes, changedPath) {
			return fmt.Errorf("%s: terminal reconciliation omits material deviation %q", p.Path, changedPath)
		}
	}
	return nil
}

func plannedPath(p plan.Plan, candidate string) bool {
	for _, phase := range p.Phases {
		for _, task := range phase.Tasks {
			for _, entry := range task.Fields.Paths {
				switch entry.Kind {
				case plan.PathLiteral:
					if entry.Value == candidate {
						return true
					}
				case plan.PathGlob:
					if ok, _ := path.Match(entry.Value, candidate); ok {
						return true
					}
				case plan.PathPathspec:
					// Pathspec syntax is Git-owned and can be non-glob semantic. Its
					// literal payload is reconciled through Notes rather than guessed.
					if entry.Value == candidate {
						return true
					}
				}
			}
		}
	}
	return false
}
