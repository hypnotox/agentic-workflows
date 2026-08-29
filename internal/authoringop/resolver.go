package authoringop

import (
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// ResolvePart delegates semantic kind, catalog/configured name, and capability
// resolution to the project and configuration authorities.
func ResolvePart(state *project.ProjectState, cfg *config.Config, kind, name, part string) (project.AuthoringTarget, error) {
	return project.ResolveAuthoringTarget(state, cfg, kind, name, part)
}
