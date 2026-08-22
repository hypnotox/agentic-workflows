// Package configcheck owns repository check configuration consistency policy.
package configcheck

import (
	"errors"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

// ValidateCommandWiring preserves the command-spec validation error identity.
func ValidateCommandWiring(cfg *config.Config) error {
	value, _ := cfg.Vars["gateCmd"].(string)
	if strings.TrimSpace(value) == "" {
		return errors.New("rendered hook payloads require vars.gateCmd: set it in .awf/config.yaml")
	}
	return nil
}
