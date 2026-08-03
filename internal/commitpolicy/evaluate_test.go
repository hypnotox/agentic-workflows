package commitpolicy

import (
	"strings"
	"testing"
)

func TestEvaluateExactCommitPolicy(t *testing.T) {
	policy := Policy{AllowedIdentities: []Identity{{Name: "Ada", Email: "ada@example.test"}}, RequireSigned: true}
	commits := []Commit{
		{ID: "b", Author: Person{"Bad", "bad@example.test"}, Committer: Person{"Ada", "ada@example.test"}, Signature: SignatureMissing},
		{ID: "a", Author: Person{"Ada", "ada@example.test"}, Committer: Person{"Bad", "bad@example.test"}, Signature: SignatureWrongKey},
		{ID: "b", Author: Person{"Bad", "bad@example.test"}, Committer: Person{"Bad", "bad@example.test"}, Signature: SignatureMalformed},
	}
	out := Evaluate(policy, commits)
	if got, want := len(out.Violations), 4; got != want {
		t.Fatalf("violations = %d, want %d: %#v", got, want, out.Violations)
	}
	got := []string{out.Violations[0].Commit + ":" + string(out.Violations[0].Field), out.Violations[1].Commit + ":" + string(out.Violations[1].Field), out.Violations[2].Commit + ":" + string(out.Violations[2].Field), out.Violations[3].Commit + ":" + string(out.Violations[3].Field)}
	want := []string{"a:committer", "a:signature", "b:author", "b:signature"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("stable violations = %v, want %v", got, want)
	}
	for _, verdict := range []SignatureVerdict{SignatureMissing, SignatureMalformed, SignatureWrongKey} {
		if got := signatureName(verdict); got == "invalid" {
			t.Errorf("verdict %v was not rendered specifically", verdict)
		}
	}
	if signatureName(SignatureVerdict(99)) != "invalid" {
		t.Fatal("unknown signature verdict")
	}
	if got := Evaluate(Policy{}, []Commit{{ID: "ok", Author: Person{"x", "y"}, Committer: Person{"z", "q"}, Signature: SignatureMalformed}}); !got.OK() {
		t.Fatalf("disabled identity/signature requirements = %#v", got)
	}
}
