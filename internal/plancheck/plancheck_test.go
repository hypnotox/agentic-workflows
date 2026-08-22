package plancheck

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

func TestDisabledValidity(t *testing.T) {
	result, err := Validity(nil, adr.Corpus{}, nil, false)
	if err != nil || len(result.Findings()) != 0 {
		t.Fatalf("disabled validity = %#v, %v", result.Findings(), err)
	}
}
