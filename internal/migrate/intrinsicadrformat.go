package migrate

import (
	"context"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

const intrinsicADRFormatGeneration = 31

// applyIntrinsicADRFormat removes the retired permanent routing payload. Parse
// accepts the old shape before this stamp, so no ADR needs to be loaded or
// rewritten to complete the migration.
func applyIntrinsicADRFormat(ctx context.Context, root string, out *Changes) error {
	return applyIntrinsicADRFormatWithSave(ctx, root, out, func(lock *manifest.Lock, path string) error {
		return lock.Save(path)
	})
}

func applyIntrinsicADRFormatWithSave(_ context.Context, root string, out *Changes, save func(*manifest.Lock, string) error) error {
	path := config.LockPath(root)
	lock, found, err := manifest.LoadOptional(path)
	if err != nil || !found || lock.SchemaVersion >= intrinsicADRFormatGeneration {
		return err
	}
	lock.SchemaVersion = intrinsicADRFormatGeneration
	if err := save(lock, path); err != nil {
		return err
	}
	fmt.Fprintln(out, "intrinsic-adr-format: discarded permanent ADR routing payload")
	return nil
}
