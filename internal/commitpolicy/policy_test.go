package commitpolicy

import "testing"

func TestPolicyValuesPreserveExactBytes(t *testing.T) {
	policy := Policy{
		AllowedIdentities: []Identity{{Name: " Ada ", Email: "ADA@example.test"}},
		RequireSigned:     true,
		AllowedSigners:    []Signer{{Principal: "ada@example.test", Key: "ssh-ed25519 AAAA"}},
	}
	commit := Commit{ID: "a", Author: Person{Name: "Ada", Email: "ADA@example.test"}, Committer: Person{Name: " Ada ", Email: "ada@example.test"}, Signature: SignatureValid}
	outcome := Evaluate(policy, []Commit{commit})
	if len(outcome.Violations) != 2 || outcome.Violations[0].Field != AuthorField || outcome.Violations[1].Field != CommitterField {
		t.Fatalf("non-exact identities were accepted: %#v", outcome)
	}
	if !policy.RequireSigned || policy.AllowedSigners[0].Principal != "ada@example.test" {
		t.Fatalf("policy values changed: %#v", policy)
	}
}
