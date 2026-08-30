package migrate

import (
	"context"
	"errors"
	"fmt"
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
func retireWorkflowConfig(_ context.Context, tree *ProposedTree, changes *Changes) ([]FileMutation, error) {
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
		return nil, nil
	}
	for _, key := range removed {
		changes.Add("removed " + key + " from .awf/config.yaml")
	}
	return []FileMutation{{Path: configPath, Content: updated, Mode: mode}}, nil
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
