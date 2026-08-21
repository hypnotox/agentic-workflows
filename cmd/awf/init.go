package main

import (
	"bytes"
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
	return runInitWithProjectLoader(ctx, root, force, describe, sets, answersFile, stdout, newProjectLoader)
}

func runInitWithProjectLoader(ctx context.Context, root string, force, describe bool, sets []string, answersFile string, stdout io.Writer, loadProject func(string) (*project.Loader, error)) error {
	cat := catalog.Standard
	descs := cat.Vars
	if describe {
		out, err := initspec.Describe(initspec.InitDescriptors(descs))
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
	// Pre-prompt probe: refuse collisions before asking a single question or
	// writing anything. The post-answer InitCollisions below remains the
	// authoritative second check. --force skips the probe.
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
	var scopes []string
	profile := catalog.ProfileCore
	ignoredAnswers := configExists && len(answers) > 0
	if configExists {
		// Descriptor answers only feed the scaffold; resolving them here would
		// prompt for (or silently accept) values init then discards.
	} else {
		var rerr error
		vars, scopes, profile, rerr = initspec.ResolveInit(descs, answers, stdin, stdout, isInteractive(), project.NeededVars)
		if rerr != nil {
			return rerr
		}
	}

	scaffolded := false
	if !configExists {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil { // coverage-ignore: entering this block needs cfgPath absent, which precludes a parent collision making MkdirAll fail
			return err
		}
		scaffold, err := project.ScaffoldConfigForProfile(filepath.Base(root), vars, scopes, profile)
		if err != nil { // coverage-ignore: ScaffoldConfig renders a static template over a dir basename; cannot fail in practice
			return err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil { // coverage-ignore: post-MkdirAll write; fails only on a permission fault that root bypasses
			return err
		}
		scaffolded = true
	}
	state, cfg, _, err := openProjectOperation(ctx, root)
	if err != nil {
		return err
	}
	collisions, err := project.InitCollisions(state, cfg)
	if err != nil {
		if scaffolded { // coverage-ignore: after first-adoption and scaffold validation, this cleanup requires a concurrent tree mutation to make InitCollisions fail
			_ = os.Remove(cfgPath)
			_ = os.Remove(filepath.Dir(cfgPath))
		}
		return err
	}
	if len(collisions) > 0 && !force { // coverage-ignore: the non-force probe now plans the same full catalog; force makes this condition false
		if scaffolded { // coverage-ignore: the enclosing post-scaffold collision path is unreachable after the identical full-catalog probe
			_ = os.Remove(cfgPath)               // remove the config we scaffolded
			_ = os.Remove(filepath.Dir(cfgPath)) // remove .awf only if now empty
		}
		return collisionRefusal(collisions) // coverage-ignore: the enclosing post-scaffold collision path is unreachable after the identical full-catalog probe
	}
	loader, syncErr := initProjectLoader(root, loadProject)
	if syncErr != nil {
		return syncErr
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
	// Under --force, the selected sync path backs up every foreign file through
	// the same confined backup policy used by ordinary sync (ADR-0035).
	var seed *project.InitAuthority
	if !configExists && !lockExists {
		seed = &project.InitAuthority{InitializedWithVersion: project.Version}
	}
	syncResult, syncedProject, syncedConfig, syncErr := syncMutation(ctx, loader, root, seed)
	if syncErr != nil {
		return finishInitSyncFailure(cfgPath, scaffolded, syncErr)
	}
	// Post-init orientation: the same advisory notes awf check prints
	// (ADR-0045, ADR-0070), then a fixed next-steps block.
	return renderInitOutcome(syncedProject, syncedConfig, initspec.Outcome{ConfigPath: cfgPath, ExistingConfig: configExists, IgnoredAnswers: ignoredAnswers, Sync: syncResult, NextActions: initNextActions}, stdout, project.AdvisoryNotes)
}

func initProjectLoader(root string, load func(string) (*project.Loader, error)) (*project.Loader, error) {
	return load(root)
}

func finishInitSyncFailure(cfgPath string, scaffolded bool, syncErr error) error {
	if scaffolded {
		_ = os.Remove(cfgPath)
		_ = os.Remove(filepath.Dir(cfgPath))
	}
	return syncErr
}

func renderInitOutcome(state *project.ProjectState, cfg *config.Config, outcome initspec.Outcome, stdout io.Writer, advisoryNotes func(*project.ProjectState, *config.Config) ([]string, error)) error {
	notes, err := advisoryNotes(state, cfg)
	if err != nil {
		return err
	}
	outcome.Advisories = notes
	document, err := outcome.Document()
	if err != nil {
		return err
	}
	return presentation.Render(stdout, document)
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
		state, cfg, _, err := openProjectOperation(ctx, root)
		if err != nil {
			return nil, err
		}
		return project.InitCollisions(state, cfg)
	}
	tmp, err := os.MkdirTemp("", "awf-init-probe-*")
	if err != nil { // coverage-ignore: MkdirTemp fails only on an unwritable TMPDIR, which a test cannot trigger portably
		return nil, err
	}
	defer os.RemoveAll(tmp)
	scaffold, err := project.ScaffoldConfig(filepath.Base(root), nil, nil)
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
	state, cfg, _, err := openProjectOperation(ctx, tmp)
	if err != nil { // coverage-ignore: a freshly-scaffolded default config always opens
		return nil, err
	}
	planned, err := project.PlannedOutputs(state, cfg)
	if err != nil { // coverage-ignore: rendering the embedded catalog over a fresh scaffold in an empty tree cannot fail
		return nil, err
	}
	return resident.CollisionsAt(root, planned)
}

var initNextActions = []string{
	"fill the Identity section at .awf/parts/agents-doc/identity.md, then run awf render",
	"set still-empty vars in .awf/config.yaml (the notes above list what each artifact misses), then run awf render",
	"wire rendered hook payloads under .awf/hooks/ into git hooks you own (see the workflow doc's local-hooks section); awf never activates hooks itself",
	"commit .awf/ and the rendered files together",
}

// writeInitDescriptorProtocol writes the documented init descriptor JSON
// unchanged. It is one of the closed successful protocol bypasses.
func writeInitDescriptorProtocol(stdout io.Writer, payload []byte) error {
	payload = bytes.TrimRight(payload, "\n")
	_, err := stdout.Write(append(payload, '\n'))
	return err
}
