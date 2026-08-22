package project

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// plansFromTree parses only regular plan files from one immutable universe.
// The caller supplies that universe's configured docs directory; it never falls
// back to working-tree paths or bytes.
func plansFromTree(tree *snapshot.Tree, docsDir string) ([]plan.Plan, []manifest.Drift, error) {
	prefix := filepath.ToSlash(filepath.Join(docsDir, "plans")) + "/"
	var sources []plan.Source
	for _, file := range tree.List() {
		if !file.Scannable() {
			continue
		}
		name, ok := strings.CutPrefix(file.Path, prefix)
		if !ok || strings.Contains(name, "/") {
			continue
		}
		sources = append(sources, plan.Source{Filename: name, Path: file.Path, Bytes: file.Bytes})
	}
	plans, err := plan.ParseSources(sources)
	if err == nil {
		return plans, nil, nil
	}
	var out []manifest.Drift
	var diagnostics *plan.DiagnosticsError
	if errors.As(err, &diagnostics) {
		for _, diagnostic := range diagnostics.Diagnostics {
			out = append(out, manifest.Drift{Path: filepath.ToSlash(filepath.Join(docsDir, "plans", diagnostic.Path)), Kind: "plan-" + diagnostic.Category, Detail: diagnostic.Detail})
		}
		return plans, out, nil
	}
	return nil, nil, fmt.Errorf("parse staged plans: %w", err) // coverage-ignore: ParseSources converts every parser failure into DiagnosticsError
}
