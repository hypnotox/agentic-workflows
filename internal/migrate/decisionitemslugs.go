package migrate

import (
	"context"
	"io"
)

// applyDecisionItemSlugs activates ADR V4 without rewriting historical ADR or
// config bytes. The regular upgrade path owns the schema stamp.
func applyDecisionItemSlugs(_ context.Context, _ string, _ io.Writer) error { return nil }
