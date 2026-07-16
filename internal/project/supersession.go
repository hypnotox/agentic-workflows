package project

import (
	"fmt"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// checkSupersessionAll runs the ADR-0120 corpus checks over the decisions dir.
// In this phase it parses the corpus and runs only the Decision-format check;
// the supersession checks join it next.
func (p *Project) checkSupersessionAll() ([]manifest.Drift, error) {
	adrs, err := adr.ParseDir(p.decisionsDir())
	if err != nil { // reachable via a direct call over a malformed ADR; pre-empted inside full Check()
		return nil, err
	}
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "decisions"))
	return p.checkDecisionFormat(adrs, rel), nil
}

// checkDecisionFormat enforces ADR-0120 item 12: every ADR's Decision section
// consists of column-0 numbered items, sequential from 1, regardless of
// status - a Superseded ADR can still be an anchor target.
// touches-invariant: decision-items-enumerable - the format check itself; proof in supersession_test.go
func (p *Project) checkDecisionFormat(adrs []adr.ADR, rel string) []manifest.Drift {
	var drift []manifest.Drift
	for _, a := range adrs {
		items := a.DecisionItems()
		if len(items) == 0 {
			drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-decision-format", Detail: fmt.Sprintf("ADR-%s: Decision section has no column-0 numbered items", a.Number)})
			continue
		}
		for i, n := range items {
			if n != i+1 {
				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-decision-format", Detail: fmt.Sprintf("ADR-%s: Decision item %d found where %d expected (gap, duplicate, or restart)", a.Number, n, i+1)})
				break
			}
		}
	}
	return drift
}
