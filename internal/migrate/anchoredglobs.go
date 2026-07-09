package migrate

import (
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyAnchoredGlobs ports a tree to the anchored path-glob dialect (ADR-0077):
// every no-slash pattern in invariants.sources[].globs and
// audit.dependencyManifests becomes `**/<pattern>`, preserving behaviour for
// every pattern valid under the old validator (doublestar brace alternation is
// the accepted edge, ADR-0077). Each rewrite prints one provenance line to out.
// Serialization stays owned by internal/config (ADR-0026); the write is atomic
// via editConfig (ADR-0076).
func applyAnchoredGlobs(root string, out io.Writer) error {
	return editConfig(root, func(src []byte) ([]byte, error) {
		updated, rewrites, err := config.AnchorNoSlashGlobs(src)
		if err != nil {
			return nil, err
		}
		for _, r := range rewrites {
			fmt.Fprintf(out, "anchored-globs: rewrote glob %q → %q (%s)\n", r.From, "**/"+r.From, r.Key)
		}
		return updated, nil
	})
}
