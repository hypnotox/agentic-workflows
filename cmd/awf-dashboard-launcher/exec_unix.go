//go:build !windows

package main

import (
	"os"
	"syscall"
)

func replaceProcess(path string, args []string) error {
	return syscall.Exec(path, append([]string{path}, args...), os.Environ())
}
