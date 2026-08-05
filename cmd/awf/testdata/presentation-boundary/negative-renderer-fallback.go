package main

import (
	"errors"
	"io"
)

func writeUnexpectedFallback(w io.Writer) { writeRendererFailure(w, errors.New("not a renderer failure")) }
