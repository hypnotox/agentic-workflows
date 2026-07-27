package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestAggregateMetricsScopesOrderingAndPrivacy(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: "direct"})
	usagePayload, _ := json.Marshal(UsageObservedPayload{Model: "model", InputTokens: 10, OutputTokens: 5, CacheReadTokens: 3, CacheWriteTokens: 2, CostUSD: 0.25, DurationMS: 20, Phase: "implementation"})
	usage := EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: "usage", ObservationID: "same-observation", EffortID: "effort", SessionID: "z-session", Timestamp: "2026-07-22T00:00:02Z", Kind: "usage_observed", Predecessors: []string{"route"}, Payload: usagePayload}
	duplicate := usage
	duplicate.EventID = "usage-duplicate"
	events = append(events, usage, duplicate)
	subagentPayload, _ := json.Marshal(SubagentObservedPayload{Role: "reviewer", RequestedModel: "model", ResolvedModel: "model", ThinkingLevel: "medium", QueueDurationMS: 4, RunDurationMS: 30, InputTokens: 7, OutputTokens: 6, CacheReadTokens: 5, CacheWriteTokens: 4, CostUSD: 0.5, Outcome: "failure", StopReason: "error", ToolCount: 3, ToolFailureCount: 2})
	subagent := EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: "subagent", ObservationID: "subagent-observation", EffortID: "effort", SessionID: "a-session", Timestamp: "2026-07-22T00:00:03Z", Kind: "subagent_observed", Predecessors: []string{"usage"}, Payload: subagentPayload}
	events = append(events, subagent)
	compactionPayload, _ := json.Marshal(CompactionObservedPayload{Count: 2})
	events = append(events, EventEnvelope{Version: ProtocolVersion{Major: 2}, EventID: "compact", ObservationID: "compact-observation", EffortID: "effort", SessionID: "a-session", Timestamp: "2026-07-22T00:00:04Z", Kind: "compaction_observed", Predecessors: []string{"subagent"}, Payload: compactionPayload})

	read := EffortRead{
		Metadata:  EffortMetadata{EffortID: "effort", CreatedAt: "2026-07-22T00:00:00Z", CreationMode: "independent"},
		Events:    events,
		Integrity: []IntegrityIssue{{Code: "partial-final-line", Scope: "z-session", EventIDs: []string{"usage"}, Detail: "/secret/repository/path"}},
	}
	generated := time.Date(2026, 7, 22, 1, 0, 0, 0, time.UTC)
	result, err := AggregateMetrics([]EffortRead{read}, Selector{}, MetricsOptions{GeneratedAt: generated, Retention: RetentionPolicy{MaxCompletedEffortAgeDays: 90, MaxCompletedEffortCount: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Efforts) != 1 || result.Efforts[0].EffortID != "effort" {
		t.Fatalf("effort projection = %#v", result.Efforts)
	}
	all := result.Efforts[0].AllWork
	if all.Usage.InputTokens != 17 || all.Usage.OutputTokens != 11 || all.Usage.CacheReadTokens != 8 || all.Usage.CacheWriteTokens != 6 || all.Usage.CostUSD != 0.75 || all.Usage.DurationMS != 50 {
		t.Fatalf("deduplicated usage totals = %#v", all.Usage)
	}
	if all.Counters.SubagentInvocations != 1 || all.Counters.ToolFailures != 2 || all.Counters.Compactions != 2 {
		t.Fatalf("counters = %#v", all.Counters)
	}
	if len(result.Efforts[0].Sessions) != 3 || result.Efforts[0].Sessions[0].ScopeID != "a-session" || result.Efforts[0].Sessions[2].ScopeID != "z-session" {
		t.Fatalf("stable session scopes = %#v", result.Efforts[0].Sessions)
	}
	if len(result.Efforts[0].Phases) != 1 || result.Efforts[0].Phases[0].ScopeID != "implementation" {
		t.Fatalf("phase scopes = %#v", result.Efforts[0].Phases)
	}
	if len(result.Integrity) != 1 || result.Integrity[0].Severity != "warning" || strings.Contains(result.Integrity[0].Explanation, "secret") {
		t.Fatalf("bounded integrity projection = %#v", result.Integrity)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"/secret/repository/path", "prompt", "assistantText", "toolArguments", "commandOutput"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("metrics leaked forbidden value %q: %s", forbidden, raw)
		}
	}
	var human bytes.Buffer
	if err := RenderMetricsHuman(&human, result); err != nil {
		t.Fatal(err)
	}
	for _, semantic := range []string{fmt.Sprintf("effort %s state=active route=direct", "effort"), "input=17 output=11", "cost=0.75 duration-ms=50", "compactions=2", "phases total=1 shown=1", "phase implementation turns=1", "diagnostics warnings=1 violations=0"} {
		if !strings.Contains(human.String(), semantic) {
			t.Fatalf("human output lacks %q:\n%s", semantic, human.String())
		}
	}
}

