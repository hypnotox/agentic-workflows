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

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

const localDocumentKind = "doc"

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
	case kind == localDocumentKind:
		return newDoc(ctx, root, args, "", stdout)
	case project.IsFreeformDomainKind(kind):
		if len(args) != 1 {
			return &usageErr{"usage: awf new domain <name>"}
		}
		return runNewDomain(ctx, root, args[0], stdout)
	default:
		return &usageErr{fmt.Sprintf("unknown kind %q (want: adr, plan, topic, domain, pitfall, doc)", kind)}
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

func newDoc(ctx context.Context, root string, args []string, title string, stdout io.Writer) error {
	if len(args) != 2 { // coverage-ignore: clispec enforces exact two positional grammar before dispatch
		return &usageErr{"usage: awf new doc <name> <description> [--title <title>]"}
	}
	if err := gate(ctx, root); err != nil {
		return err
	}
	if title == "" {
		title = derivedLocalDocTitle(args[0])
	}
	doc := config.LocalDoc{Name: args[0], Title: title, Description: args[1]}
	p, err := project.Open(ctx, root)
	if err != nil { // coverage-ignore: gate and strict config loading establish the adopted project before this second open
		return err
	}
	output := filepath.Join(root, "docs", filepath.FromSlash(doc.Name)+".md")
	if _, err := os.Lstat(output); err == nil {
		return fmt.Errorf("local document destination already exists: %s", filepath.ToSlash(filepath.Join("docs", doc.Name+".md")))
	} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: destination inspection faults are OS failures outside command semantics
		return fmt.Errorf("inspect local document destination: %w", err)
	}
	source, err := os.ReadFile(config.ConfigPath(root))
	if err != nil { // coverage-ignore: project.Open already read this path; failure requires a concurrent filesystem race
		return err
	}
	updated, err := config.AppendLocalDoc(source, doc)
	if err != nil { // coverage-ignore: strict config loading proves the source shape before this mutation
		return err
	}
	p.Cfg.LocalDocs = append(p.Cfg.LocalDocs, doc)
	if _, err := p.OutputPlan(ctx); err != nil { // coverage-ignore: pre-mutation collision planning admits this appended declaration
		return err
	}
	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil { // coverage-ignore: config publication failure is an OS fault after planning
		return err
	}
	if err := runSync(ctx, root, io.Discard); err != nil { // coverage-ignore: preflight OutputPlan validated the same configuration and destination
		return err
	}
	return writeStatus(stdout, "created: "+filepath.ToSlash(filepath.Join("docs", doc.Name+".md")))
}

func derivedLocalDocTitle(name string) string {
	segment := name[strings.LastIndex(name, "/")+1:]
	words := strings.Split(segment, "-")
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
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
