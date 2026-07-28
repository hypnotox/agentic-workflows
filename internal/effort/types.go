// Package effort owns repository-local lightweight effort records and memory.
package effort

import "time"

const SchemaVersion = 1

type State string

const (
	StateActive    State = "active"
	StateCompleted State = "completed"
	StateAbandoned State = "abandoned"
)

type Integration string

const (
	IntegrationNone        Integration = "none"
	IntegrationPending     Integration = "pending"
	IntegrationFastForward Integration = "fast-forward"
	IntegrationMerge       Integration = "merge"
	IntegrationManual      Integration = "manual"
)

type Worktree struct {
	Branch     string    `json:"branch"`
	Base       string    `json:"base"`
	AttachedAt time.Time `json:"attachedAt"`
}

// Record is the logical effort view.
type Record struct {
	SchemaVersion int         `json:"schemaVersion"`
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	State         State       `json:"state"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	MemoryPresent bool        `json:"memoryPresent"`
	Worktree      *Worktree   `json:"worktree"`
	Integration   Integration `json:"integration"`
}

// RepairChange is one deterministic correction made from confined filesystem truth.
type RepairChange struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

// RepairResult describes repair without concealing any changed field.
type RepairResult struct {
	SchemaVersion int            `json:"schemaVersion"`
	Record        Record         `json:"record"`
	Changes       []RepairChange `json:"changes"`
}
