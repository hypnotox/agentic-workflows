// Package localdocop owns local-document declaration, preflight, publication, and synchronization.
package localdocop

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// Outcome retains visible declaration and publication facts even on an ordinary error.
type Outcome struct {
	DocumentPath        string
	DeclarationReplaced bool
	Publisher           publisher.Result
}

// LeaseAcquirer is the narrow mechanism seam used by focused lease tests.
type LeaseAcquirer func(context.Context, string) (*filesystem.Lease, func() error, error)

func (o Outcome) Document() (presentation.Document, error) {
	value, err := presentation.Literal(fmt.Sprintf("%t", o.DeclarationReplaced))
	if err != nil {
		return presentation.Document{}, err
	}
	field, err := presentation.NewField("local-document declaration replacement", value)
	if err != nil {
		return presentation.Document{}, err
	}
	identity := []presentation.Field{field}
	if o.DocumentPath != "" {
		value, err := presentation.Literal(o.DocumentPath)
		if err != nil {
			return presentation.Document{}, err
		}
		field, err := presentation.NewField("local document", value)
		if err != nil {
			return presentation.Document{}, err
		}
		identity = append(identity, field)
	}
	changes := []presentation.MutationChange{}
	for _, group := range []struct {
		label string
		paths []string
	}{{"outputs", changePaths(o.Publisher)}, {"pruned", o.Publisher.Pruned()}} {
		values := []presentation.Value{}
		for _, path := range group.paths {
			value, err := presentation.Literal(path)
			if err != nil {
				return presentation.Document{}, err
			}
			values = append(values, value)
		}
		if len(values) != 0 {
			changes = append(changes, presentation.MutationChange{Label: group.label, Values: values})
		}
	}
	return (presentation.Mutation{Status: "local document created", Identity: identity, Changes: changes}).Document()
}

func changePaths(result publisher.Result) []string {
	changes := result.Changes()
	paths := make([]string, len(changes))
	for i := range changes {
		paths[i] = changes[i].Path
	}
	return paths
}

// Run adds doc after complete collision preflight, replaces the exact observed
// declaration, reloads fresh authority, and synchronizes once under one lease.
func Run(ctx context.Context, root string, doc config.LocalDoc, loader *project.Loader, acquire LeaseAcquirer) (outcome Outcome, returnErr error) {
	if loader == nil {
		return Outcome{}, errors.New("local document operation requires a project loader")
	}
	if acquire == nil {
		acquire = func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
			lease, err := loader.AcquireProjectLease(ctx, root)
			if err != nil {
				return nil, nil, err
			}
			return lease, lease.Release, nil
		}
	}
	lease, release, err := acquire(ctx, root)
	if err != nil {
		return Outcome{}, fmt.Errorf("acquire project lease for %s: %w", root, err)
	}
	if lease == nil || release == nil || !loader.CoversProjectLease(ctx, root, lease) {
		if release != nil {
			_ = release()
		} else if lease != nil {
			_ = lease.Release()
		}
		return Outcome{}, errors.New("local document operation requires a covering project lease")
	}
	defer func() {
		if err := release(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("release project lease for %s: %w", root, err))
		}
	}()
	files, err := filesystem.Open(root)
	if err != nil {
		return Outcome{}, fmt.Errorf("open project root %s: %w", root, err)
	}
	defer func() {
		if err := files.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close project root %s: %w", root, err))
		}
	}()
	session, identity, err := loader.LoadForMutation(ctx, root, files)
	if err != nil {
		return Outcome{}, fmt.Errorf("load local-document authority: %w", err)
	}
	defer identity.Release() //nolint:errcheck
	cfg := session.Config()
	declared := false
	for _, existing := range cfg.LocalDocs {
		if existing.Name != doc.Name {
			continue
		}
		if existing != doc {
			return Outcome{}, fmt.Errorf("local document %q is already declared differently", doc.Name)
		}
		declared = true
		break
	}
	updated := cfg.Source()
	if !declared {
		composed := publisher.New(session, project.Version)
		if err := composed.PreflightLocalDoc(doc); err != nil {
			return Outcome{}, err
		}
		updated, err = config.AppendLocalDoc(cfg.Source(), doc)
		if err != nil {
			return Outcome{}, err
		}
	}
	relative := filepath.ToSlash(filepath.Join("docs", doc.Name+".md"))
	if err := validateCandidate(ctx, root, loader, updated, lease); err != nil {
		return Outcome{}, err
	}
	outcome.DocumentPath = relative
	if !declared {
		if err := files.ReplaceExpected(".awf/config.yaml", identity, updated, 0o644); err != nil {
			return outcome, fmt.Errorf("replace observed local-document declaration %s: %w", config.ConfigPath(root), err)
		}
		outcome.DeclarationReplaced = true
	}
	fresh, err := loader.Load(ctx, root)
	if err != nil {
		return outcome, fmt.Errorf("reload committed local-document declaration %s: %w", config.ConfigPath(root), err)
	}
	result, err := publisher.New(fresh, project.Version).SyncLeased(ctx, lease)
	outcome.Publisher = result
	if err != nil {
		return outcome, fmt.Errorf("publish committed local document %s: %w", relative, err)
	}
	return outcome, nil
}

func validateCandidate(ctx context.Context, root string, loader *project.Loader, updated []byte, lease *filesystem.Lease) error {
	tree := publisher.NewFilesystemReader(root)
	candidateConfig, err := config.ParseTree(config.RootDir(root), updated, configTreeReader{tree: tree})
	if err != nil {
		return fmt.Errorf("validate candidate local-document config: %w", err)
	}
	candidate, err := loader.WithSelection(func(string) (*config.Config, error) { return candidateConfig, nil }, tree).Load(ctx, root)
	if err != nil {
		return fmt.Errorf("validate candidate local-document authority: %w", err)
	}
	if err := publisher.New(candidate, project.Version).PreflightSyncLeased(ctx, lease); err != nil {
		return fmt.Errorf("preflight complete candidate local-document project: %w", err)
	}
	return nil
}

type configTreeReader struct{ tree outputplan.TreeReader }

func (r configTreeReader) ReadFile(name string) ([]byte, bool) {
	bytes, found, err := r.tree.ReadFile(filepath.ToSlash(filepath.Join(config.DirName, name)))
	return bytes, found && err == nil
}
func (r configTreeReader) Paths(prefix string) []string {
	paths, err := r.tree.Paths(filepath.ToSlash(filepath.Join(config.DirName, prefix)))
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, value := range paths {
		if relative, found := strings.CutPrefix(value, config.DirName+"/"); found {
			out = append(out, relative)
		}
	}
	return out
}
