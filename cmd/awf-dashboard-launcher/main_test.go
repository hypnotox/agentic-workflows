package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLauncherClosedGrammar(t *testing.T) {
	allowed := [][]string{
		{"metrics", "protocol", "--json"},
		{"metrics", "--json"},
		{"metrics", "--json", "--effort", "e", "--session", "s", "--phase", "implementation", "--since", "2026-01-01T00:00:00Z", "--until", "2026-01-02T00:00:00Z"},
		{"metrics", "export", "--format", "json", "--phase", "implementation"},
		{"doctor", "--json", "--session", "s"},
	}
	for _, args := range allowed {
		if err := validatePublicQuery(args); err != nil {
			t.Errorf("allowed %v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{}, {"metrics", "protocol"}, {"metrics", "--json", "--session", "s", "--effort", "e"},
		{"metrics", "--json", "--effort", "e", "--effort", "e2"}, {"metrics", "export", "--format", "jsonl"},
		{"metrics", "retain", "--json"}, {"metrics", "purge", "--effort", "e", "--confirm"},
		{"metrics", "lifecycle", "--request", "-"}, {"doctor", "--json", "value"}, {"doctor", "-j"},
	} {
		if err := validatePublicQuery(args); err == nil {
			t.Errorf("forbidden argv accepted: %v", args)
		}
	}
}

func TestLauncherVerifiesAndTranslates(t *testing.T) {
	directory := t.TempDir()
	launcher := filepath.Join(directory, executableName("awf-dashboard"))
	awf := filepath.Join(directory, executableName("awf"))
	policy := filepath.Join(directory, "policy.json")
	for path, content := range map[string]string{launcher: "launcher", awf: "awf", policy: "policy"} {
		if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	metadata := runtimeMetadata{FormatVersion: 1, RepositoryID: "/repo", ObjectFormat: "sha1", Commit: strings.Repeat("a", 40), GoVersion: "go", GOOS: "linux", GOARCH: "amd64", GoFlags: []string{"-buildvcs=false"}, PolicySchema: 1, ProtocolMajor: 2}
	metadata.LauncherSHA256, _ = fileDigest(launcher)
	metadata.BinarySHA256, _ = fileDigest(awf)
	metadata.PolicySHA256, _ = fileDigest(policy)
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	oldExecutable, oldExec, oldReadFile := launcherExecutable, launcherExec, launcherReadFile
	t.Cleanup(func() { launcherExecutable, launcherExec, launcherReadFile = oldExecutable, oldExec, oldReadFile })
	launcherExecutable = func() (string, error) { return launcher, nil }
	var gotPath string
	var gotArgs []string
	launcherExec = func(path string, args []string) error {
		gotPath, gotArgs = path, append([]string(nil), args...)
		return nil
	}
	root := filepath.Join(t.TempDir(), "project")
	child := []string{"doctor", "--json", "--effort", "e"}
	if err := launch(child, root); err != nil {
		t.Fatal(err)
	}
	want := []string{"dashboard-read", "--project-root", root, "--", "doctor", "--json", "--effort", "e"}
	if gotPath != awf || !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("exec = %q %v, want %q %v", gotPath, gotArgs, awf, want)
	}
	if code := run(child, root); code != 0 {
		t.Fatalf("run success code = %d", code)
	}
	launcherExec = func(string, []string) error { return errors.New("exec failed") }
	if err := launch(child, root); err == nil || !strings.HasPrefix(err.Error(), "exec:") {
		t.Fatalf("exec error = %v", err)
	}
	launcherExec = func(path string, args []string) error {
		gotPath, gotArgs = path, append([]string(nil), args...)
		return nil
	}

	if err := os.WriteFile(policy, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := launch(child, root); err == nil || !strings.HasPrefix(err.Error(), "digest:") {
		t.Fatalf("tamper error = %v", err)
	}
	if err := os.WriteFile(policy, []byte("policy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, "metadata.json")); err != nil {
		t.Fatal(err)
	}
	if err := launch(child, root); err == nil || !strings.HasPrefix(err.Error(), "path:") {
		t.Fatalf("missing metadata error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := launch(child, root); err == nil || !strings.HasPrefix(err.Error(), "metadata: decode:") {
		t.Fatalf("malformed metadata error = %v", err)
	}
	metadata.FormatVersion = 0
	invalid, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := launch(child, root); err == nil || err.Error() != "metadata: invalid runtime metadata" {
		t.Fatalf("invalid metadata error = %v", err)
	}
	launcherReadFile = func(string) ([]byte, error) { return nil, errors.New("read failed") }
	if err := launch(child, root); err == nil || !strings.HasPrefix(err.Error(), "metadata:") {
		t.Fatalf("metadata read error = %v", err)
	}
	launcherReadFile = os.ReadFile
	if executableNameForOS("awf", "windows") != "awf.exe" || executableNameForOS("awf", "linux") != "awf" {
		t.Fatal("executable naming mismatch")
	}
	if err := requireRegular(directory); err == nil {
		t.Fatal("directory accepted as regular")
	}
	if _, err := fileDigest(filepath.Join(directory, "missing")); err == nil {
		t.Fatal("missing digest file accepted")
	}
}

func TestLauncherRejectsBeforeExecution(t *testing.T) {
	oldExecutable, oldExec, oldReadFile := launcherExecutable, launcherExec, launcherReadFile
	t.Cleanup(func() { launcherExecutable, launcherExec, launcherReadFile = oldExecutable, oldExec, oldReadFile })
	called := false
	launcherExecutable = func() (string, error) { called = true; return "", errors.New("called") }
	launcherExec = func(string, []string) error { t.Fatal("forbidden argv reached exec"); return nil }
	if err := launch([]string{"metrics", "purge", "--confirm"}, "/project"); err == nil || called {
		t.Fatalf("forbidden argv error=%v executable-called=%v", err, called)
	}
	if err := launch([]string{"metrics", "protocol", "--json"}, "relative"); err == nil || called {
		t.Fatalf("relative root error=%v executable-called=%v", err, called)
	}
	launcherExecutable = func() (string, error) { return "", errors.New("missing executable") }
	if err := launch([]string{"metrics", "protocol", "--json"}, filepath.Join(t.TempDir(), "project")); err == nil || !strings.HasPrefix(err.Error(), "executable:") {
		t.Fatalf("executable error = %v", err)
	}
	if code := run([]string{"metrics", "purge"}, "/project"); code != 1 {
		t.Fatalf("run failure code = %d", code)
	}
}

func TestReplaceProcessReportsExecFailure(t *testing.T) {
	if err := replaceProcess(filepath.Join(t.TempDir(), "missing"), nil); err == nil {
		t.Fatal("replaceProcess accepted missing executable")
	}
}
