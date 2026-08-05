package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/initspec"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

func runInit(ctx context.Context, root string, force, describe bool, sets []string, answersFile string, stdout io.Writer) error {
	cat := catalog.Standard
	descs := initspec.CatalogVars(cat)
	if describe {
		out, err := initspec.Describe(descs)
		if err != nil { // coverage-ignore: descriptors marshal to JSON; cannot fail
			return err
		}
		return writeInitDescriptorProtocol(stdout, out)
	}
	cfgPath := config.ConfigPath(root)
	lockPath := config.LockPath(root)
	_, statErr := os.Stat(cfgPath)
	configExists := statErr == nil
	_, lockStatErr := os.Stat(lockPath)
	lockExists := lockStatErr == nil
	if !configExists && !lockExists {
		if _, err := adr.LoadCorpus(filepath.Join(root, "docs", "decisions")); err != nil {
			return fmt.Errorf("validate first-adoption ADR corpus: %w", err)
		}
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
	// as the accurate second line - a trim answer can enable non-core artifacts
	// this curated-core probe set does not cover. --force skips the probe.
	if !force {
		collisions, err := probeCollisions(ctx, root)
		if err != nil {
			return err
		}
		if len(collisions) > 0 {
			return collisionRefusal(collisions)
		}
	}
	if configExists || lockExists {
		lock, found, err := manifest.LoadOptional(lockPath)
		if err != nil {
			return fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", err)
		}
		if !found {
			return errors.New("pre-tracking authority: use the bridge release to attest before initializing")
		}
		state, err := lock.AuthorityState()
		if err != nil { // coverage-ignore: LoadOptional parsed and validated this unchanged lock immediately above
			return fmt.Errorf("invalid authority: restore .awf/awf.lock from version control: %w", err)
		}
		if state != manifest.AuthorityPermanent {
			return errors.New("pre-tracking authority: use the bridge release to attest before initializing")
		}
	}
	var vars map[string]string
	var trim *config.CatalogTrim
	var scopes []string
	if configExists {
		// Descriptor answers only feed the scaffold; resolving them here would
		// prompt for (or silently accept) values init then discards.
		if err := writeStatus(stdout, cfgPath+" exists: keeping it and re-rendering only"); err != nil {
			return err
		}
		if len(answers) > 0 {
			if err := writeStatus(stdout, "note: --set/--answers values were ignored; edit .awf/config.yaml instead"); err != nil {
				return err
			}
		}
	} else {
		var rerr error
		vars, trim, scopes, rerr = initspec.Resolve(descs, answers, stdin, stdout, isInteractive(), project.NeededVars)
		if rerr != nil {
			return rerr
		}
	}

	scaffolded := false
	if !configExists {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return err
		}
		scaffold, added, err := project.ScaffoldConfig(filepath.Base(root), vars, trim, scopes)
		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
			return err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write; fails only on a permission fault that root bypasses
			return err
		}
		scaffolded = true
		if err := writeStatus(stdout, "scaffolded: "+cfgPath); err != nil {
			return err
		}
		// A trimmed selection is closure-completed (ADR-0081 Decision 9).
		for _, a := range added {
			if err := writeStatus(stdout, "note: also enabled "+a+" (required by your selection)"); err != nil {
				return err
			}
		}
	}
	p, err := project.Open(ctx, root)
	if err != nil {
		return err
	}
	collisions, err := p.InitCollisions(ctx)
	if err != nil {
		if scaffolded { // coverage-ignore: first adoption validated its ADR boundary and the generated scaffold before this second collision plan; only a concurrent tree mutation can make it fail
			_ = os.Remove(cfgPath)
			_ = os.Remove(filepath.Dir(cfgPath))
		}
		return err
	}
	if len(collisions) > 0 && !force {
		if scaffolded {
			_ = os.Remove(cfgPath)               // remove the config we scaffolded
			_ = os.Remove(filepath.Dir(cfgPath)) // remove .awf only if now empty
		}
		return collisionRefusal(collisions)
	}
	// Gate before the chained sync: init is Ungated at the driver, but re-rendering
	// an existing schema- or version-behind tree must still refuse rather than
	// re-stamp the current schema over an unmigrated config (ADR-0039). runSync no
	// longer gates itself, so the two Ungated commands that chain it (init, upgrade)
	// re-assert the gate here. A fresh scaffold is current-schema with no lock, so
	// this passes.
	if err := gate(ctx, root); err != nil {
		return err
	}
	// Under --force, the selected sync path backs up every foreign file via the
	// shared BackupFile mechanism (ADR-0035) - one backup path for init and sync alike.
	var syncErr error
	if !configExists && !lockExists {
		syncErr = runSyncInitialized(ctx, root, project.InitAuthority{InitializedWithVersion: project.Version}, stdout)
	} else {
		syncErr = runSync(ctx, root, stdout)
	}
	if syncErr != nil {
		if scaffolded { // coverage-ignore: the first-adoption boundary, scaffold, collision plan, and gate all succeeded; a failure now requires a concurrent mutation or filesystem fault
			_ = os.Remove(cfgPath)
			_ = os.Remove(filepath.Dir(cfgPath))
		}
		return syncErr
	}
	// Post-init orientation: the same advisory notes awf check prints
	// (ADR-0045, ADR-0070), then a fixed next-steps block.
	np, err := project.Open(ctx, root)
	if err != nil { // coverage-ignore: the chained runSync just opened this same tree
		return err
	}
	notes, err := np.AdvisoryNotes(ctx)
	if err != nil { // coverage-ignore: runSync just rendered this same tree and generated its domain docs - both AdvisoryNotes inputs succeeded moments ago
		return err
	}
	return writeInitOrientation(stdout, notes)
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
func probeCollisions(ctx context.Context, root string) ([]string, error) {
	if _, err := os.Stat(config.ConfigPath(root)); err == nil {
		p, err := project.Open(ctx, root)
		if err != nil {
			return nil, err
		}
		return p.InitCollisions(ctx)
	}
	tmp, err := os.MkdirTemp("", "awf-init-probe-*")
	if err != nil { // coverage-ignore: MkdirTemp fails only on an unwritable TMPDIR, which a test cannot trigger portably
		return nil, err
	}
	defer os.RemoveAll(tmp)
	scaffold, _, err := project.ScaffoldConfig(filepath.Base(root), nil, nil, nil)
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
	tp, err := project.Open(ctx, tmp)
	if err != nil { // coverage-ignore: a freshly-scaffolded default config always opens
		return nil, err
	}
	planned, err := tp.PlannedOutputs(ctx)
	if err != nil { // coverage-ignore: rendering the embedded catalog over a fresh scaffold in an empty tree cannot fail
		return nil, err
	}
	return resident.CollisionsAt(root, planned)
}

