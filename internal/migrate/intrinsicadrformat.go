package migrate

import (
	"context"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

const intrinsicADRFormatGeneration = 31

// applyIntrinsicADRFormat removes the retired permanent routing payload. Parse
// accepts the old shape before this stamp, so no ADR needs to be loaded or
// rewritten to complete the migration.
func applyIntrinsicADRFormat(_ context.Context, root string, out io.Writer) error {
	path := config.LockPath(root)
	lock, found, err := manifest.LoadOptional(path)
	if err != nil || !found || lock.SchemaVersion >= intrinsicADRFormatGeneration {
		return err
	}
	lock.SchemaVersion = intrinsicADRFormatGeneration
	if err := lock.Save(path); err != nil { // coverage-ignore: the already-loaded lock path can fail here only after an external filesystem mutation
		return err
	}
	fmt.Fprintln(out, "intrinsic-adr-format: discarded permanent ADR routing payload")
	return nil
}
