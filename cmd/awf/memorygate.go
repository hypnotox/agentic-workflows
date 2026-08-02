package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/memorycite"
)

// runMemoryGate scans the staged decision records for a citation of a specific
// working-memory file (ADR-0158). It reports the disabled child and returns nil
// without scanning when the knob is off. The scanned prefixes derive from the
// configured docs directory, so an adopter with a custom docsDir gets their own
// decisions and plans directories.
func runMemoryGate(ctx context.Context, root string, stdout io.Writer) error {
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return err
	}
	if cfg.MemoryCite == nil || !cfg.MemoryCite.Enabled {
		fmt.Fprintln(stdout, "note: memory: disabled (memoryCite.enabled)")
		return nil
	}
	tree, err := stagedTree(ctx, root)
	if err != nil {
		return fmt.Errorf("check repo memory: cannot read staged files: %w", err)
	}
	exemptions := make([]memorycite.Exemption, 0, len(cfg.MemoryCite.Exemptions))
	for _, e := range cfg.MemoryCite.Exemptions {
		exemptions = append(exemptions, memorycite.Exemption{Path: e.Path, Count: e.Count})
	}
	d := strings.TrimRight(cfg.DocsDir, "/")
	prefixes := []string{d + "/decisions/", d + "/plans/"}
	var files []memorycite.File
	for _, blob := range tree.List() {
		// A staged symlink's bytes are a target path, not document text.
		if !blob.Scannable() {
			continue
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(blob.Path, prefix) {
				files = append(files, memorycite.File{Path: blob.Path, Bytes: blob.Bytes})
				break
			}
		}
	}
	findings := memorycite.Scan(files, exemptions)
	for _, f := range findings {
		fmt.Fprintln(stdout, memorycite.Format(f))
	}
	if len(findings) > 0 {
		return errors.New("check repo memory: remove the concrete effort-owned memory citation, name the bare .awf/efforts/ directory, use an angle-bracket slug placeholder, or exempt the path in memoryCite.exemptions")
	}
	fmt.Fprintln(stdout, "check repo memory: clean")
	return nil
}
