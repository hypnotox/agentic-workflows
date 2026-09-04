// Package pitfallop owns the focused authored-pitfall creation operation.
package pitfallop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// Outcome retains the authored path when it was created or already matched.
type Outcome struct{ SourcePath string }

// LeaseAcquirer is the narrow mechanism seam used by focused lease tests.
type LeaseAcquirer func(context.Context, string) (*filesystem.Lease, func() error, error)

// Document returns the owner-produced creation report.
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
		if source == pitfall.SourceDir || info.IsDir() {
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
		return Outcome{}, err
	}
	entries := corpus.All()
	for i, existing := range entries {
		if !pitfall.EqualTitle(title, existing.Title) {
			continue
		}
		without := append(append([]pitfall.Entry(nil), entries[:i]...), entries[i+1:]...)
		entry, intended, planErr := pitfall.New(without).Scaffold(title)
		if planErr != nil {
			return Outcome{}, planErr
		}
		if entry.SourcePath != existing.SourcePath || !bytes.Equal(intended, existing.Source) {
			return Outcome{}, fmt.Errorf("pitfall source collision %s", existing.SourcePath)
		}
		return Outcome{SourcePath: existing.SourcePath}, nil
	}
	entry, source, err := corpus.Scaffold(title)
	if err != nil {
		return Outcome{}, err
	}
	if err := files.MkdirAll(pitfall.SourceDir, 0o755); err != nil {
		return Outcome{}, fmt.Errorf("create pitfall source directory %s: %w", pitfall.SourceDir, err)
	}
	if err := files.Publish(entry.SourcePath, source, 0o644); err != nil {
		return Outcome{}, fmt.Errorf("create pitfall source %s exclusively: %w", entry.SourcePath, err)
	}
	return Outcome{SourcePath: entry.SourcePath}, nil
}

// Create acquires the selected tracked lease before authority and destination
// planning and exclusively publishes one authored pitfall through a confined handle.
func Create(ctx context.Context, root, title string, loader *project.Loader, acquire LeaseAcquirer) (outcome Outcome, returnErr error) {
	return create(ctx, root, title, loader, acquire, nil)
}

func create(ctx context.Context, root, title string, loader *project.Loader, acquire LeaseAcquirer, afterOpen func()) (outcome Outcome, returnErr error) {
	if loader == nil {
		return Outcome{}, errors.New("pitfall operation requires a project loader")
	}
	if acquire == nil {
		acquire = func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
			lease, err := filesystem.AcquireTrackedLease(ctx, root)
			if err != nil {
				return nil, nil, err
			}
			return lease, lease.Release, nil
		}
	}
	lease, release, err := acquire(ctx, root)
	if err != nil {
		return Outcome{}, fmt.Errorf("acquire tracked lease for %s: %w", root, err)
	}
	if lease == nil || release == nil || !lease.CoversTracked(root) {
		if release != nil {
			_ = release()
		} else if lease != nil {
			_ = lease.Release()
		}
		return Outcome{}, errors.New("pitfall operation requires a covering tracked lease")
	}
	defer func() {
		if err := release(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("release tracked lease for %s: %w", root, err))
		}
	}()
	files, err := filesystem.Open(root)
	if err != nil {
		return Outcome{}, fmt.Errorf("open selected root %s: %w", root, err)
	}
	defer func() {
		if err := files.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close selected root %s: %w", root, err))
		}
	}()
	if afterOpen != nil {
		afterOpen()
	}
	matches, err := files.RootMatches(root)
	if err != nil {
		return Outcome{}, fmt.Errorf("verify selected root %s: %w", root, err)
	}
	if !matches {
		return Outcome{}, filesystem.ErrIdentityChanged
	}
	_, identity, err := loader.LoadForMutation(ctx, root, files)
	if err != nil {
		return Outcome{}, fmt.Errorf("load pitfall authority: %w", err)
	}
	defer identity.Release() //nolint:errcheck
	matches, err = files.RootMatches(root)
	if err != nil {
		return Outcome{}, fmt.Errorf("reverify selected root %s: %w", root, err)
	}
	if !matches {
		return Outcome{}, filesystem.ErrIdentityChanged
	}
	return createConfined(title, files)
}
