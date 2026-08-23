package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/localdocop"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/topicop"
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

func newDoc(ctx context.Context, root string, args []string, title *string, stdout io.Writer) error {
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
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	if err := localdocop.Run(ctx, root, config.LocalDoc{Name: args[0], Title: resolvedTitle, Description: args[1]}, loader); err != nil {
		return err
	}
	return writeStatus(stdout, "created: "+"docs/"+args[0]+".md")
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
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	document, err := topicop.Create(ctx, root, args[0], strings.Join(args[1:], " "), loader)
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}
