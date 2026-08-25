package upgrade

import (
	"context"
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// attestationVersion is the only accepted bridge attestation format version.
const attestationVersion = 1

// Verify checks only the sealed facts of att: its version, that current HEAD
// equals the sealed PreparedHead, and that the recomputed tree digest equals the
// sealed TreeDigest. It reads the tree read-only. The sealed legacy adjudication
// is trusted through this unchanged seal alone, because the current-state binary
// ships no inventory, approval parser, or cross-schema adapter to recompute it.
func Verify(ctx context.Context, root string, att *manifest.BridgeAttestation) error {
	tree, err := filesystem.Open(root)
	if err != nil {
		return err
	}
	// ADR-consumer-local-contracts-over-single-home-filesystem-access: a read-only close failure has no durable state implication.
	defer tree.Close()
	return verifyWithFilesystem(ctx, root, att, tree)
}

func verifyWithFilesystem(ctx context.Context, root string, att *manifest.BridgeAttestation, tree attestationTree) error {
	if att.Version != attestationVersion {
		return fmt.Errorf("unsupported current-state attestation version %d", att.Version)
	}
	repo, _, err := git.OpenContaining(root)
	if err != nil {
		return err
	}
	head, err := repo.HeadHash(ctx)
	if err != nil {
		return err
	}
	if head != att.PreparedHead {
		return fmt.Errorf("HEAD %s does not match the sealed prepared head %s; %s", head, att.PreparedHead, gitRestorationGuidance)
	}
	digest, err := treeDigest(root, tree)
	if err != nil {
		return err
	}
	if digest != att.TreeDigest {
		return fmt.Errorf("prepared tree digest %s does not match the sealed digest %s; %s", digest, att.TreeDigest, gitRestorationGuidance)
	}
	return nil
}

// FinalUpgrade consumes a sealed bridge attestation. It verifies only the sealed
// facts, then journals the cutover output plan: the single deletion of the
// migration approval file and the lock replacement last, which drops the
// consumed attestation and discards its historical routing payload. The lock
// replacement is the transaction commit point; a pre-commit
// failure rolls back, a post-commit failure leaves a recoverable journal.
func FinalUpgrade(ctx context.Context, root string, lock *manifest.Lock) (Outcome, error) {
	state, err := lock.AuthorityState()
	if err != nil {
		return Outcome{}, fmt.Errorf("invalid authority: restore .awf/awf.lock from version control; run `awf upgrade --recover` if a journal exists: %w", err)
	}
	if state != manifest.AuthorityBridge {
		return Outcome{}, errors.New("no current-state attestation to consume")
	}
	att := lock.BridgeAttestation
	if err := Verify(ctx, root, att); err != nil {
		return Outcome{}, err
	}
	operation := productionJournalOperation()
	var outcome Outcome
	err = withBoundJournalOperation(root, operation, func(bound journalOperation) error {
		ops, planErr := cutoverOperationsWith(root, lock, bound)
		if planErr != nil { // coverage-ignore: Verify sealed the inputs, so only a concurrent filesystem change can invalidate cutover planning here
			return planErr
		}
		var commitErr error
		outcome, commitErr = commitTransactionBound(root, ops, bound)
		return commitErr
	})
	return outcome, err
}

// cutoverOperations builds the two-operation cutover plan: delete the sealed
// migration approval file, then replace the lock last. The replacement lock
// drops the attestation and its historical routing payload. The approval file
// must be present so the transaction journals exactly one
// deletion of it.
func cutoverOperationsWith(root string, lock *manifest.Lock, operation journalOperation) ([]Operation, error) {
	final := *lock
	final.BridgeAttestation = nil
	finalBytes, err := final.Marshal()
	if err != nil { // coverage-ignore: the lock marshals cleanly; see manifest.Marshal
		return nil, err
	}
	approvalPrior, err := operation.imageOf(root, approvalPath)
	if err != nil {
		return nil, err
	}
	if !approvalPrior.Present {
		return nil, fmt.Errorf("the sealed migration approval file %s is absent; %s", approvalPath, gitRestorationGuidance)
	}
	lockPrior, err := operation.imageOf(root, LockRel())
	if err != nil {
		return nil, err
	}
	ops := []Operation{
		{Path: approvalPath, Prior: approvalPrior, Replacement: Image{Present: false}},
		{Path: LockRel(), Prior: lockPrior, Replacement: Image{Present: true, Mode: 0o644, Content: finalBytes}},
	}
	if err := validateOperations(ops); err != nil { // coverage-ignore: the two-op cutover plan is well-formed by construction
		return nil, err
	}
	return ops, nil
}
