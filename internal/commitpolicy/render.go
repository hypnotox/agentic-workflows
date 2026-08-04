package commitpolicy

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Render produces the single human presentation for policy outcomes.
func Render(policy Policy, outcome Outcome) string {
	if outcome.Refusal != nil {
		document, err := Presentation(policy, outcome)
		if err == nil {
			var rendered bytes.Buffer
			if presentation.Render(&rendered, document) == nil {
				return rendered.String()
			}
		}
	}
	var b strings.Builder
	if outcome.Disabled {
		return "note: commit policy is disabled; author commitPolicy to preview exact commit provenance\n"
	}
	if outcome.Refusal != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return "commit policy presentation failed\n"
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

// Presentation maps the policy-owned outcome into the representation-only
// diagnostic shape. Rendering remains exclusively presentation's concern.
func Presentation(policy Policy, outcome Outcome) (presentation.Document, error) {
	if outcome.Refusal != nil {
		r := outcome.Refusal
		refsValue, err := presentation.Literal(strconv.FormatBool(r.RefsChanged))
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
			return presentation.Document{}, err
		}
		refs, err := presentation.NewField("refs", refsValue)
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
			return presentation.Document{}, err
		}
		indexValue, err := presentation.Literal(strconv.FormatBool(r.IndexChanged))
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
			return presentation.Document{}, err
		}
		index, err := presentation.NewField("index", indexValue)
		if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
			return presentation.Document{}, err
		}
		steps := make([]presentation.Value, len(r.Actions))
		for i, action := range r.Actions {
			steps[i], err = presentation.Prose(action)
			if err != nil {
				return presentation.Document{}, err
			}
		}
		cause := ""
		if r.Cause != nil {
			cause = r.Cause.Error()
		}
		return (presentation.Diagnostic{Condition: r.Observed, State: string(r.Category), Changed: []presentation.Field{refs, index}, Cause: cause, Steps: steps}).Document()
	}
	value, err := presentation.Prose("all selected commits conform")
	if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return presentation.Document{}, err
	}
	field, err := presentation.NewField("commit policy", value)
	if err != nil { // coverage-ignore: validated inputs and fixed presentation grammar make this constructor path unreachable
		return presentation.Document{}, err
	}
	return presentation.NewDocument(field)
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
