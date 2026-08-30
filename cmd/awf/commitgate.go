package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/memorycite"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

type commitGateDependencies struct {
	readFile    func(string) ([]byte, error)
	readStdin   func(io.Reader) ([]byte, error)
	openProject func(context.Context, string) (*config.Config, error)
}

func defaultCommitGateDependencies() commitGateDependencies {
	return commitGateDependencies{readFile: os.ReadFile, readStdin: io.ReadAll, openProject: func(ctx context.Context, root string) (*config.Config, error) {
		_, cfg, _, err := openProjectOperation(ctx, root)
		return cfg, err
	}}
}

// runCommitGate validates only independently useful shared commit-message policy.
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
	cfg, err := dependencies.openProject(ctx, root)
	if err != nil {
		return fmt.Errorf("check staged commit: %w", err)
	}
	msg := commitmsg.Clean(raw)
	if msg.Subject == "" {
		return nil
	}
	if refs := memorycite.ScanText("commit message", []byte(msg.Text)); len(refs) != 0 {
		document, documentErr := memorycite.CommitGateDocument(refs)
		if documentErr != nil {
			return documentErr
		}
		if err := presentation.Render(stdout, document); err != nil {
			return err
		}
		return fmt.Errorf("check staged commit: a commit message must not cite a concrete effort-owned memory file; name the bare .awf/efforts/ directory or use an angle-bracket slug placeholder")
	}
	if isExemptCommitSubject(msg.Subject) {
		return nil
	}
	findings := audit.CheckConventionalCommit(awfgit.Commit{Subject: msg.Subject}, audit.Resolve(config.AuditScopes(cfg.Audit)))
	if len(findings) == 0 {
		return nil
	}
	document, err := audit.ConventionalCommitDocument(findings)
	if err != nil {
		return err
	}
	if err := presentation.Render(stdout, document); err != nil {
		return err
	}
	return fmt.Errorf("check staged commit: rejected %q", msg.Subject)
}

func isExemptCommitSubject(subject string) bool {
	return len(subject) >= 6 && subject[:6] == "Merge " || len(subject) >= 6 && subject[:6] == "fixup!" || len(subject) >= 7 && subject[:7] == "squash!" || len(subject) >= 6 && subject[:6] == "amend!"
}
