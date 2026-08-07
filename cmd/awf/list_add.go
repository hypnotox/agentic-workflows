package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

const domainCurrentStateStub = "Describe where the %q domain stands today: its current shape, load-bearing constraints, and what a newcomer must know before changing it. Refresh by hand when the position materially shifts. Follow `docs/doc-standard.md` for tone: terse, present tense, reference other docs rather than restate them.\n"

func scaffoldDomainCurrentState(p *project.Project, name string) error {
	path := p.Cfg.PartPath("domains", name, "current-state")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, fmt.Appendf(nil, domainCurrentStateStub, name), 0o644)
}

type domainDependencies struct {
	open        func(context.Context, string) (*project.Project, error)
	edit        func([]byte, string, string, bool) ([]byte, error)
	write       func(string, []byte, os.FileMode) error
	scaffold    func(*project.Project, string) error
	synchronize func(context.Context, string, io.Writer) error
}

func openDomainProject(ctx context.Context, root string) (*project.Project, error) {
	return project.Open(ctx, root)
}

func syncDomainProject(ctx context.Context, root string, stdout io.Writer) error {
	return runSync(ctx, root, stdout)
}

func productionDomainDependencies() domainDependencies {
	return domainDependencies{
		open:        openDomainProject,
		edit:        config.SetArrayMember,
		write:       os.WriteFile,
		scaffold:    scaffoldDomainCurrentState,
		synchronize: syncDomainProject,
	}
}

func runNewDomain(ctx context.Context, root, name string, stdout io.Writer) error {
	return runNewDomainWith(ctx, root, name, stdout, productionDomainDependencies())
}

func runNewDomainWith(ctx context.Context, root, name string, stdout io.Writer, dependencies domainDependencies) error {
	if err := config.ValidateDomainName(name); err != nil {
		return err
	}
	if err := gate(ctx, root); err != nil {
		return err
	}
	p, err := dependencies.open(ctx, root)
	if err != nil {
		return err
	}
	for _, domain := range p.Cfg.Domains {
		if domain == name {
			return fmt.Errorf("domain %q already exists", name)
		}
	}
	updated, err := dependencies.edit(p.Cfg.Source(), "domains", name, true)
	if err != nil {
		return err
	}
	if err := dependencies.write(config.ConfigPath(root), updated, 0o644); err != nil {
		return err
	}
	if err := dependencies.scaffold(p, name); err != nil {
		return err
	}
	return dependencies.synchronize(ctx, root, stdout)
}

func runRemoveDomain(ctx context.Context, root, name string, stdout io.Writer) error {
	return runRemoveDomainWith(ctx, root, name, stdout, productionDomainDependencies())
}

func runRemoveDomainWith(ctx context.Context, root, name string, stdout io.Writer, dependencies domainDependencies) error {
	if err := config.ValidateDomainName(name); err != nil {
		return err
	}
	if err := gate(ctx, root); err != nil {
		return err
	}
	p, err := dependencies.open(ctx, root)
	if err != nil {
		return err
	}
	found := false
	for _, domain := range p.Cfg.Domains {
		found = found || domain == name
	}
	if !found {
		return fmt.Errorf("domain %q is not configured", name)
	}
	updated, err := dependencies.edit(p.Cfg.Source(), "domains", name, false)
	if err != nil {
		return err
	}
	if err := dependencies.write(config.ConfigPath(root), updated, 0o644); err != nil {
		return err
	}
	if err := dependencies.synchronize(ctx, root, stdout); err != nil {
		return err
	}
	authored, err := hasDomainSidecarOrParts(root, name)
	if err != nil {
		return err
	}
	if authored {
		return writeStatus(stdout, fmt.Sprintf("note: domain %q still has a sidecar or convention parts under .awf/, now orphaned", name))
	}
	return nil
}

func hasDomainSidecarOrParts(root, name string) (bool, error) {
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

func runList(ctx context.Context, root, kindFilter string, stdout io.Writer) error {
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	document, err := p.ListDocument(kindFilter)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
