package project

import (
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// gatedCommandsDisplay renders the gated-command list from the command spec.
func gatedCommandsDisplay() string {
	return strings.Join(backtick(clispec.GatedCommandNames()), ", ")
}

func backtick(names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = "`" + name + "`"
	}
	return out
}

// CapabilityError is a stable refusal produced before a Full-only handler runs.
type CapabilityError struct {
	Command string
	Profile catalog.Profile
}

func (e *CapabilityError) Error() string {
	return fmt.Sprintf("awf %s is unavailable in the selected %s governance footprint", e.Command, e.Profile)
}

func (e *CapabilityError) Diagnostic() (presentation.Diagnostic, error) {
	profile, err := presentation.Prose(string(e.Profile))
	if err != nil { // coverage-ignore: catalog.Profile is validated before capability dispatch
		return presentation.Diagnostic{}, err
	}
	field, err := presentation.NewField("selected governance footprint", profile)
	if err != nil { // coverage-ignore: the selected governance footprint and fixed field label are presentation-valid
		return presentation.Diagnostic{}, err
	}
	step, err := presentation.Literal("set profile: full in .awf/config.yaml and run awf render")
	if err != nil { // coverage-ignore: the fixed recovery instruction is a valid literal
		return presentation.Diagnostic{}, err
	}
	return presentation.Diagnostic{Condition: e.Error(), State: "configuration", Changed: []presentation.Field{field}, Cause: "the command requires the Full governance footprint", Steps: []presentation.Value{step}}, nil
}

// RequireCapability refuses a declared Full-only command under Core.
func RequireCapability(profile catalog.Profile, command string, fullOnly bool) error {
	if fullOnly && profile == catalog.ProfileCore {
		return &CapabilityError{Command: command, Profile: profile}
	}
	return nil
}
