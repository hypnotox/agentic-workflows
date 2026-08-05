package main

import "io"

func fixtureAlternateRenderer(w io.Writer) { _, _ = io.WriteString(w, "status: alternate\n") }
