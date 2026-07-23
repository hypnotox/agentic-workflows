package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashboardRepositoryFailureBoundaries(t *testing.T) {
	oldGit, oldEval := dashboardGitOutput, dashboardEvalSymlinks
	t.Cleanup(func() { dashboardGitOutput, dashboardEvalSymlinks = oldGit, oldEval })
	root := t.TempDir()
	metadata := dashboardRuntimeMetadata{RepositoryID: filepath.Join(root, ".git"), ObjectFormat: "sha1", Commit: strings.Repeat("a", 40)}
	if err := os.Mkdir(metadata.RepositoryID, 0o700); err != nil {
		t.Fatal(err)
	}
	good := func(args ...string) string {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "--git-common-dir"):
			return metadata.RepositoryID
		case strings.Contains(joined, "--show-object-format"):
			return "sha1"
		default:
			return ""
		}
	}
	dashboardGitOutput = func(_ string, args ...string) (string, error) { return good(args...), nil }
	dashboardEvalSymlinks = func(string) (string, error) { return "", errors.New("injected") }
	if err := verifyDashboardRepository(root, metadata); err == nil {
		t.Fatal("identity evaluation fault accepted")
	}
	dashboardEvalSymlinks = filepath.EvalSymlinks
	dashboardGitOutput = func(_ string, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "--show-object-format") {
			return "", errors.New("injected")
		}
		return good(args...), nil
	}
	if err := verifyDashboardRepository(root, metadata); err == nil {
		t.Fatal("object format fault accepted")
	}
	dashboardGitOutput = func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "cat-file" {
			return "", errors.New("injected")
		}
		return good(args...), nil
	}
	if err := verifyDashboardRepository(root, metadata); err == nil {
		t.Fatal("missing commit accepted")
	}
}

func TestDashboardMetricsConfinementFaults(t *testing.T) {
	oldEval := dashboardEvalSymlinks
	t.Cleanup(func() { dashboardEvalSymlinks = oldEval })
	root := t.TempDir()
	if err := verifyMetricsConfinement(root); err == nil {
		t.Fatal("missing .awf accepted")
	}
	awf := filepath.Join(root, ".awf")
	if err := os.Mkdir(awf, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := verifyMetricsConfinement(root); err != nil {
		t.Fatalf("missing metrics rejected: %v", err)
	}
	metrics := filepath.Join(awf, "metrics")
	if err := os.WriteFile(metrics, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyMetricsConfinement(root); err == nil {
		t.Fatal("metrics file accepted")
	}
	if err := os.Remove(metrics); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(metrics, 0o700); err != nil {
		t.Fatal(err)
	}
	dashboardEvalSymlinks = func(string) (string, error) { return "", errors.New("injected") }
	if err := verifyMetricsConfinement(root); err == nil {
		t.Fatal("metrics canonicalization fault accepted")
	}
}

func TestCanonicalDashboardRootRejectsSymlink(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalDashboardRoot(link); err == nil {
		t.Fatal("symlink root accepted")
	}
}
