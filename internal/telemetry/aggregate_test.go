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
	usage := EventEnvelope{Version: ProtocolVersion{Major: 1}, EventID: "usage", ObservationID: "same-observation", EffortID: "effort", SessionID: "z-session", Timestamp: "2026-07-22T00:00:02Z", Kind: "usage_observed", Predecessors: []string{"route"}, Payload: usagePayload}
	duplicate := usage
	duplicate.EventID = "usage-duplicate"
	events = append(events, usage, duplicate)
	subagentPayload, _ := json.Marshal(SubagentObservedPayload{Role: "reviewer", RequestedModel: "model", ResolvedModel: "model", ThinkingLevel: "medium", QueueDurationMS: 4, RunDurationMS: 30, InputTokens: 7, OutputTokens: 6, CacheReadTokens: 5, CacheWriteTokens: 4, CostUSD: 0.5, Outcome: "failure", StopReason: "error", ToolCount: 3, ToolFailureCount: 2})
	subagent := EventEnvelope{Version: ProtocolVersion{Major: 1}, EventID: "subagent", ObservationID: "subagent-observation", EffortID: "effort", SessionID: "a-session", Timestamp: "2026-07-22T00:00:03Z", Kind: "subagent_observed", Predecessors: []string{"usage"}, Payload: subagentPayload}
	events = append(events, subagent)
	compactionPayload, _ := json.Marshal(CompactionObservedPayload{Count: 2})
	events = append(events, EventEnvelope{Version: ProtocolVersion{Major: 1}, EventID: "compact", ObservationID: "compact-observation", EffortID: "effort", SessionID: "a-session", Timestamp: "2026-07-22T00:00:04Z", Kind: "compaction_observed", Predecessors: []string{"subagent"}, Payload: compactionPayload})

	read := EffortRead{
		Metadata:  EffortMetadata{EffortID: "effort", CreatedAt: "2026-07-22T00:00:00Z", CheckpointID: "pi-workflow-dashboard.md", CreationMode: "independent"},
		Events:    events,
		Integrity: []IntegrityIssue{{Code: "partial-final-line", Scope: "z-session", EventIDs: []string{"usage"}, Detail: "/secret/repository/path"}},
	}
	generated := time.Date(2026, 7, 22, 1, 0, 0, 0, time.UTC)
	result, err := AggregateMetrics([]EffortRead{read}, Selector{}, MetricsOptions{GeneratedAt: generated, Retention: RetentionPolicy{MaxCompletedEffortAgeDays: 90, MaxCompletedEffortCount: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Efforts) != 1 || result.Efforts[0].EffortID != "effort" || result.Efforts[0].CheckpointID != "pi-workflow-dashboard.md" {
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
	for _, semantic := range []string{fmt.Sprintf("effort %s state=active route=direct", "effort"), "input=17 output=11", "cost=0.75 duration-ms=50", "compactions=2", "integrity partial-final-line severity=warning"} {
		if !strings.Contains(human.String(), semantic) {
			t.Fatalf("human output lacks %q:\n%s", semantic, human.String())
		}
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
	parentRead := EffortRead{Metadata: EffortMetadata{EffortID: "parent-effort", CreatedAt: "2026-07-01T00:00:00Z", CheckpointID: "parent.md", CreationMode: "independent"}, Events: parentEvents}

	origin := &OriginMetadata{EffortID: "parent-effort", TrajectoryID: "parent", AnchorID: "anchor"}
	childRead := EffortRead{Metadata: EffortMetadata{EffortID: "child-effort", CreatedAt: "2026-07-02T00:00:00Z", CheckpointID: "child.md", CreationMode: "derived", Origin: origin}, Events: lifecycleBaseEvents()}
	terminalEvents := completedRoute("direct")
	for index := range terminalEvents {
		terminalEvents[index].EffortID = "terminal-effort"
		terminalEvents[index].Timestamp = "2026-01-02T00:00:00Z"
	}
	terminalRead := EffortRead{Metadata: EffortMetadata{EffortID: "terminal-effort", CreatedAt: "2026-01-01T00:00:00Z", CheckpointID: "terminal.md", CreationMode: "independent"}, Events: terminalEvents}

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

func TestStableMetricsJSONContractAndSaturatingTotals(t *testing.T) {
	generated := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	result := MetricsResult{SchemaVersion: 1, ProtocolMajor: 1, GeneratedAt: generated, Selector: Selector{}, Efforts: []EffortProjection{}, Retention: RetentionState{MaxAgeDays: 90, MaxCount: 100, Candidates: []string{}}, Integrity: []IntegrityNotice{}}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schemaVersion":1,"protocolMajor":1,"generatedAt":"2026-07-22T00:00:00Z","selector":{},"efforts":[],"retention":{"maxAgeDays":90,"maxCount":100,"terminalEffortCount":0,"candidates":[]},"integrity":[]}`
	if string(raw) != want {
		t.Fatalf("stable JSON = %s\nwant %s", raw, want)
	}
	if got := saturatingAdd(math.MaxUint64-1, 2); got != math.MaxUint64 {
		t.Fatalf("saturating total = %d", got)
	}
}
