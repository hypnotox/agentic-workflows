// Package pitfallop owns the focused authored-pitfall creation operation.
package pitfallop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
)

// Outcome retains every committed fact from one authored-pitfall creation.
type Outcome struct {
	SourcePath  string
	ResiduePath string
}

// PartialError retains a committed authored source and any cleanup residue
// together with the original post-commit cause and safe recovery policy.
type PartialError struct {
	projectmutation.Partial[Outcome]
}

func newPartial(outcome Outcome, cause error, recovery ...string) *PartialError {
	return &PartialError{Partial: projectmutation.NewPartial(outcome, cause, recovery)}
}

func committed(outcome Outcome) bool { return outcome.SourcePath != "" }

func failure(phase projectmutation.Phase, cause error) error {
	if cause == nil || containsPhase(cause, phase) {
		return cause
	}
	return &projectmutation.Failure{Phase: phase, Cause: cause}
}

func containsPhase(err error, phase projectmutation.Phase) bool {
	if err == nil {
		return false
	}
	var typed *projectmutation.Failure
	if errors.As(err, &typed) && typed.Phase == phase {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range many.Unwrap() {
			if containsPhase(cause, phase) {
				return true
			}
		}
		return false
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return containsPhase(one.Unwrap(), phase)
	}
	return false
}

// Finish classifies caller-owned lease release after Create has returned and
// before either the complete or partial document is rendered.
func Finish(outcome Outcome, operationErr, releaseErr error) error {
	if releaseErr != nil {
		releaseErr = failure(projectmutation.PhaseRelease, releaseErr)
	}
	var partial *PartialError
	if errors.As(operationErr, &partial) {
		if releaseErr != nil {
			partial.JoinCause(releaseErr)
		}
		partial.Recovery = recoveryFor(outcome, containsPhase(operationErr, projectmutation.PhaseCleanup), releaseErr != nil)
		return operationErr
	}
	if releaseErr != nil {
		cause := errors.Join(operationErr, releaseErr)
		if committed(outcome) {
			return newPartial(outcome, cause, recoveryFor(outcome, containsPhase(operationErr, projectmutation.PhaseCleanup), true)...)
		}
		return cause
	}
	if operationErr != nil && committed(outcome) {
		return newPartial(outcome, operationErr, recoveryFor(outcome, containsPhase(operationErr, projectmutation.PhaseCleanup), false)...)
	}
	return operationErr
}

func recoveryFor(outcome Outcome, cleanup, release bool) []string {
	recovery := []string{"inspect and treat the authored source " + outcome.SourcePath + " as committed"}
	if outcome.ResiduePath != "" {
		recovery = append(recovery, "remove publication cleanup residue "+outcome.ResiduePath+" before further project mutation")
	} else if cleanup {
		recovery = append(recovery, "repair the post-commit cleanup fault before further project mutation")
	}
	if release {
		recovery = append(recovery, "repair the lease-release fault before further project mutation")
	}
	if outcome.ResiduePath == "" && !cleanup && !release {
		recovery = append(recovery, "repair the post-commit fault before further project mutation")
	}
	return append(recovery, "do not rerun awf new pitfall with the same title; the committed duplicate will be refused")
}

// Document returns the complete owner-produced creation report.
func (o Outcome) Document() (presentation.Document, error) {
	statusValue, err := presentation.Prose("pitfall created")
	if err != nil {
		return presentation.Document{}, err
	}
	status, err := presentation.NewField("status", statusValue)
	if err != nil {
		return presentation.Document{}, err
	}
	pathValue, err := presentation.Literal(o.SourcePath)
	if err != nil {
		return presentation.Document{}, err
	}
	authoredPath, err := presentation.NewField("authored path", pathValue)
	if err != nil {
		return presentation.Document{}, err
	}
	return presentation.NewDocument(status, authoredPath)
}

// Document returns the owner-produced partial report with exact committed and
// residue paths plus ordered recovery actions.
func (e *PartialError) Document() (presentation.Document, error) {
	pathValue, err := presentation.Literal(e.Outcome.SourcePath)
	if err != nil {
		return presentation.Document{}, err
	}
	pathField, err := presentation.NewField("committed authored path", pathValue)
	if err != nil {
		return presentation.Document{}, err
	}
	identity := []presentation.Field{pathField}
	if e.Outcome.ResiduePath != "" {
		residueValue, err := presentation.Literal(e.Outcome.ResiduePath)
		if err != nil {
			return presentation.Document{}, err
		}
		residue, err := presentation.NewField("cleanup residue", residueValue)
		if err != nil {
			return presentation.Document{}, err
		}
		identity = append(identity, residue)
	}
	recovery := e.Recovery
	if len(recovery) == 0 {
		recovery = recoveryFor(e.Outcome, e.Outcome.ResiduePath != "", false)
	}
	next := make([]presentation.Value, 0, len(recovery))
	for _, action := range recovery {
		value, err := presentation.Prose(action)
		if err != nil {
			return presentation.Document{}, err
		}
		next = append(next, value)
	}
	return (presentation.Mutation{Status: "pitfall scaffold partially committed", Identity: identity, NextActions: next}).Document()
}

