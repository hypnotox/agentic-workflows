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
)

func runInit(root string, force, describe bool, sets []string, answersFile string, stdout io.Writer) error {
	cat := catalog.Standard
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
	// Pre-prompt probe (conservative): refuse collisions before asking a single
	// question or writing anything. The post-answer InitCollisions below stays
	// as the accurate second line — a trim answer can enable non-core artifacts
	// this curated-core probe set does not cover. --force skips the probe.
	if !force {
		collisions, err := probeCollisions(root)
		if err != nil {
			return err
		}
		if len(collisions) > 0 {
			return collisionRefusal(collisions)
		}
	}
	vars, trim, scopes, err := initspec.Resolve(descs, answers, stdin, stdout, isInteractive())
	if err != nil {
		return err
	}

	cfgPath := config.ConfigPath(root)
	scaffolded := false
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return err
		}
		scaffold, err := project.ScaffoldConfig(filepath.Base(root), vars, trim, scopes)
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
		return collisionRefusal(collisions)
	}
	// Under --force, the chained runSync backs up every foreign file via the shared
	// BackupFile mechanism (ADR-0035) — one backup path for init and sync alike.
	if err := runSync(root, stdout); err != nil {
		return err
	}
	// Post-init orientation: the same advisory notes awf check prints
	// (ADR-0045, ADR-0070), then a fixed next-steps block.
	np, err := project.Open(root)
	if err != nil { // coverage-ignore: the chained runSync just opened this same tree
		return err
	}
	notes, err := np.AdvisoryNotes()
	if err != nil { // coverage-ignore: runSync just rendered this same tree and generated its domain docs — both AdvisoryNotes inputs succeeded moments ago
		return err
	}
	for _, n := range notes {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	fmt.Fprint(stdout, initNextSteps)
	return nil
}

// collisionRefusal is the shared refusal for both collision checks, so the
// probe and the post-answer check read identically.
func collisionRefusal(collisions []string) error {
	return fmt.Errorf("awf init: refusing to overwrite existing files (use --force):\n  %s",
		strings.Join(collisions, "\n  "))
}

// probeCollisions computes the collision set before any prompt. With an
// existing config tree it asks the real project; otherwise it scaffolds a
// default (curated-core) config into a throwaway temp dir, plans that
// project's outputs, and tests the project-relative paths against root.
func probeCollisions(root string) ([]string, error) {
	if _, err := os.Stat(config.ConfigPath(root)); err == nil {
		p, err := project.Open(root)
		if err != nil {
			return nil, err
		}
		return p.InitCollisions()
	}
	tmp, err := os.MkdirTemp("", "awf-init-probe-*")
	if err != nil { // coverage-ignore: MkdirTemp fails only on an unwritable TMPDIR, which a test cannot trigger portably
		return nil, err
	}
	defer os.RemoveAll(tmp)
	scaffold, err := project.ScaffoldConfig(filepath.Base(root), nil, nil, nil)
	if err != nil { // coverage-ignore: ScaffoldConfig over the embedded catalog cannot fail at runtime
		return nil, err
	}
	cfgPath := config.ConfigPath(tmp)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: a fresh temp dir's child MkdirAll fails only on a permission fault root bypasses
		return nil, err
	}
	if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write into a fresh temp dir cannot fail in practice
		return nil, err
	}
	tp, err := project.Open(tmp)
	if err != nil { // coverage-ignore: a freshly-scaffolded default config always opens
		return nil, err
	}
	planned, err := tp.PlannedOutputs()
	if err != nil { // coverage-ignore: rendering the embedded catalog over a fresh scaffold in an empty tree cannot fail
		return nil, err
	}
	return project.CollisionsAt(root, planned), nil
}

// initNextSteps is the fixed orientation block init prints after a
// successful render.
const initNextSteps = `
next steps:
  1. Fill the Identity section: edit .awf/parts/agents-doc/identity.md, then run awf sync.
  2. Set any still-empty vars in .awf/config.yaml (the notes above list what each artifact misses), then run awf sync.
  3. Wire the rendered hook payloads under .awf/hooks/ into git hooks you own (see the workflow doc's local-hooks section) — awf never activates hooks itself.
  4. Commit .awf/ and the rendered files together.
`
