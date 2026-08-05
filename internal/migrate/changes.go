package migrate

import (
	"strings"
)

// Change is one ordered semantic fact produced by a migration. It is data for
// the upgrade owner, not terminal output.
type Change struct {
	Text string
}

// Changes collects migration facts in mutation order. Write exists only to
// adapt the migration-local formatting helpers while they are progressively
// expressed as typed Change values; no command writer reaches a migration.
type Changes struct {
	items []Change
}

// Add records one nonempty migration fact.
func (c *Changes) Add(text string) {
	text = strings.TrimSuffix(text, "\n")
	if text != "" {
		c.items = append(c.items, Change{Text: text})
	}
}

// Write collects one migration-local formatted fact.
func (c *Changes) Write(p []byte) (int, error) {
	c.Add(string(p))
	return len(p), nil
}

// Reset clears collected facts for migration-local assertions.
func (c *Changes) Reset() { c.items = nil }

// Len returns the byte length of the collected text for migration-local assertions.
func (c *Changes) Len() int { return len(c.String()) }

// String exposes collected facts for migration-local assertions.
func (c *Changes) String() string {
	parts := make([]string, len(c.items))
	for i, item := range c.items {
		parts[i] = item.Text
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n") + "\n"
}

// Items returns the collected facts in production order.
func (c *Changes) Items() []Change { return append([]Change(nil), c.items...) }
