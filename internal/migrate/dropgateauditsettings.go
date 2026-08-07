package migrate

import (
	"bytes"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// applyDropGateAuditSettings ports schema 37 -> 38 by removing the retired
// gate, payload, audit, and fan-out settings directly from raw YAML.
func applyDropGateAuditSettings(root string, w *Changes) error {
	return editConfig(root, w, func(src []byte, planned *Changes) ([]byte, error) {
		out := src
		for _, removal := range gateAuditRetiredKeys {
			var next []byte
			var err error
			if removal.parent == "" {
				next, err = config.RemoveKey(out, removal.key)
			} else {
				next, err = config.RemoveMappingKey(out, removal.parent, removal.key)
			}
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(next, out) {
				if removal.parent == "" {
					planned.Add(fmt.Sprintf("drop-gate-audit-settings: removed %s\n", removal.key))
				} else {
					planned.Add(fmt.Sprintf("drop-gate-audit-settings: removed %s.%s\n", removal.parent, removal.key))
				}
			}
			out = next
		}
		return out, nil
	})
}

var gateAuditRetiredKeys = []struct{ parent, key string }{
	{"", "hooks"}, {"", "runner"},
	{"proseGate", "enabled"}, {"memoryCite", "enabled"},
	{"audit", "allowedTypes"}, {"audit", "subjectMaxLength"}, {"audit", "diffThreshold"},
	{"audit", "dependencyManifests"}, {"audit", "domainDocStaleness"}, {"audit", "domainCodeStaleness"},
	{"audit", "undocumentedDomain"}, {"audit", "plainPunctuation"}, {"audit", "uncommittedChanges"},
	{"currentState", "maxTopicsPerPath"},
}
