// Package domainop owns configured-domain mutation, authored scaffold, synchronization, and orphan inspection.
package domainop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

const currentStateStub = "Describe where the %q domain stands today: its current shape, load-bearing constraints, and what a newcomer must know before changing it. Refresh by hand when the position materially shifts. Follow `docs/doc-standard.md` for tone: terse, present tense, reference other docs rather than restate them.\n"

// Outcome retains visible domain and publication facts even on an ordinary error.
type Outcome struct {
	ConfigReplaced, ScaffoldCreated, Orphaned bool
	Publisher                                 publisher.Result
}

// LeaseAcquirer is the narrow mechanism seam used by focused lease tests.
type LeaseAcquirer func(context.Context, string) (*filesystem.Lease, func() error, error)

func (o Outcome) Document() (presentation.Document, error) {
	fields := []presentation.Field{}
	for _, fact := range []struct {
		label   string
		changed bool
	}{{"config replacement", o.ConfigReplaced}, {"authored scaffold", o.ScaffoldCreated}, {"orphaned authored inputs", o.Orphaned}} {
		value, err := presentation.Literal(fmt.Sprintf("%t", fact.changed))
		if err != nil {
			return presentation.Document{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil {
			return presentation.Document{}, err
		}
		fields = append(fields, field)
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
	return (presentation.Mutation{Status: "domain mutation completed", Identity: fields, Changes: changes}).Document()
}

func changePaths(result publisher.Result) []string {
	changes := result.Changes()
	paths := make([]string, len(changes))
	for i := range changes {
		paths[i] = changes[i].Path
	}
	return paths
}

// Add configures name, creates its initial current-state part, reloads authority,
// and synchronizes once under a complete project lease.
func Add(ctx context.Context, root, name string, loader *project.Loader, acquire LeaseAcquirer) (outcome Outcome, returnErr error) {
	if err := config.ValidateDomainName(name); err != nil {
		return Outcome{}, err
	}
	return run(ctx, root, loader, acquire, func(files *filesystem.Handle, session *project.Session, identity *filesystem.ExpectedIdentity, lease *filesystem.Lease) (Outcome, error) {
		cfg := session.Config()
		configured := slices.Contains(cfg.Domains, name)
		updated := cfg.Source()
		var err error
		if !configured {
			updated, err = config.SetArrayMember(cfg.Source(), "domains", name, true)
		}
		if err != nil {
			return Outcome{}, err
		}
		path, content, err := currentStatePlan(root, cfg, name)
		if err != nil {
			return Outcome{}, err
		}
		existing, err := observeExclusiveOrExact(files, path, content)
		if err != nil {
			return Outcome{}, err
		}
		if existing != nil {
			defer func() { _ = existing.Release() }()
		}
		if err := validateCandidate(ctx, root, loader, updated, sourceOverlay{base: publisher.NewFilesystemReader(root), path: path, bytes: content, present: true}, lease); err != nil {
			return Outcome{}, err
		}
		result := Outcome{}
		if !configured {
			if err := files.ReplaceExpected(".awf/config.yaml", identity, updated, 0o644); err != nil {
				return result, fmt.Errorf("replace observed config %s: %w", config.ConfigPath(root), err)
			}
			result.ConfigReplaced = true
		}
		created, err := createExclusiveOrExact(files, path, content, existing)
		result.ScaffoldCreated = created
		if err != nil {
			return result, err
		}
		return result, nil
	})
}

// Remove unconfigures name, synchronizes once, and reports remaining authored inputs.
func Remove(ctx context.Context, root, name string, loader *project.Loader, acquire LeaseAcquirer) (outcome Outcome, returnErr error) {
	if err := config.ValidateDomainName(name); err != nil {
		return Outcome{}, err
	}
	return run(ctx, root, loader, acquire, func(files *filesystem.Handle, session *project.Session, identity *filesystem.ExpectedIdentity, lease *filesystem.Lease) (Outcome, error) {
		cfg := session.Config()
		configured := slices.Contains(cfg.Domains, name)
		updated := cfg.Source()
		var err error
		if configured {
			updated, err = config.SetArrayMember(cfg.Source(), "domains", name, false)
		}
		if err != nil {
			return Outcome{}, err
		}
		orphaned, err := hasSidecarOrParts(files, name)
		if err != nil {
			return Outcome{}, err
		}
		if err := validateCandidate(ctx, root, loader, updated, publisher.NewFilesystemReader(root), lease); err != nil {
			return Outcome{}, err
		}
		result := Outcome{Orphaned: orphaned}
		if !configured {
			return result, nil
		}
		if err := files.ReplaceExpected(".awf/config.yaml", identity, updated, 0o644); err != nil {
			return result, fmt.Errorf("replace observed config %s: %w", config.ConfigPath(root), err)
		}
		result.ConfigReplaced = true
		return result, nil
	})
}

type mutation func(*filesystem.Handle, *project.Session, *filesystem.ExpectedIdentity, *filesystem.Lease) (Outcome, error)

func run(ctx context.Context, root string, loader *project.Loader, acquire LeaseAcquirer, mutate mutation) (outcome Outcome, returnErr error) {
	if loader == nil {
		return Outcome{}, errors.New("domain operation requires a project loader")
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
		return Outcome{}, errors.New("domain operation requires a covering project lease")
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
		return Outcome{}, fmt.Errorf("load domain authority: %w", err)
	}
	defer identity.Release() //nolint:errcheck
	outcome, err = mutate(files, session, identity, lease)
	if err != nil {
		return outcome, err
	}
	fresh, err := loader.Load(ctx, root)
	if err != nil {
		return outcome, fmt.Errorf("reload committed domain config %s: %w", config.ConfigPath(root), err)
	}
	result, err := publisher.New(fresh, project.Version).SyncLeased(ctx, lease)
	outcome.Publisher = result
	if err != nil {
		return outcome, fmt.Errorf("publish committed domain config %s: %w", config.ConfigPath(root), err)
	}
	return outcome, nil
}

func validateCandidate(ctx context.Context, root string, loader *project.Loader, updated []byte, tree outputplan.TreeReader, lease *filesystem.Lease) error {
	candidateConfig, err := config.ParseTree(config.RootDir(root), updated, configTreeReader{tree: tree})
	if err != nil {
		return fmt.Errorf("validate candidate domain config: %w", err)
	}
	candidate, err := loader.WithSelection(func(string) (*config.Config, error) { return candidateConfig, nil }, tree).Load(ctx, root)
	if err != nil {
		return fmt.Errorf("validate candidate domain authority: %w", err)
	}
	if err := publisher.New(candidate, project.Version).PreflightSyncLeased(ctx, lease); err != nil {
		return fmt.Errorf("preflight complete candidate domain project: %w", err)
	}
	return nil
}

type sourceOverlay struct {
	base    outputplan.TreeReader
	path    string
	bytes   []byte
	present bool
}

func (r sourceOverlay) ReadFile(name string) ([]byte, bool, error) {
	if filepath.ToSlash(name) == r.path {
		return slices.Clone(r.bytes), r.present, nil
	}
	return r.base.ReadFile(name)
}
func (r sourceOverlay) Paths(prefix string) ([]string, error) {
	paths, err := r.base.Paths(prefix)
	if err != nil {
		return nil, err
	}
	paths = slices.DeleteFunc(paths, func(value string) bool { return value == r.path })
	if r.present && (prefix == "" || r.path == prefix || strings.HasPrefix(r.path, strings.TrimSuffix(prefix, "/")+"/")) {
		paths = append(paths, r.path)
	}
	slices.Sort(paths)
	return slices.Compact(paths), nil
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

func currentStatePlan(root string, cfg *config.Config, name string) (string, []byte, error) {
	path, err := filepath.Rel(root, cfg.PartPath("domains", name, "current-state"))
	if err != nil {
		return "", nil, err
	}
	return filepath.ToSlash(path), fmt.Appendf(nil, currentStateStub, name), nil
}

func observeExclusiveOrExact(files *filesystem.Handle, path string, intended []byte) (*filesystem.ExpectedIdentity, error) {
	info, err := files.LinkInfo(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect authored domain path %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("authored domain path collision %s", path)
	}
	identity, err := files.ExpectedIdentity(path)
	if err != nil {
		return nil, fmt.Errorf("observe authored domain path %s: %w", path, err)
	}
	got, _, err := files.ReadExpected(path, identity)
	if err != nil {
		_ = identity.Release()
		return nil, fmt.Errorf("read observed authored domain path %s: %w", path, err)
	}
	if !bytes.Equal(got, intended) {
		_ = identity.Release()
		return nil, fmt.Errorf("authored domain path collision %s", path)
	}
	return identity, nil
}

func createExclusiveOrExact(files *filesystem.Handle, path string, intended []byte, existing *filesystem.ExpectedIdentity) (bool, error) {
	if existing != nil {
		got, _, err := files.ReadExpected(path, existing)
		if err != nil {
			return false, fmt.Errorf("recheck observed authored domain path %s: %w", path, err)
		}
		if !bytes.Equal(got, intended) {
			return false, fmt.Errorf("authored domain path collision %s", path)
		}
		return false, nil
	}
	if err := files.MkdirAll(filepath.ToSlash(filepath.Dir(path)), 0o755); err != nil {
		return false, fmt.Errorf("create parent for authored domain path %s: %w", path, err)
	}
	if err := files.Publish(path, intended, 0o644); err != nil {
		return false, fmt.Errorf("create authored domain path %s exclusively: %w", path, err)
	}
	return true, nil
}

func hasSidecarOrParts(files *filesystem.Handle, name string) (bool, error) {
	for _, path := range []string{filepath.ToSlash(filepath.Join(config.DirName, "domains", name+".yaml")), filepath.ToSlash(filepath.Join(config.DirName, "domains", "parts", name))} {
		if _, err := files.LinkInfo(path); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("inspect authored domain path %s: %w", path, err)
		}
	}
	return false, nil
}
