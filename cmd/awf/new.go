package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/localdocop"
	"github.com/hypnotox/agentic-workflows/internal/pitfallop"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topicop"
)

const localDocumentKind = "doc"

// runNew scaffolds one of the surviving authored artifacts.
func runNew(ctx context.Context, root, kind string, args []string, stdout io.Writer) error {
	switch {
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
		return &usageErr{fmt.Sprintf("unknown kind %q (want: topic, domain, pitfall, doc)", kind)}
	}
}

func newDoc(ctx context.Context, root string, args []string, title *string, stdout io.Writer) error {
	if len(args) != 2 {
		return &usageErr{"usage: awf new doc <name> <description> [--title <title>]"}
	}
	resolvedTitle := derivedLocalDocTitle(args[0])
	if title != nil {
		resolvedTitle = *title
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	outcome, err := localdocop.Run(ctx, root, config.LocalDoc{Name: args[0], Title: resolvedTitle, Description: args[1]}, loader, gatedLeaseAcquirer(loader))
	if err != nil {
		touched := publisherPaths(outcome.Publisher)
		if outcome.DeclarationReplaced {
			touched = append([]string{".awf/config.yaml"}, touched...)
		}
		return mutationFailure{condition: "local document creation did not complete", cause: err, touched: touched, rerun: "awf new doc " + args[0]}
	}
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
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
	return newPitfallWith(ctx, root, args, stdout, nil, pitfallop.Create)
}

type pitfallCreate func(context.Context, string, string, *project.Loader, pitfallop.LeaseAcquirer) (pitfallop.Outcome, error)

func newPitfallWith(ctx context.Context, root string, args []string, stdout io.Writer, acquire pitfallop.LeaseAcquirer, create pitfallCreate) error {
	if len(args) != 1 {
		return &usageErr{"usage: awf new pitfall <title>"}
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	if acquire == nil {
		acquire = gatedLeaseAcquirer(loader)
	}
	outcome, err := create(ctx, root, args[0], loader, acquire)
	if err != nil {
		touched := []string(nil)
		if outcome.SourcePath != "" {
			touched = append(touched, outcome.SourcePath)
		}
		return mutationFailure{condition: "pitfall creation did not complete", cause: err, touched: touched, rerun: "awf new pitfall"}
	}
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

func newTopic(ctx context.Context, root string, args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return &usageErr{"usage: awf new topic <domain> <title>"}
	}
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	outcome, err := topicop.Create(ctx, root, args[0], strings.Join(args[1:], " "), loader, gatedLeaseAcquirer(loader))
	if err != nil {
		return mutationFailure{condition: "topic creation did not complete", cause: err, touched: outcome.Created, rerun: "awf new topic"}
	}
	return presentation.Render(stdout, outcome.Document)
}
