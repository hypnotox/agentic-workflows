package migrate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

const structuralHeadingsGeneration = 36

// structuralHeadingSnapshot is the cutover population, deliberately frozen rather
// than derived from templates. Later headings are body content to this migration.
var structuralHeadingSnapshot = []struct{ path, heading string }{
	{"agents/parts/adr-reviewer/project-focus.md", "## Project-specific focus items"},
	{"agents/parts/adr-reviewer/universal-lenses.md", "## Universal lenses"},
	{"agents/parts/code-reviewer/doc-currency.md", "## Doc-currency checklist"},
	{"agents/parts/code-reviewer/project-focus.md", "## Project-specific focus items"},
	{"agents/parts/code-reviewer/universal-lenses.md", "## Universal lenses"},
	{"agents/parts/explorer/breadth.md", "## Breadth"},
	{"agents/parts/explorer/grounding-and-outcomes.md", "## Grounding and outcomes"},
	{"agents/parts/explorer/identity.md", "## Identity"},
	{"agents/parts/explorer/report-detail.md", "## Report detail"},
	{"agents/parts/explorer/report-discipline.md", "## Report discipline"},
	{"agents/parts/explorer/single-need.md", "## One information need"},
	{"agents/parts/grounding-checker/identity.md", "## Identity"},
	{"agents/parts/grounding-checker/return-schema.md", "## What to return"},
	{"agents/parts/grounding-checker/verification-scope.md", "## What to verify"},
	{"agents/parts/implementer/escalation.md", "## No user is reachable"},
	{"agents/parts/implementer/green-obligation.md", "## Reaching green is the job"},
	{"agents/parts/implementer/guide-authority.md", "## What binds you in the agent guide, and what does not"},
	{"agents/parts/implementer/identity.md", "## Identity and authority mode"},
	{"agents/parts/implementer/owner-transaction.md", "## The owner transaction"},
	{"agents/parts/implementer/return-schema.md", "## What to return"},
	{"agents/parts/implementer/task-scope.md", "## The task is the complete scope"},
	{"agents/parts/plan-reviewer/doc-currency.md", "## Doc-currency checklist"},
	{"agents/parts/plan-reviewer/project-focus.md", "## Project-specific focus items"},
	{"agents/parts/plan-reviewer/resync-note.md", "## Resync mode"},
	{"agents/parts/plan-reviewer/universal-lenses.md", "## Universal lenses"},
	{"docs/parts/architecture/components.md", "## Components"},
	{"docs/parts/architecture/data-flow.md", "## Data flow"},
	{"docs/parts/architecture/dependencies.md", "## Key dependencies"},
	{"docs/parts/architecture/overview.md", "## Overview"},
	{"docs/parts/debugging/recipes.md", "## Recipes"},
	{"docs/parts/debugging/surfaces.md", "## Inspection surfaces"},
	{"docs/parts/development/command-runner.md", "## Command runner"},
	{"docs/parts/development/dependencies.md", "## Dependencies"},
	{"docs/parts/development/setup.md", "## Setup"},
	{"docs/parts/roadmap/deferred.md", "## Deferred"},
	{"docs/parts/roadmap/ideas.md", "## Ideas"},
	{"docs/parts/testing/gate.md", "## The gate"},
	{"docs/parts/testing/layout.md", "## Test layout"},
	{"docs/parts/testing/tiers.md", "## Tiers"},
	{"domains/parts/*/current-state.md", "## Current state"},
	{"parts/adr-readme/frontmatter.md", "## Frontmatter"},
	{"parts/adr-readme/index.md", "## INDEX.md"},
	{"parts/adr-readme/lifecycle.md", "## Status and lifecycle"},
	{"parts/adr-readme/naming.md", "## Naming & location"},
	{"parts/adr-readme/state-changes.md", "## The Decision and State changes sections"},
	{"parts/adr-readme/when.md", "## When to write an ADR"},
	{"parts/adr-template/body.md", "## Context"},
	{"parts/agents-doc/awf-setup.md", "## Working with awf"},
	{"parts/agents-doc/commands.md", "## Commands"},
	{"parts/agents-doc/document-map.md", "## Document map"},
	{"parts/agents-doc/identity.md", "## Identity"},
	{"parts/agents-doc/invariants.md", "## Invariants"},
	{"parts/agents-doc/workflow.md", "## Workflow"},
	{"parts/agents-doc/working-memory.md", "## Working memory"},
	{"parts/agents-doc/you-and-this-project.md", "## You and this project"},
	{"parts/agents-md-standard/content.md", "## Content"},
	{"parts/agents-md-standard/layout.md", "## Layout"},
	{"parts/agents-md-standard/rules.md", "## Rules"},
	{"parts/config-reference/intro.md", "# Configuration Reference"},
	{"parts/doc-standard/principles.md", "## Principles"},
	{"parts/doc-standard/rules.md", "## Rules"},
	{"parts/doc-standard/structure.md", "## Structure"},
	{"parts/maintainable-code-design/boundaries-and-dependencies.md", "## Boundaries and dependency direction"},
	{"parts/maintainable-code-design/contextual-heuristics.md", "## SOLID, DRY, and YAGNI"},
	{"parts/maintainable-code-design/decision-posture.md", "## Decision posture"},
	{"parts/maintainable-code-design/failure-modes.md", "## Failure modes"},
	{"parts/maintainable-code-design/pattern-toolbox.md", "## Illustrative pattern toolbox"},
	{"parts/maintainable-code-design/preparatory-refactoring.md", "## Preparatory refactoring"},
	{"parts/maintainable-code-design/readability.md", "## Readability"},
	{"parts/maintainable-code-design/semantic-modeling.md", "## Semantic modeling"},
	{"parts/plans-readme/naming.md", "## Naming & location"},
	{"parts/plans-readme/structure.md", "## What a plan contains"},
	{"parts/plans-template/header.md", "## Goal"},
	{"parts/plans-template/notes.md", "## Notes"},
	{"parts/plans-template/phases.md", "## Phase 1: <name>"},
	{"parts/plans-template/verification.md", "## Definition of done"},
	{"parts/workflow/chain.md", "## The chain"},
	{"parts/workflow/ci.md", "## Continuous integration"},
	{"parts/workflow/commit-discipline.md", "## Commit discipline"},
	{"parts/workflow/composing-the-gate.md", "## Composing the gate"},
	{"parts/workflow/doc-currency.md", "## Documentation currency"},
	{"parts/workflow/local-hooks.md", "## Local git hooks"},
	{"parts/workflow/principles.md", "## Principles"},
	{"parts/workflow/working-memory.md", "## Working memory"},
	{"parts/working-with-awf/commands.md", "## Commands"},
	{"parts/working-with-awf/config-and-overrides.md", "## Config and overrides"},
	{"parts/working-with-awf/overview.md", "# Working with awf"},
	{"parts/working-with-awf/placeholders.md", "## Placeholders in overrides"},
	{"parts/working-with-awf/sync-and-drift.md", "## Keeping in sync"},
	{"parts/working-with-awf/upgrading.md", "## Upgrading awf"},
	{"skills/parts/adr-lifecycle/amendment-until-terminal.md", "## Amendment-until-terminal"},
	{"skills/parts/adr-lifecycle/commit-templates.md", "## Commit subject templates"},
	{"skills/parts/adr-lifecycle/notes.md", "## Notes"},
	{"skills/parts/adr-lifecycle/state-changes.md", "## State changes and the claim handshake"},
	{"skills/parts/adr-lifecycle/states.md", "## The states"},
	{"skills/parts/adr-lifecycle/transitions.md", "## Transitions"},
	{"skills/parts/brainstorming/anti-patterns.md", "## Anti-patterns to avoid"},
	{"skills/parts/brainstorming/definitions.md", "## Definitions"},
	{"skills/parts/brainstorming/preamble.md", "# {{ .prefix }}-brainstorming"},
	{"skills/parts/brainstorming/procedure.md", "## Procedure"},
	{"skills/parts/brainstorming/when-to-invoke.md", "## When to invoke"},
	{"skills/parts/bugfix/oracle-note.md", "## Oracle invariant"},
	{"skills/parts/bugfix/test-tiers.md", "## Test tiers"},
	{"skills/parts/debugging/devdb-note.md", "## Dev environment note"},
	{"skills/parts/debugging/oracle-invariant.md", "## Oracle invariant"},
	{"skills/parts/debugging/red-flags.md", "## Red flags"},
	{"skills/parts/executing-plans/positioning.md", "# {{ .prefix }}-executing-plans"},
	{"skills/parts/executing-plans/procedure-resolve-plan.md", "## Procedure"},
	{"skills/parts/executing-plans/project-invariants.md", "## Notes"},
	{"skills/parts/executing-plans/red-flags.md", "## Red flags"},
	{"skills/parts/executing-plans/when-to-invoke.md", "## When to invoke"},
	{"skills/parts/exploring/boundaries.md", "## Boundaries"},
	{"skills/parts/exploring/breadth.md", "## Breadth"},
	{"skills/parts/exploring/detail.md", "## Detail"},
	{"skills/parts/exploring/dispatch.md", "## Dispatch"},
	{"skills/parts/exploring/results.md", "## Results"},
	{"skills/parts/exploring/when-to-invoke.md", "# {{ .prefix }}-exploring"},
	{"skills/parts/orienting/context-command.md", "## Managed context"},
	{"skills/parts/orienting/guide-ladder.md", "## Grounding ladder"},
	{"skills/parts/orienting/hand-off.md", "## Hand-off"},
	{"skills/parts/orienting/resume-revalidation.md", "## Resume revalidation"},
	{"skills/parts/orienting/when-to-invoke.md", "# {{ .prefix }}-orienting"},
	{"skills/parts/proposing-adr/conventions.md", "## Conventions enforced"},
	{"skills/parts/proposing-adr/notes.md", "## Notes"},
	{"skills/parts/proposing-adr/positioning.md", "# {{ .prefix }}-proposing-adr"},
	{"skills/parts/proposing-adr/when-to-invoke.md", "## When to invoke"},
	{"skills/parts/refactor-coupling-audit/audit-shape-selection.md", "## Procedure"},
	{"skills/parts/refactor-coupling-audit/category-1-top-level-files.md", "### Top-level package files"},
	{"skills/parts/refactor-coupling-audit/category-2-sibling-tests.md", "### Sibling test files"},
	{"skills/parts/refactor-coupling-audit/category-3-subpackages.md", "### Subpackages"},
	{"skills/parts/refactor-coupling-audit/category-4-codegen.md", "### Codegen emit sites"},
	{"skills/parts/refactor-coupling-audit/category-5-constructors.md", "### Constructor-init paths"},
	{"skills/parts/refactor-coupling-audit/category-6-init-visibility.md", "### Initialization ordering and cross-module visibility"},
	{"skills/parts/refactor-coupling-audit/notes.md", "## Notes"},
	{"skills/parts/refactor-coupling-audit/output-format.md", "## Output"},
	{"skills/parts/refactor-coupling-audit/scope-shrink-rule.md", "## Scope shrink rule"},
	{"skills/parts/refactor-coupling-audit/when-to-invoke.md", "## When to invoke"},
	{"skills/parts/retrospective/control.md", "## Control"},
	{"skills/parts/retrospective/notes.md", "## Notes"},
	{"skills/parts/retrospective/procedure.md", "## Procedure"},
	{"skills/parts/retrospective/promotion-ladder.md", "## The promotion ladder"},
	{"skills/parts/retrospective/recurrence-signal.md", "## Recurrence signal"},
	{"skills/parts/retrospective/when-fires.md", "## When this skill fires"},
	{"skills/parts/reviewing-adr/notes.md", "## Notes"},
	{"skills/parts/reviewing-adr/when-fires.md", "# {{ .prefix }}-reviewing-adr"},
	{"skills/parts/reviewing-impl/notes.md", "## Notes"},
	{"skills/parts/reviewing-impl/sha-range-detection.md", "## Procedure"},
	{"skills/parts/reviewing-impl/when-fires.md", "## When this skill fires"},
	{"skills/parts/reviewing-plan-resync/notes.md", "## Notes"},
	{"skills/parts/reviewing-plan-resync/when-fires.md", "# {{ .prefix }}-reviewing-plan-resync"},
	{"skills/parts/reviewing-plan/notes.md", "## Notes"},
	{"skills/parts/reviewing-plan/when-fires.md", "# {{ .prefix }}-reviewing-plan"},
	{"skills/parts/roadmap-graduation/doc-currency.md", "### 6. Doc-currency"},
	{"skills/parts/roadmap-graduation/explicit-drop.md", "### 5. Explicit drop"},
	{"skills/parts/roadmap-graduation/graduate-single-commit.md", "### 3. Confirm first effort creation"},
	{"skills/parts/roadmap-graduation/identify-entry.md", "### 1. Identify the roadmap entry"},
	{"skills/parts/roadmap-graduation/notes.md", "## Notes"},
	{"skills/parts/roadmap-graduation/reverify-measurements.md", "### 2. Re-verify cited measurements"},
	{"skills/parts/roadmap-graduation/when-fires.md", "## When this skill fires"},
	{"skills/parts/subagent-driven-development/notes.md", "## Notes"},
	{"skills/parts/subagent-driven-development/positioning.md", "# {{ .prefix }}-subagent-driven-development"},
	{"skills/parts/subagent-driven-development/procedure-resolve-plan.md", "## Procedure"},
	{"skills/parts/subagent-driven-development/red-flags.md", "## Red flags"},
	{"skills/parts/subagent-driven-development/when-to-invoke.md", "## When to invoke"},
	{"skills/parts/tdd/notes.md", "## Notes"},
	{"skills/parts/tdd/red-flags.md", "## Red flags"},
	{"skills/parts/tdd/surfaces.md", "## Pick the right test surface"},
	{"skills/parts/writing-plans/conventions-path.md", "## Conventions enforced"},
	{"skills/parts/writing-plans/plan-lifecycle.md", "## Notes"},
	{"skills/parts/writing-plans/positioning.md", "# {{ .prefix }}-writing-plans"},
	{"skills/parts/writing-plans/when-to-invoke.md", "## When to invoke"},
}

