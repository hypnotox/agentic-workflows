//go:build unix

// lockrun runs one command while holding a checkout-local advisory lock.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() { os.Exit(run(os.Args)) }

func run(args []string) int {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: lockrun <lock-file> <command> [args...]")
		return 2
	}
	lock, err := os.OpenFile(args[1], os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pi-extension-test: open lock: %v\n", err)
		return 1
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		fmt.Fprintf(os.Stderr, "pi-extension-test: acquire lock: %v\n", err)
		return 1
	}

	cmd := exec.Command(args[2], args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{lock} // fd 3 stays open across descendant execs.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "pi-extension-test: start worker: %v\n", err)
		return 1
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	go func() {
		for sig := range signals {
			_ = syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal))
		}
	}()
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				return 128 + int(status.Signal())
			}
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "pi-extension-test: wait for worker: %v\n", err)
		return 1
	}
	return 0
}
