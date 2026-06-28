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

func TestAwfVersionLdflagsPrecedence(t *testing.T) {
	t.Cleanup(func() { version = "" })
	version = "v9.9.9-test"
	if got := awfVersion(); got != "v9.9.9-test" {
		t.Errorf("awfVersion() = %q, want the ldflags-injected %q", got, "v9.9.9-test")
	}
}
