package main

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runInvariants reports the current-state topic corpus's invariant claims: their
// backing mode, an unbacked claim's Verify guidance, and a test-backed claim's
// proof-marker sites (ADR-0134). Authority is the typed claim set - the
// test-backed proof and unbacked Verify contracts are enforced when the corpus
// loads, so a violation surfaces as a load error here, not a reported finding.
func runInvariants(root string, stdout io.Writer) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	invs, err := p.CurrentStateInvariants()
	if err != nil {
		return err
	}
	if len(invs) == 0 {
		fmt.Fprintln(stdout, "awf invariants: no invariant claims")
		return nil
	}
	for _, iv := range invs {
		fmt.Fprintf(stdout, "  %s [%s]\n", iv.ID, iv.Backing)
		if iv.Verify != "" {
			fmt.Fprintf(stdout, "    Verify: %s\n", iv.Verify)
		}
		for _, site := range iv.Proofs {
			fmt.Fprintf(stdout, "    proof: %s\n", site)
		}
	}
	return nil
}
