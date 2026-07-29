// Package effort owns repository-local immutable effort residents and their memory.
package effort

import "time"

const SchemaVersion = 2

// Record is the public protocol-2 effort view. SchemaVersion belongs to the
// containing reply, while static state carries it directly.
type Record struct {
	SchemaVersion int       `json:"-"`
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"createdAt"`
	MemoryPath    string    `json:"memoryPath"`
}

// FinishResult reports the restartable deletion mutations separately.
type FinishResult struct {
	Renamed bool
	Cleaned bool
}
