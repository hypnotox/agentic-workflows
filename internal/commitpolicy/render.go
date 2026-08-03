package commitpolicy

import (
	"fmt"
	"strings"
)

// Render produces the single human presentation for policy outcomes.
func Render(policy Policy, outcome Outcome) string {
	var b strings.Builder
	if outcome.Disabled {
		return "note: commit policy is disabled; author commitPolicy to preview exact commit provenance\n"
	}
	if outcome.Refusal != nil {
		r := outcome.Refusal
		fmt.Fprintf(&b, "commit policy refused (%s): %s\n", r.Category, r.Observed)
		if r.Cause != nil {
			fmt.Fprintf(&b, "cause: %v\n", r.Cause)
		}
		fmt.Fprintf(&b, "refs changed: %t\nindex changed: %t\n", r.RefsChanged, r.IndexChanged)
		for _, action := range r.Actions {
			fmt.Fprintf(&b, "next: %s\n", action)
		}
		return b.String()
	}
	if len(outcome.Violations) == 0 {
		return "commit policy: all selected commits conform\n"
	}
	identityViolation := false
	signatureViolation := false
	for _, violation := range outcome.Violations {
		fmt.Fprintf(&b, "commit %s %s: ", violation.Commit, violation.Field)
		if violation.Field == SignatureField {
			signatureViolation = true
			fmt.Fprintf(&b, "%s; commits must be signed by an allowed signer\n", violation.Observed)
		} else {
			identityViolation = true
			fmt.Fprintf(&b, "identity %s is not allowed\n", violation.Observed)
		}
	}
	if len(policy.AllowedIdentities) > 0 {
		fmt.Fprintf(&b, "allowed identities: %s\n", identities(policy.AllowedIdentities))
	}
	if len(policy.AllowedSigners) > 0 {
		fmt.Fprintf(&b, "allowed signers: %s\n", signers(policy.AllowedSigners))
	}
	if identityViolation {
		b.WriteString("correct the author or committer identity to one allowed identity\n")
	}
	if signatureViolation {
		b.WriteString("configure commit.gpgSign, gpg.format, and user.signingKey for an allowed signer\n")
	}
	b.WriteString("refs changed: false\nindex changed: false\n")
	b.WriteString("reconcile the listed commits, then rerun awf check commit-policy with the same explicit targets\n")
	return b.String()
}

func identities(values []Identity) string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = value.Name + " <" + value.Email + ">"
	}
	return strings.Join(out, ", ")
}

func signers(values []Signer) string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = value.Principal + " " + value.Key
	}
	return strings.Join(out, ", ")
}
