package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestRunStaleCleanup(t *testing.T) {
	var stdout, stderr bytes.Buffer
	called := false
	code := run(nil, &stdout, &stderr, func(mode testsupport.CleanupMode, output io.Writer) error {
		called = true
		if mode != testsupport.CleanupStale {
			t.Fatalf("mode = %v", mode)
		}
		_, _ = io.WriteString(output, "manager summary\n")
		return nil
	})
	if code != 0 || !called || stdout.String() != "manager summary\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d called=%t stdout=%q stderr=%q", code, called, stdout.String(), stderr.String())
	}
}

func TestRunAllWarnsBeforeCleanup(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--all"}, &stdout, &stderr, func(mode testsupport.CleanupMode, output io.Writer) error {
		if mode != testsupport.CleanupAll {
			t.Fatalf("mode = %v", mode)
		}
		if got := stderr.String(); got != "testtmpclean: warning: --all can remove homes used by concurrent test processes\n" {
			t.Fatalf("warning before cleanup = %q", got)
		}
		_, _ = io.WriteString(output, "manager summary\n")
		return nil
	})
	if code != 0 || stdout.String() != "manager summary\n" {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"unexpected"}, {"--all", "unexpected"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			called := false
			code := run(args, &stdout, &stderr, func(testsupport.CleanupMode, io.Writer) error { called = true; return nil })
			if code != 2 || called || stdout.Len() != 0 || stderr.String() != "usage: testtmpclean [--all]\n" {
				t.Fatalf("code=%d called=%t stdout=%q stderr=%q", code, called, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunCleanupFailures(t *testing.T) {
	failure := errors.New("cannot clean")
	for _, tc := range []struct {
		name        string
		writeOutput bool
	}{
		{"partial", true},
		{"root", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(nil, &stdout, &stderr, func(_ testsupport.CleanupMode, output io.Writer) error {
				if tc.writeOutput {
					_, _ = io.WriteString(output, "manager summary\n")
				}
				return failure
			})
			if code != 1 || stdout.String() != map[bool]string{true: "manager summary\n", false: ""}[tc.writeOutput] || stderr.String() != "testtmpclean: cannot clean\n" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRepositoryCleanupCommandContract(t *testing.T) {
	root := repositoryRoot(t)
	x, err := os.ReadFile(filepath.Join(root, "x"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"clean-test-tmp)", "go run ./cmd/testtmpclean \"$@\"", "clean-test-tmp [--all]"} {
		if !strings.Contains(string(x), want) {
			t.Errorf("x missing %q", want)
		}
	}
	for _, path := range []string{filepath.Join(root, "cmd", "testtmpclean", "main.go"), filepath.Join(root, "cmd", "testtmpclean", "main_test.go")} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		forbidden := "test temp " + "cleanup:"
		if strings.Contains(string(contents), forbidden) {
			t.Errorf("%s duplicates manager-owned summary", path)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