type scaffoldFilesystem interface {
	LinkInfo(string) (fs.FileInfo, error)
	Walk(string, func(string, fs.FileInfo) (bool, error)) error
	Read(string) ([]byte, error)
	MkdirAll(string, fs.FileMode) error
	Publish(string, []byte, fs.FileMode) error
}

func loadCorpus(tree scaffoldFilesystem) (pitfall.Corpus, error) {
	info, err := tree.LinkInfo(pitfall.SourceDir)
	if errors.Is(err, fs.ErrNotExist) {
		return pitfall.Load(nil)
	}
	if err != nil {
		return pitfall.Corpus{}, fmt.Errorf("inspect pitfall source root %s: %w", pitfall.SourceDir, err)
	}
	if !info.IsDir() {
		return pitfall.Corpus{}, fmt.Errorf("pitfall source root %s is not a directory", pitfall.SourceDir)
	}
	var files []pitfall.SourceFile
	err = tree.Walk(pitfall.SourceDir, func(source string, info fs.FileInfo) (bool, error) {
		if source == pitfall.SourceDir {
			return true, nil
		}
		if info.IsDir() {
			return true, nil
		}
		file := pitfall.SourceFile{Path: source, Regular: info.Mode().IsRegular()}
		if file.Regular {
			file.Bytes, err = tree.Read(source)
			if err != nil {
				return false, fmt.Errorf("read pitfall source %s: %w", source, err)
			}
		}
		files = append(files, file)
		return false, nil
	})
	if err != nil {
		return pitfall.Corpus{}, err
	}
	return pitfall.Load(files)
}

func createConfined(title string, files scaffoldFilesystem) (Outcome, error) {
	corpus, err := loadCorpus(files)
	if err != nil {
		return Outcome{}, failure(projectmutation.PhaseAuthority, err)
	}
	entry, source, err := corpus.Scaffold(title)
	if err != nil {
		return Outcome{}, err
	}
	if err := files.MkdirAll(pitfall.SourceDir, 0o755); err != nil {
		return Outcome{}, failure(projectmutation.PhasePublication, fmt.Errorf("create pitfall source directory: %w", err))
	}
	if err := files.Publish(entry.SourcePath, source, 0o644); err != nil {
		var cleanup *filepublication.CommittedCleanupError
		if errors.As(err, &cleanup) {
			outcome := Outcome{SourcePath: entry.SourcePath, ResiduePath: cleanup.ResiduePath}
			cause := failure(projectmutation.PhaseCleanup, err)
			return outcome, newPartial(outcome, cause, recoveryFor(outcome, true, false)...)
		}
		return Outcome{}, failure(projectmutation.PhasePublication, fmt.Errorf("create pitfall source %s exclusively: %w", entry.SourcePath, err))
	}
	return Outcome{SourcePath: entry.SourcePath}, nil
}

func finishClose(outcome Outcome, operationErr, closeErr error) error {
	if closeErr == nil {
		return operationErr
	}
	closeErr = failure(projectmutation.PhaseCleanup, fmt.Errorf("close selected root after pitfall scaffold: %w", closeErr))
	var partial *PartialError
	if errors.As(operationErr, &partial) {
		partial.JoinCause(closeErr)
		partial.Recovery = recoveryFor(outcome, true, containsPhase(operationErr, projectmutation.PhaseRelease))
		return operationErr
	}
	if committed(outcome) {
		return newPartial(outcome, errors.Join(operationErr, closeErr), recoveryFor(outcome, true, false)...)
	}
	return errors.Join(operationErr, closeErr)
}

// Create validates a tracked transaction, selects Session-bound authority, and
// exclusively publishes one authored pitfall through its confined handle. It
// deliberately performs no synchronization or generated-output mutation.
func Create(ctx context.Context, title string, tx *projectmutation.Transaction) (outcome Outcome, err error) {
	return create(ctx, title, tx, nil)
}

func create(_ context.Context, title string, tx *projectmutation.Transaction, afterOpen func()) (outcome Outcome, err error) {
	if tx == nil || tx.Scope() != projectmutation.TrackedScope {
		return Outcome{}, errors.New("pitfall operation requires a covering tracked lease")
	}
	files, err := tx.Open()
	if err != nil {
		return Outcome{}, err
	}
	defer func() { err = finishClose(outcome, err, files.Close()) }()
	if afterOpen != nil {
		afterOpen()
	}
	matches, err := files.RootMatches(tx.Root())
	if err != nil {
		return Outcome{}, failure(projectmutation.PhaseAuthority, err)
	}
	if !matches {
		return Outcome{}, failure(projectmutation.PhaseAuthority, filesystem.ErrIdentityChanged)
	}
	_, configIdentity, err := tx.LoadForMutation(files)
	if err != nil {
		return Outcome{}, err
	}
	defer configIdentity.Release() //nolint:errcheck // descriptor cleanup cannot change the scaffold outcome
	matches, err = files.RootMatches(tx.Root())
	if err != nil {
		return Outcome{}, failure(projectmutation.PhaseAuthority, err)
	}
	if !matches {
		return Outcome{}, failure(projectmutation.PhaseAuthority, filesystem.ErrIdentityChanged)
	}
	return createConfined(title, files)
}
