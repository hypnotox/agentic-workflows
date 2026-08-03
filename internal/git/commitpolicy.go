package git

import (
	"context"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
)

// CommitPolicyErrorKind identifies the Git operation that prevented policy evaluation.
type CommitPolicyErrorKind string

const (
	CommitPolicyBaselineError CommitPolicyErrorKind = "baseline"
	CommitPolicyRevisionError CommitPolicyErrorKind = "revision-resolution"
	CommitPolicyTagPeelError  CommitPolicyErrorKind = "tag-peel"
	CommitPolicyTrustError    CommitPolicyErrorKind = "temporary-trust-file"
	CommitPolicyVerifyError   CommitPolicyErrorKind = "signature-process"
)

// CommitPolicyError preserves one operational Git failure for project-level translation.
type CommitPolicyError struct {
	Kind   CommitPolicyErrorKind
	Target string
	Err    error
}

func (e *CommitPolicyError) Error() string {
	if e == nil {
		return "commit policy Git failure"
	}
	if e.Target == "" {
		return fmt.Sprintf("%s: %v", e.Kind, e.Err)
	}
	return fmt.Sprintf("%s %q: %v", e.Kind, e.Target, e.Err)
}

func (e *CommitPolicyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// FullOID resolves rev to its complete object ID and Git object type.
func (r *Repo) FullOID(ctx context.Context, rev string) (string, string, error) {
	out, err := r.runner.run(ctx, "rev-parse", "--verify", rev+"^{object}")
	if err != nil {
		return "", "", fmt.Errorf("resolve %q: %w", rev, err)
	}
	id := strings.TrimSpace(string(out))
	width, err := r.objectIDWidth(ctx)
	if err != nil {
		return "", "", err
	}
	if !validObjectID(id, width) {
		return "", "", fmt.Errorf("resolve %q: invalid full object ID %q", rev, id)
	}
	typ, err := r.runner.run(ctx, "cat-file", "-t", id)
	if err != nil { // coverage-ignore: requires the resolved object to disappear between adjacent read-only invocations
		return "", "", fmt.Errorf("inspect %q: %w", id, err)
	}
	return id, strings.TrimSpace(string(typ)), nil
}

func (r *Repo) objectIDWidth(ctx context.Context) (int, error) {
	out, err := r.runner.run(ctx, "rev-parse", "--show-object-format")
	if err != nil {
		return 0, fmt.Errorf("resolve repository object format: %w", err)
	}
	switch strings.TrimSpace(string(out)) {
	case "sha1":
		return 40, nil
	case "sha256":
		return 64, nil
	default:
		return 0, fmt.Errorf("unsupported repository object format %q", strings.TrimSpace(string(out)))
	}
}

func validObjectID(id string, width int) bool {
	if len(id) != width {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

// CommitFacts loads immutable author and committer facts for one commit object.
func (r *Repo) CommitFacts(ctx context.Context, id string) (commitpolicy.Commit, error) {
	raw, err := r.runner.run(ctx, "cat-file", "commit", id)
	if err != nil {
		return commitpolicy.Commit{}, fmt.Errorf("load commit %q: %w", id, err)
	}
	author, committer, err := parseCommitPeople(raw)
	if err != nil {
		return commitpolicy.Commit{}, fmt.Errorf("load commit %q: %w", id, err)
	}
	return commitpolicy.Commit{ID: id, Author: author, Committer: committer}, nil
}

func parseCommitPeople(raw []byte) (commitpolicy.Person, commitpolicy.Person, error) {
	var author, committer commitpolicy.Person
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			break
		}
		switch {
		case strings.HasPrefix(line, "author "):
			var err error
			author, err = parseCommitPerson(strings.TrimPrefix(line, "author "))
			if err != nil {
				return commitpolicy.Person{}, commitpolicy.Person{}, fmt.Errorf("parse author: %w", err)
			}
		case strings.HasPrefix(line, "committer "):
			var err error
			committer, err = parseCommitPerson(strings.TrimPrefix(line, "committer "))
			if err != nil {
				return commitpolicy.Person{}, commitpolicy.Person{}, fmt.Errorf("parse committer: %w", err)
			}
		}
	}
	if author == (commitpolicy.Person{}) || committer == (commitpolicy.Person{}) {
		return commitpolicy.Person{}, commitpolicy.Person{}, errors.New("missing author or committer header")
	}
	return author, committer, nil
}

func parseCommitPerson(value string) (commitpolicy.Person, error) {
	end := strings.LastIndex(value, "> ")
	start := strings.LastIndex(value[:max(end, 0)], " <")
	if start < 0 || end <= start+2 {
		return commitpolicy.Person{}, fmt.Errorf("malformed identity header %q", value)
	}
	return commitpolicy.Person{Name: value[:start], Email: value[start+2 : end]}, nil
}

// CommitsAfter expands explicit revision or range targets to unique commits after baseline.
func (r *Repo) CommitsAfter(ctx context.Context, baseline string, targets []string) ([]commitpolicy.Commit, error) {
	if len(targets) == 0 {
		return nil, policyGitError(CommitPolicyRevisionError, "", errors.New("at least one explicit revision or range is required"))
	}
	width, err := r.objectIDWidth(ctx)
	if err != nil {
		return nil, policyGitError(CommitPolicyBaselineError, baseline, err)
	}
	if !validObjectID(baseline, width) {
		return nil, policyGitError(CommitPolicyBaselineError, baseline, fmt.Errorf("must be one full object ID of width %d", width))
	}
	base, typ, err := r.FullOID(ctx, baseline)
	if err != nil {
		return nil, policyGitError(CommitPolicyBaselineError, baseline, err)
	}
	if typ != "commit" || base != baseline {
		return nil, policyGitError(CommitPolicyBaselineError, baseline, errors.New("must resolve exactly to a commit object"))
	}
	seen := map[string]bool{}
	for _, target := range targets {
		args := []string{"rev-list", target, "^" + base}
		if !isRangeTarget(target) {
			peeled, terminal, peelErr := r.PeelCommit(ctx, target)
			if peelErr != nil {
				return nil, policyGitError(CommitPolicyRevisionError, target, peelErr)
			}
			if terminal != "commit" {
				return nil, policyGitError(CommitPolicyTagPeelError, target, fmt.Errorf("terminal target type is %s", terminal))
			}
			args = []string{"rev-list", peeled, "^" + base}
		}
		out, runErr := r.runner.run(ctx, args...)
		if runErr != nil {
			return nil, policyGitError(CommitPolicyRevisionError, target, fmt.Errorf("expand target: %w", runErr))
		}
		for _, id := range strings.Fields(string(out)) {
			seen[id] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	commits := make([]commitpolicy.Commit, 0, len(ids))
	for _, id := range ids {
		fact, factErr := r.CommitFacts(ctx, id)
		if factErr != nil { // coverage-ignore: requires a rev-list object to disappear between adjacent read-only invocations
			return nil, policyGitError(CommitPolicyRevisionError, id, factErr)
		}
		commits = append(commits, fact)
	}
	return commits, nil
}

func isRangeTarget(target string) bool {
	return strings.Contains(target, "..")
}

func policyGitError(kind CommitPolicyErrorKind, target string, err error) error {
	return &CommitPolicyError{Kind: kind, Target: target, Err: err}
}

// VerifySSH verifies one commit through an operation-local allowed-signers file.
func (r *Repo) VerifySSH(ctx context.Context, id string, signers []commitpolicy.Signer) (commitpolicy.SignatureVerdict, error) {
	raw, err := r.runner.run(ctx, "cat-file", "commit", id)
	if err != nil {
		return commitpolicy.SignatureMissing, policyGitError(CommitPolicyVerifyError, id, fmt.Errorf("read commit signature: %w", err))
	}
	signature := commitSignature(raw)
	if signature == "" {
		return commitpolicy.SignatureMissing, nil
	}
	block, rest := pem.Decode([]byte(signature))
	if block == nil || block.Type != "SSH SIGNATURE" || len(block.Bytes) == 0 || len(strings.TrimSpace(string(rest))) != 0 {
		return commitpolicy.SignatureMalformed, nil
	}
	create := r.createTemp
	if create == nil {
		create = os.CreateTemp
	}
	f, err := create(r.root, ".awf-allowed-signers-")
	if err != nil {
		return commitpolicy.SignatureMissing, policyGitError(CommitPolicyTrustError, r.root, err)
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0o600); err != nil { // coverage-ignore: requires a filesystem fault after successful temporary-file creation
		_ = f.Close()
		return commitpolicy.SignatureMissing, policyGitError(CommitPolicyTrustError, name, err)
	}
	for _, signer := range signers {
		if _, err := fmt.Fprintln(f, signer.Principal, signer.Key); err != nil { // coverage-ignore: requires a filesystem fault after successful temporary-file creation
			_ = f.Close()
			return commitpolicy.SignatureMissing, policyGitError(CommitPolicyTrustError, name, err)
		}
	}
	if err := f.Close(); err != nil { // coverage-ignore: requires a filesystem fault after successful temporary-file creation
		return commitpolicy.SignatureMissing, policyGitError(CommitPolicyTrustError, name, err)
	}
	_, err = r.runner.run(ctx, "-c", "gpg.format=ssh", "-c", "gpg.ssh.allowedSignersFile="+name, "verify-commit", id)
	if err == nil {
		return commitpolicy.SignatureValid, nil
	}
	var command *CommandError
	if errors.As(err, &command) && command.ExitCode == 1 && command.Err == nil {
		return commitpolicy.SignatureWrongKey, nil
	}
	return commitpolicy.SignatureMissing, policyGitError(CommitPolicyVerifyError, id, err)
}

func commitSignature(raw []byte) string {
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "gpgsig ") {
			continue
		}
		parts := []string{strings.TrimPrefix(line, "gpgsig ")}
		for j := i + 1; j < len(lines) && strings.HasPrefix(lines[j], " "); j++ {
			parts = append(parts, strings.TrimPrefix(lines[j], " "))
		}
		return strings.Join(parts, "\n") + "\n"
	}
	return ""
}

// PeelCommit recursively peels annotated tags and returns a commit or terminal target type.
func (r *Repo) PeelCommit(ctx context.Context, rev string) (string, string, error) {
	if _, _, err := r.FullOID(ctx, rev); err != nil {
		return "", "", err
	}
	peeled, err := r.runner.run(ctx, "rev-parse", "--verify", rev+"^{}")
	if err != nil { // coverage-ignore: requires the already-resolved ref or object to change between adjacent read-only invocations
		return "", "", fmt.Errorf("peel %q: %w", rev, err)
	}
	id := strings.TrimSpace(string(peeled))
	typ, err := r.runner.run(ctx, "cat-file", "-t", id)
	if err != nil { // coverage-ignore: requires the peeled object to disappear between adjacent read-only invocations
		return "", "", fmt.Errorf("inspect peeled target %q: %w", rev, err)
	}
	return id, strings.TrimSpace(string(typ)), nil
}
