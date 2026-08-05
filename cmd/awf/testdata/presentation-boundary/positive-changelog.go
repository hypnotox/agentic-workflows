package main

import "io"

func writeChangelogPayload(w io.Writer) { _, _ = io.WriteString(w, "payload") }
