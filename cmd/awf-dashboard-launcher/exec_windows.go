//go:build windows

package main

import (
	"os"
	"os/exec"
)

// Windows has no exec-style process replacement. Run the verified child with
// inherited streams and return its status through the launcher.
func replaceProcess(path string, args []string) error {
	command := exec.Command(path, args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}
