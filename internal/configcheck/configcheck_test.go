package configcheck

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

func TestValidateCommandWiring(t *testing.T) {
	if err := ValidateCommandWiring(&config.Config{Vars: map[string]any{"gateCmd": " ./x gate "}}); err != nil {
		t.Fatal(err)
	}
	err := ValidateCommandWiring(&config.Config{})
	if err == nil || !strings.Contains(err.Error(), "vars.gateCmd") {
		t.Fatalf("error = %v", err)
	}
}
