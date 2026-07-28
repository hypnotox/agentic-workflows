package telemetry

import (
	"fmt"
	"io"
)

func RenderHuman(out io.Writer, r Report) error {
	for _, e := range r.Efforts {
		if _, err := fmt.Fprintf(out, "effort %s title=%q state=%s\n  current input=%d output=%d cost=%g tool-failures=%d gates-failed=%d subagents=%d handoffs=%d\n  legacy input=%d output=%d cost=%g\n", e.ID, e.Title, e.State, e.Current.InputTokens, e.Current.OutputTokens, e.Current.CostUSD, e.Current.ToolFailures, e.Current.GatesFailed, e.Current.Subagents, e.Current.Handoffs, e.Legacy.InputTokens, e.Legacy.OutputTokens, e.Legacy.CostUSD); err != nil {
			return err
		}
	}
	for _, s := range r.Sessions {
		effort := "unassigned"
		if s.EffortID != nil {
			effort = *s.EffortID
		}
		if _, err := fmt.Fprintf(out, "session %s effort=%s input=%d output=%d cost=%g\n", s.SessionID, effort, s.Counters.InputTokens, s.Counters.OutputTokens, s.Counters.CostUSD); err != nil {
			return err
		}
	}
	return nil
}
func RenderDoctorHuman(out io.Writer, r DoctorReport) error {
	for _, f := range r.Findings {
		if _, err := fmt.Fprintf(out, "%s session=%s code=%s\n", f.Source, f.SessionID, f.Code); err != nil {
			return err
		}
	}
	return nil
}
