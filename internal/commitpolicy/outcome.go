package commitpolicy

import "fmt"

// Field identifies the exact commit fact that violated policy.
type Field string

const (
	AuthorField    Field = "author"
	CommitterField Field = "committer"
	SignatureField Field = "signature"
)

// Violation is one stable, actionable policy mismatch.
type Violation struct {
	Commit   string
	Field    Field
	Observed string
}

// Category classifies an operational refusal without reducing it to a violation.
type Category string

const (
	ConfigFailure           Category = "config"
	BaselineFailure         Category = "baseline"
	RevisionFailure         Category = "revision-resolution"
	TagPeelFailure          Category = "tag-peel"
	LinkedWorktreeFailure   Category = "linked-worktree"
	TrustFileFailure        Category = "temporary-trust-file"
	SignatureProcessFailure Category = "signature-process"
)

// Refusal reports an operational failure, its preserved cause, and reconciliation facts.
type Refusal struct {
	Category                  Category
	Observed                  string
	RefsChanged, IndexChanged bool
	Actions                   []string
	Cause                     error
}

func (r *Refusal) Error() string {
	if r == nil {
		return "commit policy refusal"
	}
	if r.Cause == nil {
		return fmt.Sprintf("%s: %s", r.Category, r.Observed)
	}
	return fmt.Sprintf("%s: %s: %v", r.Category, r.Observed, r.Cause)
}
func (r *Refusal) Unwrap() error {
	if r == nil {
		return nil
	}
	return r.Cause
}

// Outcome is the complete result of one policy operation.
type Outcome struct {
	Disabled   bool
	Violations []Violation
	Refusal    *Refusal
}

// OK reports whether the operation had neither violations nor an operational refusal.
func (o Outcome) OK() bool { return o.Refusal == nil && len(o.Violations) == 0 }
