package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnrelatedCommandsDoNotRequireOrChangeVersionRecord(t *testing.T) {
	for _, record := range []string{"", "0.0.0\n", "not configuration\n"} {
		t.Run(record, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PATH", t.TempDir())
			path := filepath.Join(root, ".awf", "VERSION")
			if record != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, args := range [][]string{
				{"docs"}, {"docs", "agents"}, {"docs", "changes"}, {"docs", "completion"}, {"version"},
				{"new", "intent", "test"}, {"new", "spec", "test"}, {"new", "plan", "test"}, {"new", "adr", "test"},
				{"new", "topic", "test", "src/**"}, {"resolve", "src/future.go"}, {"resolve", "--coverage", "src/future.go"},
				{"new", "effort", "test"}, {"effort", "list"}, {"effort", "show", "test"}, {"effort", "finish", "test"},
			} {
				if code, _, stderr := runCLI(t, root, args...); code != 0 || stderr != "" {
					t.Errorf("%v with record %q = code %d, stderr %q", args, record, code, stderr)
				}
			}
			got, err := os.ReadFile(path)
			if record == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("unrelated command created version record: %q, %v", got, err)
				}
			} else if err != nil || !bytes.Equal(got, []byte(record)) {
				t.Fatalf("unrelated command changed version record: %q, %v", got, err)
			}
		})
	}
}

func TestCheckWithoutInstallationReportsMissingOutputs(t *testing.T) {
	code, stdout, stderr := runCLI(t, t.TempDir(), "check")
	if code != 1 || stderr != "" || !strings.Contains(stdout, ".awf/VERSION: missing generated file") {
		t.Fatalf("uninstalled check = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
