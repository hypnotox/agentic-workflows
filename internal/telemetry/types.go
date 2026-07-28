// Package telemetry reads the independent, session-keyed telemetry streams.
package telemetry

import (
	"encoding/json"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

const SchemaVersion = 1
const maxSafeInteger uint64 = 9007199254740991

type Selector struct {
	EffortID  *string    `json:"effortId,omitempty"`
	SessionID *string    `json:"sessionId,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
}

type Header struct {
	Record        string    `json:"record"`
	SchemaVersion int       `json:"schemaVersion"`
	SessionID     string    `json:"sessionId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Observation struct {
	Record        string          `json:"record"`
	SchemaVersion int             `json:"schemaVersion"`
	ObservationID string          `json:"observationId"`
	Timestamp     time.Time       `json:"timestamp"`
	Kind          string          `json:"kind"`
	Payload       json.RawMessage `json:"payload"`
	Raw           json.RawMessage `json:"-"`
}

type IntegrityFinding struct {
	Source    string `json:"source"`
	SessionID string `json:"sessionId"`
	Code      string `json:"code"`
}

type SessionRead struct {
	SessionID    string
	Header       Header
	Observations []Observation
	Records      []json.RawMessage
	Findings     []IntegrityFinding
}

type LegacyEffortRead struct {
	EffortID string
	Records  []json.RawMessage
	Findings []IntegrityFinding
}

type ReadSet struct {
	Sessions    []SessionRead
	Legacy      []LegacyEffortRead
	Records     map[string]effort.Record
	Assignments map[string]string
	Findings    []IntegrityFinding
}

type Counters struct {
	InputTokens      uint64  `json:"inputTokens"`
	OutputTokens     uint64  `json:"outputTokens"`
	CacheReadTokens  uint64  `json:"cacheReadTokens"`
	CacheWriteTokens uint64  `json:"cacheWriteTokens"`
	CostUSD          float64 `json:"costUsd"`
	ToolSuccesses    uint64  `json:"toolSuccesses"`
	ToolFailures     uint64  `json:"toolFailures"`
	ToolCancelled    uint64  `json:"toolCancelled"`
	GatesPassed      uint64  `json:"gatesPassed"`
	GatesFailed      uint64  `json:"gatesFailed"`
	GatesCancelled   uint64  `json:"gatesCancelled"`
	Subagents        uint64  `json:"subagents"`
	Compactions      uint64  `json:"compactions"`
	Handoffs         uint64  `json:"handoffs"`
	DurationMS       uint64  `json:"durationMs"`
}

type EffortReport struct {
	ID      string       `json:"id"`
	Title   string       `json:"title"`
	State   effort.State `json:"state"`
	Current Counters     `json:"current"`
	Legacy  Counters     `json:"legacy"`
}

type SessionReport struct {
	SessionID string   `json:"sessionId"`
	EffortID  *string  `json:"effortId"`
	Counters  Counters `json:"counters"`
}

type Report struct {
	SchemaVersion int             `json:"schemaVersion"`
	Selector      Selector        `json:"selector"`
	Efforts       []EffortReport  `json:"efforts"`
	Sessions      []SessionReport `json:"sessions,omitempty"`
}

type DoctorReport struct {
	SchemaVersion int                `json:"schemaVersion"`
	Selector      Selector           `json:"selector"`
	Findings      []IntegrityFinding `json:"findings"`
}
