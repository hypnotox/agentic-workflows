package migrate

import (
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyAnchoredGlobs ports a tree to the anchored path-glob dialect (ADR-0077):
// every no-slash pattern in invariants.sources[].globs and
// audit.dependencyManifests becomes `**/<pattern>`, preserving behaviour for
// every pattern valid under the old validator (doublestar brace alternation is
// the accepted edge, ADR-0077). Serialization stays owned by internal/config
// (ADR-0026); the write is atomic via editConfig (ADR-0076).
func applyAnchoredGlobs(root string, _ io.Writer) error {
	return editConfig(root, config.AnchorNoSlashGlobs)
}
