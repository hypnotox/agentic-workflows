package migrate

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// adrFormatV3Generation is the schema generation that seals the V3 cutoff.
const adrFormatV3Generation = 28

func applyADRFormatV3Cutoff(root string, out io.Writer) error {
	return applyADRFormatV3CutoffWithSave(root, out, func(lock *manifest.Lock, path string) error {
		return lock.Save(path)
	})
}

// applyADRFormatV3CutoffWithSave seals the permanent V3 boundary at the corpus's
// next identity, so every record authored from here on is current-state-v3 and
// every existing record is grandfathered at its authored format (ADR-0194 item
// 1). The schema stamp and the cutoff travel in one atomic lock save.
func applyADRFormatV3CutoffWithSave(root string, out io.Writer, save lockSaver) error {
	lockPath := config.LockPath(root)
	lock, found, err := manifest.LoadOptional(lockPath)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if lock.SchemaVersion >= adrFormatV3Generation {
		return nil
	}
	state, err := lock.AuthorityState()
	if err != nil { // coverage-ignore: LoadOptional parsed and validated this unchanged lock immediately above
		return err
	}
	if state != manifest.AuthorityPermanent || lock.ADRFormatV1From == 0 {
		lock.SchemaVersion = adrFormatV3Generation
		lock.AWFVersion = "0.30.0"
		return save(lock, lockPath)
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return err
	}
	corpus, err := adr.LoadCorpus(filepath.Join(root, cfg.DocsDir, "decisions"))
	if err != nil {
		return fmt.Errorf("compute ADR V3 cutoff: %w", err)
	}
	cutoff, err := corpus.NextIdentity()
	if err != nil { // coverage-ignore: parsed ADR identities always match the four-digit filename grammar
		return fmt.Errorf("compute ADR V3 cutoff: %w", err)
	}
	lock.ADRFormatV3From = cutoff
	lock.SchemaVersion = adrFormatV3Generation
	lock.AWFVersion = "0.30.0"
	if err := save(lock, lockPath); err != nil {
		return err
	}
	fmt.Fprintf(out, "adr-format-v3-cutoff: sealed ADR V3 cutoff at %d\n", cutoff)
	return nil
}