func TestCurrentOpenPhase(t *testing.T) {
	for _, test := range []struct {
		name      string
		lifecycle LifecycleProjection
		want      Phase
	}{
		{"single active phase", LifecycleProjection{State: EffortActive, OpenPhases: map[string]PhaseInterval{"start": {Phase: "implementation"}}}, "implementation"},
		{"discovery", LifecycleProjection{State: EffortDiscovery, OpenPhases: map[string]PhaseInterval{"start": {Phase: "brainstorming"}}}, ""},
		{"no open phase", LifecycleProjection{State: EffortActive, OpenPhases: map[string]PhaseInterval{}}, ""},
		{"multiple open phases", LifecycleProjection{State: EffortActive, OpenPhases: map[string]PhaseInterval{"one": {Phase: "brainstorming"}, "two": {Phase: "implementation"}}}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := currentOpenPhase(test.lifecycle); got != test.want {
				t.Errorf("current open phase = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRenderMetricsHumanSelectedEffortIsConciseAcrossLifecycleStates(t *testing.T) {
	scope := func(id string) ScopeProjection {
		return ScopeProjection{ScopeID: id, Usage: UsageTotals{InputTokens: 1}, EventIDs: []string{"event"}}
	}
	result := MetricsResult{Efforts: []EffortProjection{
		{EffortID: "open", State: string(EffortActive), Route: "direct", openPhase: "implementation", CurrentPath: scope("current-path"), AllWork: scope("all-work"), Sessions: []ScopeProjection{scope("session")}},
		{EffortID: "terminal", State: string(EffortCompleted), Route: "direct", CurrentPath: scope("current-path"), AllWork: scope("all-work"), Phases: []ScopeProjection{scope("implementation")}},
		{EffortID: "discovery", State: string(EffortDiscovery), CurrentPath: scope("current-path"), AllWork: scope("all-work"), Trajectories: []ScopeProjection{scope("trajectory")}},
	}, Integrity: []IntegrityNotice{{Severity: "warning"}, {Severity: "violation"}}}
	var out bytes.Buffer
	if err := RenderMetricsHuman(&out, result); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"effort open state=active route=direct phase=implementation",
		"effort terminal state=completed route=direct outcome=completed",
		"effort discovery state=discovery route= discovery",
		"scope current-path input=1", "scope all-work input=1", "phases total=1 shown=1", "phase implementation turns=0", "diagnostics warnings=1 violations=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{"scope session", "scope trajectory", "retention ", "integrity ", "trajectory="} {
		if strings.Contains(got, forbidden) {
			t.Errorf("human output is not concise; contains %q:\n%s", forbidden, got)
		}
	}
}

func TestRenderMetricsHumanBoundsPhaseSummaries(t *testing.T) {
	phases := make([]ScopeProjection, maximumHumanPhaseSummaries+1)
	for index := range phases {
		phases[index] = ScopeProjection{ScopeID: fmt.Sprintf("phase-%02d", index)}
	}
	var out bytes.Buffer
	if err := RenderMetricsHuman(&out, MetricsResult{Efforts: []EffortProjection{{EffortID: "effort", CurrentPath: ScopeProjection{ScopeID: "current-path"}, AllWork: ScopeProjection{ScopeID: "all-work"}, Phases: phases}}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "phases total=11 shown=10") || strings.Contains(out.String(), "phase phase-10") {
		t.Fatalf("phase summary was not bounded deterministically: %s", out.String())
	}
}

func TestAggregateMetricsSelectorsTrajectoriesFamiliesAndRetention(t *testing.T) {
	parentEvents := lifecycleBaseEvents()
	parentEvents = appendEvent(parentEvents, "parent", "trajectory_started", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "anchor"})
	parentEvents[len(parentEvents)-1].TrajectoryID = "parent"
	parentEvents = append(parentEvents, passiveProjectionEvent("parent-work", "parent"))
	parentEvents = appendEvent(parentEvents, "fork", "trajectory_forked", TrajectoryForkedPayload{TrajectoryID: "discarded", ParentTrajectoryID: "parent", ForkAnchorID: "fork-anchor"})
	parentEvents[len(parentEvents)-1].TrajectoryID = "discarded"
	parentEvents = append(parentEvents, passiveProjectionEvent("discarded-work", "discarded"))
	parentEvents = appendEvent(parentEvents, "resume", "trajectory_resumed", TrajectoryPayload{TrajectoryID: "parent", AnchorID: "anchor"})
	parentEvents[len(parentEvents)-1].TrajectoryID = "parent"
	parentRead := EffortRead{Metadata: EffortMetadata{EffortID: "parent-effort", CreatedAt: "2026-07-01T00:00:00Z", CreationMode: "independent"}, Events: parentEvents}

	origin := &OriginMetadata{EffortID: "parent-effort", TrajectoryID: "parent", AnchorID: "anchor"}
	childRead := EffortRead{Metadata: EffortMetadata{EffortID: "child-effort", CreatedAt: "2026-07-02T00:00:00Z", CreationMode: "derived", Origin: origin}, Events: lifecycleBaseEvents()}
	terminalEvents := completedRoute("direct")
	for index := range terminalEvents {
		terminalEvents[index].EffortID = "terminal-effort"
		terminalEvents[index].Timestamp = "2026-01-02T00:00:00Z"
	}
	terminalRead := EffortRead{Metadata: EffortMetadata{EffortID: "terminal-effort", CreatedAt: "2026-01-01T00:00:00Z", CreationMode: "independent"}, Events: terminalEvents}

	generated := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	result, err := AggregateMetrics([]EffortRead{terminalRead, childRead, parentRead}, Selector{}, MetricsOptions{GeneratedAt: generated, Retention: RetentionPolicy{MaxCompletedEffortAgeDays: 30}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Efforts) != 3 || result.Efforts[0].EffortID != "child-effort" || result.Efforts[1].EffortID != "parent-effort" {
		t.Fatalf("effort ordering = %#v", result.Efforts)
	}
	parent := result.Efforts[1]
	if parent.CurrentPath.Usage.InputTokens != 1 || parent.AllWork.Usage.InputTokens != 2 || len(parent.Trajectories) != 2 || len(parent.DerivedEffortIDs) != 1 || parent.DerivedEffortIDs[0] != "child-effort" {
		t.Fatalf("trajectory/family accounting = %#v", parent)
	}
	if result.Retention.TerminalEffortCount != 1 || len(result.Retention.Candidates) != 1 || result.Retention.Candidates[0] != "terminal-effort" {
		t.Fatalf("retention state = %#v", result.Retention)
	}

	effortID := "child-effort"
	filtered, err := AggregateMetrics([]EffortRead{parentRead, childRead}, Selector{EffortID: &effortID}, MetricsOptions{GeneratedAt: generated})
	if err != nil || len(filtered.Efforts) != 1 || filtered.Efforts[0].Origin == nil || filtered.Efforts[0].AllWork.Usage.InputTokens != 0 {
		t.Fatalf("effort selector and no parent double count = %#v err=%v", filtered, err)
	}
}

// invariant: tooling/workflow-telemetry:event-protocol-and-ledger
func TestAggregateSuppressesIncompatibleEffortFromMetricsAndRetention(t *testing.T) {
	read := heuristicSignalRead("incompatible-effort")
	read.Integrity = []IntegrityIssue{{Code: "unsupported-protocol", Scope: "session", Line: 2}}
	read.Records = []LedgerRecord{{SessionID: "session", Line: 2, Raw: json.RawMessage(`{"version":{"major":2,"minor":2},"kind":"future_required_kind"}`)}}

	result, err := AggregateMetrics([]EffortRead{read}, Selector{}, MetricsOptions{
		GeneratedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Retention:   RetentionPolicy{MaxCompletedEffortAgeDays: 1, MaxCompletedEffortCount: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Efforts) != 0 {
		t.Fatalf("incompatible effort contributed metrics: %#v", result.Efforts)
	}
	if result.Retention.TerminalEffortCount != 0 || len(result.Retention.Candidates) != 0 {
		t.Fatalf("incompatible effort contributed retention state: %#v", result.Retention)
	}
	if len(result.Integrity) != 1 || result.Integrity[0].Code != "unsupported-protocol" {
		t.Fatalf("compatibility result is not one bounded notice: %#v", result.Integrity)
	}
}

func TestAggregateProjectsBoundedAdoptionAndDetourReturnState(t *testing.T) {
	adopted := protocol21Envelope(t, "adopt", "effort_adopted", nil, map[string]any{
		"creationMode": "adopted", "phase": "planning", "workflow": "writing-plans",
		"trajectoryId": "trajectory", "anchorId": "anchor", "associationOrigin": "manual",
	})
	adoptRead := EffortRead{Metadata: protocol21Metadata(t, map[string]any{"effortId": "effort-id", "createdAt": adopted.Timestamp, "creationMode": "adopted"}), Events: []EventEnvelope{adopted}}
	result, err := AggregateMetrics([]EffortRead{adoptRead}, Selector{}, MetricsOptions{})
	if err != nil || len(result.Efforts) != 1 {
		t.Fatalf("aggregate adoption = %#v, %v", result, err)
	}
	projected := result.Efforts[0]
	if projected.AdoptionBoundary == nil || projected.AdoptionBoundary.EventID != "adopt" || projected.CurrentWorkflow != "writing-plans" || projected.DetourReturn != nil {
		t.Fatalf("bounded canonical adoption fields = %#v", projected)
	}

	origin := map[string]any{"effortId": "parent", "trajectoryId": "parent-trajectory", "anchorId": "parent-anchor"}
	started := protocol21Envelope(t, "detour", "detour_started", nil, map[string]any{
		"creationMode": "derived", "origin": origin, "returnPhase": "implementation", "returnPhaseStartEventId": "parent-start",
		"trajectoryId": "child-trajectory", "anchorId": "child-anchor", "workflow": "brainstorming", "associationOrigin": "detour",
	})
	abandoned := protocol21Envelope(t, "abandon", "effort_abandoned", []string{"detour"}, map[string]any{"terminalEpoch": 1})
	detourRead := EffortRead{Metadata: protocol21Metadata(t, map[string]any{
		"effortId": "effort-id", "createdAt": started.Timestamp, "creationMode": "derived", "origin": origin,
		"detourReturn": map[string]any{"sessionId": "session-id", "phase": "implementation", "phaseStartEventId": "parent-start"},
	}), Events: []EventEnvelope{started, abandoned}}
	result, err = AggregateMetrics([]EffortRead{detourRead}, Selector{}, MetricsOptions{})
	if err != nil || len(result.Efforts) != 1 || result.Efforts[0].DetourReturn == nil || !result.Efforts[0].DetourReturn.Pending || result.Efforts[0].DetourReturn.Settled {
		t.Fatalf("bounded canonical pending return = %#v, %v", result, err)
	}
	returned := protocol21Envelope(t, "returned", "detour_returned", []string{"abandon"}, map[string]any{"terminalOutcome": "abandoned", "parentAssociationEventId": "parent-association"})
	detourRead.Events = append(detourRead.Events, returned)
	result, err = AggregateMetrics([]EffortRead{detourRead}, Selector{}, MetricsOptions{})
	if err != nil || result.Efforts[0].DetourReturn == nil || result.Efforts[0].DetourReturn.Pending || !result.Efforts[0].DetourReturn.Settled || result.Efforts[0].DetourReturn.ParentAssociationEventID != "parent-association" {
		t.Fatalf("bounded canonical settled return = %#v, %v", result, err)
	}
}

func TestStableMetricsJSONContractAndSaturatingTotals(t *testing.T) {
	generated := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	result := MetricsResult{SchemaVersion: 1, ProtocolMajor: 2, GeneratedAt: generated, Selector: Selector{}, Efforts: []EffortProjection{}, Retention: RetentionState{MaxAgeDays: 90, MaxCount: 100, Candidates: []string{}}, Integrity: []IntegrityNotice{}}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schemaVersion":1,"protocolMajor":2,"generatedAt":"2026-07-22T00:00:00Z","selector":{},"efforts":[],"retention":{"maxAgeDays":90,"maxCount":100,"terminalEffortCount":0,"candidates":[]},"integrity":[]}`
	if string(raw) != want {
		t.Fatalf("stable JSON = %s\nwant %s", raw, want)
	}
	if got := saturatingAdd(math.MaxUint64-1, 2); got != math.MaxUint64 {
		t.Fatalf("saturating total = %d", got)
	}
}
