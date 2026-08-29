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
}

// LeaseAcquirer is the narrow operation dependency used to retain the complete
// project lease and directly inject release faults without mutable globals.
type LeaseAcquirer func(context.Context, string) (*filesystem.Lease, func() error, error)

// Run executes one complete semantic authoring transaction.
func Run(ctx context.Context, root string, request Request, loader *project.Loader, acquire LeaseAcquirer) (Outcome, error) {
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
		return Outcome{}, err
	}
	if lease == nil || release == nil || !loader.CoversProjectLease(ctx, root, lease) {
		if release != nil {
			_ = release()
		}
		return Outcome{}, errors.New("authoring operation requires a covering project lease")
	}
	outcome, operationErr := runLeased(ctx, root, request, loader, lease)
	operationErr = normalizePostRunError(outcome, operationErr)
	releaseErr := release()
	if releaseErr == nil {
		return outcome, operationErr
	}
	var partial *PartialError
	if errors.As(operationErr, &partial) {
		partial.Cause = errors.Join(partial.Cause, releaseErr)
		return partial.Outcome, partial
	}
	if committed(outcome) {
		return outcome, &PartialError{
			Outcome:  outcome,
			Cause:    errors.Join(operationErr, releaseErr),
			Recovery: recoveryFor(outcome, "remove reported residue first, then rerun awf render after the lease-release fault is repaired"),
		}
	}
	return outcome, errors.Join(operationErr, releaseErr)
}

func committed(outcome Outcome) bool {
	return outcome.Source != SourceNone || len(outcome.CreatedParents) != 0 || len(outcome.Residue) != 0 || outcome.Publisher.HasCommittedEffects()
}

// normalizePostRunError retains committed effects when deferred cleanup turns an
// otherwise successful leased operation into a plain error.
func normalizePostRunError(outcome Outcome, err error) error {
	if err == nil || !committed(outcome) {
		return err
	}
	var existing *PartialError
	if errors.As(err, &existing) {
		return err
	}
	return partial(outcome, err, "repair the post-commit fault, then rerun awf render")
}

func runLeased(ctx context.Context, root string, request Request, loader *project.Loader, lease *filesystem.Lease) (outcome Outcome, returnErr error) {
	files, err := filesystem.Open(root)
	if err != nil {
		return Outcome{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, files.Close()) }()

	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return Outcome{}, err
	}
	target, err := ResolvePart(state, cfg, request.Kind, request.Name, request.Part)
	if err != nil {
		return Outcome{}, err
	}
	outcome = Outcome{Kind: target.Kind, Name: target.Name, Part: target.Part, SourcePath: target.SourcePath, Source: SourceNone}

	identity, identityErr := files.ExpectedIdentity(target.SourcePath)
	if identityErr != nil && !errors.Is(identityErr, fs.ErrNotExist) {
		return outcome, fmt.Errorf("observe authoring source %s: %w", target.SourcePath, identityErr)
	}
	if errors.Is(identityErr, fs.ErrNotExist) {
		identity = nil
	}
	defer identity.Release() //nolint:errcheck // replacement/removal consumes it; cleanup cannot alter the mutation outcome

	var before []byte
	mode := fs.FileMode(0o644)
	if identity != nil {
		before, mode, err = files.ReadExpected(target.SourcePath, identity)
		if err != nil {
			return outcome, err
		}
	}
	if target.Local && identity == nil {
		return outcome, fmt.Errorf("configured local document output %s is absent; run awf render first", target.SourcePath)
	}

	candidateBytes, candidatePresent, err := candidateSource(request, target.Local, before)
	if err != nil {
		return outcome, err
	}
	overlay := candidateOverlay{
		base: publisher.NewFilesystemReader(root), path: target.SourcePath,
		bytes: slices.Clone(candidateBytes), present: candidatePresent,
	}
	candidateConfig, err := config.ParseTree(config.RootDir(root), cfg.Source(), configOverlay{tree: overlay})
	if err != nil {
		return outcome, fmt.Errorf("validate candidate config tree: %w", err)
	}
	candidateLoader := loader.WithConfigLoader(func(string) (*config.Config, error) { return candidateConfig, nil })
	candidateState, candidateConfig, err := candidateLoader.OpenForOperation(ctx, root)
	if err != nil {
		return outcome, fmt.Errorf("validate candidate project authority: %w", err)
	}
	candidatePublisher := publisher.New(candidateState.OutputState(), candidateConfig, overlay, project.Version)
	if _, err := candidatePublisher.Prepare(); err != nil {
		return outcome, fmt.Errorf("validate complete candidate project: %w", err)
	}

	if !target.Local && request.Mode == Edit {
		parents := missingParents(files, path.Dir(target.SourcePath))
		if len(parents) != 0 {
			if err := files.MkdirAll(path.Dir(target.SourcePath), 0o755); err != nil {
				outcome.CreatedParents = existingDirectories(files, parents)
				if committed(outcome) {
					return outcome, partial(outcome, err, "repair the source parent, then retry the edit")
				}
				return outcome, err
			}
			outcome.CreatedParents = existingDirectories(files, parents)
		}
	}

	switch {
	case target.Local:
		err = files.ReplaceExpected(target.SourcePath, identity, candidateBytes, mode)
		if err == nil {
			outcome.Source = SourceLocalBody
		}
	case request.Mode == Edit:
		err = files.ReplaceExpected(target.SourcePath, identity, candidateBytes, mode)
		if err == nil {
			if identity == nil {
				outcome.Source = SourceCreated
			} else {
				outcome.Source = SourceReplaced
			}
		}
	case identity != nil:
		err = files.RemoveExpected(target.SourcePath, identity)
		if err == nil {
			outcome.Source = SourceRemoved
		}
	}
	if err != nil {
		if committedPath, residue, didCommit := filesystem.CommittedPublication(err); didCommit {
			_ = committedPath
			if target.Local {
				outcome.Source = SourceLocalBody
			} else if request.Mode == Edit {
				if identity == nil {
					outcome.Source = SourceCreated
				} else {
					outcome.Source = SourceReplaced
				}
			}
			if residue != "" {
				outcome.Residue = append(outcome.Residue, residue)
			}
		}
		if committed(outcome) {
			return outcome, partial(outcome, err, "remove reported residue first, inspect the committed source, then retry")
		}
		return outcome, err
	}

	committedState, committedConfig, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return outcome, partial(outcome, err, "repair committed source authority, then rerun awf render")
	}
	result, syncErr := publisher.New(committedState.OutputState(), committedConfig, publisher.NewFilesystemReader(root), project.Version).SyncLeased(ctx, lease)
	outcome.Publisher = result
	if syncErr != nil {
		return outcome, partial(outcome, syncErr, "remove reported residue first, repair the publisher fault, then rerun awf render")
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

func partial(outcome Outcome, cause error, action string) *PartialError {
	return &PartialError{Outcome: outcome, Cause: cause, Recovery: recoveryFor(outcome, action)}
}

func recoveryFor(outcome Outcome, action string) []string {
	recovery := []string{}
	if len(outcome.Residue) != 0 {
		recovery = append(recovery, "remove the reported publication residue")
	}
	return append(recovery, action)
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

// candidateOverlay supplies the exact same candidate replacement/removal to
// both the project-tree and config-tree reader contracts.
type candidateOverlay struct {
	base    publisher.ProjectTreeReader
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
	rooted := filepath.ToSlash(filepath.Join(config.DirName, prefix))
	paths, err := r.tree.Paths(rooted)
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
