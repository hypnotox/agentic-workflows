package telemetry

import (
	"fmt"
	"io"
	"sort"
)

const maximumHumanPhaseSummaries = 10

// RenderMetricsHuman renders the selected effort as a concise footer-like
// summary. It never reads storage or recomputes projection state.
func RenderMetricsHuman(out io.Writer, result MetricsResult) error {
	for _, effort := range result.Efforts {
		lifecycle := "discovery"
		if effort.State == string(EffortActive) {
			lifecycle = "phase=" + string(effort.openPhase)
		} else if effort.State == string(EffortCompleted) || effort.State == string(EffortAbandoned) {
			lifecycle = "outcome=" + effort.State
		}
		if _, err := fmt.Fprintf(out, "effort %s state=%s route=%s %s\n", effort.EffortID, effort.State, effort.Route, lifecycle); err != nil {
			return err
		}
		for _, scope := range []ScopeProjection{effort.CurrentPath, effort.AllWork} {
			if err := renderScopeHuman(out, "scope", scope); err != nil {
				return err
			}
		}
		phases := append([]ScopeProjection(nil), effort.Phases...)
		sort.Slice(phases, func(i, j int) bool { return phases[i].ScopeID < phases[j].ScopeID })
		shown := len(phases)
		if shown > maximumHumanPhaseSummaries {
			shown = maximumHumanPhaseSummaries
		}
		if _, err := fmt.Fprintf(out, "phases total=%d shown=%d\n", len(phases), shown); err != nil {
			return err
		}
		for _, phase := range phases[:shown] {
			if _, err := fmt.Fprintf(out, "  phase %s turns=%d input=%d output=%d cache-read=%d cache-write=%d cost=%g\n", phase.ScopeID, phase.turns, phase.Usage.InputTokens, phase.Usage.OutputTokens, phase.Usage.CacheReadTokens, phase.Usage.CacheWriteTokens, phase.Usage.CostUSD); err != nil {
				return err
			}
		}
	}
	warnings, violations := countIntegritySeverities(result.Integrity)
	_, err := fmt.Fprintf(out, "diagnostics warnings=%d violations=%d\n", warnings, violations)
	return err
}

func renderScopeHuman(out io.Writer, label string, scope ScopeProjection) error {
	_, err := fmt.Fprintf(out, "  %s %s input=%d output=%d cache-read=%d cache-write=%d cost=%g duration-ms=%d compactions=%d handoffs=%d tool-failures=%d gate-failures=%d subagents=%d rework=%d events=%d\n",
		label, scope.ScopeID, scope.Usage.InputTokens, scope.Usage.OutputTokens, scope.Usage.CacheReadTokens, scope.Usage.CacheWriteTokens, scope.Usage.CostUSD, scope.Usage.DurationMS,
		scope.Counters.Compactions, scope.Counters.Handoffs, scope.Counters.ToolFailures, scope.Counters.GateFailures, scope.Counters.SubagentInvocations, scope.Counters.ImplementationRework, len(scope.EventIDs))
	return err
}

func countIntegritySeverities(notices []IntegrityNotice) (warnings, violations int) {
	for _, notice := range notices {
		if notice.Severity == "warning" {
			warnings++
		} else {
			violations++
		}
	}
	return warnings, violations
}

// RenderEffortListHuman renders the bounded resident discovery page.
func RenderEffortListHuman(out io.Writer, page EffortListPage) error {
	for _, effort := range page.Efforts {
		if effort.Incompatible {
			if _, err := fmt.Fprintf(out, "effort %s created=%s incompatible\n", effort.EffortID, effort.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
				return err
			}
			continue
		}
		lastApplied := ""
		if effort.LastAppliedAt != nil {
			lastApplied = effort.LastAppliedAt.Format("2006-01-02T15:04:05.999999999Z07:00")
		}
		if _, err := fmt.Fprintf(out, "effort %s created=%s applied=%s state=%s route=%s phase=%s outcome=%s discovery=%t\n", effort.EffortID, effort.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), lastApplied, effort.State, effort.Route, effort.Phase, effort.Outcome, effort.Discovery); err != nil {
			return err
		}
	}
	if page.NextCursor != "" {
		_, err := fmt.Fprintf(out, "next-cursor %s\n", page.NextCursor)
		return err
	}
	return nil
}

// RenderDoctorHuman reports only deterministic finding and integrity counters.
// Detailed findings and their evidence remain available in canonical JSON.
func RenderDoctorHuman(out io.Writer, result DoctorResult) error {
	if result.Selector.EffortID != nil {
		if _, err := fmt.Fprintf(out, "doctor effort=%s\n", *result.Selector.EffortID); err != nil {
			return err
		}
	}
	severity := map[string]int{}
	rules := map[string]int{}
	for _, finding := range result.Findings {
		severity[finding.Severity]++
		rules[finding.Code]++
	}
	if _, err := fmt.Fprintf(out, "findings warnings=%d violations=%d\n", severity["warning"], severity["violation"]); err != nil {
		return err
	}
	if err := renderCountSummary(out, "rules", "rule", rules); err != nil {
		return err
	}
	integritySeverity := map[string]int{}
	integrityRules := map[string]int{}
	for _, notice := range result.Integrity {
		integritySeverity[notice.Severity]++
		integrityRules[notice.Code]++
	}
	if _, err := fmt.Fprintf(out, "integrity warnings=%d violations=%d\n", integritySeverity["warning"], integritySeverity["violation"]); err != nil {
		return err
	}
	return renderCountSummary(out, "integrity-rules", "rule", integrityRules)
}

func renderCountSummary(out io.Writer, label, key string, counts map[string]int) error {
	keys := make([]string, 0, len(counts))
	for value := range counts {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		_, err := fmt.Fprintf(out, "%s none=0\n", label)
		return err
	}
	for _, value := range keys {
		if _, err := fmt.Fprintf(out, "%s %s=%s count=%d\n", label, key, value, counts[value]); err != nil {
			return err
		}
	}
	return nil
}
