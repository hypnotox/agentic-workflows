package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	"github.com/hypnotox/agentic-workflows/internal/memorycite"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// runMemoryGate selects the memory step from the shared repository-check plan.
func runMemoryGate(ctx context.Context, root string, stdout io.Writer) error {
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepMemory}, execution.StopOnFailure, false, productionRepoCheckDependencies())
}

func runMemoryAction(stdout io.Writer, cfg *config.Config, tree *snapshot.Tree) error {
	if cfg.MemoryCite == nil || !cfg.MemoryCite.Enabled {
		fmt.Fprintln(stdout, "note: memory: disabled (memoryCite.enabled)")
		return nil
	}
	exemptions := make([]memorycite.Exemption, 0, len(cfg.MemoryCite.Exemptions))
	for _, e := range cfg.MemoryCite.Exemptions {
		exemptions = append(exemptions, memorycite.Exemption{Path: e.Path, Count: e.Count})
	}
	d := strings.TrimRight(cfg.DocsDir, "/")
	prefixes := []string{d + "/decisions/", d + "/plans/"}
	var files []memorycite.File
	for _, blob := range tree.List() {
		if !blob.Scannable() {
			continue
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(blob.Path, prefix) {
				files = append(files, memorycite.File{Path: blob.Path, Bytes: blob.Bytes})
				break
			}
		}
	}
	findings := memorycite.Scan(files, exemptions)
	for _, finding := range findings {
		fmt.Fprintln(stdout, memorycite.Format(finding))
	}
	if len(findings) > 0 {
		return errors.New("check repo memory: remove the concrete effort-owned memory citation, name the bare .awf/efforts/ directory, use an angle-bracket slug placeholder, or exempt the path in memoryCite.exemptions")
	}
	fmt.Fprintln(stdout, "check repo memory: clean")
	return nil
}
