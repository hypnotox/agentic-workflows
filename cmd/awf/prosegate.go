package main

import (
	"context"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	"github.com/hypnotox/agentic-workflows/internal/prosegate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

type proseDependencies struct {
	scan func([]prosegate.File, []prosegate.Exemption) ([]prosegate.Finding, []string, error)
}

func productionProseDependencies() proseDependencies { return proseDependencies{scan: prosegate.Scan} }

// runProseGate selects the prose step from the shared repository-check plan.
func runProseGate(ctx context.Context, root string, stdout io.Writer) error {
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepProse}, execution.StopOnFailure, false, productionRepoCheckDependencies())
}

func proseCheckResult(cfg *config.Config, tree *snapshot.Tree) (checkresult.Result, error) {
	findings, err := scanProse(cfg, tree, productionProseDependencies())
	if err != nil {
		return checkresult.Result{}, err
	}
	return prosegate.Result(findings)
}

func scanProse(cfg *config.Config, tree *snapshot.Tree, dependencies proseDependencies) ([]prosegate.Finding, error) {
	var configured []config.ProseExemption
	if cfg.ProseGate != nil {
		configured = cfg.ProseGate.Exemptions
	}
	exemptions := make([]prosegate.Exemption, 0, len(configured))
	for _, e := range configured {
		r, err := prosegate.ParseCodepoint(e.Codepoint)
		if err != nil {
			return nil, fmt.Errorf("check repo prose: exemption for %s: %w", e.Path, err)
		}
		exemptions = append(exemptions, prosegate.Exemption{Path: e.Path, Codepoint: r, Count: e.Count})
	}
	blobs := tree.List()
	files := make([]prosegate.File, len(blobs))
	for i, blob := range blobs {
		files[i] = prosegate.File{Path: blob.Path, Bytes: blob.Bytes}
	}
	findings, _, err := dependencies.scan(files, exemptions)
	if err != nil {
		return nil, fmt.Errorf("check repo prose: %w", err)
	}
	return findings, nil
}
