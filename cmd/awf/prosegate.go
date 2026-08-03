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

func runProseAction(stdout io.Writer, cfg *config.Config, tree *snapshot.Tree) error {
	if cfg.ProseGate == nil || !cfg.ProseGate.Enabled {
		fmt.Fprintln(stdout, "note: prose: disabled (proseGate.enabled)")
		return nil
	}
	exemptions := make([]prosegate.Exemption, 0, len(cfg.ProseGate.Exemptions))
	for _, e := range cfg.ProseGate.Exemptions {
		r, err := prosegate.ParseCodepoint(e.Codepoint)
		if err != nil {
			return fmt.Errorf("check repo prose: exemption for %s: %w", e.Path, err)
		}
		exemptions = append(exemptions, prosegate.Exemption{Path: e.Path, Codepoint: r, Count: e.Count})
	}
	blobs := tree.List()
	files := make([]prosegate.File, len(blobs))
	for i, blob := range blobs {
		files[i] = prosegate.File{Path: blob.Path, Bytes: blob.Bytes}
	}
	findings, skipped, err := prosegate.Scan(files, exemptions)
	if err != nil { // coverage-ignore: Scan receives in-memory staged bytes and parsed exemptions and has no fallible operation
		return fmt.Errorf("check repo prose: %w", err)
	}
	for _, path := range skipped {
		fmt.Fprintf(stdout, "skipped binary: %s\n", path)
	}
	for _, finding := range findings {
		fmt.Fprintln(stdout, prosegate.Format(finding))
	}
	if len(findings) > 0 {
		return errors.New("check repo prose: use plain punctuation, or exempt the path in proseGate.exemptions")
	}
	fmt.Fprintln(stdout, "check repo prose: clean")
	return nil
}
