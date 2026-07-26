package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/memorycite"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// runMemoryGate scans the staged decision records for a citation of a specific
// working-memory file (ADR-0158). It returns nil without scanning when the knob
// is off, so a hook or a runner may invoke it unconditionally. The scanned
// prefixes derive from the configured docs directory, so an adopter with a
// custom docsDir gets their own decisions and plans directories.
func runMemoryGate(root string, stdout io.Writer) error {
	tree, err := snapshot.IndexTree(root)
	if err != nil {
		return fmt.Errorf("check memory: cannot read staged files: %w", err)
	}
	stagedConfig, ok := tree.Lookup(".awf/config.yaml")
	if !ok {
		return errors.New("check memory: staged snapshot has no .awf/config.yaml")
	}
	cfg, err := config.Parse(config.RootDir(root), stagedConfig.Bytes)
	if err != nil {
		return err
	}
	if cfg.MemoryCite == nil || !cfg.MemoryCite.Enabled {
		return nil
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
		return errors.New("check memory: name the working-memory file separately from the prefix or use the placeholder form, or exempt the path in memoryCite.exemptions")
	}
	fmt.Fprintln(stdout, "check memory: clean")
	return nil
}
