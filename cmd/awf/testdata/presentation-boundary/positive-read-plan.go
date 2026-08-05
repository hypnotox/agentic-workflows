package main

import "io"

func runReadPlan(w io.Writer) { _, _ = w.Write(nil) }
