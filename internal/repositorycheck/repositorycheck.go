// Package repositorycheck owns policy-free ordered aggregation of repository check results.
package repositorycheck

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// Slot supplies one completed owner result and whether its unranked information
// contributes to the direct result consumed by repository checks.
type Slot struct {
	Result                     checkresult.Result
	IncludeInformationInDirect bool
}

// Inputs names the established working-check destinations. Callers place each
// completed owner result in its semantic slot before aggregation.
type Inputs struct {
	Tracking            Slot
	ProducerResults     []Slot
	OrdinaryAdvisories  Slot
	TrackingInformation Slot
}

// Report carries the typed aggregate and its explicitly placed direct result.
type Report struct {
	Result checkresult.Result
	// DirectResult excludes aggregate advisories so a direct drift check reports
	// only producer-owned failures and explicitly included information.
	DirectResult checkresult.Result

	aggregateAdvisoryResult   checkresult.Result
	trackingInformationResult checkresult.Result
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
	appendSlot := func(slot Slot, includeInformationInDirect bool) {
		for _, finding := range slot.Result.Findings() {
			findings = append(findings, finding)
			if finding.Rank == severity.Error {
				directFindings = append(directFindings, finding)
			}
		}
		for _, item := range slot.Result.Information() {
			information = append(information, item)
			if includeInformationInDirect {
				directInformation = append(directInformation, item)
			}
		}
	}
	appendSlot(input.Tracking, input.Tracking.IncludeInformationInDirect)
	for _, slot := range input.ProducerResults {
		appendSlot(slot, slot.IncludeInformationInDirect)
	}
	appendSlot(input.OrdinaryAdvisories, input.OrdinaryAdvisories.IncludeInformationInDirect)
	appendSlot(input.TrackingInformation, input.TrackingInformation.IncludeInformationInDirect)

	result, err := checkresult.New(findings, information)
	if err != nil {
		return Report{}, fmt.Errorf("finalize owner-classified check results: %w", err)
	}
	direct, err := checkresult.New(directFindings, directInformation)
	if err != nil {
		return Report{}, fmt.Errorf("finalize direct owner-classified check results: %w", err)
	}
	return Report{
		Result:                    result,
		DirectResult:              direct,
		aggregateAdvisoryResult:   input.OrdinaryAdvisories.Result,
		trackingInformationResult: input.TrackingInformation.Result,
	}, nil
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

func requireInformation(role string, result checkresult.Result) error {
	if len(result.Findings()) > 0 {
		return fmt.Errorf("%s includes ranked finding", role)
	}
	return nil
}
