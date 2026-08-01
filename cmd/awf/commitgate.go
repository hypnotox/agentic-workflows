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
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runCommitGate validates one commit message and returns an error (mapped to a
// non-zero exit) on any violation, so a commit-msg hook calling it blocks the
// commit. It applies two checks: the shared Conventional Commits rule, and,
// while memoryCite.enabled is true, a scan for a citation of a specific
// working-memory file (ADR-0158). The git-generated-subject exemption scopes the
// Conventional Commits check alone: git writes the subject, but a person may
// edit a merge or autosquash body, so the citation scan applies to every
// recorded message. The message comes from msgPath (the file a commit-msg hook
// passes as $1) or stdin when msgPath is empty; citation line numbers are
// relative to the git-cleaned message, not to the raw file.
func runCommitGate(ctx context.Context, root, msgPath string, stdin io.Reader, stdout io.Writer) error {
	var raw []byte
	var err error
	if msgPath != "" {
		raw, err = os.ReadFile(msgPath)
	} else {
		raw, err = io.ReadAll(stdin)
	}
	if err != nil {
		return fmt.Errorf("check commit: read message: %w", err)
	}
	msg := commitmsg.Clean(raw)
	// An empty subject aborts the commit in git itself, so nothing is recorded
	// and there is nothing to guard.
	if msg.Subject == "" {
		return nil
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return fmt.Errorf("check commit: %w", err)
	}
	if p.Cfg.MemoryCite != nil && p.Cfg.MemoryCite.Enabled {
		// Scan the cleaned message, never the raw bytes: `git commit -v` appends
		// the staged diff below a scissors line, and a diff may legitimately carry
		// text git itself discards.
		refs := memorycite.ScanText("commit message", []byte(msg.Text))
		for _, r := range refs {
			fmt.Fprintf(stdout, "check commit: %s line %d names the effort-owned memory file %q\n", r.Path, r.Line, r.Segment)
		}
		if len(refs) > 0 {
			return errors.New("check commit: a commit message must not cite a concrete effort-owned memory file; name the bare .awf/efforts/ directory or use an angle-bracket slug placeholder")
		}
	}
	// A git-generated merge or autosquash subject is exempt from the Conventional
	// Commits rule - never block what git produced or will rewrite.
	if !isExemptSubject(msg.Subject) {
		findings := audit.CheckConventionalCommit(
			git.Commit{Subject: msg.Subject}, audit.Resolve(p.Cfg.Audit))
		if len(findings) > 0 {
			for _, f := range findings {
				fmt.Fprintf(stdout, "check commit: %s\n", f.Detail)
			}
			return fmt.Errorf("check commit: rejected %q", msg.Subject)
		}
	}
	result, authErr := p.CheckCommitAuthorization(ctx, msg)
	if result.Category != "" {
		fmt.Fprintln(stdout, result.String())
	}
	if authErr != nil {
		var syntax *commitmsg.SyntaxError
		if errors.As(authErr, &syntax) {
			return fmt.Errorf("check commit: stale merge authorization refused: %w", authErr)
		}
		return fmt.Errorf("check commit: stale merge authorization: %w", authErr)
	}
	if len(result.NextActions) > 0 {
		return errors.New("check commit: stale merge authorization refused")
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
