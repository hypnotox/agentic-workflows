// Package domainop owns configured-domain mutation, authored scaffold, synchronization, and orphan inspection.
package domainop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

const currentStateStub = "Describe where the %q domain stands today: its current shape, load-bearing constraints, and what a newcomer must know before changing it. Refresh by hand when the position materially shifts. Follow `docs/doc-standard.md` for tone: terse, present tense, reference other docs rather than restate them.\n"

// Add configures name, creates its initial current-state part, and synchronizes.
func Add(ctx context.Context, root, name string, loader *project.Loader) (presentation.Document, error) {
	if err := config.ValidateDomainName(name); err != nil {
		return presentation.Document{}, err
	}
	_, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return presentation.Document{}, err
	}
	for _, domain := range cfg.Domains {
		if domain == name {
			return presentation.Document{}, fmt.Errorf("domain %q already exists", name)
		}
	}
	updated, err := config.SetArrayMember(cfg.Source(), "domains", name, true)
	if err != nil { // coverage-ignore: Loader parsed and validated this exact domains sequence immediately above
		return presentation.Document{}, err
	}
	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil { // coverage-ignore: the config was just read from this path; failure requires a permission, storage, or concurrent namespace fault
		return presentation.Document{}, err
	}
	if err := scaffoldCurrentState(cfg, name); err != nil {
		return presentation.Document{}, err
	}
	return synchronize(ctx, root, loader)
}

// Remove unconfigures name, synchronizes, and reports whether authored domain inputs remain orphaned.
func Remove(ctx context.Context, root, name string, loader *project.Loader) (presentation.Document, bool, error) {
	if err := config.ValidateDomainName(name); err != nil {
		return presentation.Document{}, false, err
	}
	_, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return presentation.Document{}, false, err
	}
	found := false
	for _, domain := range cfg.Domains {
		found = found || domain == name
	}
	if !found {
		return presentation.Document{}, false, fmt.Errorf("domain %q is not configured", name)
	}
	updated, err := config.SetArrayMember(cfg.Source(), "domains", name, false)
	if err != nil { // coverage-ignore: Loader parsed and validated this exact domains sequence immediately above
		return presentation.Document{}, false, err
	}
	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil { // coverage-ignore: the config was just read from this path; failure requires a permission, storage, or concurrent namespace fault
		return presentation.Document{}, false, err
	}
	document, err := synchronize(ctx, root, loader)
	if err != nil {
		return presentation.Document{}, false, err
	}
	orphaned, err := hasSidecarOrParts(root, name)
	return document, orphaned, err
}

func scaffoldCurrentState(cfg *config.Config, name string) error {
	path := cfg.PartPath("domains", name, "current-state")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, fmt.Appendf(nil, currentStateStub, name), 0o644)
}

func synchronize(ctx context.Context, root string, loader *project.Loader) (presentation.Document, error) {
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil { // coverage-ignore: the operation just wrote a Loader-derived valid config; failure requires concurrent project mutation
		return presentation.Document{}, err
	}
	result, err := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version).Sync()
	if err != nil {
		return presentation.Document{}, err
	}
	mutation, err := result.Mutation()
	if err != nil { // coverage-ignore: a nil Sync error returns a completed result with a mutation
		return presentation.Document{}, err
	}
	return mutation.Document()
}

func hasSidecarOrParts(root, name string) (bool, error) {
	awf := config.RootDir(root)
	for _, path := range []string{filepath.Join(awf, "domains", name+".yaml"), filepath.Join(awf, "domains", "parts", name)} {
		if _, err := os.Stat(path); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("inspect authored domain path %s: %w", filepath.ToSlash(path), err)
		}
	}
	return false, nil
}
