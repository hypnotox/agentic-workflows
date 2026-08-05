package main

import (
	"errors"
	"io"
)

func writeOutcome(w io.Writer) {
	writeRendererFailure(w, errors.New("not a renderer failure"))
}
