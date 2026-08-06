package main

import "io"

func writeEffortMemoryProtocol(w io.Writer) { _, _ = w.Write(nil) }
