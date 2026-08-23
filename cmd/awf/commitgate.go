package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/commitgateop"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

type commitGateDependencies struct {
	readFile           func(string) ([]byte, error)
	readStdin          func(io.Reader) ([]byte, error)
	openProject        func(context.Context, string) (*config.Config, *git.Repo, error)
	authorize          func(context.Context, string, *git.Repo, commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error)
	diagnostic         func(currentstatecoord.CommitAuthorizationResult) (presentation.Diagnostic, error)
	diagnosticDocument func(presentation.Diagnostic) (presentation.Document, error)
	render             func(io.Writer, presentation.Document) error
}

func defaultCommitGateDependencies() commitGateDependencies {
	return commitGateDependencies{
		readFile:    os.ReadFile,
		readStdin:   io.ReadAll,
		openProject: openCommitGateProjectFromDisk,
		authorize: func(ctx context.Context, root string, repo *git.Repo, msg commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error) {
			return currentstatecoord.CheckCommitAuthorization(root, repo, ctx, msg)
		},
		diagnostic: commitgateop.AuthorizationDiagnostic,
		diagnosticDocument: func(diagnostic presentation.Diagnostic) (presentation.Document, error) {
			return diagnostic.Document()
		},
		render: presentation.Render,
	}
}

func openCommitGateProjectFromDisk(ctx context.Context, root string) (*config.Config, *git.Repo, error) {
	_, cfg, repo, err := openProjectOperation(ctx, root)
	return cfg, repo, err
}

// runCommitGate reads one CLI-selected message source, composes the focused
// staged-commit operation, renders its optional partial result, and preserves
// the operation's refusal for process-level exit mapping.
func runCommitGate(ctx context.Context, root, msgPath string, stdin io.Reader, stdout io.Writer) error {
	return runCommitGateWithDependencies(ctx, root, msgPath, stdin, stdout, defaultCommitGateDependencies())
}

func runCommitGateWithDependencies(ctx context.Context, root, msgPath string, stdin io.Reader, stdout io.Writer, dependencies commitGateDependencies) error {
	var raw []byte
	var err error
	if msgPath != "" {
		raw, err = dependencies.readFile(msgPath)
	} else {
		raw, err = dependencies.readStdin(stdin)
	}
	if err != nil {
		return fmt.Errorf("check staged commit: read message: %w", err)
	}
	cfg, repo, err := dependencies.openProject(ctx, root)
	if err != nil {
		return fmt.Errorf("check staged commit: %w", err)
	}
	result, operationErr := commitgateop.RunWithDependencies(ctx, root, cfg, repo, raw, commitgateop.Dependencies{
		Authorize: dependencies.authorize, Diagnostic: dependencies.diagnostic, DiagnosticDocument: dependencies.diagnosticDocument,
	})
	if result.HasDocument {
		if err := dependencies.render(stdout, result.Document); err != nil {
			if result.Authorization {
				return fmt.Errorf("check staged commit: render authorization diagnostic: %w", err)
			}
			return err
		}
	}
	return operationErr
}
