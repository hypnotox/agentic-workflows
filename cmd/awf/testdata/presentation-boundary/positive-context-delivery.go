package contextdelivery

import "io"

func Deliver(w io.Writer) { _, _ = w.Write(nil) }
