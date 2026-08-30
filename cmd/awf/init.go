package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/initop"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func runInitWithProjectLoader(ctx context.Context, root string, force, describe bool, sets []string, answersFile string, promptInput io.Reader, interactive bool, stdout io.Writer, loadProject initop.LoadProject, compatibilityGate initop.Gate) error {
	if describe {
		out, err := initspec.Describe(catalog.Standard.Vars)
		if err != nil {
			return err
		}
		return writeInitDescriptorProtocol(stdout, out)
	}
	answers := map[string]string{}
	if answersFile != "" {
		b, err := os.ReadFile(answersFile)
		if err != nil {
			return fmt.Errorf("awf init: read --answers: %w", err)
		}
		if answers, err = initspec.ParseAnswersFile(b); err != nil {
			return err
		}
	}
	if err := initspec.MergeSetFlags(answers, sets); err != nil {
		return err
	}
	outcome, err := initop.Run(ctx, initop.Input{
		Root: root, ResidentRoot: awfgit.ProjectResidentRoot(ctx, root), Force: force, Answers: answers,
		PromptInput: promptInput, PromptOutput: stdout, Interactive: interactive,
	}, loadProject, compatibilityGate)
	if err != nil {
		var partial *initop.PartialError
		if errors.As(err, &partial) {
			if presentErr := renderInitOutcome(partial.Outcome, stdout); presentErr != nil {
				return errors.Join(err, presentErr)
			}
		}
		return err
	}
	return renderInitOutcome(outcome, stdout)
}

func renderInitOutcome(outcome initspec.Outcome, stdout io.Writer) error {
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
}

// writeInitDescriptorProtocol writes the documented init descriptor JSON
// unchanged. It is one of the closed successful protocol bypasses.
func writeInitDescriptorProtocol(stdout io.Writer, payload []byte) error {
	payload = bytes.TrimRight(payload, "\n")
	_, err := stdout.Write(append(payload, '\n'))
	return err
}
