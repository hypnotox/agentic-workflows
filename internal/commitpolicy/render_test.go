package commitpolicy

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func renderOutcome(t *testing.T, policy Policy, outcome Outcome) string {
	t.Helper()
	document, err := Presentation(policy, outcome)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := presentation.Render(&output, document); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestRenderOutcomes(t *testing.T) {
	policy := Policy{AllowedIdentities: []Identity{{"Ada", "ada@example.test"}}, RequireSigned: true, AllowedSigners: []Signer{{"ada@example.test", "ssh-ed25519 key"}}}
	cases := []struct {
		name    string
		outcome Outcome
		want    []string
	}{
		{"disabled", Outcome{Disabled: true}, []string{"disabled"}},
		{"success", Outcome{}, []string{"all selected commits conform"}},
		{"refusal", Outcome{Refusal: &Refusal{Category: RevisionFailure, Observed: "missing", RefsChanged: false, IndexChanged: true, Actions: []string{"fix", "rerun"}, Cause: errors.New("x")}}, []string{"state: revision-resolution", "refs: false", "index: true", "step 1: fix", "step 2: rerun"}},
		{"violations", Outcome{Violations: []Violation{{Commit: "abc", Field: AuthorField, Observed: "Bad <bad>"}, {Commit: "abc", Field: SignatureField, Observed: "missing"}}}, []string{"commit policy violations", "refs changed: false", "index changed: false", "allowed identities: Ada <ada@example.test>", "allowed signers: ada@example.test ssh-ed25519 key", "identity remedy: correct the author or committer identity to one allowed identity", "signature remedy: configure commit.gpgSign, gpg.format, and user.signingKey for an allowed signer", "retry: reconcile the listed commits", "abc | author | Bad <bad>", "abc | signature | missing"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderOutcome(t, policy, tc.outcome)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("Render = %q, missing %q", got, want)
				}
			}
		})
	}
}

func TestPresentationRejectsInvalidPolicySemantics(t *testing.T) {
	for _, test := range []struct {
		name    string
		policy  Policy
		outcome Outcome
	}{
		{name: "refusal action", outcome: Outcome{Refusal: &Refusal{Observed: "refused", Actions: []string{" \n\t"}}}},
		{name: "violation commit", outcome: Outcome{Violations: []Violation{{Commit: "bad\ncommit", Field: AuthorField, Observed: "observed"}}}},
		{name: "violation field", outcome: Outcome{Violations: []Violation{{Commit: "abc", Field: Field("bad\nfield"), Observed: "observed"}}}},
		{name: "violation observed", outcome: Outcome{Violations: []Violation{{Commit: "abc", Field: AuthorField, Observed: " \n\t"}}}},
		{name: "allowed signer", policy: Policy{AllowedSigners: []Signer{{}}}, outcome: Outcome{Violations: []Violation{{Commit: "abc", Field: SignatureField, Observed: "observed"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Presentation(test.policy, test.outcome); err == nil {
				t.Fatal("invalid policy semantics produced a document")
			}
		})
	}
}

func TestPolicyFieldValidatesTextAndLabel(t *testing.T) {
	if _, err := policyField("status", " \n\t"); err == nil {
		t.Fatal("empty normalized text accepted")
	}
	if _, err := policyField("Bad", "value"); err == nil {
		t.Fatal("invalid label accepted")
	}
}
