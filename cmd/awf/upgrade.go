package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/bridge"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// runUpgradeFlags routes the mutually exclusive upgrade modes. Plain upgrade
// migrates the schema and syncs; --check reports readiness read-only; --json
// requires --check; --attest-current-state seals a clean prepared tree through
// the recoverable journal; --recover replays the journal recovery table.
func runUpgradeFlags(root string, check, jsonOutput, attest, recoverMode bool, stdout io.Writer) error {
	modes := 0
	for _, on := range []bool{check, attest, recoverMode} {
		if on {
			modes++
		}
	}
	if modes > 1 {
		return &usageErr{"awf upgrade: --check, --attest-current-state, and --recover are mutually exclusive"}
	}
	if jsonOutput && !check {
		return &usageErr{"awf upgrade: --json requires --check"}
	}
	switch {
	case recoverMode:
		return runRecover(root, stdout)
	case attest:
		return runAttest(root, stdout)
	case !check:
		return runUpgrade(root, stdout)
	}
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	report := bridge.Check(root)
	if jsonOutput {
		if err := writeReadinessJSON(stdout, report); err != nil {
			return err
		}
		if !report.Ready {
			return errors.New("current-state upgrade is not ready")
		}
		return nil
	}
	writeReadinessHuman(stdout, report)
	if !report.Ready {
		return errors.New("current-state upgrade is not ready")
	}
	return nil
}

// runRecover replays the current-state upgrade journal recovery table. It never
// runs project tests or gates and prints deterministic operation lines.
func runRecover(root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	return bridge.Recover(root, stdout)
}

// runAttest seals a clean prepared tree. It requires readiness and a clean HEAD,
// records the digest, cutoff, and gaps in the lock, and applies every
// normalization, marker, status, and terminal-output write through a recoverable
// journal with the attestation lock committed last. It never runs project tests
// or gates and prints one deterministic operation line per applied path.
func runAttest(root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	report := bridge.Check(root)
	if !report.Ready {
		writeReadinessHuman(stdout, report)
		return errors.New("current-state upgrade is not ready")
	}
	head, err := awfgit.HeadAndClean(root)
	if err != nil {
		return err
	}
	lockPath := config.LockPath(root)
	lock, found, err := manifest.LoadOptional(lockPath)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("no .awf/awf.lock to attest (run `awf sync` first)")
	}
	priorBytes, err := os.ReadFile(lockPath)
	if err != nil { // coverage-ignore: LoadOptional just read the lock successfully; failure requires a concurrent filesystem race
		return err
	}
	priorInfo, err := os.Stat(lockPath)
	if err != nil { // coverage-ignore: the same lock path was read immediately above; failure requires a concurrent filesystem race
		return err
	}
	digest, err := bridge.Digest(root, report.PlannedMutations)
	if err != nil { // coverage-ignore: readiness validated the same config and inputs the digest reads
		return err
	}
	cutoff, gaps, err := bridge.CutoffFacts(root)
	if err != nil { // coverage-ignore: readiness loaded the same corpus before this call
		return err
	}
	lock.BridgeAttestation = &manifest.BridgeAttestation{Version: 1, PreparedHead: head, TreeDigest: digest, ADRFormatV1From: cutoff, LegacyADRGaps: gaps}
	finalBytes, err := lock.Marshal()
	if err != nil { // coverage-ignore: the lock marshals cleanly; see manifest.Marshal
		return err
	}
	lockPrior := bridge.Image{Present: true, Mode: uint32(priorInfo.Mode().Perm()), Content: priorBytes}
	lockFinal := bridge.Image{Present: true, Mode: 0o644, Content: finalBytes}
	ops, err := bridge.OperationsFromMutations(root, report.PlannedMutations, lockPrior, lockFinal)
	if err != nil { // coverage-ignore: readiness mutations are validated and unique; the lock op closes the set
		return err
	}
	return bridge.CommitTransaction(root, ops, stdout)
}

func writeReadinessHuman(w io.Writer, report bridge.Report) {
	fmt.Fprintf(w, "ready: %t\n", report.Ready)
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "finding: %s %s: %s\n", finding.Code, finding.Path, finding.Detail)
	}
	for _, adjudication := range report.InvariantAdjudications {
		fmt.Fprintf(w, "invariant: %s %s destination=%s origin=%s backing=%s approved=%t\n", adjudication.Key, adjudication.Disposition, adjudication.Destination, adjudication.Origin, adjudication.Backing, adjudication.Approved)
	}
	for _, mutation := range report.PlannedMutations {
		fmt.Fprintf(w, "mutation: %s %t/%04o/%s -> %t/%04o/%s\n", mutation.Path, mutation.BeforePresent, mutation.BeforeMode, mutation.BeforeSHA256, mutation.AfterPresent, mutation.AfterMode, mutation.AfterSHA256)
	}
}

type readinessJSON struct {
	Ready                  bool                     `json:"ready"`
	Findings               []readinessFindingJSON   `json:"findings"`
	InvariantAdjudications []readinessInvariantJSON `json:"invariantAdjudications"`
	PlannedMutations       []bridge.Mutation        `json:"plannedMutations"`
}
type readinessFindingJSON struct {
	Code   string `json:"code"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}
type readinessInvariantJSON struct {
	Key         string `json:"key"`
	Disposition string `json:"disposition"`
	Destination string `json:"destination"`
	Origin      string `json:"origin"`
	Backing     string `json:"backing"`
	Approved    bool   `json:"approved"`
}

func writeReadinessJSON(w io.Writer, report bridge.Report) error {
	out := readinessJSON{Ready: report.Ready, PlannedMutations: report.PlannedMutations}
	out.Findings = make([]readinessFindingJSON, 0, len(report.Findings))
	for _, f := range report.Findings {
		out.Findings = append(out.Findings, readinessFindingJSON{f.Code, f.Path, f.Detail})
	}
	out.InvariantAdjudications = make([]readinessInvariantJSON, 0, len(report.InvariantAdjudications))
	for _, a := range report.InvariantAdjudications {
		out.InvariantAdjudications = append(out.InvariantAdjudications, readinessInvariantJSON{a.Key, a.Disposition, a.Destination, a.Origin, a.Backing, a.Approved})
	}
	out.PlannedMutations = append([]bridge.Mutation{}, out.PlannedMutations...)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

// runUpgrade applies every registered migration past the project's current
// schema generation, then always runs a normal sync - even when no migration
// applies - so a same-schema binary bump still re-renders every managed file
// and re-pins the bootstrap (ADR-0085 Decision 4). Truthful edge states
// (ADR-0076 Decision 4): no config layout at all → the awf init hint; a tree
// whose schema is ahead of this binary → the version-gate guidance.
func runUpgrade(root string, stdout io.Writer) error {
	if !migrate.ProjectPresent(root) {
		return errors.New("not an awf project (run `awf init`)")
	}
	state, gen, err := migrate.GateState(root)
	if err != nil {
		return err
	}
	if state == "ahead" {
		return schemaAheadError(gen)
	}
	applied, err := migrate.Upgrade(root, stdout)
	if err != nil {
		return err
	}
	// invariant: upgrade-always-syncs
	if len(applied) == 0 {
		fmt.Fprintf(stdout, "awf upgrade: config already at schema %d\n", gen)
	}
	for _, name := range applied {
		fmt.Fprintf(stdout, "awf upgrade: applied %s\n", name)
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
