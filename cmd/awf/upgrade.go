package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/bridge"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

func runUpgradeFlags(root string, check, jsonOutput bool, stdout io.Writer) error {
	if jsonOutput && !check {
		return &usageErr{"awf upgrade: --json requires --check"}
	}
	if !check {
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
