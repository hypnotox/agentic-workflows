package migrate

// Change is one ordered semantic fact produced by a migration. It is data for
// the upgrade owner, not terminal output.
type Change struct {
	Text string
}

// CurrentSchemaChange reports the migration domain's proven no-migration state
// to its terminal owner.
func CurrentSchemaChange() Change {
	return Change{Text: "config schema already current"}
}

// Changes is the fact collector passed through the supported migration seam.
type Changes struct{ items []Change }

// Items returns a defensive copy of the facts collected by the migration chain.
func (c *Changes) Items() []Change { return append([]Change(nil), c.items...) }
