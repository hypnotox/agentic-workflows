package migrate

import (
	"bytes"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

const profileGeneration = 46

// applyProfile preserves every existing adopter's complete governance workflow.
// Fresh initialization is separately Core by default.
func applyProfile(root string, out *Changes) error {
	return editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		valued, err := config.HasValue(src, "profile")
		if err != nil { // coverage-ignore: editConfig parsed this unchanged config mapping before invoking the migration callback
			return nil, err
		}
		if valued {
			return src, nil
		}
		next, err := config.SetString(src, "profile", "full")
		if err != nil { // coverage-ignore: HasValue parsed this unchanged config mapping immediately above
			return nil, err
		}
		if !bytes.Equal(next, src) {
			planned.Add("workflow-profile: selected full for an existing repository")
		}
		return next, nil
	})
}
