package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

// runUpgradeFlags routes the two current-state upgrade modes. Plain upgrade
// either consumes a sealed bridge attestation (the final cutover) or, with no
// attestation, migrates the schema and syncs. --recover replays the journal
// recovery table. Attestation, readiness, and their reporting live only in the
// preceding bridge release; the current-state binary consumes seals, it never
// produces them.
func runUpgradeFlags(root string, recoverMode bool, stdout io.Writer) error {
	if recoverMode {
		return runRecover(root, stdout)
	}
	return runUpgrade(root, stdout)
}

// runRecover replays the current-state upgrade journal recovery table. It never
// runs project tests or gates and prints deterministic operation lines.
func runRecover(root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	return upgrade.Recover(root, stdout)
}

// runUpgrade consumes a sealed attestation when the lock carries one: the final
// current-state cutover verifies only the sealed facts and journals the approval
// deletion plus the permanent lock (promoting the sealed cutoff/gaps). With no
// attestation it applies every registered migration past the project's current
// schema generation, then always runs a normal sync - even when no migration
// applies - so a same-schema binary bump still re-renders every managed file and
// re-pins the bootstrap (ADR-0085 Decision 4). Truthful edge states
// (ADR-0076 Decision 4): no config layout at all → the awf init hint; a tree
// whose schema is ahead of this binary → the version-gate guidance.
func runUpgrade(root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	authorityPath := authorityLockPath(root)
	lock, found, err := manifest.LoadOptional(authorityPath)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("pre-tracking authority: use the bridge release to attest before upgrading")
	}
	authority, err := lock.AuthorityState()
	if err != nil { // coverage-ignore: LoadOptional parsed and validated this unchanged lock
		return fmt.Errorf("invalid authority: restore .awf/awf.lock from version control; if a cutover journal exists run `awf upgrade --recover`: %w", err)
	}
	switch authority {
	case manifest.AuthorityBridge:
		return upgrade.FinalUpgrade(root, lock, stdout)
	case manifest.AuthorityPermanent:
		// Continue with ordinary schema migration and sync.
	case manifest.AuthorityPreTracking:
		return errors.New("pre-tracking authority: use the bridge release to attest before upgrading")
	default: // coverage-ignore: AuthorityState returns only the closed enum values
		return errors.New("invalid authority: restore .awf/awf.lock from version control")
	}
	state, gen, err := migrate.GateState(root)
	if err != nil { // coverage-ignore: the authority load just read the same active layout lock; only a concurrent fault can newly fail
		return err
	}
	if state == "ahead" {
		return schemaAheadError(gen)
	}
	applied, err := migrate.Upgrade(root, stdout)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Fprintf(stdout, "awf upgrade: config already at schema %d\n", gen)
	}
	for _, name := range applied {
		fmt.Fprintf(stdout, "awf upgrade: applied %s\n", name)
	}
	if authorityPath != config.LockPath(root) {
		lock.SchemaVersion = migrate.Current()
		if err := lock.Save(config.LockPath(root)); err != nil { // coverage-ignore: migration just created the writable current config directory
			return err
		}
	}
	// Gate before the terminal sync: migration brings the schema current, but a
	// binary behind the lock's awfVersion (version axis, schema equal) must still
	// refuse rather than re-stamp a downgraded version. runSync no longer self-gates,
	// so upgrade re-asserts it here (schema-ahead is already refused above).
	if err := gate(root); err != nil {
		return err
	}
	return runSync(root, stdout)
}
