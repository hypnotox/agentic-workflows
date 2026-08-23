// Package commitgateop owns staged commit-message policy and stale-merge authorization sequencing.
package commitgateop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/memorycite"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Result carries an optional owner-produced diagnostic alongside the operation error.
type Result struct {
	Document      presentation.Document
	HasDocument   bool
	Authorization bool
}

// Dependencies are the operation-specific seams used to prove mechanism failures.
type Dependencies struct {
	Authorize          func(context.Context, string, *git.Repo, commitmsg.Message) (currentstatecoord.CommitAuthorizationResult, error)
	Diagnostic         func(currentstatecoord.CommitAuthorizationResult) (presentation.Diagnostic, error)
	DiagnosticDocument func(presentation.Diagnostic) (presentation.Document, error)
}

// RunWithDependencies applies the operation with explicit bounded mechanism seams.
func RunWithDependencies(ctx context.Context, root string, cfg *config.Config, repo *git.Repo, raw []byte, dependencies Dependencies) (Result, error) {
	msg := commitmsg.Clean(raw)
	if msg.Subject != "" {
		refs := memorycite.ScanText("commit message", []byte(msg.Text))
		if len(refs) > 0 {
			document, err := memorycite.CommitGateDocument(refs)
			if err != nil { // coverage-ignore: ScanText produces nonempty single-line reference fields and the owner mapping uses fixed grammar
				return Result{}, err
			}
			return Result{Document: document, HasDocument: true}, errors.New("check staged commit: a commit message must not cite a concrete effort-owned memory file; name the bare .awf/efforts/ directory or use an angle-bracket slug placeholder")
		}
		if !IsExemptSubject(msg.Subject) {
			findings := audit.CheckConventionalCommit(
				git.Commit{Subject: msg.Subject}, audit.Resolve(config.AuditScopes(cfg.Audit)))
			if len(findings) > 0 {
				document, err := audit.ConventionalCommitDocument(findings)
				if err != nil { // coverage-ignore: CheckConventionalCommit produces fixed single-line detail and the owner mapping uses fixed grammar
					return Result{}, err
				}
				return Result{Document: document, HasDocument: true}, fmt.Errorf("check staged commit: rejected %q", msg.Subject)
			}
		}
	}
	if cfg.Profile == catalog.ProfileCore {
		return Result{}, nil
	}
	result, authErr := dependencies.Authorize(ctx, root, repo, msg)
	var operation Result
	if result.Category != "" {
		diagnostic, err := dependencies.Diagnostic(result)
		if err != nil {
			return Result{}, fmt.Errorf("check staged commit: render authorization diagnostic: %w", err)
		}
		document, err := dependencies.DiagnosticDocument(diagnostic)
		if err != nil {
			return Result{}, fmt.Errorf("check staged commit: render authorization diagnostic: %w", err)
		}
		operation = Result{Document: document, HasDocument: true, Authorization: true}
	}
	if authErr != nil {
		var syntax *commitmsg.SyntaxError
		if errors.As(authErr, &syntax) {
			return operation, fmt.Errorf("check staged commit: stale merge authorization refused: %w", authErr)
		}
		return operation, fmt.Errorf("check staged commit: stale merge authorization: %w", authErr)
	}
	if len(result.NextActions) > 0 {
		return operation, errors.New("check staged commit: stale merge authorization refused")
	}
	return operation, nil
}

// AuthorizationDiagnostic maps a completed coordinator result to command output.
func AuthorizationDiagnostic(result currentstatecoord.CommitAuthorizationResult) (presentation.Diagnostic, error) {
	yesNo := func(changed bool) string {
		if changed {
			return "yes"
		}
		return "no"
	}
	changed := make([]presentation.Field, 0, 3)
	for _, axis := range []struct{ label, value string }{
		{"index", yesNo(result.ChangedIndex)},
		{"message", yesNo(result.ChangedMessage)},
		{"merge state", yesNo(result.ChangedMergeState)},
	} {
		value, err := presentation.Literal(axis.value)
		if err != nil { // coverage-ignore: yes/no literal is fixed valid text
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(axis.label, value)
		if err != nil { // coverage-ignore: fixed axis labels are presentation-valid
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	steps := make([]presentation.Value, len(result.NextActions))
	for i, action := range result.NextActions {
		value, err := presentation.Prose(action)
		if err != nil {
			return presentation.Diagnostic{}, err
		}
		steps[i] = value
	}
	return presentation.Diagnostic{Condition: result.Condition, State: result.Category, Changed: changed, Steps: steps}, nil
}

// IsExemptSubject reports whether Git itself generates the subject.
func IsExemptSubject(subject string) bool {
	return strings.HasPrefix(subject, "Merge ") ||
		strings.HasPrefix(subject, "fixup!") ||
		strings.HasPrefix(subject, "squash!") ||
		strings.HasPrefix(subject, "amend!")
}
