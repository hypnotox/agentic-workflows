package main

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/execution"
	"github.com/hypnotox/agentic-workflows/internal/memorycite"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// runMemoryGate selects the memory step from the shared repository-check plan.
func runMemoryGate(ctx context.Context, root string, stdout io.Writer) error {
	return runRepoCheckSelection(ctx, root, stdout, []execution.StepID{repoStepMemory}, execution.StopOnFailure, false, productionRepoCheckDependencies())
}

func memoryCheckFindings(cfg *config.Config, tree *snapshot.Tree) ([]presentation.ReportCategory, error) {
	var configured []config.MemoryExemption
	if cfg.MemoryCite != nil {
		configured = cfg.MemoryCite.Exemptions
	}
	exemptions := make([]memorycite.Exemption, 0, len(configured))
	for _, e := range configured {
		exemptions = append(exemptions, memorycite.Exemption{Path: e.Path, Count: e.Count})
	}
	prefixes := []string{config.DocsDir + "/decisions/", config.DocsDir + "/plans/"}
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
	categories, err := memorycite.Categories(findings)
	if err != nil { // coverage-ignore: Categories receives only scanner Findings and builds every nonempty record from fixed templates
		return nil, err
	}
	if len(findings) > 0 {
		return categories, producedCheckFailure{errors.New("check repo memory: remove the concrete effort-owned memory citation, name the bare .awf/efforts/ directory, use an angle-bracket slug placeholder, or exempt the path in memoryCite.exemptions")}
	}
	return categories, nil
}
