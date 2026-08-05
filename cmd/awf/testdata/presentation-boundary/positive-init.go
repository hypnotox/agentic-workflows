package main

import "io"

func writeInitDescriptorProtocol(w io.Writer) { _, _ = w.Write(nil) }
