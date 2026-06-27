package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func TestRunVersion(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "version"}, &out, &errb); code != 0 {
		t.Fatalf("version exited %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "awf ") || !strings.Contains(out.String(), project.Version) {
		t.Errorf("version output = %q, want it to contain %q", out.String(), project.Version)
	}
}
