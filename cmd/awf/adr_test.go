package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// pendingCLIRecord is a valid Proposed pending current-state-v3 record.
const pendingCLIRecord = "---\nformat: current-state-v3\nslug: pending-one\nstatus: Proposed\ndate: 2026-07-31\n---\n" +
	"# ADR-pending-one: A decision\n\n" +
	"## Context\n\nBackground prose.\n\n" +
	"## Decision\n\n1. The only decision.\n\n" +
	"## State changes\n\nNone.\n\n" +
	"## Consequences\n\nConsequence prose.\n\n" +
	"## Alternatives Considered\n\nNone considered.\n\n" +
	"## Status history\n\n- 2026-07-31: Proposed\n"

// TestRunADRNumbersAPendingRecord drives the handler end to end: it opens the
// project itself, numbers the corpus's one pending record, and prints the
// mapping the integration commit message quotes.
func TestRunADRNumbersAPendingRecord(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/pending-one.md"), pendingCLIRecord)

	var out bytes.Buffer
	if err := runADR(&cmdCtx{ctx: testContext(t), root: root, sub: "number", stdout: &out}); err != nil {
		t.Fatalf("runADR: %v", err)
	}
	if got := out.String(); got != "pending-one -> 0001\n" {
		t.Errorf("runADR printed %q", got)
	}
	numbered, err := os.ReadFile(filepath.Join(root, "docs/decisions/0001-pending-one.md"))
	if err != nil {
		t.Fatalf("numbered record missing: %v", err)
	}
	if !strings.Contains(string(numbered), "# ADR-0001: A decision") {
		t.Errorf("heading not renumbered:\n%s", numbered)
	}
}

// TestRunADRRefusals covers the handler's three remaining exits: an absent or
// unknown subcommand is CLI misuse, a tree that is no project fails at the open,
// and an engine refusal propagates unchanged.
func TestRunADRRefusals(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer

	err := runADR(&cmdCtx{ctx: testContext(t), root: root, sub: "", stdout: &out})
	if err == nil || err.Error() != "usage: awf adr number [<slug>...]" {
		t.Fatalf("bare group error = %v", err)
	}
	var usage *usageErr
	if !errors.As(err, &usage) {
		t.Errorf("a missing subcommand must be CLI misuse, got %T", err)
	}
	if err := runADR(&cmdCtx{ctx: testContext(t), root: t.TempDir(), sub: "number", stdout: &out}); err == nil {
		t.Error("a tree that is no project must fail at the open")
	}
	if err := runADR(&cmdCtx{ctx: testContext(t), root: root, sub: "number", stdout: &out}); err == nil || err.Error() != "no pending ADR to number" {
		t.Fatalf("engine refusal = %v", err)
	}
}
