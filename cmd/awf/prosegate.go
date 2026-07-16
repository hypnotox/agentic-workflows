package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/prosegate"
)

// runProseGate scans the project's tracked text files for banned typographic
// punctuation substitutes (ADR-0119). It returns nil without scanning when the
// knob is off, so a hook or a runner may invoke it unconditionally.
func runProseGate(root string, stdout io.Writer) error {
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return err
	}
	if cfg.ProseGate == nil || !cfg.ProseGate.Enabled {
		return nil
	}
	exemptions := make([]prosegate.Exemption, 0, len(cfg.ProseGate.Exemptions))
	for _, e := range cfg.ProseGate.Exemptions {
		r, perr := prosegate.ParseCodepoint(e.Codepoint)
		if perr != nil {
			return fmt.Errorf("prose-gate: exemption for %s: %w", e.Path, perr)
		}
		exemptions = append(exemptions, prosegate.Exemption{Path: e.Path, Codepoint: r, Count: e.Count})
	}
	paths, err := git.IndexPaths(root)
	if err != nil {
		return fmt.Errorf("prose-gate: cannot enumerate tracked files: %w", err)
	}
	findings, err := prosegate.Scan(root, paths, exemptions)
	if err != nil {
		return fmt.Errorf("prose-gate: %w", err)
	}
	for _, f := range findings {
		fmt.Fprintln(stdout, prosegate.Format(f))
	}
	if len(findings) > 0 {
		return errors.New("prose-gate: use plain punctuation, or exempt the path in proseGate.exemptions")
	}
	fmt.Fprintln(stdout, "prose-gate: clean")
	return nil
}
