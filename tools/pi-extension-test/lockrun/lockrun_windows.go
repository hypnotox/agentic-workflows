//go:build windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "pi-extension-test: lockrun requires Unix advisory locks and is unsupported on Windows")
	os.Exit(1)
}
