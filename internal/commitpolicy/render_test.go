package commitpolicy

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderOutcomes(t *testing.T) {
	policy := Policy{AllowedIdentities: []Identity{{"Ada", "ada@example.test"}}, RequireSigned: true, AllowedSigners: []Signer{{"ada@example.test", "ssh-ed25519 key"}}}
	cases := []struct {
		name    string
		outcome Outcome
		want    []string
	}{
		{"disabled", Outcome{Disabled: true}, []string{"disabled"}},
		{"success", Outcome{}, []string{"all selected commits conform"}},
		{"refusal", Outcome{Refusal: &Refusal{Category: RevisionFailure, Observed: "missing", RefsChanged: false, IndexChanged: true, Actions: []string{"fix", "rerun"}, Cause: errors.New("x")}}, []string{"refused (revision-resolution)", "refs changed: false", "index changed: true", "next: fix", "next: rerun"}},
		{"violations", Outcome{Violations: []Violation{{Commit: "abc", Field: AuthorField, Observed: "Bad <bad>"}, {Commit: "abc", Field: SignatureField, Observed: "missing"}}}, []string{"commit abc author", "commit abc signature", "allowed identities: Ada <ada@example.test>", "allowed signers: ada@example.test ssh-ed25519 key", "rerun"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(policy, tc.outcome)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("Render = %q, missing %q", got, want)
				}
			}
		})
	}
	identityOnly := Render(Policy{AllowedIdentities: policy.AllowedIdentities}, Outcome{Violations: []Violation{{Field: CommitterField, Observed: "bad"}}})
	if !strings.Contains(identityOnly, "correct the author or committer identity") {
		t.Fatal(identityOnly)
	}
}
