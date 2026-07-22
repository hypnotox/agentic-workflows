package telemetry

import (
	"fmt"
	"io"
)

// RenderMetricsHuman renders only the stable MetricsResult model. It never
// reads storage or recomputes projection state.
func RenderMetricsHuman(out io.Writer, result MetricsResult) error {
	if _, err := fmt.Fprintf(out, "workflow metrics schema %d protocol %d generated %s\n", result.SchemaVersion, result.ProtocolMajor, result.GeneratedAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
		return err
	}
	for _, effort := range result.Efforts {
		if _, err := fmt.Fprintf(out, "effort %s state=%s route=%s trajectory=%s checkpoint=%s\n", effort.EffortID, effort.State, effort.Route, effort.ActiveTrajectoryID, effort.CheckpointID); err != nil {
			return err
		}
		for _, scope := range append([]ScopeProjection{effort.CurrentPath, effort.AllWork}, append(append([]ScopeProjection{}, effort.Sessions...), append(effort.Phases, effort.Trajectories...)...)...) {
			if _, err := fmt.Fprintf(out, "  scope %s input=%d output=%d cache-read=%d cache-write=%d cost=%g duration-ms=%d compactions=%d handoffs=%d tool-failures=%d gate-failures=%d subagents=%d rework=%d events=%d\n",
				scope.ScopeID, scope.Usage.InputTokens, scope.Usage.OutputTokens, scope.Usage.CacheReadTokens, scope.Usage.CacheWriteTokens, scope.Usage.CostUSD, scope.Usage.DurationMS,
				scope.Counters.Compactions, scope.Counters.Handoffs, scope.Counters.ToolFailures, scope.Counters.GateFailures, scope.Counters.SubagentInvocations, scope.Counters.ImplementationRework, len(scope.EventIDs)); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(out, "retention terminal=%d candidates=%d max-age-days=%d max-count=%d\n", result.Retention.TerminalEffortCount, len(result.Retention.Candidates), result.Retention.MaxAgeDays, result.Retention.MaxCount); err != nil {
		return err
	}
	for _, notice := range result.Integrity {
		if _, err := fmt.Fprintf(out, "integrity %s severity=%s scope=%s events=%d %s\n", notice.Code, notice.Severity, notice.Scope, len(notice.EventIDs), notice.Explanation); err != nil {
			return err
		}
	}
	return nil
}

// RenderDoctorHuman renders only the stable DoctorResult model. Findings are
// advisory output and do not imply a process exit status.
func RenderDoctorHuman(out io.Writer, result DoctorResult) error {
	if _, err := fmt.Fprintf(out, "workflow doctor schema %d protocol %d generated %s\n", result.SchemaVersion, result.ProtocolMajor, result.GeneratedAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
		return err
	}
	for _, finding := range result.Findings {
		if _, err := fmt.Fprintf(out, "finding %s type=%s severity=%s confidence=%s scope=%s waived=%t\n  evidence events=%v counters=%v",
			finding.Code, finding.Type, finding.Severity, finding.Confidence, finding.Scope, finding.Waived, finding.Evidence.EventIDs, finding.Evidence.CounterIDs); err != nil {
			return err
		}
		if finding.Evidence.ObservedValue != nil {
			if _, err := fmt.Fprintf(out, " observed=%g%s", *finding.Evidence.ObservedValue, finding.Evidence.Unit); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		if finding.Threshold != nil {
			if _, err := fmt.Fprintf(out, "  threshold kind=%s comparator=%s value=%g%s\n", finding.Threshold.Kind, finding.Threshold.Comparator, finding.Threshold.Value, finding.Threshold.Unit); err != nil {
				return err
			}
		}
		if finding.Baseline != nil {
			if _, err := fmt.Fprintf(out, "  baseline route=%s rule=%d samples=%d percentile=%d value=%g%s\n", finding.Baseline.Route, finding.Baseline.RuleVersion, finding.Baseline.SampleCount, finding.Baseline.Percentile, finding.Baseline.Value, finding.Baseline.Unit); err != nil {
				return err
			}
		}
		if finding.Reconciliation != nil {
			if _, err := fmt.Fprintf(out, "  reconciliation kind=%s sources=%v replacement=%s payload=%s\n", finding.Reconciliation.Kind, finding.Reconciliation.SourceEventIDs, finding.Reconciliation.Replacement.EventKind, finding.Reconciliation.Replacement.Payload); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "  %s\n  next: %s\n", finding.Explanation, finding.NextAction); err != nil {
			return err
		}
	}
	for _, notice := range result.Integrity {
		if _, err := fmt.Fprintf(out, "integrity %s severity=%s scope=%s events=%d %s\n", notice.Code, notice.Severity, notice.Scope, len(notice.EventIDs), notice.Explanation); err != nil {
			return err
		}
	}
	return nil
}
