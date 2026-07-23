// Command awf-dashboard-launcher is the immutable, read-only dashboard entrypoint.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type runtimeMetadata struct {
	FormatVersion  int      `json:"formatVersion"`
	RepositoryID   string   `json:"repositoryID"`
	ObjectFormat   string   `json:"objectFormat"`
	Commit         string   `json:"commit"`
	GoVersion      string   `json:"goVersion"`
	GoExperiment   string   `json:"goExperiment"`
	GoToolchain    string   `json:"goToolchain"`
	GOOS           string   `json:"goos"`
	GOARCH         string   `json:"goarch"`
	GoFlags        []string `json:"goFlags"`
	PolicySchema   int      `json:"policySchema"`
	ProtocolMajor  int      `json:"protocolMajor"`
	BinarySHA256   string   `json:"binarySHA256"`
	LauncherSHA256 string   `json:"launcherSHA256"`
	PolicySHA256   string   `json:"policySHA256"`
}

var launcherExecutable = os.Executable
var launcherExec = replaceProcess
var launcherReadFile = os.ReadFile

func main() { os.Exit(run(os.Args[1:], os.Getenv("AWF_DASHBOARD_PROJECT_ROOT"))) } // coverage-ignore: process wrapper; run is unit-tested

func run(child []string, root string) int {
	if err := launch(child, root); err != nil {
		fmt.Fprintln(os.Stderr, "dashboard launcher:", err)
		return 1
	}
	return 0
}

func launch(child []string, root string) error {
	if err := validatePublicQuery(child); err != nil {
		return fmt.Errorf("argv: %w", err)
	}
	if root == "" || !filepath.IsAbs(root) {
		return errors.New("project-root: AWF_DASHBOARD_PROJECT_ROOT must be absolute")
	}
	executable, err := launcherExecutable()
	if err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	directory := filepath.Dir(executable)
	awf := filepath.Join(directory, executableName("awf"))
	metadataPath := filepath.Join(directory, "metadata.json")
	policyPath := filepath.Join(directory, "policy.json")
	for name, path := range map[string]string{"launcher": executable, "awf": awf, "metadata": metadataPath, "policy": policyPath} {
		if err := requireRegular(path); err != nil {
			return fmt.Errorf("path: %s: %w", name, err)
		}
	}
	metadataBytes, err := launcherReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("metadata: %w", err)
	}
	var metadata runtimeMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return fmt.Errorf("metadata: decode: %w", err)
	}
	canonical, err := json.Marshal(metadata)
	if err != nil || !bytes.Equal(canonical, metadataBytes) || metadata.FormatVersion != 1 || metadata.PolicySchema != 1 || metadata.ProtocolMajor != 2 {
		return errors.New("metadata: invalid runtime metadata")
	}
	checks := []struct{ path, want string }{{executable, metadata.LauncherSHA256}, {awf, metadata.BinarySHA256}, {policyPath, metadata.PolicySHA256}}
	for _, check := range checks {
		got, digestErr := fileDigest(check.path)
		if digestErr != nil || got != check.want {
			return errors.New("digest: runtime artifact mismatch")
		}
	}
	translated := []string{"dashboard-read", "--project-root", root, "--"}
	translated = append(translated, child...)
	if err := launcherExec(awf, translated); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}

func validatePublicQuery(args []string) error {
	if equalArgs(args, []string{"metrics", "protocol", "--json"}) {
		return nil
	}
	var start int
	switch {
	case len(args) >= 2 && args[0] == "metrics" && args[1] == "--json":
		start = 2
	case len(args) >= 4 && args[0] == "metrics" && args[1] == "export" && args[2] == "--format" && args[3] == "json":
		start = 4
	case len(args) >= 2 && args[0] == "doctor" && args[1] == "--json":
		start = 2
	default:
		return errors.New("query shape is not allowed")
	}
	return validateOrderedSelectors(args[start:])
}

func validateOrderedSelectors(args []string) error {
	order := map[string]int{"--effort": 0, "--session": 1, "--phase": 2, "--since": 3, "--until": 4}
	last := -1
	for len(args) > 0 {
		position, ok := order[args[0]]
		if !ok || position <= last || len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return errors.New("selectors must be unique and in fixed order")
		}
		last, args = position, args[2:]
	}
	return nil
}

func equalArgs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func requireRegular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("not a non-symlink regular file")
	}
	return nil
}

func fileDigest(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func executableName(base string) string { return executableNameForOS(base, runtime.GOOS) }

func executableNameForOS(base, goos string) string {
	if goos == "windows" {
		return base + ".exe"
	}
	return base
}
