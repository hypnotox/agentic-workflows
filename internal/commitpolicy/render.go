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
		fmt.Fprintf(&b, "commit policy refused (%s): %s\nrefs changed: %t\nindex changed: %t\n", r.Category, r.Observed, r.RefsChanged, r.IndexChanged)
		for _, a := range r.Actions {
			fmt.Fprintf(&b, "next: %s\n", a)
		}
		return b.String()
	}
	if len(outcome.Violations) == 0 {
		return "commit policy: all selected commits conform\n"
	}
	for _, v := range outcome.Violations {
		fmt.Fprintf(&b, "commit %s %s: ", v.Commit, v.Field)
		if v.Field == SignatureField {
			fmt.Fprintf(&b, "%s; commits must be signed by an allowed signer\n", v.Observed)
		} else {
			fmt.Fprintf(&b, "identity %s is not allowed\n", v.Observed)
		}
	}
	if len(policy.AllowedIdentities) > 0 {
		fmt.Fprintf(&b, "allowed identities: %s\n", identities(policy.AllowedIdentities))
	}
	if policy.RequireSigned {
		fmt.Fprintf(&b, "allowed signers: %s\n", signers(policy.AllowedSigners))
		b.WriteString("configure commit.gpgSign, gpg.format, and user.signingKey, then rerun\n")
	} else {
		b.WriteString("correct Git identity and rerun the refused operation\n")
	}
	return b.String()
}
func identities(values []Identity) string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.Name + " <" + v.Email + ">"
	}
	return strings.Join(out, ", ")
}
func signers(values []Signer) string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.Principal + " " + v.Key
	}
	return strings.Join(out, ", ")
}
