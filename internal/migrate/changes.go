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
// The schema-46 floor entry is a no-op; the next supported migration will add
// the first concrete collection operation with its semantic fact.
type Changes struct{}

// Items returns the facts collected by the current supported migration chain.
func (*Changes) Items() []Change { return nil }
