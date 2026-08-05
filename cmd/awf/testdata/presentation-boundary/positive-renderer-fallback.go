package main

import (
	"io"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func writeOutcome(w io.Writer, document presentation.Document) {
	if err := presentation.Render(w, document); err != nil {
		writeRendererFailure(w, err)
	}
}