type structuralHeadingEdit struct {
	path            string
	source, updated []byte
	mode            os.FileMode
}
type structuralHeadingWriter func(string, []byte, os.FileMode) error

// applyStructuralHeadings removes only the exact legacy structural line. It
// performs complete preflight before replacing any file, so ambiguity leaves the
// tree untouched; each replacement itself is atomic and a retry is a no-op.
func applyStructuralHeadings(root string, out io.Writer) error {
	return applyStructuralHeadingsWithWriter(root, out, manifest.WriteFileAtomicMode)
}

func applyStructuralHeadingsWithWriter(root string, out io.Writer, write structuralHeadingWriter) error {
	var edits []structuralHeadingEdit
	for _, entry := range structuralHeadingSnapshot {
		pattern := filepath.Join(root, config.DirName, filepath.FromSlash(entry.path))
		paths := []string{pattern}
		if strings.ContainsAny(entry.path, "*?[") {
			var err error
			paths, err = filepath.Glob(pattern)
			if err != nil { // coverage-ignore: the frozen migration contains only compile-time-valid glob patterns
				return fmt.Errorf("expand structural-heading part pattern %s: %w", entry.path, err)
			}
		}
		for _, path := range paths {
			source, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil { // coverage-ignore: a path read immediately above can only fail here through an OS-specific directory or permission fault; migration callers surface it unchanged
				return fmt.Errorf("read structural-heading part %s: %w", path, err)
			}
			start := leadingStructuralOffset(source)
			remaining := source[start:]
			lineEnd := bytes.IndexByte(remaining, '\n')
			line := remaining
			if lineEnd >= 0 {
				line = remaining[:lineEnd]
			}
			if bytes.Equal(line, []byte(entry.heading)) {
				// A second adjacent ATX heading is ambiguous: it could be a custom
				// heading intended as body content or a second legacy structure.
				after := remaining[len(line):]
				if len(after) > 0 {
					after = after[1:]
				}
				if firstLineIsATX(after) {
					return structuralHeadingRefusal(root, path, entry.heading)
				}
				info, err := os.Stat(path)
				if err != nil { // coverage-ignore: the file was read in this preflight and no concurrent filesystem mutation is representable by migration input
					return fmt.Errorf("stat structural-heading part %s: %w", path, err)
				}
				updated := append([]byte{}, source[:start]...)
				updated = append(updated, after...)
				edits = append(edits, structuralHeadingEdit{path, source, updated, info.Mode().Perm()})
				continue
			}
			if firstLineIsATX(remaining) {
				return structuralHeadingRefusal(root, path, entry.heading)
			}
		}
	}
	for _, e := range edits {
		if err := write(e.path, e.updated, e.mode); err != nil {
			return fmt.Errorf("write structural-heading part %s: %w", e.path, err)
		}
		rel, err := filepath.Rel(root, e.path)
		if err != nil { // coverage-ignore: every edit path is constructed below root from a frozen relative path or its rooted glob matches
			return fmt.Errorf("relativize structural-heading part %s: %w", e.path, err)
		}
		if _, err := fmt.Fprintf(out, "structural-headings: updated %s\n", filepath.ToSlash(rel)); err != nil {
			return fmt.Errorf("announce structural-heading update for %s: %w", e.path, err)
		}
	}
	return nil
}

