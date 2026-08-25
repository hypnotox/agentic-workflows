package audit

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// historicalLock is audit-owned evidence. It intentionally retains only the
// schema needed to select historical audit behavior; unknown historical fields
// are evidence, not live authority.
type historicalLock struct{ SchemaVersion int }

func decodeHistoricalLock(src []byte) (*historicalLock, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(src, &raw); err != nil {
		return nil, fmt.Errorf("parse historical lock: %w", err)
	}
	var lock historicalLock
	if err := json.Unmarshal(src, &lock); err != nil {
		return nil, fmt.Errorf("parse historical lock: %w", err)
	}
	return &lock, nil
}

// decodeHistoricalConfig owns the byte adaptation required to inspect the
// represented audit horizon. It is deliberately separate from live migration:
// it changes an in-memory historical projection only and never writes a tree.
func decodeHistoricalConfig(src []byte, schema int) ([]byte, error) {
	var value map[string]any
	if err := yaml.Unmarshal(src, &value); err != nil {
		return nil, fmt.Errorf("parse historical config: %w", err)
	}
	if value == nil {
		return nil, fmt.Errorf("parse historical config: expected mapping")
	}
	for _, key := range []string{"invariants", "workflowTelemetry", "hooks", "runner", "skills", "agents", "docs", "targets", "docsDir"} {
		delete(value, key)
	}
	deleteNested(value, "audit", "baseBranch", "allowedTypes", "subjectMaxLength", "diffThreshold", "dependencyManifests", "domainDocStaleness", "domainCodeStaleness", "undocumentedDomain", "plainPunctuation", "uncommittedChanges")
	deleteNested(value, "currentState", "topicCoverage", "topicFanout", "maxClaimsPerTopic", "maxTopicsPerPath")
	deleteNested(value, "proseGate", "enabled")
	deleteNested(value, "memoryCite", "enabled")
	// These facts were materialized by historical migrations before today's
	// strict semantic parser consumed the tree.
	if schema < 30 && emptyValue(value["integrationBranch"]) {
		value["integrationBranch"] = "main"
	}
	if schema < 46 && emptyValue(value["profile"]) {
		value["profile"] = "full"
	}
	out, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal historical config: %w", err)
	}
	return out, nil
}

func deleteNested(value map[string]any, parent string, keys ...string) {
	child, ok := value[parent].(map[string]any)
	if !ok {
		return
	}
	for _, key := range keys {
		delete(child, key)
	}
}

func emptyValue(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	return ok && text == ""
}
