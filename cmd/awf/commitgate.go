package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runCommitGate validates one commit message against the shared Conventional
// Commits rule and returns an error (mapped to a non-zero exit) on any violation,
// so a commit-msg hook calling it blocks the commit. The message comes from msgPath
// (the file a commit-msg hook passes as $1) or stdin when msgPath is empty.
func runCommitGate(root, msgPath string, stdin io.Reader, stdout io.Writer) error {
	var raw []byte
	var err error
	if msgPath != "" {
		raw, err = os.ReadFile(msgPath)
	} else {
		raw, err = io.ReadAll(stdin)
	}
	if err != nil {
		return fmt.Errorf("commit-gate: read message: %w", err)
	}
	subject := cleanCommitSubject(string(raw))
	// An empty subject (git aborts the commit itself) or a git-generated merge /
	// autosquash subject is exempt - never block what git produced or will rewrite.
	if subject == "" || isExemptSubject(subject) {
		return nil
	}
	p, err := project.Open(root)
	if err != nil {
		return fmt.Errorf("commit-gate: %w", err)
	}
	findings := audit.CheckConventionalCommit(
		audit.Commit{Subject: subject}, audit.Resolve(p.Cfg.Audit))
	if len(findings) == 0 {
		return nil
	}
	for _, f := range findings {
		fmt.Fprintf(stdout, "commit-gate: %s\n", f.Detail)
	}
	return fmt.Errorf("commit-gate: rejected %q", subject)
}

// cleanCommitSubject mirrors git's default commit.cleanup=strip: it drops comment
// lines (first non-blank char is the default '#'), stops at a verbose scissors
// line, and returns the first surviving non-blank line as the subject.
func cleanCommitSubject(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			if strings.Contains(t, ">8") { // scissors line: ignore everything below
				break
			}
			continue
		}
		return strings.TrimRight(line, " ")
	}
	return ""
}

// isExemptSubject reports whether a subject is one git itself generates - a merge
// or an autosquash (fixup!/squash!/amend!) - which the gate must not block.
func isExemptSubject(s string) bool {
	return strings.HasPrefix(s, "Merge ") ||
		strings.HasPrefix(s, "fixup!") ||
		strings.HasPrefix(s, "squash!") ||
		strings.HasPrefix(s, "amend!")
}
