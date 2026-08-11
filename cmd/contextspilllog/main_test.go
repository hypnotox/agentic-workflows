//go:build linux || darwin

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	root := t.TempDir()
	capture := filepath.Join(t.TempDir(), "capture")
	spill := filepath.Join(t.TempDir(), "spill")
	if err := os.WriteFile(spill, []byte("spill"), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := "AWF_CONTEXT_SPILL_V1 bytes=9000 format=text\n" + spill + "\n"
	if err := os.WriteFile(capture, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"--root", root, "--notice-file", capture, "--", "./x", "context", "internal/project"}
	if status := run(args, &stdout, &stderr); status != 0 {
		t.Fatalf("run(valid) = %d, stderr %q", status, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("valid output stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	logged, err := os.ReadFile(filepath.Join(root, ".cache", "awf-context", "context-spills.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), "bytes=9000\tinvocation='./x' 'context' 'internal/project'") || strings.Contains(string(logged), spill) {
		t.Fatalf("unexpected log: %q", logged)
	}
}

func TestRunSafeLogAdvisory(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, ".cache", "awf-context")
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--check-log", "--root", root}, &stdout, &stderr); status != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("empty status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if err := os.WriteFile(filepath.Join(local, "context-spills.log"), []byte("record\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status := run([]string{"--check-log", "--root", root}, &stdout, &stderr); status != 0 || !strings.Contains(stderr.String(), ".cache/awf-context/context-spills.log") || strings.Contains(stderr.String(), ".awf/local/context-spills.log") {
		t.Fatalf("nonempty status=%d stderr=%q", status, stderr.String())
	}
	if err := os.Chmod(filepath.Join(local, "context-spills.log"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if status := run([]string{"--check-log", "--root", root}, &stdout, &stderr); status == 0 || !strings.Contains(stderr.String(), "want 0600") {
		t.Fatalf("unsafe status=%d stderr=%q", status, stderr.String())
	}
}

// TestXContextConsumerUsesOnlySpillNoticeProtocol executes the checked-in x
// runner with controlled child and logger fixtures. The malformed case proves
// the observation instrument can fail without changing the child status.
func TestXContextConsumerUsesOnlySpillNoticeProtocol(t *testing.T) {
	root := t.TempDir()
	body, err := os.ReadFile(filepath.Join("..", "..", "x"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "x"), body, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "awf"), []byte("#!/bin/sh\nprintf '%s' \"$AWF_CONTEXT_FIXTURE\"\nexit \"$AWF_CONTEXT_STATUS\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logger := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$AWF_GO_LOG\"\n" +
		"capture=; root=; previous=\n" +
		"for arg in \"$@\"; do if [ \"$previous\" = --notice-file ]; then capture=$arg; fi; if [ \"$previous\" = --root ]; then root=$arg; fi; previous=$arg; done\n" +
		"if grep -q '^AWF_CONTEXT_SPILL_V1 bytes=[0-9][0-9]* format=text$' \"$capture\"; then mkdir -p \"$root/.cache/awf-context\"; printf 'observed\\n' > \"$root/.cache/awf-context/context-spills.log\"; exit 0; fi\n" +
		"if grep -q '^AWF_CONTEXT_SPILL_V1' \"$capture\"; then printf 'malformed notice\\n' >&2; exit 1; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(logger), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, fixture, wantErr, wantLog string
		status                          int
	}{
		{"ordinary", "context: static\n", "", "", 0},
		{"spill", "AWF_CONTEXT_SPILL_V1 bytes=9 format=text\n/tmp/spill\n", "", "observed\n", 0},
		{"malformed", "AWF_CONTEXT_SPILL_V1 bytes=x format=text\n/tmp/spill\n", "malformed notice\ncontext: warning: spill delivered but local observability logging failed\n", "", 0},
		{"child-failure", "child failed\n", "", "", 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.RemoveAll(filepath.Join(root, ".cache")); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(t.TempDir(), "go.log")
			cmd := exec.Command("bash", "./x", "context", "internal/project")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "AWF_CONTEXT_FIXTURE="+tc.fixture, "AWF_CONTEXT_STATUS="+strconv.Itoa(tc.status), "AWF_GO_LOG="+logPath)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			status := 0
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				status = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if status != tc.status || stdout.String() != tc.fixture || stderr.String() != tc.wantErr {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
			logged, _ := os.ReadFile(filepath.Join(root, ".cache", "awf-context", "context-spills.log"))
			if string(logged) != tc.wantLog {
				t.Fatalf("spill log=%q, want %q", logged, tc.wantLog)
			}
			invocations, _ := os.ReadFile(logPath)
			if (tc.status == 0) != (len(invocations) > 0) {
				t.Fatalf("logger invocation=%q", invocations)
			}
		})
	}
}

func TestRunUnrecognizedIsSilent(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "capture")
	// This is ordinary presentation output, not a protocol sentinel. The x
	// consumer must leave it alone even though it starts with the context label.
	const ordinaryContext = "context: static not inside an awf project\n\nrequests:\n  status: none\n"
	if err := os.WriteFile(capture, []byte(ordinaryContext), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--root", t.TempDir(), "--notice-file", capture, "--", "./x", "context"}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, stderr %q", status, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunErrors(t *testing.T) {
	oversized := filepath.Join(t.TempDir(), "oversized")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte("x"), maxNoticeBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"grammar":      {"--root", "missing"},
		"missing file": {"--root", t.TempDir(), "--notice-file", filepath.Join(t.TempDir(), "missing"), "--", "./x"},
		"read error":   {"--root", t.TempDir(), "--notice-file", t.TempDir(), "--", "./x"},
		"oversized":    {"--root", t.TempDir(), "--notice-file", oversized, "--", "./x"},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if status := run(args, &stdout, &stderr); status == 0 {
				t.Fatal("expected nonzero status")
			}
			if stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}

	capture := filepath.Join(t.TempDir(), "capture")
	if err := os.WriteFile(capture, []byte("AWF_CONTEXT_SPILL_V1 bytes=x format=text\n/tmp/a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--root", t.TempDir(), "--notice-file", capture, "--", "./x"}, &stdout, &stderr); status == 0 {
		t.Fatal("recognized invalid notice must fail")
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	validCapture := filepath.Join(t.TempDir(), "capture")
	spill := filepath.Join(t.TempDir(), "spill")
	if err := os.WriteFile(validCapture, []byte("AWF_CONTEXT_SPILL_V1 bytes=1 format=text\n"+spill+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsafeRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(unsafeRoot, ".cache"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if status := run([]string{"--root", unsafeRoot, "--notice-file", validCapture, "--", "./x"}, &stdout, &stderr); status == 0 {
		t.Fatal("logging against unsafe cache must fail")
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
