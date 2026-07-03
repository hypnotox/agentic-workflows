package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/templates"
)

func runInit(root string, force, describe bool, sets []string, answersFile string, stdout io.Writer) error {
	cat, err := catalog.Load(templates.FS)
	if err != nil { // coverage-ignore: catalog.Load over the embedded FS cannot fail at runtime
		return err
	}
	descs := initspec.CatalogVars(cat)
	if describe {
		out, err := initspec.Describe(descs)
		if err != nil { // coverage-ignore: descriptors marshal to JSON; cannot fail
			return err
		}
		fmt.Fprintln(stdout, string(out))
		return nil
	}
	answers := map[string]string{}
	if answersFile != "" {
		b, err := os.ReadFile(answersFile)
		if err != nil {
			return fmt.Errorf("awf init: read --answers: %w", err)
		}
		if answers, err = initspec.ParseAnswersFile(b); err != nil {
			return err
		}
	}
	if err := initspec.MergeSetFlags(answers, sets); err != nil {
		return err
	}
	vars, inv, trim, scopes, err := initspec.Resolve(descs, answers, stdin, stdout, isInteractive())
	if err != nil {
		return err
	}

	cfgPath := config.ConfigPath(root)
	scaffolded := false
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return err
		}
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, inv, trim, scopes)
		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
			return err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write; fails only on a permission fault that root bypasses
			return err
		}
		scaffolded = true
		fmt.Fprintf(stdout, "scaffolded %s\n", cfgPath)
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	collisions, err := p.InitCollisions()
	if err != nil {
		return err
	}
	if len(collisions) > 0 && !force {
		if scaffolded {
			_ = os.Remove(cfgPath)               // remove the config we scaffolded
			_ = os.Remove(filepath.Dir(cfgPath)) // remove .awf only if now empty
		}
		return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
			strings.Join(collisions, "\n  "))
	}
	// Under --force, the chained runSync backs up every foreign file via the shared
	// BackupFile mechanism (ADR-0035) — one backup path for init and sync alike.
	if err := runSync(root, stdout); err != nil {
		return err
	}
	return nil
}