// leadingStructuralOffset skips only authoring comments, which are removed before
// part assembly and therefore cannot make a copied heading body content.
func leadingStructuralOffset(source []byte) int {
	offset := 0
	for {
		end := bytes.IndexByte(source[offset:], '\n')
		if end < 0 {
			return offset
		}
		end += offset
		line := strings.TrimSpace(string(source[offset:end]))
		if !strings.HasPrefix(line, "<!-- awf:comment ") || !strings.HasSuffix(line, " -->") {
			return offset
		}
		offset = end + 1
	}
}

func firstLineIsATX(source []byte) bool {
	if end := bytes.IndexByte(source, '\n'); end >= 0 {
		source = source[:end]
	}
	if len(source) < 2 || source[0] != '#' {
		return false
	}
	i := 0
	for i < len(source) && source[i] == '#' {
		i++
	}
	return i <= 6 && i < len(source) && (source[i] == ' ' || source[i] == '\t')
}

func structuralHeadingRefusal(root, path, heading string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil { // coverage-ignore: migration paths are rooted descendants, so Rel cannot fail for supported filesystem inputs
		rel = path
	}
	rel = filepath.ToSlash(rel)
	return fmt.Errorf("operation: structural-heading migration refuses because %s must begin with the exact removable heading %q or unambiguously body content; changed bytes: no; changed index: no; changed message: no; changed merge state: no; next actions: 1. edit %s so its leading heading is %q or body content 2. run `awf upgrade`", rel, heading, rel, heading)
}
