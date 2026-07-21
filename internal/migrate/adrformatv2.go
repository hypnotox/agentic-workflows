package migrate

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// lockSaver is the one persistence seam for the schema-15 authority change.
type lockSaver func(*manifest.Lock, string) error

func applyADRFormatV2Cutoff(root string, out io.Writer) error {
	return applyADRFormatV2CutoffWithSave(root, out, func(lock *manifest.Lock, path string) error {
		return lock.Save(path)
	})
}

func applyADRFormatV2CutoffWithSave(root string, out io.Writer, save lockSaver) error {
	lockPath := config.LockPath(root)
	lock, found, err := manifest.LoadOptional(lockPath)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if lock.SchemaVersion >= 15 {
		return nil
	}
	state, err := lock.AuthorityState()
	if err != nil { // coverage-ignore: LoadOptional parsed and validated this unchanged lock immediately above
		return err
	}
	if state != manifest.AuthorityPermanent || lock.ADRFormatV1From == 0 {
		lock.SchemaVersion = 15
		lock.AWFVersion = "0.20.0"
		return save(lock, lockPath)
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return err
	}
	corpus, err := adr.LoadCorpus(filepath.Join(root, cfg.DocsDir, "decisions"))
	if err != nil {
		return fmt.Errorf("compute ADR V2 cutoff: %w", err)
	}
	cutoff, err := corpus.NextIdentity()
	if err != nil { // coverage-ignore: parsed ADR identities always match the four-digit filename grammar
		return fmt.Errorf("compute ADR V2 cutoff: %w", err)
	}
	lock.ADRFormatV2From = cutoff
	lock.SchemaVersion = 15
	lock.AWFVersion = "0.20.0"
	if err := save(lock, lockPath); err != nil {
		return err
	}
	fmt.Fprintf(out, "adr-format-v2-cutoff: sealed ADR V2 cutoff at %d\n", cutoff)
	return nil
}
