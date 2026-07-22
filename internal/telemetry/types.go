package telemetry

import "encoding/json"

type ProtocolVersion struct {
	Major uint16 `json:"major"`
	Minor uint16 `json:"minor"`
}

type EventKind string
type Route string
type Phase string
type Activity string
type TerminalOutcome string
type AssociationOrigin string
type CreationMode string
type DetachReason string
type ProposalKind string
type Outcome string
type StopReason string
type ErrorCategory string
type ShellClassification string
type GateMode string
type WaiverReasonCode string
type DiagnosticRuleCode string
type BoundedCategory string
type ModelName string
type ToolName string

type EventEnvelope struct {
	Version            ProtocolVersion            `json:"version"`
	EventID            string                     `json:"eventId"`
	IdempotencyKey     string                     `json:"idempotencyKey,omitempty"`
	ObservationID      string                     `json:"observationId,omitempty"`
	EffortID           string                     `json:"effortId"`
	SessionID          string                     `json:"sessionId"`
	TrajectoryID       string                     `json:"trajectoryId,omitempty"`
	ParentTrajectoryID string                     `json:"parentTrajectoryId,omitempty"`
	PiAnchorID         string                     `json:"piAnchorId,omitempty"`
	ForkAnchorID       string                     `json:"forkAnchorId,omitempty"`
	Timestamp          string                     `json:"timestamp"`
	Kind               EventKind                  `json:"kind"`
	Predecessors       []string                   `json:"predecessors"`
	Payload            json.RawMessage            `json:"payload"`
	EnvelopeExtensions map[string]json.RawMessage `json:"-"`
	PayloadExtensions  map[string]json.RawMessage `json:"-"`
}

type EffortCreatedPayload struct {
	CheckpointID       string       `json:"checkpointId"`
	CreationMode       CreationMode `json:"creationMode"`
	OriginEffortID     string       `json:"originEffortId,omitempty"`
	OriginTrajectoryID string       `json:"originTrajectoryId,omitempty"`
	OriginAnchorID     string       `json:"originAnchorId,omitempty"`
}

type SessionAssociatedPayload struct {
	AssociationOrigin AssociationOrigin `json:"associationOrigin"`
	TrajectoryID      string            `json:"trajectoryId"`
	HandoffEventID    string            `json:"handoffEventId,omitempty"`
}

type SessionDetachedPayload struct {
	Reason DetachReason `json:"reason"`
}

type RoutePayload struct {
	Route Route `json:"route"`
}

type PhaseStartedPayload struct {
	Phase              Phase           `json:"phase"`
	Activity           Activity        `json:"activity,omitempty"`
	ImplementationMode BoundedCategory `json:"implementationMode,omitempty"`
}

type PhaseFinishedPayload struct {
	Phase        Phase   `json:"phase"`
	StartEventID string  `json:"startEventId"`
	Outcome      Outcome `json:"outcome,omitempty"`
}

type TrajectoryPayload struct {
	TrajectoryID string          `json:"trajectoryId"`
	AnchorID     string          `json:"anchorId"`
	Reason       BoundedCategory `json:"reason,omitempty"`
}

type TrajectoryForkedPayload struct {
	TrajectoryID       string `json:"trajectoryId"`
	ParentTrajectoryID string `json:"parentTrajectoryId"`
	ForkAnchorID       string `json:"forkAnchorId"`
}

type EffortTerminalPayload struct {
	TerminalEpoch uint64 `json:"terminalEpoch"`
}

type EffortReopenedPayload struct {
	TerminalEpoch uint64 `json:"terminalEpoch"`
	TrajectoryID  string `json:"trajectoryId"`
	AnchorID      string `json:"anchorId"`
}

type FindingWaivedPayload struct {
	RuleCode    DiagnosticRuleCode `json:"ruleCode"`
	Scope       BoundedCategory    `json:"scope"`
	EvidenceIDs []string           `json:"evidenceIds"`
	ReasonCode  WaiverReasonCode   `json:"reasonCode"`
}

