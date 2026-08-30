package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	retireWorkflowConfigName = "retire-workflow-profile-and-vars"
	workflowConfigGeneration = 49
)

var retiredWorkflowVars = []string{
	"activeMdRegenCmd",
	"commitGateCmd",
	"gateCmdFull",
	"invariantTestPath",
}

// retireWorkflowConfig removes the profile selector and retired, unset workflow
// variables. A non-empty retired variable is an adopter-owned command or path,
// so the migration refuses to discard it without an explicit reconciliation.
func retireWorkflowConfig(ctx context.Context, tree *ProposedTree, changes *Changes) ([]FileMutation, error) {
	configPath := config.DirName + "/config.yaml"
	source, mode, err := tree.Read(configPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", configPath, err)
	}
	updated, removed, err := retireWorkflowConfigBytes(source)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}
	if len(removed) == 0 {
		return retireWorkflowArtifacts(ctx, tree, changes)
	}
	for _, key := range removed {
		changes.Add("removed " + key + " from .awf/config.yaml")
	}
	artifacts, err := retireWorkflowArtifacts(ctx, tree, changes)
	if err != nil {
		return nil, err
	}
	return append([]FileMutation{{Path: configPath, Content: updated, Mode: mode}}, artifacts...), nil
}

func retireWorkflowConfigBytes(source []byte) ([]byte, []string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(source, &document); err != nil {
		return nil, nil, err
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil, nil, errors.New("expected a top-level mapping")
	}
	mapping := document.Content[0]
	profile := mappingValue(mapping, "profile")
	if profile != nil && (profile.Kind != yaml.ScalarNode || (profile.Value != "core" && profile.Value != "full")) {
		return nil, nil, fmt.Errorf("profile must be core or full before migration, got %q", profile.Value)
	}
	vars := mappingValue(mapping, "vars")
	if vars != nil && vars.Kind != yaml.MappingNode {
		return nil, nil, errors.New("vars must be a mapping")
	}
	for _, key := range retiredWorkflowVars {
		value := mappingValue(vars, key)
		if value != nil && meaningfulRetiredValue(value) {
			return nil, nil, fmt.Errorf("vars.%s has a meaningful retired override; remove it after reconciling its behavior, then retry upgrade", key)
		}
	}

	removed := make([]string, 0, len(retiredWorkflowVars)+1)
	for _, key := range retiredWorkflowVars {
		if removeMappingEntry(vars, key) {
			removed = append(removed, "vars."+key)
		}
	}
	if removeMappingEntry(mapping, "profile") {
		removed = append(removed, "profile")
	}
	if len(removed) == 0 {
		return append([]byte(nil), source...), nil, nil
	}
	updated, err := yaml.Marshal(&document)
	if err != nil {
		return nil, nil, err
	}
	return updated, removed, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func removeMappingEntry(mapping *yaml.Node, key string) bool {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return true
		}
	}
	return false
}

func meaningfulRetiredValue(value *yaml.Node) bool {
	if value.Tag == "!!null" {
		return false
	}
	return value.Kind != yaml.ScalarNode || strings.TrimSpace(value.Value) != ""
}

type retiredWorkflowArtifact struct {
	sections map[string]bool
}

func retiredSections(names ...string) retiredWorkflowArtifact {
	sections := make(map[string]bool, len(names))
	for _, name := range names {
		sections[name] = true
	}
	return retiredWorkflowArtifact{sections: sections}
}

