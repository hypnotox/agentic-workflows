package migrate

import "io"

// applyCommitPolicy advances the schema for the optional commitPolicy mapping.
// The mapping is intentionally absent-safe: existing trees keep their exact
// bytes and opt in only by authoring a policy themselves.
func applyCommitPolicy(_ string, _ io.Writer) error { return nil }
