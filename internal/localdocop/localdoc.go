// Package localdocop owns local-document declaration, preflight, publication, and synchronization.
package localdocop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// Run adds doc after checking its output collision, then synchronizes and
// returns the synchronization presentation for the command to render.
func Run(ctx context.Context, root string, doc config.LocalDoc, loader *project.Loader) (err error) {
	lease, err := loader.AcquireProjectLease(ctx, root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lease.Release()) }()
	files, err := filesystem.Open(root)
	if err != nil {
		return err
	}
	defer files.Close()
	state, cfg, configIdentity, err := loader.OpenForMutation(ctx, root, files)
	if err != nil {
		return err
	}
	composed := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version)
	if err := composed.PreflightLocalDoc(doc); err != nil {
		return err
	}
	updated, err := config.AppendLocalDoc(cfg.Source(), doc)
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
	if err := files.ReplaceExpected(".awf/config.yaml", configIdentity, updated, 0o644); err != nil {
		return err
	}
	state, cfg, err = loader.OpenForOperation(ctx, root)
	if err != nil { // coverage-ignore: the operation just wrote a Loader-derived valid config; failure requires concurrent project mutation
		return err
	}
	result, err := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version).SyncLeased(ctx, lease)
	if err != nil {
		return err
	}
	mutation, err := result.Mutation()
	if err != nil { // coverage-ignore: a nil Sync error returns a completed result with a mutation
		return err
	}
	_, err = mutation.Document()
	return err
}