// retiredWorkflowArtifacts is the frozen schema-48 artifact and section set.
// The migration must not consult the live catalog, whose membership can evolve.
var retiredWorkflowArtifacts = map[string]map[string]retiredWorkflowArtifact{
	"skills": {
		"grounding":               retiredSections("invocation", "brief-construction-and-dispatch", "finding-classification", "boundaries", "notes"),
		"executing-direct":        retiredSections(),
		"writing-docs":            retiredSections("procedure"),
		"tdd":                     retiredSections("surfaces", "notes", "red-flags"),
		"exploring":               retiredSections("when-to-invoke", "breadth", "detail", "dispatch", "results", "boundaries", "notes"),
		"orienting":               retiredSections("when-to-invoke", "guide-ladder", "context-command", "resume-revalidation", "hand-off"),
		"proposing-adr":           retiredSections("positioning", "when-to-invoke", "conventions", "procedure-number", "procedure-write", "state-doc-update", "procedure-state-changes", "procedure-regen", "procedure-commit", "autonomous-rule", "terminal-step", "notes"),
		"adr-lifecycle":           retiredSections("states", "transitions", "state-changes", "procedure-status-edit", "procedure-claim-mutation", "state-doc-update", "procedure-regen", "procedure-gate", "commit-templates", "amendment-until-terminal", "notes"),
		"bugfix":                  retiredSections("test-tiers", "pitfalls-check", "oracle-note", "memory-checkpoint"),
		"reviewing-adr":           retiredSections("when-fires", "procedure", "artifact-path-detection", "dispatch-subagent", "classify-route-findings", "apply-fixes-commit", "re-review-loop", "status-flip", "hand-off-to-plan-review", "notes"),
		"reviewing-impl":          retiredSections("when-fires", "sha-range-detection", "dispatch-subagent", "classify-route-findings", "apply-fixes-commit", "run-audit", "re-review-loop", "hand-off", "notes"),
		"retrospective":           retiredSections("when-fires", "procedure", "recurrence-signal", "promotion-ladder", "control", "notes"),
		"refactor-coupling-audit": retiredSections("when-to-invoke", "audit-shape-selection", "category-1-top-level-files", "category-2-sibling-tests", "category-3-subpackages", "category-4-codegen", "category-5-constructors", "category-6-init-visibility", "test-coupling-planning-rule", "output-format", "scope-shrink-rule", "notes"),
		"roadmap-graduation":      retiredSections("when-fires", "failure-modes", "identify-entry", "reverify-measurements", "graduate-single-commit", "explicit-drop", "same-commit", "doc-currency", "notes"),
	},
	"agents": {
		"adr-reviewer":      retiredSections("universal-lenses", "project-focus"),
		"code-reviewer":     retiredSections("universal-lenses", "project-focus", "doc-currency"),
		"grounding-checker": retiredSections("identity", "verification-scope", "return-schema"),
	},
}

// retiredWorkflowPartSections records schema-48 sections removed from artifact
// identities that remain live under a new contract. Their sidecars remain live;
// only overrides at these retired section paths are migration-owned.
var retiredWorkflowPartSections = map[string]map[string]retiredWorkflowArtifact{
	"skills": {
		"brainstorming": retiredSections("preamble", "when-to-invoke", "example-clarifying-questions", "design-sections", "no-spec-rule", "terminal-step", "definitions", "anti-patterns"),
		"using-awf":     retiredSections("procedure"),
		"debugging":     retiredSections("symptom-list", "debugging-surfaces", "test-isolation", "oracle-invariant", "devdb-note", "red-flags", "memory-checkpoint"),
	},
	"agents": {
		"explorer":    retiredSections("identity", "single-need", "breadth", "report-detail", "grounding-and-outcomes", "report-discipline"),
		"implementer": retiredSections("identity", "task-scope", "guide-authority", "green-obligation", "escalation", "return-schema"),
	},
}