type RepairReplacement struct {
	EventKind EventKind       `json:"eventKind"`
	Payload   json.RawMessage `json:"payload"`
}

type RepairAppliedPayload struct {
	ProposalKind   ProposalKind      `json:"proposalKind"`
	SourceEventIDs []string          `json:"sourceEventIds"`
	Replacement    RepairReplacement `json:"replacement"`
}

type UsageObservedPayload struct {
	Model            ModelName `json:"model"`
	InputTokens      uint64    `json:"inputTokens"`
	OutputTokens     uint64    `json:"outputTokens"`
	CacheReadTokens  uint64    `json:"cacheReadTokens"`
	CacheWriteTokens uint64    `json:"cacheWriteTokens"`
	CostUSD          float64   `json:"costUsd"`
	DurationMS       uint64    `json:"durationMs"`
	Phase            Phase     `json:"phase,omitempty"`
	Activity         Activity  `json:"activity,omitempty"`
}

type ToolObservedPayload struct {
	Tool          ToolName      `json:"tool"`
	Outcome       Outcome       `json:"outcome"`
	DurationMS    uint64        `json:"durationMs"`
	ErrorCategory ErrorCategory `json:"errorCategory,omitempty"`
}

type ShellObservedPayload struct {
	Classification ShellClassification `json:"classification"`
	Outcome        Outcome             `json:"outcome"`
	GateMode       GateMode            `json:"gateMode,omitempty"`
}

type CompactionObservedPayload struct {
	Count uint64 `json:"count"`
}

type HandoffObservedPayload struct {
	Outcome         Outcome       `json:"outcome"`
	TargetSessionID string        `json:"targetSessionId"`
	DurationMS      uint64        `json:"durationMs,omitempty"`
	ErrorCategory   ErrorCategory `json:"errorCategory,omitempty"`
}

type SubagentObservedPayload struct {
	Role             BoundedCategory `json:"role"`
	RequestedModel   ModelName       `json:"requestedModel"`
	ResolvedModel    ModelName       `json:"resolvedModel"`
	ThinkingLevel    BoundedCategory `json:"thinkingLevel"`
	QueueDurationMS  uint64          `json:"queueDurationMs"`
	RunDurationMS    uint64          `json:"runDurationMs"`
	InputTokens      uint64          `json:"inputTokens"`
	OutputTokens     uint64          `json:"outputTokens"`
	CacheReadTokens  uint64          `json:"cacheReadTokens"`
	CacheWriteTokens uint64          `json:"cacheWriteTokens"`
	CostUSD          float64         `json:"costUsd"`
	Outcome          Outcome         `json:"outcome"`
	StopReason       StopReason      `json:"stopReason"`
	ToolCount        uint64          `json:"toolCount"`
	ToolFailureCount uint64          `json:"toolFailureCount"`
	ErrorCategory    ErrorCategory   `json:"errorCategory,omitempty"`
}

type SessionObservedPayload struct {
	Outcome       Outcome       `json:"outcome"`
	DurationMS    uint64        `json:"durationMs,omitempty"`
	ErrorCategory ErrorCategory `json:"errorCategory,omitempty"`
}

type OriginMetadata struct {
	EffortID     string `json:"effortId"`
	TrajectoryID string `json:"trajectoryId"`
	AnchorID     string `json:"anchorId"`
}

type EffortMetadata struct {
	EffortID     string          `json:"effortId"`
	CreatedAt    string          `json:"createdAt"`
	CheckpointID string          `json:"checkpointId"`
	CreationMode CreationMode    `json:"creationMode"`
	Origin       *OriginMetadata `json:"origin,omitempty"`
}

type Association struct {
	EffortID          string            `json:"effortId"`
	SessionID         string            `json:"sessionId"`
	TrajectoryID      string            `json:"trajectoryId"`
	AssociationOrigin AssociationOrigin `json:"associationOrigin"`
}

type LifecycleRequest interface {
	lifecycleAction() string
}

