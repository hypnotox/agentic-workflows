package upgrade

import (
	"errors"
	"fmt"
	"io"

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
func Verify(root string, att *manifest.BridgeAttestation) error {
	if att.Version != attestationVersion {
		return fmt.Errorf("unsupported current-state attestation version %d", att.Version)
	}
	head, err := git.HeadHash(root)
	if err != nil {
		return err
	}
	if head != att.PreparedHead {
		return fmt.Errorf("HEAD %s does not match the sealed prepared head %s; %s", head, att.PreparedHead, gitRestorationGuidance)
	}
	digest, err := treeDigest(root)
	if err != nil { // coverage-ignore: a matching PreparedHead means the sealed config parsed at seal time; re-reading that same committed tree does not fault here
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
// consumed attestation and promotes the sealed cutoff/gaps into permanent lock
// fields. The lock replacement is the transaction commit point; a pre-commit
// failure rolls back, a post-commit failure leaves a recoverable journal.
func FinalUpgrade(root string, lock *manifest.Lock, log io.Writer) error {
	state, err := lock.AuthorityState()
	if err != nil {
		return fmt.Errorf("invalid authority: restore .awf/awf.lock from version control; run `awf upgrade --recover` if a journal exists: %w", err)
	}
	if state != manifest.AuthorityBridge {
		return errors.New("no current-state attestation to consume")
	}
	att := lock.BridgeAttestation
	if err := Verify(root, att); err != nil {
		return err
	}
	ops, err := cutoverOperations(root, lock, att)
	if err != nil { // coverage-ignore: Verify already required the approval file present via the sealed digest, so cutoverOperations' only reachable error branch cannot fire here
		return err
	}
	return commitTransaction(root, ops, log)
}

// cutoverOperations builds the two-operation cutover plan: delete the sealed
// migration approval file, then replace the lock last. The replacement lock
// drops the attestation and stores the sealed cutoff/gaps permanently. The
// approval file must be present so the transaction journals exactly one
// deletion of it.
func cutoverOperations(root string, lock *manifest.Lock, att *manifest.BridgeAttestation) ([]Operation, error) {
	final := *lock
	final.BridgeAttestation = nil
	final.ADRFormatV1From = att.ADRFormatV1From
	final.LegacyADRGaps = att.LegacyADRGaps
	finalBytes, err := final.Marshal()
	if err != nil { // coverage-ignore: the lock marshals cleanly; see manifest.Marshal
		return nil, err
	}
	approvalPrior, err := imageOf(root, approvalPath)
	if err != nil { // coverage-ignore: the approval path reads cleanly unless a concurrent removal races
		return nil, err
	}
	if !approvalPrior.Present {
		return nil, fmt.Errorf("the sealed migration approval file %s is absent; %s", approvalPath, gitRestorationGuidance)
	}
	lockPrior, err := imageOf(root, LockRel())
	if err != nil { // coverage-ignore: the lock was read by LoadOptional immediately before this call
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
