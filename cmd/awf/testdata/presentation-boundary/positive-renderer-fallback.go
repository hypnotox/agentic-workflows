package main

import "io"

func fixtureRendererFailure(w io.Writer, render func(io.Writer) error) {
	if err := render(w); err != nil {
		writeRendererFailure(w, err)
	}
}
