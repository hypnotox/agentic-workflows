package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// runConfig prints the configuration reference: full or single-entry, with
// live state inside an adopted tree and a static catalog-wide fallback
// outside one (pre-adoption discovery is a supported audience). Rendering
// the typed model - live or static - lives in internal/project, the package
// that owns the model.
func runConfig(ctx context.Context, cwd, key string, stdout io.Writer) error {
	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
		// Only a genuinely absent config means pre-adoption; any other stat
		// fault (permissions, a file where .awf should be) is an error state,
		// not a reason to silently print the static reference.
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		document, presentationErr := publisher.ConfigReferencePresentation(key, nil, "config reference static (not inside an awf project)")
		if presentationErr != nil {
			return presentationErr
		}
		return presentation.Render(stdout, document)
	}
	if err := gate(ctx, cwd); err != nil {
		return err
	}
	state, cfg, _, err := openProjectOperation(ctx, cwd)
	if err != nil {
		return err
	}
	model, err := composePublisher(state, cfg).BuildConfigReference()
	if err != nil {
		return err
	}
	document, presentationErr := publisher.ConfigReferencePresentation(key, &model, "config reference live")
	if presentationErr != nil {
		return presentationErr
	}
	return presentation.Render(stdout, document)
}
