// Package localdocop owns local-document declaration, preflight, publication, and synchronization.
package localdocop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// Run adds doc after checking its output collision, then synchronizes and
// returns the synchronization presentation for the command to render.
func Run(ctx context.Context, root string, doc config.LocalDoc, loader *project.Loader) error {
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return err
	}
	composed := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version)
	if err := composed.PreflightLocalDoc(doc); err != nil {
		return err
	}
	source, err := os.ReadFile(config.ConfigPath(root))
	if err != nil { // coverage-ignore: Loader just read this exact config path; failure requires a concurrent namespace or storage fault
		return err
	}
	updated, err := config.AppendLocalDoc(source, doc)
	if err != nil {
		return err
	}
	relative := filepath.ToSlash(filepath.Join("docs", doc.Name+".md"))
	output := filepath.Join(root, "docs", filepath.FromSlash(doc.Name)+".md")
	if _, err := os.Lstat(output); err == nil {
		return fmt.Errorf("local document destination already exists: %s", relative)
	} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: PreflightLocalDoc inspected this exact destination moments earlier; a different result requires concurrent namespace mutation
		return fmt.Errorf("inspect local document destination: %w", err)
	}
	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil { // coverage-ignore: the config was just read from this path; failure requires a permission, storage, or concurrent namespace fault
		return err
	}
	state, cfg, err = loader.OpenForOperation(ctx, root)
	if err != nil { // coverage-ignore: the operation just wrote a Loader-derived valid config; failure requires concurrent project mutation
		return err
	}
	result, err := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version).Sync()
	if err != nil { // coverage-ignore: PreflightLocalDoc prepared and validated this output universe; failure now requires concurrent source or storage mutation
		return err
	}
	mutation, err := result.Mutation()
	if err != nil { // coverage-ignore: a nil Sync error returns a completed result with a mutation
		return err
	}
	_, err = mutation.Document()
	return err
}