// retireWorkflowArtifacts removes only overrides proven equivalent to the
// schema-48 defaults. Meaningful or unrecognized state refuses before upgrade
// can discard adopter-owned bytes.
func retireWorkflowArtifacts(_ context.Context, tree *ProposedTree, changes *Changes) ([]FileMutation, error) {
	var mutations []FileMutation
	for _, kind := range []string{"agents", "skills"} {
		names := retiredWorkflowArtifacts[kind]
		orderedNames := make([]string, 0, len(names))
		for name := range names {
			orderedNames = append(orderedNames, name)
		}
		slices.Sort(orderedNames)
		for _, name := range orderedNames {
			artifact := names[name]
			sidecar := config.DirName + "/" + kind + "/" + name + ".yaml"
			if source, _, err := tree.Read(sidecar); err == nil {
				if err := validateDefaultRetiredSidecar(source, artifact); err != nil {
					return nil, fmt.Errorf("%s has a meaningful retired override: %w; reconcile it before upgrade", sidecar, err)
				}
				mutations = append(mutations, FileMutation{Path: sidecar, Remove: true})
				changes.Add("removed default retired " + sidecar)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("read %s: %w", sidecar, err)
			}

			partsRoot := config.DirName + "/" + kind + "/parts/" + name
			paths, err := tree.Paths(partsRoot)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("enumerate %s: %w", partsRoot, err)
			}
			for _, sourcePath := range paths {
				section, ok := retiredPartSection(partsRoot, sourcePath, artifact.sections)
				if !ok {
					return nil, fmt.Errorf("%s is not a recognized schema-48 retired section part; reconcile it before upgrade", sourcePath)
				}
				source, _, err := tree.Read(sourcePath)
				if err != nil {
					return nil, fmt.Errorf("read %s: %w", sourcePath, err)
				}
				if strings.TrimSpace(string(source)) != "{{=awf:sectionDefault}}" {
					return nil, fmt.Errorf("%s has a meaningful retired part replacement for section %s; reconcile it before upgrade", sourcePath, section)
				}
				mutations = append(mutations, FileMutation{Path: sourcePath, Remove: true})
				changes.Add("removed default retired " + sourcePath)
			}
		}
	}
	for _, kind := range []string{"agents", "skills"} {
		names := retiredWorkflowPartSections[kind]
		orderedNames := make([]string, 0, len(names))
		for name := range names {
			orderedNames = append(orderedNames, name)
		}
		slices.Sort(orderedNames)
		for _, name := range orderedNames {
			artifact := names[name]
			partsRoot := config.DirName + "/" + kind + "/parts/" + name
			paths, err := tree.Paths(partsRoot)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("enumerate %s: %w", partsRoot, err)
			}
			for _, sourcePath := range paths {
				section, ok := retiredPartSection(partsRoot, sourcePath, artifact.sections)
				if !ok {
					continue
				}
				source, _, err := tree.Read(sourcePath)
				if err != nil {
					return nil, fmt.Errorf("read %s: %w", sourcePath, err)
				}
				if strings.TrimSpace(string(source)) != "{{=awf:sectionDefault}}" {
					return nil, fmt.Errorf("%s has a meaningful retired part replacement for section %s; reconcile it before upgrade", sourcePath, section)
				}
				mutations = append(mutations, FileMutation{Path: sourcePath, Remove: true})
				changes.Add("removed default retired " + sourcePath)
			}
		}
	}
	slices.SortFunc(mutations, func(a, b FileMutation) int { return strings.Compare(a.Path, b.Path) })
	return mutations, nil
}

func validateDefaultRetiredSidecar(source []byte, artifact retiredWorkflowArtifact) error {
	var sidecar config.Sidecar
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	decoder.KnownFields(true)
	if err := decoder.Decode(&sidecar); err != nil {
		return err
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple YAML documents")
		}
		return err
	}
	if len(sidecar.Data) != 0 {
		return errors.New("data changes rendered defaults")
	}
	for key, keep := range sidecar.DataDefaults {
		if !keep {
			return fmt.Errorf("dataDefaults.%s disables a rendered default", key)
		}
	}
	for section, override := range sidecar.Sections {
		if !artifact.sections[section] {
			return fmt.Errorf("sections.%s is not a schema-48 section", section)
		}
		if override.Drop {
			return fmt.Errorf("sections.%s.drop removes rendered content", section)
		}
	}
	if len(sidecar.Paths) != 0 {
		return errors.New("paths declares adopter-owned territory")
	}
	return nil
}

func retiredPartSection(root, path string, sections map[string]bool) (string, bool) {
	rel := strings.TrimPrefix(path, root+"/")
	if rel == path || strings.Contains(rel, "/") || filepath.Ext(rel) != ".md" {
		return "", false
	}
	section := strings.TrimSuffix(rel, ".md")
	return section, sections[section]
}
