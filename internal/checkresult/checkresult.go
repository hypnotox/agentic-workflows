// Package checkresult owns immutable owner-classified repository check results.
package checkresult

import (
	"errors"
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// Property identifies the validity, safety, authority, reproducibility, or
// other protected property that justifies a ranked finding.
type Property string

// Evidence is the producer-supplied semantic source of a result. Kind and
// Detail are required; Path is empty when the source has no path.
type Evidence struct {
	Kind   string
	Path   string
	Detail string
}

// Finding is a ranked result whose owner explicitly supplies its fixed rank,
// protected property, and presentation evidence.
type Finding struct {
	Rank     severity.Rank
	Property Property
	Evidence Evidence
}

// Information is an unranked result with producer-supplied presentation
// evidence. It cannot enter the ranked Finding model.
type Information struct {
	Evidence Evidence
}

// Result is an immutable collection of one check owner's ranked findings and
// unranked information.
type Result struct {
	findings    []Finding
	information []Information
}

// New constructs a Result after checking the owner-classified values at their
// boundary. It snapshots both input slices.
func New(findings []Finding, information []Information) (Result, error) {
	for i, finding := range findings {
		if err := validateFinding(finding); err != nil {
			return Result{}, fmt.Errorf("finding %d: %w", i, err)
		}
	}
	for i, item := range information {
		if err := validateEvidence(item.Evidence); err != nil {
			return Result{}, fmt.Errorf("information %d: %w", i, err)
		}
	}
	return Result{
		findings:    slices.Clone(findings),
		information: slices.Clone(information),
	}, nil
}

// Findings returns a defensive projection of this result's ranked findings.
func (r Result) Findings() []Finding {
	return slices.Clone(r.findings)
}

// Information returns a defensive projection of this result's unranked
// information.
func (r Result) Information() []Information {
	return slices.Clone(r.information)
}

func validateFinding(f Finding) error {
	if f.Rank != severity.Error && f.Rank != severity.Warn {
		return fmt.Errorf("rank %d is not Error or Warn", f.Rank)
	}
	if f.Property == "" {
		return errors.New("protected property is empty")
	}
	return validateEvidence(f.Evidence)
}

func validateEvidence(e Evidence) error {
	if e.Kind == "" {
		return errors.New("kind is empty")
	}
	if e.Detail == "" {
		return errors.New("detail is empty")
	}
	return nil
}
