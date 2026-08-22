package project

import (
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// checkBatch is the private transitional result produced by the ordinary
// working check. Producers add owner-classified values; CheckReport only
// projects the completed batch for compatibility.
type checkBatch struct {
	findings    []checkresult.Finding
	information []checkresult.Information
	// projection retains producer order across ranked and unranked semantic
	// results. It contains Evidence, never a reverse manifest representation.
	projection []checkresult.Evidence
}

func (b *checkBatch) errorDrift(property checkresult.Property, drift []manifest.Drift) {
	for _, item := range drift {
		b.error(property, item.Kind, item.Path, item.Detail)
	}
}

func (b *checkBatch) error(property checkresult.Property, kind, path, detail string) {
	b.findings = append(b.findings, checkresult.Finding{Rank: severity.Error, Property: property, Evidence: checkresult.Evidence{Kind: kind, Path: path, Detail: detail}})
	b.projection = append(b.projection, checkresult.Evidence{Kind: kind, Path: path, Detail: detail})
}

func (b *checkBatch) warning(property checkresult.Property, kind, detail string) {
	b.findings = append(b.findings, checkresult.Finding{Rank: severity.Warn, Property: property, Evidence: checkresult.Evidence{Kind: kind, Detail: detail}})
}

func (b *checkBatch) informationItem(kind, path, detail string) {
	b.information = append(b.information, checkresult.Information{Evidence: checkresult.Evidence{Kind: kind, Path: path, Detail: detail}})
	if kind != "advisory" && kind != "tracking" {
		b.projection = append(b.projection, checkresult.Evidence{Kind: kind, Path: path, Detail: detail})
	}
}

func (b *checkBatch) informationDrift(drift []manifest.Drift) {
	for _, item := range drift {
		b.informationItem(item.Kind, item.Path, item.Detail)
	}
}

func (b *checkBatch) result() (checkresult.Result, error) {
	return checkresult.New(b.findings, b.information)
}

func (b *checkBatch) append(other checkBatch) {
	b.findings = append(b.findings, other.findings...)
	b.information = append(b.information, other.information...)
	b.projection = append(b.projection, other.projection...)
}

func (b checkBatch) withoutWarnings() checkBatch {
	out := checkBatch{information: slices.Clone(b.information), projection: slices.Clone(b.projection)}
	for _, finding := range b.findings {
		if finding.Rank != severity.Warn {
			out.findings = append(out.findings, finding)
		}
	}
	return out
}

func (b checkBatch) warningsOnly() checkBatch {
	out := checkBatch{}
	for _, finding := range b.findings {
		if finding.Rank == severity.Warn {
			out.findings = append(out.findings, finding)
		}
	}
	return out
}

func driftProjection(entries []checkresult.Evidence) []manifest.Drift {
	out := make([]manifest.Drift, 0, len(entries))
	for _, entry := range entries {
		out = append(out, manifest.Drift{Kind: entry.Kind, Path: entry.Path, Detail: entry.Detail})
	}
	return out
}

func reportFromBatch(batch checkBatch) (CheckReport, error) {
	result, err := batch.result()
	if err != nil {
		return CheckReport{}, fmt.Errorf("finalize owner-classified check results: %w", err)
	}
	report := CheckReport{Result: result, Drift: driftProjection(batch.projection), classified: true}
	for _, finding := range result.Findings() {
		switch finding.Rank {
		case severity.Error:
			// Drift is projected from the producer-ordered batch above.
		case severity.Warn:
			switch finding.Evidence.Kind {
			case "advisory":
				report.Warnings = append(report.Warnings, finding.Evidence.Detail)
			case "plan-advisory":
				report.PlanWarnings = append(report.PlanWarnings, finding.Evidence.Detail)
			}
		}
	}
	for _, item := range result.Information() {
		switch item.Evidence.Kind {
		case "tracking":
			report.TrackingInformation = append(report.TrackingInformation, item.Evidence.Detail)
		case "advisory":
			report.Information = append(report.Information, item.Evidence.Detail)
		}
	}
	report.Notes = append(slices.Clone(report.Information), report.Warnings...)
	report.TrackingNotes = slices.Clone(report.TrackingInformation)
	report.PlanNotes = slices.Clone(report.PlanWarnings)
	return report, nil
}
