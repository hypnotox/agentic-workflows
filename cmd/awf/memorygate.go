package main

import (
	"context"
	"errors"
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

func memoryCheckFindings(cfg *config.Config, tree *snapshot.Tree) ([]checkFinding, error) {
	if cfg.MemoryCite == nil || !cfg.MemoryCite.Enabled {
		return []checkFinding{{severity: "warn", check: "memory", detail: "disabled (memoryCite.enabled)"}}, nil
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
	result := make([]checkFinding, len(findings))
	for i, finding := range findings {
		result[i] = checkFinding{severity: "error", check: "memory", detail: memorycite.Format(finding)}
	}
	if len(findings) > 0 {
		return result, errors.New("check repo memory: remove the concrete effort-owned memory citation, name the bare .awf/efforts/ directory, use an angle-bracket slug placeholder, or exempt the path in memoryCite.exemptions")
	}
	return result, nil
}
