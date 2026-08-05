package migrate

import "strings"

// Reset clears collected facts for migration-local assertions.
func (c *Changes) Reset() { c.items = nil }

// Len returns the byte length of collected test facts.
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
