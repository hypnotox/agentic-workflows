package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	"github.com/hypnotox/agentic-workflows/internal/prosegate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// runProseGate selects the prose step from the shared repository-check plan.
func runProseGate(ctx context.Context, root string, stdout io.Writer) error {
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepProse}, execution.StopOnFailure, false, productionRepoCheckDependencies())
}

func proseCheckFindings(cfg *config.Config, tree *snapshot.Tree) ([]checkFinding, error) {
	if cfg.ProseGate == nil || !cfg.ProseGate.Enabled {
		return []checkFinding{{severity: "warn", check: "prose", detail: "disabled (proseGate.enabled)"}}, nil
	}
	exemptions := make([]prosegate.Exemption, 0, len(cfg.ProseGate.Exemptions))
	for _, e := range cfg.ProseGate.Exemptions {
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
	findings, skipped, err := prosegate.Scan(files, exemptions)
	if err != nil { // coverage-ignore: Scan only traverses in-memory bytes and already parsed exemptions
		return nil, fmt.Errorf("check repo prose: %w", err)
	}
	result := make([]checkFinding, 0, len(findings)+len(skipped))
	for _, path := range skipped {
		result = append(result, checkFinding{severity: "warn", check: "prose", detail: "skipped binary: " + path})
	}
	for _, finding := range findings {
		result = append(result, checkFinding{severity: "error", check: "prose", detail: prosegate.Format(finding)})
	}
	if len(findings) > 0 {
		return result, errors.New("check repo prose: use plain punctuation, or exempt the path in proseGate.exemptions")
	}
	return result, nil
}
