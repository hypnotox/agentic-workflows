package project

import (
	"os"
	"strings"
	"testing"
)

// ADR-0090: the committed example adopter is kept deterministic through ./x -
// sync re-renders it from source; check drift-, invariant-, note-, and
// test-gates it. The example is its own Go module so the enclosing ./...
// sweeps never see it; this test pins the wiring so it cannot be silently
// dropped.
//
// invariant: example-adopter-checked
// invariant: example-zero-notes
// invariant: example-module-isolated
func TestExampleAdopterWiring(t *testing.T) {
	raw, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatalf("read x: %v", err)
	}
	script := string(raw)
	for _, want := range []string{
		`(cd examples/sundial && "$bindir/awf" sync)`,
		`out="$(cd examples/sundial && "$bindir/awf" check)"`,
		`grep -q '^note: '`,
		`(cd examples/sundial && "$bindir/awf" invariants)`,
		`(cd examples/sundial && go test ./...)`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("x lost the example-adopter step %q (ADR-0090)", want)
		}
	}
	if _, err := os.Stat("../../examples/sundial/go.mod"); err != nil {
		t.Errorf("examples/sundial must stay its own Go module (ADR-0090): %v", err)
	}
}

// The sundial example adopts the managed runner: it enables the singleton and its
// rendered `x` carries the awf-owned dispatch (including the `context` verb its
// hand-written runner was missing, ADR-0092) and its ported project verbs in the
// in-place section.
//
// invariant: runner-example-adopted
func TestExampleAdoptsRunner(t *testing.T) {
	cfg, err := os.ReadFile("../../examples/sundial/.awf/config.yaml")
	if err != nil {
		t.Fatalf("read example config: %v", err)
	}
	if !strings.Contains(string(cfg), "runner:") {
		t.Error("the sundial example must enable the runner singleton (ADR-0101)")
	}
	raw, err := os.ReadFile("../../examples/sundial/x")
	if err != nil {
		t.Fatalf("the sundial example must render x: %v", err)
	}
	x := string(raw)
	for _, want := range []string{
		"#!/usr/bin/env bash",
		`"$(bash .awf/bootstrap.sh)" "$cmd" "$@" ;;`, // awf-owned dispatch
		"context",      // the verb the hand-written runner was missing (ADR-0092)
		"go vet ./...", // sundial's ported gate verb, preserved in the in-place section
		"# awf:edit-in-place runner-project-verbs: ",
	} {
		if !strings.Contains(x, want) {
			t.Errorf("rendered examples/sundial/x missing %q", want)
		}
	}
}
