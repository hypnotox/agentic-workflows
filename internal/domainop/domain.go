// Package domainop owns configured-domain mutation, authored scaffold, synchronization, and orphan inspection.
package domainop

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

const currentStateStub = "Describe where the %q domain stands today: its current shape, load-bearing constraints, and what a newcomer must know before changing it. Refresh by hand when the position materially shifts. Follow `docs/doc-standard.md` for tone: terse, present tense, reference other docs rather than restate them.\n"

// Outcome is the complete observed state of a domain mutation. Publisher
// remains the publication owner; this operation owns config and authored-source facts.
type Outcome struct {
	ConfigReplaced  bool
	ScaffoldCreated bool
	Orphaned        bool
	Publisher       publisher.Result
}

// PartialError retains every committed domain effect and the original cause.
type PartialError struct {
	Outcome  Outcome
	Cause    error
	Recovery []string
}

func (e *PartialError) Error() string { return e.Cause.Error() }
func (e *PartialError) Unwrap() error { return e.Cause }

func domainDocument(status string, o Outcome, recovery []string) (presentation.Document, error) {
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
	return (presentation.Mutation{Status: status, Identity: fields, Changes: changes, NextActions: next}).Document()
}
func (o Outcome) Document() (presentation.Document, error) {
	return domainDocument("domain mutation completed", o, nil)
}
func (e *PartialError) Document() (presentation.Document, error) {
	recovery := e.Recovery
	if len(recovery) == 0 {
		recovery = []string{"inspect the reported cause, then retry the domain command"}
	}
	return domainDocument("domain mutation partially committed", e.Outcome, recovery)
}

// AddLeased configures name, creates its initial current-state part, and synchronizes under a caller-held complete project transaction.
func AddLeased(ctx context.Context, root, name string, loader *project.Loader, lease *filesystem.Lease) (outcome Outcome, err error) {
	if err := config.ValidateDomainName(name); err != nil {
		return Outcome{}, err
	}
	if !loader.CoversProjectLease(ctx, root, lease) {
		return Outcome{}, errors.New("domain operation requires a covering project lease")
	}
	files, err := filesystem.Open(root)
	if err != nil {
		return Outcome{}, err
	}
	defer files.Close()
	_, cfg, configIdentity, err := loader.OpenForMutation(ctx, root, files)
	if err != nil {
		return Outcome{}, err
	}
	defer configIdentity.Release() //nolint:errcheck // descriptor cleanup cannot change the filesystem mutation outcome
	for _, domain := range cfg.Domains {
		if domain == name {
			return Outcome{}, fmt.Errorf("domain %q already exists", name)
		}
	}
	updated, err := config.SetArrayMember(cfg.Source(), "domains", name, true)
	if err != nil {
		return Outcome{}, err
	}
	if err := replaceConfig(files, configIdentity, updated); err != nil {
		return Outcome{}, err
	}
	outcome.ConfigReplaced = true
	created, err := scaffoldCurrentStateConfined(files, root, cfg, name)
	if err != nil {
		return outcome, &PartialError{Outcome: outcome, Cause: err, Recovery: []string{"repair the authored domain path, then retry"}}
	}
	outcome.ScaffoldCreated = created
	result, syncErr := synchronize(ctx, root, loader, lease)
	outcome.Publisher = result
	if syncErr != nil {
		return outcome, &PartialError{Outcome: outcome, Cause: syncErr, Recovery: []string{"repair the reported publication fault, then retry"}}
	}
	return outcome, nil
}

// RemoveLeased unconfigures name, synchronizes, and reports whether authored domain inputs remain orphaned under a caller-held complete project transaction.
func RemoveLeased(ctx context.Context, root, name string, loader *project.Loader, lease *filesystem.Lease) (outcome Outcome, err error) {
	if err := config.ValidateDomainName(name); err != nil {
		return Outcome{}, err
	}
	if !loader.CoversProjectLease(ctx, root, lease) {
		return Outcome{}, errors.New("domain operation requires a covering project lease")
	}
	files, err := filesystem.Open(root)
	if err != nil {
		return Outcome{}, err
	}
	defer files.Close()
	_, cfg, configIdentity, err := loader.OpenForMutation(ctx, root, files)
	if err != nil {
		return Outcome{}, err
	}
	defer configIdentity.Release() //nolint:errcheck // descriptor cleanup cannot change the filesystem mutation outcome
	found := false
	for _, domain := range cfg.Domains {
		found = found || domain == name
	}
	if !found {
		return Outcome{}, fmt.Errorf("domain %q is not configured", name)
	}
	updated, err := config.SetArrayMember(cfg.Source(), "domains", name, false)
	if err != nil {
		return Outcome{}, err
	}
	if err := replaceConfig(files, configIdentity, updated); err != nil {
		return Outcome{}, err
	}
	outcome.ConfigReplaced = true
	result, syncErr := synchronize(ctx, root, loader, lease)
	outcome.Publisher = result
	if syncErr != nil {
		return outcome, &PartialError{Outcome: outcome, Cause: syncErr, Recovery: []string{"repair the reported publication fault, then retry"}}
	}
	outcome.Orphaned, err = hasSidecarOrParts(files, name)
	if err != nil {
		return outcome, &PartialError{Outcome: outcome, Cause: err, Recovery: []string{"inspect authored domain paths, then retry"}}
	}
	return outcome, nil
}

func replaceConfig(files *filesystem.Handle, expected *filesystem.ExpectedIdentity, updated []byte) error {
	return files.ReplaceExpected(".awf/config.yaml", expected, updated, 0o644)
}

func scaffoldCurrentStateConfined(files *filesystem.Handle, root string, cfg *config.Config, name string) (bool, error) {
	path, err := filepath.Rel(root, cfg.PartPath("domains", name, "current-state"))
	if err != nil {
		return false, err
	}
	path = filepath.ToSlash(path)
	if err := files.MkdirAll(filepath.ToSlash(filepath.Dir(path)), 0o755); err != nil {
		return false, err
	}
	if err := files.Publish(path, fmt.Appendf(nil, currentStateStub, name), 0o644); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return false, err
		}
		info, inspectErr := files.LinkInfo(path)
		if inspectErr != nil {
			return false, inspectErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false, fmt.Errorf("unsafe current-state scaffold path %q", path)
		}
		return false, nil
	}
	return true, nil
}

func synchronize(ctx context.Context, root string, loader *project.Loader, lease *filesystem.Lease) (publisher.Result, error) {
	if !loader.CoversProjectLease(ctx, root, lease) {
		return publisher.Result{}, errors.New("domain synchronization requires a covering project lease")
	}
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return publisher.Result{}, err
	}
	composed := publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version)
	return composed.SyncLeased(ctx, lease)
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
