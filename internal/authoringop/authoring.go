// Package authoringop owns semantic part resolution, complete candidate
// validation, confined source publication, and ordinary leased synchronization.
package authoringop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// Mode is one closed part-authoring operation.
type Mode string

const (
	Edit  Mode = "edit"
	Reset Mode = "reset"
)

// Request identifies one semantic part. Content is meaningful only for Edit;
// its empty value is a valid explicit authored override.
type Request struct {
	Mode             Mode
	Kind, Name, Part string
	Content          []byte
	Sidecar          bool
	SidecarMode      string
	Value            any
}

// LeaseAcquirer is the narrow mechanism seam used by focused lease tests.
type LeaseAcquirer func(context.Context, string) (*filesystem.Lease, func() error, error)

// Run validates and applies one authored source change while holding the
// complete project lease from the first mutable authority read through sync.
func Run(ctx context.Context, root string, request Request, loader *project.Loader, acquire LeaseAcquirer) (outcome Outcome, returnErr error) {
	if loader == nil {
		return Outcome{}, errors.New("authoring operation requires a project loader")
	}
	if request.Mode != Edit && request.Mode != Reset {
		return Outcome{}, fmt.Errorf("unknown authoring mode %q", request.Mode)
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
		return Outcome{}, errors.New("authoring operation requires a covering project lease")
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
	return runLeased(ctx, root, request, loader, lease, files)
}

func runLeased(ctx context.Context, root string, request Request, loader *project.Loader, lease *filesystem.Lease, files *filesystem.Handle) (outcome Outcome, returnErr error) {
	session, identity, err := loader.LoadForMutation(ctx, root, files)
	if err != nil {
		return Outcome{}, fmt.Errorf("load authoring authority: %w", err)
	}
	defer identity.Release() //nolint:errcheck // descriptor cleanup only
	cfg := session.Config()
	var target project.AuthoringTarget
	if request.Sidecar {
		target, err = ResolveSidecar(session, cfg, request.Kind, request.Name, request.Part)
	} else {
		target, err = ResolvePart(session, cfg, request.Kind, request.Name, request.Part)
	}
	if err != nil {
		return Outcome{}, err
	}
	outcome = Outcome{Kind: target.Kind, Name: target.Name, Part: target.Part, SourcePath: target.SourcePath, Source: SourceNone}

	sourceIdentity, identityErr := files.ExpectedIdentity(target.SourcePath)
	if identityErr != nil && !errors.Is(identityErr, fs.ErrNotExist) {
		return outcome, fmt.Errorf("observe authoring source %s: %w", target.SourcePath, identityErr)
	}
	if errors.Is(identityErr, fs.ErrNotExist) {
		sourceIdentity = nil
	}
	if sourceIdentity != nil {
		defer sourceIdentity.Release() //nolint:errcheck // consumed by exact mutation or descriptor cleanup
	}
	var before []byte
	mode := fs.FileMode(0o644)
	if sourceIdentity != nil {
		before, mode, err = files.ReadExpected(target.SourcePath, sourceIdentity)
		if err != nil {
			return outcome, fmt.Errorf("read observed authoring source %s: %w", target.SourcePath, err)
		}
	}
	if target.Local && sourceIdentity == nil {
		return outcome, fmt.Errorf("configured local document output %s is absent; run awf render first", target.SourcePath)
	}

	var candidate []byte
	var present, changed bool
	if request.Sidecar {
		candidate, present, changed, err = config.EditSidecar(before, config.SidecarEdit{Field: request.Part, Mode: request.SidecarMode, Value: request.Value})
	} else {
		candidate, present, err = candidateSource(request, target.Local, before)
		changed = true
	}
	if err != nil {
		return outcome, err
	}
	overlay := candidateOverlay{base: publisher.NewFilesystemReader(root), path: target.SourcePath, bytes: slices.Clone(candidate), present: present}
	candidateConfig, err := config.ParseTree(config.RootDir(root), cfg.Source(), configOverlay{tree: overlay})
	if err != nil {
		return outcome, fmt.Errorf("validate candidate config tree: %w", err)
	}
	candidateSession, err := loader.WithSelection(func(string) (*config.Config, error) { return candidateConfig, nil }, overlay).Load(ctx, root)
	if err != nil {
		return outcome, fmt.Errorf("validate candidate project authority: %w", err)
	}
	if _, err := publisher.New(candidateSession, project.Version).Plan(); err != nil {
		return outcome, fmt.Errorf("validate complete candidate project: %w", err)
	}

	if request.Sidecar && !changed {
		return synchronize(ctx, root, target.SourcePath, loader, lease, outcome)
	}
	if !target.Local && (request.Mode == Edit || request.Sidecar) {
		parents := missingParents(files, path.Dir(target.SourcePath))
		if err := files.MkdirAll(path.Dir(target.SourcePath), 0o755); err != nil {
			outcome.CreatedParents = existingDirectories(files, parents)
			return outcome, fmt.Errorf("create parent for authoring source %s: %w", target.SourcePath, err)
		}
		outcome.CreatedParents = existingDirectories(files, parents)
	}

	switch {
	case target.Local:
		err = files.ReplaceExpected(target.SourcePath, sourceIdentity, candidate, mode)
		if err == nil {
			outcome.Source = SourceLocalBody
		}
	case request.Sidecar && !present && sourceIdentity != nil:
		err = files.RemoveExpected(target.SourcePath, sourceIdentity)
		if err == nil {
			outcome.Source = SourceRemoved
		}
	case request.Mode == Edit || request.Sidecar:
		err = files.ReplaceExpected(target.SourcePath, sourceIdentity, candidate, mode)
		if err == nil {
			if sourceIdentity == nil {
				outcome.Source = SourceCreated
			} else {
				outcome.Source = SourceReplaced
			}
		}
	case sourceIdentity != nil:
		err = files.RemoveExpected(target.SourcePath, sourceIdentity)
		if err == nil {
			outcome.Source = SourceRemoved
		}
	}
	if err != nil {
		return outcome, fmt.Errorf("mutate observed authoring source %s: %w", target.SourcePath, err)
	}
	return synchronize(ctx, root, target.SourcePath, loader, lease, outcome)
}

func synchronize(ctx context.Context, root, sourcePath string, loader *project.Loader, lease *filesystem.Lease, outcome Outcome) (Outcome, error) {
	fresh, err := loader.Load(ctx, root)
	if err != nil {
		return outcome, fmt.Errorf("reload committed authoring source %s: %w", sourcePath, err)
	}
	result, err := publisher.New(fresh, project.Version).SyncLeased(ctx, lease)
	outcome.Publisher = result
	if err != nil {
		return outcome, fmt.Errorf("publish committed authoring source %s: %w", sourcePath, err)
	}
	return outcome, nil
}

func candidateSource(request Request, local bool, before []byte) ([]byte, bool, error) {
	if local {
		body := ""
		if request.Mode == Edit {
			body = string(request.Content)
		}
		updated, err := publisher.ReplaceLocalDocumentBody(before, body)
		return updated, true, err
	}
	if request.Mode == Reset {
		return nil, false, nil
	}
	return slices.Clone(request.Content), true, nil
}

func missingParents(files *filesystem.Handle, directory string) []string {
	if directory == "." {
		return nil
	}
	var missing []string
	for current := directory; current != "."; current = path.Dir(current) {
		if _, err := files.LinkInfo(current); errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, current)
		}
	}
	slices.Reverse(missing)
	return missing
}

func existingDirectories(files *filesystem.Handle, candidates []string) []string {
	var existing []string
	for _, candidate := range candidates {
		if info, err := files.LinkInfo(candidate); err == nil && info.IsDir() {
			existing = append(existing, candidate)
		}
	}
	return existing
}

// candidateOverlay supplies the same candidate to project and config readers.
type candidateOverlay struct {
	base    outputplan.TreeReader
	path    string
	bytes   []byte
	present bool
}

func (r candidateOverlay) ReadFile(name string) ([]byte, bool, error) {
	if filepath.ToSlash(name) == r.path {
		return slices.Clone(r.bytes), r.present, nil
	}
	return r.base.ReadFile(name)
}
func (r candidateOverlay) Paths(prefix string) ([]string, error) {
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

type configOverlay struct{ tree candidateOverlay }

func (r configOverlay) ReadFile(name string) ([]byte, bool) {
	bytes, found, err := r.tree.ReadFile(filepath.ToSlash(filepath.Join(config.DirName, name)))
	return bytes, found && err == nil
}
func (r configOverlay) Paths(prefix string) []string {
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
