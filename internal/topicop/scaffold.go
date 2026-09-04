package topicop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Outcome retains the semantic document and authored paths created by this run.
type Outcome struct {
	Document presentation.Document
	Created  []string
}

// LeaseAcquirer is the narrow mechanism seam used by focused lease tests.
type LeaseAcquirer func(context.Context, string) (*filesystem.Lease, func() error, error)

// Create acquires the selected tracked lease before authority and destination
// planning, then creates each missing intended source exclusively. Exact files
// from an earlier interrupted run are accepted; differing collisions are not.
func Create(ctx context.Context, root, domain, title string, loader *project.Loader, acquire LeaseAcquirer) (outcome Outcome, returnErr error) {
	if loader == nil {
		return Outcome{}, errors.New("topic operation requires a project loader")
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
		return Outcome{}, errors.New("topic operation requires a covering tracked lease")
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
	matches, err := files.RootMatches(root)
	if err != nil {
		return Outcome{}, fmt.Errorf("verify selected root %s: %w", root, err)
	}
	if !matches {
		return Outcome{}, filesystem.ErrIdentityChanged
	}
	session, identity, err := loader.LoadForMutation(ctx, root, files)
	if err != nil {
		return Outcome{}, fmt.Errorf("load topic authority: %w", err)
	}
	defer identity.Release() //nolint:errcheck
	matches, err = files.RootMatches(root)
	if err != nil {
		return Outcome{}, fmt.Errorf("reverify selected root %s: %w", root, err)
	}
	if !matches {
		return Outcome{}, filesystem.ErrIdentityChanged
	}
	planned, err := topic.ScaffoldFilesWithExists(session.Config(), domain, title, func(string) (bool, error) { return false, nil })
	if err != nil {
		return Outcome{}, err
	}
	outcome.Document, err = topic.CreatedDocument(planned)
	if err != nil {
		return Outcome{}, err
	}
	outcome.Created, err = createPlanned(files, planned)
	return outcome, err
}

type scaffoldFilesystem interface {
	LinkInfo(string) (os.FileInfo, error)
	Read(string) ([]byte, error)
	MkdirAll(string, os.FileMode) error
	Publish(string, []byte, os.FileMode) error
}

func createPlanned(files scaffoldFilesystem, planned []topic.ScaffoldFile) ([]string, error) {
	created := []string{}
	missing := make([]topic.ScaffoldFile, 0, len(planned))
	for _, file := range planned {
		rel := filepath.ToSlash(file.Path)
		info, inspectErr := files.LinkInfo(rel)
		if errors.Is(inspectErr, os.ErrNotExist) {
			missing = append(missing, file)
			continue
		}
		if inspectErr != nil {
			return created, fmt.Errorf("inspect topic scaffold path %s: %w", rel, inspectErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return created, fmt.Errorf("topic scaffold path collision %s", rel)
		}
		got, readErr := files.Read(rel)
		if readErr != nil {
			return created, fmt.Errorf("read topic scaffold path %s: %w", rel, readErr)
		}
		if !bytes.Equal(got, file.Content) {
			return created, fmt.Errorf("topic scaffold path collision %s", rel)
		}
	}
	for _, file := range missing {
		rel := filepath.ToSlash(file.Path)
		if err := files.MkdirAll(filepath.ToSlash(filepath.Dir(rel)), 0o755); err != nil {
			return created, fmt.Errorf("create parent for topic scaffold path %s: %w", rel, err)
		}
		if err := files.Publish(rel, file.Content, 0o644); err != nil {
			return created, fmt.Errorf("create topic scaffold path %s exclusively: %w", rel, err)
		}
		created = append(created, rel)
	}
	return created, nil
}
