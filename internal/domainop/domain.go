// Package domainop owns configured-domain mutation, authored scaffold, synchronization, and orphan inspection.
package domainop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

const currentStateStub = "Describe where the %q domain stands today: its current shape, load-bearing constraints, and what a newcomer must know before changing it. Refresh by hand when the position materially shifts. Follow `docs/doc-standard.md` for tone: terse, present tense, reference other docs rather than restate them.\n"

// Add configures name, creates its initial current-state part, and synchronizes.
func Add(ctx context.Context, root, name string, loader *project.Loader) (document presentation.Document, err error) {
	if err := config.ValidateDomainName(name); err != nil {
		return presentation.Document{}, err
	}
	lease, err := loader.AcquireProjectLease(ctx, root)
	if err != nil {
		return presentation.Document{}, err
	}
	defer func() { err = errors.Join(err, lease.Release()) }()
	files, err := filesystem.Open(root)
	if err != nil {
		return presentation.Document{}, err
	}
	defer files.Close()
	_, cfg, configIdentity, err := loader.OpenForMutation(ctx, root, files)
	if err != nil {
		return presentation.Document{}, err
	}
	for _, domain := range cfg.Domains {
		if domain == name {
			return presentation.Document{}, fmt.Errorf("domain %q already exists", name)
		}
	}
	updated, err := config.SetArrayMember(cfg.Source(), "domains", name, true)
	if err != nil {
		return presentation.Document{}, err
	}
	if err := replaceConfig(files, configIdentity, updated); err != nil {
		return presentation.Document{}, err
	}
	if err := scaffoldCurrentStateConfined(files, root, cfg, name); err != nil {
		return presentation.Document{}, err
	}
	return synchronize(ctx, root, loader, lease)
}

// Remove unconfigures name, synchronizes, and reports whether authored domain inputs remain orphaned.
func Remove(ctx context.Context, root, name string, loader *project.Loader) (document presentation.Document, orphaned bool, err error) {
	if err := config.ValidateDomainName(name); err != nil {
		return presentation.Document{}, false, err
	}
	lease, err := loader.AcquireProjectLease(ctx, root)
	if err != nil {
		return presentation.Document{}, false, err
	}
	defer func() { err = errors.Join(err, lease.Release()) }()
	files, err := filesystem.Open(root)
	if err != nil {
		return presentation.Document{}, false, err
	}
	defer files.Close()
	_, cfg, configIdentity, err := loader.OpenForMutation(ctx, root, files)
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
	if err != nil {
		return presentation.Document{}, false, err
	}
	if err := replaceConfig(files, configIdentity, updated); err != nil {
		return presentation.Document{}, false, err
	}
	document, err = synchronize(ctx, root, loader, lease)
	if err != nil {
		return presentation.Document{}, false, err
	}
	orphaned, err = hasSidecarOrParts(root, name)
	return document, orphaned, err
}

func replaceConfig(files *filesystem.Handle, expected fs.FileInfo, updated []byte) error {
	return files.ReplaceExpected(".awf/config.yaml", expected, updated, 0o644)
}

func scaffoldCurrentStateConfined(files *filesystem.Handle, root string, cfg *config.Config, name string) error {
	path, err := filepath.Rel(root, cfg.PartPath("domains", name, "current-state"))
	if err != nil {
		return err
	}
	path = filepath.ToSlash(path)
	if err := files.MkdirAll(filepath.ToSlash(filepath.Dir(path)), 0o755); err != nil {
		return err
	}
	if err := files.Publish(path, fmt.Appendf(nil, currentStateStub, name), 0o644); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		info, inspectErr := files.LinkInfo(path)
		if inspectErr != nil {
			return inspectErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("current-state scaffold path %q is a symlink", path)
		}
	}
	return nil
}

func synchronize(ctx context.Context, root string, loader *project.Loader, leases ...*filesystem.Lease) (presentation.Document, error) {
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return presentation.Document{}, err
	}
	composed := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version)
	var result publisher.Result
	if len(leases) == 0 || leases[0] == nil {
		result, err = composed.Sync()
	} else {
		result, err = composed.SyncLeased(ctx, leases[0])
	}
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
