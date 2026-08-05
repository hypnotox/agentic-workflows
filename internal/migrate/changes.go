package migrate

import (
	"strings"
)

// Change is one ordered semantic fact produced by a migration. It is data for
// the upgrade owner, not terminal output.
type Change struct {
	Text string
}

// Changes collects typed migration facts in mutation order; no command writer
// reaches a migration.
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

// Items returns the collected facts in production order.
func (c *Changes) Items() []Change { return append([]Change(nil), c.items...) }
