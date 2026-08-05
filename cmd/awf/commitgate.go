package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/memorycite"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

type commitGateDependencies struct {
	readFile           func(string) ([]byte, error)
	readStdin          func(io.Reader) ([]byte, error)
	openProject        func(context.Context, string) (*project.Project, error)
	authorize          func(context.Context, *project.Project, commitmsg.Message) (project.CommitAuthorizationResult, error)
	diagnostic         func(project.CommitAuthorizationResult) (presentation.Diagnostic, error)
	diagnosticDocument func(presentation.Diagnostic) (presentation.Document, error)
	render             func(io.Writer, presentation.Document) error
}

func defaultCommitGateDependencies() commitGateDependencies {
	return commitGateDependencies{
		readFile:    os.ReadFile,
		readStdin:   io.ReadAll,
		openProject: openCommitGateProjectFromDisk,
		authorize: func(ctx context.Context, p *project.Project, msg commitmsg.Message) (project.CommitAuthorizationResult, error) {
			return p.CheckCommitAuthorization(ctx, msg)
		},
		diagnostic: func(result project.CommitAuthorizationResult) (presentation.Diagnostic, error) {
			return result.Diagnostic()
		},
		diagnosticDocument: func(diagnostic presentation.Diagnostic) (presentation.Document, error) {
			return diagnostic.Document()
		},
		render: presentation.Render,
	}
}

func openCommitGateProjectFromDisk(ctx context.Context, root string) (*project.Project, error) {
	return project.Open(ctx, root)
}

// runCommitGate validates one commit message and returns an error (mapped to a
// non-zero exit) on any violation, so a commit-msg hook calling it blocks the
// commit. It applies the shared Conventional Commits rule, the definitive
// stale-merge authorization check, and, while memoryCite.enabled is true, a scan
// for a citation of a specific working-memory file (ADR-0158). The
// git-generated-subject exemption scopes the Conventional Commits check alone:
// git writes the subject, but a person may edit a merge or autosquash body, so
// the citation and authorization checks apply to every recorded message. The
// message comes from msgPath (the file a commit-msg hook
// passes as $1) or stdin when msgPath is empty; citation line numbers are
// relative to the git-cleaned message, not to the raw file.
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
	msg := commitmsg.Clean(raw)
	p, err := dependencies.openProject(ctx, root)
	if err != nil {
		return fmt.Errorf("check staged commit: %w", err)
	}
	if msg.Subject != "" {
		if p.Cfg.MemoryCite != nil && p.Cfg.MemoryCite.Enabled {
			// Scan the cleaned message, never the raw bytes: `git commit -v` appends
			// the staged diff below a scissors line, and a diff may legitimately carry
			// text git itself discards.
			refs := memorycite.ScanText("commit message", []byte(msg.Text))
			if len(refs) > 0 {
				values := make([]presentation.Value, 0, len(refs))
				for _, r := range refs {
					value, valueErr := presentation.Prose(fmt.Sprintf("%s line %d names the effort-owned memory file %q", r.Path, r.Line, r.Segment))
					if valueErr != nil { // coverage-ignore: scanner findings are single-line and the fixed prose keeps the value nonempty
						return valueErr
					}
					values = append(values, value)
				}
				list, listErr := presentation.NewList("errors", values...)
				if listErr != nil { // coverage-ignore: at least one validated finding and a fixed grammar-valid label are supplied
					return listErr
				}
				section, sectionErr := presentation.NewSection("check staged commit", list)
				if sectionErr != nil { // coverage-ignore: the validated List is always an admitted Section child
					return sectionErr
				}
				document, documentErr := presentation.NewDocument(section)
				if documentErr != nil { // coverage-ignore: the validated Section is always an admitted root node
					return documentErr
				}
				if renderErr := dependencies.render(stdout, document); renderErr != nil {
					return renderErr
				}
			}
			if len(refs) > 0 {
				return errors.New("check staged commit: a commit message must not cite a concrete effort-owned memory file; name the bare .awf/efforts/ directory or use an angle-bracket slug placeholder")
			}
		}
		// A git-generated merge or autosquash subject is exempt from the Conventional
		// Commits rule - never block what git produced or will rewrite.
		if !isExemptSubject(msg.Subject) {
			findings := audit.CheckConventionalCommit(
				git.Commit{Subject: msg.Subject}, audit.Resolve(p.Cfg.Audit))
			if len(findings) > 0 {
				values := make([]presentation.Value, 0, len(findings))
				for _, finding := range findings {
					value, valueErr := presentation.Prose(finding.Detail)
					if valueErr != nil { // coverage-ignore: conventional-commit findings have fixed nonempty single-line detail
						return valueErr
					}
					values = append(values, value)
				}
				list, listErr := presentation.NewList("errors", values...)
				if listErr != nil { // coverage-ignore: at least one validated finding and a fixed grammar-valid label are supplied
					return listErr
				}
				section, sectionErr := presentation.NewSection("check staged commit", list)
				if sectionErr != nil { // coverage-ignore: the validated List is always an admitted Section child
					return sectionErr
				}
				document, documentErr := presentation.NewDocument(section)
				if documentErr != nil { // coverage-ignore: the validated Section is always an admitted root node
					return documentErr
				}
				if renderErr := dependencies.render(stdout, document); renderErr != nil {
					return renderErr
				}
				return fmt.Errorf("check staged commit: rejected %q", msg.Subject)
			}
		}
	}
	result, authErr := dependencies.authorize(ctx, p, msg)
	if result.Category != "" {
		diagnostic, diagnosticErr := dependencies.diagnostic(result)
		if diagnosticErr != nil {
			return fmt.Errorf("check staged commit: render authorization diagnostic: %w", diagnosticErr)
		}
		document, diagnosticErr := dependencies.diagnosticDocument(diagnostic)
		if diagnosticErr != nil {
			return fmt.Errorf("check staged commit: render authorization diagnostic: %w", diagnosticErr)
		}
		if diagnosticErr := dependencies.render(stdout, document); diagnosticErr != nil {
			return fmt.Errorf("check staged commit: render authorization diagnostic: %w", diagnosticErr)
		}
	}
	if authErr != nil {
		var syntax *commitmsg.SyntaxError
		if errors.As(authErr, &syntax) {
			return fmt.Errorf("check staged commit: stale merge authorization refused: %w", authErr)
		}
		return fmt.Errorf("check staged commit: stale merge authorization: %w", authErr)
	}
	if len(result.NextActions) > 0 {
		return errors.New("check staged commit: stale merge authorization refused")
	}
	return nil
}

// isExemptSubject reports whether a subject is one git itself generates - a merge
// or an autosquash (fixup!/squash!/amend!) - which the gate must not block.
func isExemptSubject(s string) bool {
	return strings.HasPrefix(s, "Merge ") ||
		strings.HasPrefix(s, "fixup!") ||
		strings.HasPrefix(s, "squash!") ||
		strings.HasPrefix(s, "amend!")
}
