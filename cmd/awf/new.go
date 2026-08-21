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
		return newDoc(ctx, root, args, nil, stdout)
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
	state, cfg, repo, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	path, err := project.NewADR(state.Root(), cfg, repo, ctx, strings.Join(titleWords, " "))
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
	state, _, _, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	path, err := project.NewPlan(state.Root(), strings.Join(titleWords, " "))
	if err != nil {
		return err
	}
	return writeStatus(stdout, "created: "+path)
}

type localDocPreflight func(context.Context, config.LocalDoc) error

type localDocDependencies struct {
	prepare     func(context.Context, string) (localDocPreflight, error)
	read        func(string) ([]byte, error)
	append      func([]byte, config.LocalDoc) ([]byte, error)
	inspect     func(string) (os.FileInfo, error)
	write       func(string, []byte, os.FileMode) error
	synchronize func(context.Context, string, io.Writer) error
}

func productionLocalDocDependencies() localDocDependencies {
	return localDocDependencies{
		prepare: func(ctx context.Context, root string) (localDocPreflight, error) {
			state, cfg, _, err := openProjectOperation(ctx, root)
			if err != nil {
				return nil, err
			}
			return func(ctx context.Context, doc config.LocalDoc) error {
				return project.PreflightLocalDoc(state, cfg, ctx, doc)
			}, nil
		},
		read:        os.ReadFile,
		append:      config.AppendLocalDoc,
		inspect:     os.Lstat,
		write:       os.WriteFile,
		synchronize: runSync,
	}
}

func newDoc(ctx context.Context, root string, args []string, title *string, stdout io.Writer) error {
	return newDocWith(ctx, root, args, title, stdout, productionLocalDocDependencies())
}

func newDocWith(ctx context.Context, root string, args []string, title *string, stdout io.Writer, dependencies localDocDependencies) error {
	if len(args) != 2 {
		return &usageErr{"usage: awf new doc <name> <description> [--title <title>]"}
	}
	if err := gate(ctx, root); err != nil {
		return err
	}
	resolvedTitle := derivedLocalDocTitle(args[0])
	if title != nil {
		resolvedTitle = *title
	}
	doc := config.LocalDoc{Name: args[0], Title: resolvedTitle, Description: args[1]}
	preflight, err := dependencies.prepare(ctx, root)
	if err != nil {
		return err
	}
	source, err := dependencies.read(config.ConfigPath(root))
	if err != nil {
		return err
	}
	updated, err := dependencies.append(source, doc)
	if err != nil {
		return err
	}
	relativeOutput := filepath.ToSlash(filepath.Join("docs", doc.Name+".md"))
	output := filepath.Join(root, "docs", filepath.FromSlash(doc.Name)+".md")
	if _, err := dependencies.inspect(output); err == nil {
		return fmt.Errorf("local document destination already exists: %s", relativeOutput)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect local document destination: %w", err)
	}
	if err := preflight(ctx, doc); err != nil {
		return err
	}
	if err := dependencies.write(config.ConfigPath(root), updated, 0o644); err != nil {
		return err
	}
	if err := dependencies.synchronize(ctx, root, io.Discard); err != nil {
		return err
	}
	return writeStatus(stdout, "created: "+relativeOutput)
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
	state, _, _, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	document, err := project.NewPitfall(state.Root(), args[0])
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
	_, cfg, _, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	files, err := topic.ScaffoldFiles(root, cfg, args[0], strings.Join(args[1:], " "))
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