// writeInitOrientation presents post-init advisories and independently
// executable next actions without compressing their semantic boundaries.
func writeInitOrientation(stdout io.Writer, advisories []string) error {
	notes := make([]presentation.Value, 0, len(advisories))
	for _, advisory := range advisories {
		value, err := presentation.Prose(advisory)
		if err != nil {
			return err
		}
		notes = append(notes, value)
	}
	actions := make([]presentation.Value, len(initNextActions))
	for i, action := range initNextActions {
		value, err := presentation.Prose(action)
		if err != nil { // coverage-ignore: fixed nonempty action prose contains no forbidden line break
			return err
		}
		actions[i] = value
	}
	document, err := (presentation.Mutation{Status: "initialization completed", Notes: notes, NextActions: actions}).Document()
	if err != nil { // coverage-ignore: values are validated above and Mutation uses fixed grammar-valid labels
		return err
	}
	return presentation.Render(stdout, document)
}

var initNextActions = []string{
	"fill the Identity section at .awf/parts/agents-doc/identity.md",
	"set still-empty vars in .awf/config.yaml",
	"wire rendered hooks under .awf/hooks/",
	"commit .awf and rendered files together",
}

// writeInitDescriptorProtocol writes the documented init descriptor JSON
// unchanged. It is one of the closed successful protocol bypasses.
func writeInitDescriptorProtocol(stdout io.Writer, payload []byte) error {
	_, err := stdout.Write(append(payload, '\n'))
	return err
}
