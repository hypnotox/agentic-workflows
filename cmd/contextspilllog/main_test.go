//go:build linux || darwin

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
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
	logged, err := os.ReadFile(filepath.Join(root, ".awf", "local", "context-spills.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), "bytes=9000\tinvocation='./x' 'context' 'internal/project'") || strings.Contains(string(logged), spill) {
		t.Fatalf("unexpected log: %q", logged)
	}
}

func TestRunSafeLogAdvisory(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, ".awf", "local")
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
	if status := run([]string{"--check-log", "--root", root}, &stdout, &stderr); status != 0 || !strings.Contains(stderr.String(), "resolve or promote") {
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

func TestRunUnrecognizedIsSilent(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "capture")
	if err := os.WriteFile(capture, []byte("ordinary context\n"), 0o600); err != nil {
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
	stdout.Reset()
	stderr.Reset()
	if status := run([]string{"--root", t.TempDir(), "--notice-file", validCapture, "--", "./x"}, &stdout, &stderr); status == 0 {
		t.Fatal("logging against missing .awf must fail")
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
