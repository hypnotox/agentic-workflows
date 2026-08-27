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
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// Outcome retains the declaration replacement and Publisher result for a local-document transaction.
type Outcome struct {
	DocumentPath        string
	DeclarationReplaced bool
	Publisher           publisher.Result
}
type PartialError struct {
	Outcome  Outcome
	Cause    error
	Recovery []string
}

func (e *PartialError) Error() string { return e.Cause.Error() }
func (e *PartialError) Unwrap() error { return e.Cause }
func localDocument(status string, o Outcome, recovery []string) (presentation.Document, error) {
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
		path, err := presentation.Literal(o.DocumentPath)
		if err != nil {
			return presentation.Document{}, err
		}
		document, err := presentation.NewField("local document", path)
		if err != nil {
			return presentation.Document{}, err
		}
		identity = append(identity, document)
	}
	changes := []presentation.MutationChange{}
	for _, effect := range o.Publisher.Effects() {
		value, err := presentation.Literal(fmt.Sprintf("%s %s; recovery: %s", effect.Kind, effect.Path, effect.Recovery))
		if err != nil {
			return presentation.Document{}, err
		}
		changes = append(changes, presentation.MutationChange{Label: "publisher effects", Values: []presentation.Value{value}})
	}
	next := make([]presentation.Value, 0, len(recovery))
	for _, action := range recovery {
		value, err := presentation.Prose(action)
		if err != nil {
			return presentation.Document{}, err
		}
		next = append(next, value)
	}
	return (presentation.Mutation{Status: status, Identity: identity, Changes: changes, NextActions: next}).Document()
}
func (o Outcome) Document() (presentation.Document, error) {
	return localDocument("local document created", o, nil)
}
func (e *PartialError) Document() (presentation.Document, error) {
	recovery := e.Recovery
	if len(recovery) == 0 {
		recovery = []string{"inspect the reported cause, then retry awf new doc"}
	}
	return localDocument("local document partially committed", e.Outcome, recovery)
}

// RunLeased adds doc after checking its output collision, then synchronizes under a caller-held complete project transaction.
func RunLeased(ctx context.Context, root string, doc config.LocalDoc, loader *project.Loader, lease *filesystem.Lease) (outcome Outcome, err error) {
	if !loader.CoversProjectLease(ctx, root, lease) {
		return Outcome{}, errors.New("local document operation requires a covering project lease")
	}
	files, err := filesystem.Open(root)
	if err != nil {
		return Outcome{}, err
	}
	defer files.Close()
	state, cfg, configIdentity, err := loader.OpenForMutation(ctx, root, files)
	if err != nil {
		return Outcome{}, err
	}
	defer configIdentity.Release() //nolint:errcheck // descriptor cleanup cannot change the filesystem mutation outcome
	composed := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version)
	if err := composed.PreflightLocalDoc(doc); err != nil {
		return Outcome{}, err
	}
	updated, err := config.AppendLocalDoc(cfg.Source(), doc)
	if err != nil {
		return Outcome{}, err
	}
	relative := filepath.ToSlash(filepath.Join("docs", doc.Name+".md"))
	output := filepath.Join(root, "docs", filepath.FromSlash(doc.Name)+".md")
	if _, err := os.Lstat(output); err == nil {
		return Outcome{}, fmt.Errorf("local document destination already exists: %s", relative)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Outcome{}, fmt.Errorf("inspect local document destination: %w", err)
	}
	if err := files.ReplaceExpected(".awf/config.yaml", configIdentity, updated, 0o644); err != nil {
		return Outcome{}, err
	}
	outcome.DocumentPath = relative
	outcome.DeclarationReplaced = true
	state, cfg, err = loader.OpenForOperation(ctx, root)
	if err != nil {
		return outcome, &PartialError{Outcome: outcome, Cause: err, Recovery: []string{"repair config authority, then retry"}}
	}
	result, syncErr := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version).SyncLeased(ctx, lease)
	outcome.Publisher = result
	if syncErr != nil {
		return outcome, &PartialError{Outcome: outcome, Cause: syncErr, Recovery: []string{"repair the reported publication fault, then retry"}}
	}
	return outcome, nil
}
