package telemetry

import (
	"encoding/json"
	"math"
	"sort"
)

func Aggregate(reads ReadSet, selector Selector) (Report, error) {
	if err := ValidateSelector(selector); err != nil {
		return Report{}, err
	}
	current := map[string]Counters{}
	legacy := map[string]Counters{}
	sessionCounters := map[string]Counters{}
	for _, s := range reads.Sessions {
		var total Counters
		for _, o := range s.Observations {
			if !selectObservation(o, selector) {
				continue
			}
			addObservation(&total, o)
		}
		sessionCounters[s.SessionID] = total
		if id := reads.Assignments[s.SessionID]; id != "" {
			current[id] = addCounters(current[id], total)
		}
	}
	for _, l := range reads.Legacy {
		var c Counters
		for _, raw := range l.Records {
			addLegacy(&c, raw)
		}
		legacy[l.EffortID] = addCounters(legacy[l.EffortID], c)
	}
	result := Report{SchemaVersion: SchemaVersion, Selector: selector, Efforts: []EffortReport{}}
	for id, r := range reads.Records {
		if selector.EffortID != nil && *selector.EffortID != id {
			continue
		}
		if selector.SessionID != nil {
			if assigned := reads.Assignments[*selector.SessionID]; assigned != id {
				continue
			}
		}
		result.Efforts = append(result.Efforts, EffortReport{ID: id, Title: r.Title, State: r.State, Current: current[id], Legacy: legacy[id]})
	}
	sort.Slice(result.Efforts, func(i, j int) bool { return result.Efforts[i].ID < result.Efforts[j].ID })
	if selector.SessionID != nil {
		id, ok := reads.Assignments[*selector.SessionID]
		var pointer *string
		if ok {
			copy := id
			pointer = &copy
		}
		result.Sessions = []SessionReport{{SessionID: *selector.SessionID, EffortID: pointer, Counters: sessionCounters[*selector.SessionID]}}
	}
	return result, nil
}
func addCounters(a, b Counters) Counters {
	a.InputTokens = sat(a.InputTokens, b.InputTokens)
	a.OutputTokens = sat(a.OutputTokens, b.OutputTokens)
	a.CacheReadTokens = sat(a.CacheReadTokens, b.CacheReadTokens)
	a.CacheWriteTokens = sat(a.CacheWriteTokens, b.CacheWriteTokens)
	a.CostUSD += b.CostUSD
	a.ToolSuccesses = sat(a.ToolSuccesses, b.ToolSuccesses)
	a.ToolFailures = sat(a.ToolFailures, b.ToolFailures)
	a.ToolCancelled = sat(a.ToolCancelled, b.ToolCancelled)
	a.GatesPassed = sat(a.GatesPassed, b.GatesPassed)
	a.GatesFailed = sat(a.GatesFailed, b.GatesFailed)
	a.GatesCancelled = sat(a.GatesCancelled, b.GatesCancelled)
	a.Subagents = sat(a.Subagents, b.Subagents)
	a.Compactions = sat(a.Compactions, b.Compactions)
	a.Handoffs = sat(a.Handoffs, b.Handoffs)
	a.DurationMS = sat(a.DurationMS, b.DurationMS)
	return a
}
func sat(a, b uint64) uint64 {
	if math.MaxUint64-a < b {
		return math.MaxUint64
	}
	return a + b
}
func addObservation(c *Counters, o Observation) {
	var p map[string]json.RawMessage
	if json.Unmarshal(o.Payload, &p) != nil {
		return
	}
	integer := func(k string) uint64 { var n uint64; _ = json.Unmarshal(p[k], &n); return n }
	duration := func() { c.DurationMS = sat(c.DurationMS, integer("durationMs")) }
	switch o.Kind {
	case "usage":
		c.InputTokens = sat(c.InputTokens, integer("inputTokens"))
		c.OutputTokens = sat(c.OutputTokens, integer("outputTokens"))
		c.CacheReadTokens = sat(c.CacheReadTokens, integer("cacheReadTokens"))
		c.CacheWriteTokens = sat(c.CacheWriteTokens, integer("cacheWriteTokens"))
		var cost float64
		_ = json.Unmarshal(p["costUsd"], &cost)
		c.CostUSD += cost
	case "tool":
		var v string
		_ = json.Unmarshal(p["outcome"], &v)
		switch v {
		case "success":
			c.ToolSuccesses++
		case "failure":
			c.ToolFailures++
		default:
			c.ToolCancelled++
		}
		duration()
	case "gate":
		var v string
		_ = json.Unmarshal(p["outcome"], &v)
		switch v {
		case "success":
			c.GatesPassed++
		case "failure":
			c.GatesFailed++
		default:
			c.GatesCancelled++
		}
		duration()
	case "subagent":
		c.Subagents++
		duration()
	case "compaction":
		c.Compactions++
	case "handoff":
		c.Handoffs++
		duration()
	}
}
func addLegacy(c *Counters, raw json.RawMessage) {
	var x struct {
		Kind    string          `json:"kind"`
		Payload json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(raw, &x) != nil {
		return
	}
	if x.Kind == "usage_observed" {
		var p struct {
			InputTokens, OutputTokens, CacheReadTokens, CacheWriteTokens uint64
			CostUSD                                                      float64
		}
		if json.Unmarshal(x.Payload, &p) == nil {
			c.InputTokens = sat(c.InputTokens, p.InputTokens)
			c.OutputTokens = sat(c.OutputTokens, p.OutputTokens)
			c.CacheReadTokens = sat(c.CacheReadTokens, p.CacheReadTokens)
			c.CacheWriteTokens = sat(c.CacheWriteTokens, p.CacheWriteTokens)
			c.CostUSD += p.CostUSD
		}
	}
}
