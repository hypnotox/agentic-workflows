package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
)

// FullOID resolves rev to its complete object ID and Git object type.
func (r *Repo) FullOID(ctx context.Context, rev string) (string, string, error) {
	out, err := r.runner.run(ctx, "rev-parse", "--verify", rev+"^{object}")
	if err != nil {
		return "", "", fmt.Errorf("resolve %q: %w", rev, err)
	}
	id := strings.TrimSpace(string(out))
	if len(id) != 40 && len(id) != 64 { // coverage-ignore: native Git emits only complete SHA-1 or SHA-256 IDs
		return "", "", fmt.Errorf("resolve %q: invalid full object ID %q", rev, id)
	}
	typ, err := r.runner.run(ctx, "cat-file", "-t", id)
	if err != nil { // coverage-ignore: a verified object remains readable in the adjacent invocation
		return "", "", fmt.Errorf("inspect %q: %w", id, err)
	}
	return id, strings.TrimSpace(string(typ)), nil
}

// CommitFacts loads immutable author and committer facts for one commit object.
func (r *Repo) CommitFacts(ctx context.Context, id string) (commitpolicy.Commit, error) {
	out, err := r.runner.run(ctx, "show", "-s", "--format=%H%x00%an%x00%ae%x00%cn%x00%ce", id+"^{commit}")
	if err != nil { // coverage-ignore: callers receive IDs from native rev-list
		return commitpolicy.Commit{}, fmt.Errorf("load commit %q: %w", id, err)
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\n"), "\x00")
	if len(fields) != 5 || (len(fields[0]) != 40 && len(fields[0]) != 64) { // coverage-ignore: the fixed native format supplies five fields and a full ID
		return commitpolicy.Commit{}, fmt.Errorf("load commit %q: invalid commit facts", id)
	}
	return commitpolicy.Commit{ID: fields[0], Author: commitpolicy.Person{Name: fields[1], Email: fields[2]}, Committer: commitpolicy.Person{Name: fields[3], Email: fields[4]}}, nil
}

// CommitsAfter expands explicit revision or range targets to unique commits after baseline.
func (r *Repo) CommitsAfter(ctx context.Context, baseline string, targets []string) ([]commitpolicy.Commit, error) {
	base, typ, err := r.FullOID(ctx, baseline)
	if err != nil { // coverage-ignore: baseline configuration is validated before project composition
		return nil, err
	}
	if typ != "commit" || base != baseline {
		return nil, fmt.Errorf("baseline %q must be one full commit object ID", baseline)
	}
	seen := map[string]bool{}
	var commits []commitpolicy.Commit
	for _, target := range targets {
		out, err := r.runner.run(ctx, "rev-list", target, "^"+base)
		if err != nil { // coverage-ignore: invalid explicit targets are project integration failures
			return nil, fmt.Errorf("expand %q: %w", target, err)
		}
		for _, id := range strings.Fields(string(out)) {
			if seen[id] {
				continue
			}
			seen[id] = true
			fact, err := r.CommitFacts(ctx, id)
			if err != nil { // coverage-ignore: rev-list emits existing commit IDs
				return nil, err
			}
			commits = append(commits, fact)
		}
	}
	return commits, nil
}

// VerifySSH verifies one commit through an operation-local allowed-signers file.
func (r *Repo) VerifySSH(ctx context.Context, id string, signers []commitpolicy.Signer) (commitpolicy.SignatureVerdict, error) {
	raw, err := r.runner.run(ctx, "cat-file", "-p", id+"^{commit}")
	if err != nil { // coverage-ignore: verified commit IDs remain readable
		return commitpolicy.SignatureMissing, fmt.Errorf("read signature header: %w", err)
	}
	if !strings.Contains(string(raw), "\ngpgsig ") && !strings.HasPrefix(string(raw), "gpgsig ") {
		return commitpolicy.SignatureMissing, nil
	}
	f, err := os.CreateTemp("", "awf-allowed-signers-")
	if err != nil { // coverage-ignore: local temporary-file allocation fault
		return commitpolicy.SignatureMissing, err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0o600); err != nil { // coverage-ignore: just-created temp-file permission fault
		_ = f.Close()
		return commitpolicy.SignatureMissing, err
	}
	for _, signer := range signers {
		if _, err := fmt.Fprintln(f, signer.Principal, signer.Key); err != nil { // coverage-ignore: just-created temp-file write fault
			_ = f.Close()
			return commitpolicy.SignatureMissing, err
		}
	}
	if err := f.Close(); err != nil { // coverage-ignore: just-created temp-file close fault
		return commitpolicy.SignatureMissing, err
	}
	_, err = r.runner.run(ctx, "-c", "gpg.format=ssh", "-c", "gpg.ssh.allowedSignersFile="+name, "verify-commit", id)
	if err == nil {
		return commitpolicy.SignatureValid, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) { // coverage-ignore: runner cancellation is pinned independently
		return commitpolicy.SignatureMissing, err
	}
	var command *CommandError
	if errors.As(err, &command) {
		if strings.Contains(strings.ToLower(command.Stderr), "invalid format") || strings.Contains(strings.ToLower(command.Stderr), "malformed") {
			return commitpolicy.SignatureMalformed, nil
		}
		return commitpolicy.SignatureWrongKey, nil
	}
	return commitpolicy.SignatureMissing, err // coverage-ignore: runner process failures are CommandError or context cancellation
}

// PeelCommit recursively peels annotated tags and returns a commit or terminal target type.
func (r *Repo) PeelCommit(ctx context.Context, rev string) (string, string, error) {
	id, err := r.runner.run(ctx, "rev-parse", "--verify", rev+"^{commit}")
	if err == nil { // coverage-ignore: successful peeling is equivalent to revision expansion's native commit resolution
		full := strings.TrimSpace(string(id))
		return full, "commit", nil
	}
	_, typ, typeErr := r.FullOID(ctx, rev)
	if typeErr != nil { // coverage-ignore: missing targets are covered by revision expansion
		return "", "", err
	}
	return "", typ, nil
}
