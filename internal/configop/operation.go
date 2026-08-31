// Package configop owns static-versus-live configuration-reference orchestration.
package configop

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// LoadProject is the command-composed project loader constructor used for a
// live configuration reference.
type LoadProject func(string) (*project.Loader, error)

// Gate is the command-composed compatibility gate for a live project.
type Gate func(context.Context, string) error

// Run builds the static pre-adoption or live project configuration-reference
// document selected by root. Publisher retains the reference model and its
// semantic presentation mapping.
func Run(ctx context.Context, root, key string, loadProject LoadProject, gate Gate) (presentation.Document, error) {
	if _, err := os.Stat(config.ConfigPath(root)); err != nil {
		// Only a genuinely absent config means pre-adoption; any other stat
		// fault is an error state, not a reason to print the static reference.
		if !errors.Is(err, fs.ErrNotExist) {
			return presentation.Document{}, err
		}
		return publisher.ConfigReferencePresentation(key, nil, "config reference static (not inside an awf project)")
	}
	if err := gate(ctx, root); err != nil {
		return presentation.Document{}, err
	}
	loader, err := loadProject(root)
	if err != nil {
		return presentation.Document{}, err
	}
	session, err := loader.Load(ctx, root)
	if err != nil {
		return presentation.Document{}, err
	}
	model, err := publisher.New(session, project.Version).BuildConfigReference()
	if err != nil {
		return presentation.Document{}, err
	}
	return publisher.ConfigReferencePresentation(key, &model, "config reference live")
}