type LifecycleRequestBase struct {
	Action         string   `json:"action"`
	IdempotencyKey string   `json:"idempotencyKey"`
	EventID        string   `json:"eventId"`
	EffortID       string   `json:"effortId"`
	SessionID      string   `json:"sessionId"`
	Timestamp      string   `json:"timestamp"`
	Predecessors   []string `json:"predecessors"`
}

type CreateLifecycleRequest struct {
	LifecycleRequestBase
	CheckpointID string          `json:"checkpointId"`
	CreationMode CreationMode    `json:"creationMode"`
	Origin       *OriginMetadata `json:"origin,omitempty"`
}

func (r CreateLifecycleRequest) lifecycleAction() string { return r.Action }

type AssociateLifecycleRequest struct {
	LifecycleRequestBase
	TrajectoryID      string            `json:"trajectoryId"`
	AssociationOrigin AssociationOrigin `json:"associationOrigin"`
	HandoffEventID    string            `json:"handoffEventId,omitempty"`
}

func (r AssociateLifecycleRequest) lifecycleAction() string { return r.Action }

type DetachLifecycleRequest struct {
	LifecycleRequestBase
	Reason DetachReason `json:"reason"`
}

func (r DetachLifecycleRequest) lifecycleAction() string { return r.Action }

type RouteLifecycleRequest struct {
	LifecycleRequestBase
	Route Route `json:"route"`
}

func (r RouteLifecycleRequest) lifecycleAction() string { return r.Action }

type StartPhaseLifecycleRequest struct {
	LifecycleRequestBase
	Phase              Phase           `json:"phase"`
	Activity           Activity        `json:"activity,omitempty"`
	ImplementationMode BoundedCategory `json:"implementationMode,omitempty"`
}

func (r StartPhaseLifecycleRequest) lifecycleAction() string { return r.Action }

type FinishPhaseLifecycleRequest struct {
	LifecycleRequestBase
	Phase        Phase   `json:"phase"`
	StartEventID string  `json:"startEventId"`
	Outcome      Outcome `json:"outcome,omitempty"`
}

func (r FinishPhaseLifecycleRequest) lifecycleAction() string { return r.Action }

type TrajectoryLifecycleRequest struct {
	LifecycleRequestBase
	TrajectoryID string          `json:"trajectoryId"`
	AnchorID     string          `json:"anchorId"`
	Reason       BoundedCategory `json:"reason,omitempty"`
}

func (r TrajectoryLifecycleRequest) lifecycleAction() string { return r.Action }

type ForkTrajectoryLifecycleRequest struct {
	LifecycleRequestBase
	TrajectoryID       string `json:"trajectoryId"`
	ParentTrajectoryID string `json:"parentTrajectoryId"`
	ForkAnchorID       string `json:"forkAnchorId"`
}

func (r ForkTrajectoryLifecycleRequest) lifecycleAction() string { return r.Action }

type TerminalLifecycleRequest struct {
	LifecycleRequestBase
}

func (r TerminalLifecycleRequest) lifecycleAction() string { return r.Action }

type ReopenLifecycleRequest struct {
	LifecycleRequestBase
	TrajectoryID string `json:"trajectoryId"`
	AnchorID     string `json:"anchorId"`
}

func (r ReopenLifecycleRequest) lifecycleAction() string { return r.Action }

type WaiveLifecycleRequest struct {
	LifecycleRequestBase
	RuleCode    DiagnosticRuleCode `json:"ruleCode"`
	Scope       BoundedCategory    `json:"scope"`
	EvidenceIDs []string           `json:"evidenceIds"`
	ReasonCode  WaiverReasonCode   `json:"reasonCode"`
}

func (r WaiveLifecycleRequest) lifecycleAction() string { return r.Action }

type RepairProposal struct {
	Kind           ProposalKind      `json:"kind"`
	SourceEventIDs []string          `json:"sourceEventIds"`
	Replacement    RepairReplacement `json:"replacement"`
}

type RepairLifecycleRequest struct {
	LifecycleRequestBase
	Proposal RepairProposal `json:"proposal"`
}

func (r RepairLifecycleRequest) lifecycleAction() string { return r.Action }
