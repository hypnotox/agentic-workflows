// Package repositorycheck owns policy-free ordered aggregation of repository check results.
package repositorycheck

import (
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// Slot supplies one completed owner result and whether its unranked information
// contributes to the legacy drift projection.
type Slot struct {
	Result                    checkresult.Result
	IncludeInformationInDrift bool
}

// Inputs names the established working-check destinations. Callers place each
// completed owner result in its semantic slot before aggregation.
type Inputs struct {
	Tracking            Slot
	ProducerResults     []Slot
	OrdinaryAdvisories  Slot
	TrackingInformation Slot
}

// Report preserves the existing repository-check projections.
type Report struct {
	Drift               []manifest.Drift
	Warnings            []string
	Information         []string
	TrackingInformation []string
	Result              checkresult.Result
	// DirectResult is the explicitly placed direct-drift projection. Aggregate
	// advisories remain in Result but never appear in a direct child.
	DirectResult checkresult.Result
	// Typed aggregate partitions retain owner classification for command consumers.
	aggregateAdvisoryResult   checkresult.Result
	trackingInformationResult checkresult.Result
	Notes                     []string
	TrackingNotes             []string
}

// Presentation is the explicit ranked projection of owner-classified results.
// Its fields preserve deterministic append order without recovering meaning from category labels.
type Presentation struct {
	Errors      []presentation.Record
	Warnings    []presentation.Record
	Information []presentation.Record
}

// Append preserves ordinary evidence multiplicity and deterministic category order.
func (p Presentation) Append(other Presentation) Presentation {
	p.Errors = append(p.Errors, other.Errors...)
	p.Warnings = append(p.Warnings, other.Warnings...)
	p.Information = append(p.Information, other.Information...)
	return p
}

// Present projects one owner-classified result using its explicitly supplied
// check label. Rank, not evidence spelling, selects the destination.
func Present(result checkresult.Result, check string) (Presentation, error) {
	return present(result, check, false)
}

// PresentEvidence is Present for a destination whose established output
// includes producer evidence before its detail.
func PresentEvidence(result checkresult.Result, check string) (Presentation, error) {
	return present(result, check, true)
}

func present(result checkresult.Result, check string, evidencePrefix bool) (Presentation, error) {
	var out Presentation
	record := func(detail string) (presentation.Record, error) {
		name, err := presentation.Prose(check)
		if err != nil {
			return presentation.Record{}, err
		}
		value, err := presentation.Prose(detail)
		if err != nil {
			return presentation.Record{}, err
		}
		return presentation.NewRecord(name, value)
	}
	for _, finding := range result.Findings() {
		detail := finding.Evidence.Detail
		if evidencePrefix {
			detail = fmt.Sprintf("%s: %s: %s", finding.Evidence.Kind, finding.Evidence.Path, detail)
		}
		item, err := record(detail)
		if err != nil {
			return Presentation{}, err
		}
		if finding.Rank == severity.Error {
			out.Errors = append(out.Errors, item)
		} else {
			out.Warnings = append(out.Warnings, item)
		}
	}
	for _, item := range result.Information() {
		detail := item.Evidence.Detail
		if evidencePrefix {
			detail = fmt.Sprintf("%s: %s: %s", item.Evidence.Kind, item.Evidence.Path, detail)
		}
		record, err := record(detail)
		if err != nil {
			return Presentation{}, err
		}
		out.Information = append(out.Information, record)
	}
	return out, nil
}

// HasErrors reports whether owner-classified results contain Error findings.
func HasErrors(result checkresult.Result) bool {
	for _, finding := range result.Findings() {
		if finding.Rank == severity.Error {
			return true
		}
	}
	return false
}

// Compose aggregates completed owner results in their explicit established
// order. It does not select result destinations from evidence spelling.
func Compose(input Inputs) (Report, error) {
	if err := requireAdvisories("ordinary advisories", input.OrdinaryAdvisories.Result); err != nil {
		return Report{}, err
	}
	if err := requireInformation("tracking information", input.TrackingInformation.Result); err != nil {
		return Report{}, err
	}
	var findings []checkresult.Finding
	var information []checkresult.Information
	var directFindings []checkresult.Finding
	var directInformation []checkresult.Information
	var drift []manifest.Drift
	appendSlot := func(slot Slot, includeDriftInformation bool) {
		for _, finding := range slot.Result.Findings() {
			findings = append(findings, finding)
			if finding.Rank == severity.Error {
				directFindings = append(directFindings, finding)
				drift = append(drift, manifest.Drift{Kind: finding.Evidence.Kind, Path: finding.Evidence.Path, Detail: finding.Evidence.Detail})
			}
		}
		for _, item := range slot.Result.Information() {
			information = append(information, item)
			if includeDriftInformation {
				directInformation = append(directInformation, item)
				drift = append(drift, manifest.Drift{Kind: item.Evidence.Kind, Path: item.Evidence.Path, Detail: item.Evidence.Detail})
			}
		}
	}
	appendSlot(input.Tracking, input.Tracking.IncludeInformationInDrift)
	for _, slot := range input.ProducerResults {
		appendSlot(slot, slot.IncludeInformationInDrift)
	}
	appendSlot(input.OrdinaryAdvisories, input.OrdinaryAdvisories.IncludeInformationInDrift)
	appendSlot(input.TrackingInformation, input.TrackingInformation.IncludeInformationInDrift)

	result, err := checkresult.New(findings, information)
	if err != nil {
		return Report{}, fmt.Errorf("finalize owner-classified check results: %w", err)
	}
	direct, err := checkresult.New(directFindings, directInformation)
	if err != nil {
		return Report{}, fmt.Errorf("finalize direct owner-classified check results: %w", err)
	}
	report := Report{
		Result:                    result,
		DirectResult:              direct,
		Drift:                     drift,
		aggregateAdvisoryResult:   input.OrdinaryAdvisories.Result,
		trackingInformationResult: input.TrackingInformation.Result,
	}
	report.Warnings = findingDetails(input.OrdinaryAdvisories.Result)
	report.Information = informationDetails(input.OrdinaryAdvisories.Result)
	report.TrackingInformation = append(informationDetails(input.Tracking.Result), informationDetails(input.TrackingInformation.Result)...)
	report.Notes = append(slices.Clone(report.Information), report.Warnings...)
	report.TrackingNotes = slices.Clone(report.TrackingInformation)
	return report, nil
}

// SplitWarnings separates a completed owner result for explicitly different
// aggregate destinations without deriving a rank from evidence.
func SplitWarnings(result checkresult.Result) (withoutWarnings, warnings checkresult.Result, err error) {
	var nonWarnings, warningFindings []checkresult.Finding
	for _, finding := range result.Findings() {
		if finding.Rank == severity.Warn {
			warningFindings = append(warningFindings, finding)
		} else {
			nonWarnings = append(nonWarnings, finding)
		}
	}
	withoutWarnings, err = checkresult.New(nonWarnings, result.Information())
	if err != nil {
		return checkresult.Result{}, checkresult.Result{}, err
	}
	warnings, err = checkresult.New(warningFindings, nil)
	return withoutWarnings, warnings, err
}

// AggregateAdvisoryResult returns the typed non-plan Warning and Information partition.
func (r Report) AggregateAdvisoryResult() checkresult.Result {
	return r.aggregateAdvisoryResult
}

// TrackingInformationResult returns the typed aggregate tracking Information partition.
func (r Report) TrackingInformationResult() checkresult.Result {
	return r.trackingInformationResult
}

func requireAdvisories(role string, result checkresult.Result) error {
	for _, finding := range result.Findings() {
		if finding.Rank != severity.Warn {
			return fmt.Errorf("%s includes non-warning finding", role)
		}
	}
	return nil
}

func requireWarnings(role string, result checkresult.Result) error {
	if len(result.Information()) > 0 {
		return fmt.Errorf("%s includes information", role)
	}
	return requireAdvisories(role, result)
}

func requireInformation(role string, result checkresult.Result) error {
	if len(result.Findings()) > 0 {
		return fmt.Errorf("%s includes ranked finding", role)
	}
	return nil
}

func findingDetails(result checkresult.Result) []string {
	var details []string
	for _, finding := range result.Findings() {
		details = append(details, finding.Evidence.Detail)
	}
	return details
}

func informationDetails(result checkresult.Result) []string {
	var details []string
	for _, information := range result.Information() {
		details = append(details, information.Evidence.Detail)
	}
	return details
}
