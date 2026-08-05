package commitpolicy

import (
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// Presentation maps the policy-owned outcome into representation-only shapes.
// Rendering remains exclusively presentation's concern.
func Presentation(policy Policy, outcome Outcome) (presentation.Document, error) {
	if outcome.Disabled {
		value, err := presentation.Prose("commit policy is disabled; author commitPolicy to preview exact commit provenance")
		if err != nil { // coverage-ignore: fixed nonempty prose contains no forbidden line break
			return presentation.Document{}, err
		}
		field, err := presentation.NewField("status", value)
		if err != nil { // coverage-ignore: Prose validated the value and status is a fixed grammar-valid label
			return presentation.Document{}, err
		}
		return presentation.NewDocument(field)
	}
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
	if len(outcome.Violations) == 0 {
		value, err := presentation.Prose("all selected commits conform")
		if err != nil { // coverage-ignore: fixed nonempty prose contains no forbidden line break
			return presentation.Document{}, err
		}
		field, err := presentation.NewField("status", value)
		if err != nil { // coverage-ignore: Prose validated the value and status is a fixed grammar-valid label
			return presentation.Document{}, err
		}
		return presentation.NewDocument(field)
	}
	records := make([]presentation.Record, 0, len(outcome.Violations))
	identityViolation := false
	signatureViolation := false
	for _, violation := range outcome.Violations {
		commit, err := presentation.Literal(violation.Commit)
		if err != nil {
			return presentation.Document{}, err
		}
		field, err := presentation.Literal(string(violation.Field))
		if err != nil {
			return presentation.Document{}, err
		}
		observed, err := presentation.Prose(violation.Observed)
		if err != nil {
			return presentation.Document{}, err
		}
		record, err := presentation.NewRecord(commit, field, observed)
		if err != nil { // coverage-ignore: three validated nonempty values always form a valid record
			return presentation.Document{}, err
		}
		records = append(records, record)
		if violation.Field == SignatureField {
			signatureViolation = true
		} else {
			identityViolation = true
		}
	}
	refs, err := policyField("refs changed", "false")
	if err != nil { // coverage-ignore: fixed label and value are grammar-valid
		return presentation.Document{}, err
	}
	index, err := policyField("index changed", "false")
	if err != nil { // coverage-ignore: fixed label and value are grammar-valid
		return presentation.Document{}, err
	}
	summary := make([]presentation.Field, 0, 5)
	if len(policy.AllowedIdentities) > 0 {
		values := make([]string, len(policy.AllowedIdentities))
		for i, identity := range policy.AllowedIdentities {
			values[i] = identity.Name + " <" + identity.Email + ">"
		}
		allowed, fieldErr := policyField("allowed identities", strings.Join(values, ", "))
		if fieldErr != nil { // coverage-ignore: each identity spelling contains literal angle brackets and the label is fixed
			return presentation.Document{}, fieldErr
		}
		summary = append(summary, allowed)
	}
	if len(policy.AllowedSigners) > 0 {
		values := make([]string, len(policy.AllowedSigners))
		for i, signer := range policy.AllowedSigners {
			values[i] = signer.Principal + " " + signer.Key
		}
		allowed, fieldErr := policyField("allowed signers", strings.Join(values, ", "))
		if fieldErr != nil {
			return presentation.Document{}, fieldErr
		}
		summary = append(summary, allowed)
	}
	if identityViolation {
		remedy, fieldErr := policyField("identity remedy", "correct the author or committer identity to one allowed identity")
		if fieldErr != nil { // coverage-ignore: fixed label and value are grammar-valid
			return presentation.Document{}, fieldErr
		}
		summary = append(summary, remedy)
	}
	if signatureViolation {
		remedy, fieldErr := policyField("signature remedy", "configure commit.gpgSign, gpg.format, and user.signingKey for an allowed signer")
		if fieldErr != nil { // coverage-ignore: fixed label and value are grammar-valid
			return presentation.Document{}, fieldErr
		}
		summary = append(summary, remedy)
	}
	retry, err := policyField("retry", "reconcile the listed commits, then rerun awf check commit-policy with the same explicit targets")
	if err != nil { // coverage-ignore: fixed label and value are grammar-valid
		return presentation.Document{}, err
	}
	summary = append(summary, retry)
	return (presentation.Report{Status: "commit policy violations", Context: []presentation.Field{refs, index}, Summary: summary, Categories: []presentation.ReportCategory{{Label: "errors", Schema: []string{"commit", "field", "observed"}, Records: records}}}).Document()
}

func policyField(label, text string) (presentation.Field, error) {
	value, err := presentation.Prose(text)
	if err != nil {
		return presentation.Field{}, err
	}
	return presentation.NewField(label, value)
}
