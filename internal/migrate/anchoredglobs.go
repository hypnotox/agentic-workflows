package migrate

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyAnchoredGlobs ports a tree to the anchored path-glob dialect (ADR-0077):
// every no-slash pattern in invariants.sources[].globs and
// audit.dependencyManifests becomes `**/<pattern>`, preserving behaviour for
// every pattern valid under the old validator (doublestar brace alternation is
// the accepted edge, ADR-0077). Each rewrite collects one ordered change fact.
// Serialization stays owned by internal/config (ADR-0026); the write is atomic
// via editConfig (ADR-0076).
func applyAnchoredGlobs(root string, out *Changes) error {
	return editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		updated, rewrites, err := config.AnchorNoSlashGlobs(src)
		if err != nil {
			return nil, err
		}
		for _, r := range rewrites {
			planned.Add(fmt.Sprintf("anchored-globs: rewrote glob %q → %q (%s)\n", r.From, "**/"+r.From, r.Key))
		}
		return updated, nil
	})
}
