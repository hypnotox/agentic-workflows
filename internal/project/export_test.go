package project

import (
	"context"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// Sync renders and writes the project like SyncReport, discarding the backup,
// change, and prune reports - a test-only convenience for the many in-package
// tests that only care whether the sync errors. Production uses SyncReport
// directly (ADR-0063).
func (p *Project) Sync() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, found, err := manifest.LoadOptional(lockPath(p.Root))
	if err != nil {
		return err
	}
	if !found {
		_, _, _, err = p.InitializeReport(ctx, InitAuthority{InitializedWithVersion: Version})
		return err
	}
	_, _, _, err = p.SyncReport(ctx)
	return err
}

// RenderAll renders only plan write nodes in deterministic path order - a
// test-only convenience for the many in-package tests that assert over the
// rendered set. Production operations derive their own state once and enter
// through outputPlan, so no production caller remains (ADR-0063, ADR-0180).
func (p *Project) RenderAll() ([]RenderedFile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	op, err := p.OutputPlan(ctx)
	if err != nil {
		return nil, err
	}
	return op.writeFiles(), nil
}

func testTargets(p *Project) []Target {
	return p.state.resolvedTargets()
}

func setTestTargets(p *Project, targets []Target) {
	state := *p.state
	state.targets = cloneTargets(targets)
	p.state = &state
}

func setTestRoots(p *Project, roots resident.Roots) {
	state := *p.state
	state.roots = roots
	p.state = &state
}

func renderInputsForTest(p *Project) renderInputs {
	state := p.state
	if state == nil {
		selected := p.cat
		if selected == nil {
			selected = p.completeCat
		}
		if selected == nil {
			selected = catalog.Standard
		}
		state = &projectState{
			invokingRoot: p.Root,
			roots:        p.roots,
			nested:       p.nested,
			targets:      cloneTargets(p.Targets),
		}
		if selected != nil {
			state.selectedCat = catalog.NewView(selected)
			state.completeCat = catalog.NewView(selected)
		}
	}
	return newRenderInputs(state, p.Cfg, p.read)
}
