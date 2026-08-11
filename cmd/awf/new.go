package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// runNew scaffolds one of the surviving authored artifacts: an ADR, plan,
// current-state topic, domain, or pitfall. Each arm owns its kind-specific arguments.
// touches-state: tooling/cli:adr-new-version-gated - new-command version gate site; proof in gate_test.go
func runNew(ctx context.Context, root, kind string, args []string, stdout io.Writer) error {
	switch {
	case kind == "adr":
		return newADR(ctx, root, args, stdout)
	case kind == "plan":
		return newPlan(ctx, root, args, stdout)
	case kind == "topic":
		return newTopic(ctx, root, args, stdout)
	case kind == "pitfall":
		return newPitfall(ctx, root, args, stdout)
	case project.IsFreeformDomainKind(kind):
		if len(args) != 1 {
			return &usageErr{"usage: awf new domain <name>"}
		}
		return runNewDomain(ctx, root, args[0], stdout)
	default:
		return &usageErr{fmt.Sprintf("unknown kind %q (want: adr, plan, topic, domain, pitfall)", kind)}
	}
}

func newADR(ctx context.Context, root string, titleWords []string, stdout io.Writer) error {
	if len(titleWords) == 0 {
		return &usageErr{"usage: awf new adr <title>"}
	}
	if err := gate(ctx, root); err != nil {
		return err
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	path, err := p.NewADR(ctx, strings.Join(titleWords, " "))
	if err != nil {
		return err
	}
	return writeStatus(stdout, "created: "+path)
}

func newPlan(ctx context.Context, root string, titleWords []string, stdout io.Writer) error {
	if len(titleWords) == 0 {
		return &usageErr{"usage: awf new plan <title>"}
	}
	if err := gate(ctx, root); err != nil {
		return err
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	path, err := p.NewPlan(strings.Join(titleWords, " "))
	if err != nil {
		return err
	}
	return writeStatus(stdout, "created: "+path)
}

func newPitfall(ctx context.Context, root string, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return &usageErr{"usage: awf new pitfall <title>"}
	}
	if err := gate(ctx, root); err != nil {
		return err
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	document, err := p.NewPitfall(args[0])
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

func newTopic(ctx context.Context, root string, args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return &usageErr{"usage: awf new topic <domain> <title>"}
	}
	if err := gate(ctx, root); err != nil {
		return err
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	files, err := topic.ScaffoldFiles(root, p.Cfg, args[0], strings.Join(args[1:], " "))
	if err != nil {
		return err
	}
	createdFiles := make([]string, 0, len(files))
	var createdDirs []string
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		dirs, err := createTopicParents(filepath.Dir(path))
		createdDirs = append(createdDirs, dirs...)
		if err != nil {
			return rollbackTopicScaffold(fmt.Errorf("create parent for topic scaffold path %q: %w", filepath.ToSlash(path), err), createdFiles, createdDirs)
		}
		writer, err := topicOpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return rollbackTopicScaffold(fmt.Errorf("create topic scaffold path %q exclusively: %w", filepath.ToSlash(path), err), createdFiles, createdDirs)
		}
		createdFiles = append(createdFiles, path)
		if err := writeAndCloseTopicFile(path, writer, file.Content); err != nil {
			return rollbackTopicScaffold(err, createdFiles, createdDirs)
		}
	}
	document, err := topic.CreatedDocument(files)
	if err != nil { // coverage-ignore: ScaffoldFiles returns validated repository-relative single-line paths
		return err
	}
	return presentation.Render(stdout, document)
}

type topicWriteCloser interface {
	io.Writer
	Close() error
}

func writeAndCloseTopicFile(path string, writer topicWriteCloser, content []byte) error {
	_, writeErr := io.Copy(writer, bytes.NewReader(content))
	closeErr := writer.Close()
	var failures []error
	if writeErr != nil {
		failures = append(failures, fmt.Errorf("write topic scaffold path %q: %w", filepath.ToSlash(path), writeErr))
	}
	if closeErr != nil {
		failures = append(failures, fmt.Errorf("close topic scaffold path %q: %w", filepath.ToSlash(path), closeErr))
	}
	return errors.Join(failures...)
}

func createTopicParents(parent string) ([]string, error) {
	var missing []string
	for current := filepath.Clean(parent); ; current = filepath.Dir(current) {
		info, err := topicStat(current)
		if err == nil {
			if !info.IsDir() {
				return nil, fmt.Errorf("parent path %q is not a directory", filepath.ToSlash(current))
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect topic scaffold parent %q: %w", filepath.ToSlash(current), err)
		}
		missing = append(missing, current)
		next := filepath.Dir(current)
		if next == current { // coverage-ignore: scaffold paths are rooted below an existing project root
			break
		}
	}
	slices.Reverse(missing)
	mkdirErr := topicMkdirAll(parent, 0o755)
	created := make([]string, 0, len(missing))
	for _, path := range missing {
		if info, err := topicStat(path); err == nil && info.IsDir() {
			created = append(created, path)
		}
	}
	return created, mkdirErr
}

func rollbackTopicScaffold(primary error, createdFiles, createdDirs []string) error {
	failures := []error{primary}
	for i := len(createdFiles) - 1; i >= 0; i-- {
		if err := topicRemove(createdFiles[i]); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Errorf("remove created topic scaffold path %q: %w", filepath.ToSlash(createdFiles[i]), err))
		}
	}
	dirs := slices.Clone(createdDirs)
	slices.SortStableFunc(dirs, func(a, b string) int {
		return strings.Count(b, string(filepath.Separator)) - strings.Count(a, string(filepath.Separator))
	})
	for _, dir := range dirs {
		entries, err := topicReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("inspect created topic scaffold directory %q: %w", filepath.ToSlash(dir), err))
			continue
		}
		if len(entries) != 0 {
			continue
		}
		if err := topicRemove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Errorf("remove created topic scaffold directory %q: %w", filepath.ToSlash(dir), err))
		}
	}
	return errors.Join(failures...)
}

var (
	topicMkdirAll = os.MkdirAll
	topicReadDir  = os.ReadDir
	topicStat     = os.Stat
	topicOpenFile = func(path string, flag int, mode os.FileMode) (topicWriteCloser, error) {
		return os.OpenFile(path, flag, mode)
	}
	topicRemove = os.Remove
)
