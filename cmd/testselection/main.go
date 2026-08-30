// Command testselection emits typed, machine-readable affected-test evidence.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testselection"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 && args[1] == "full-linux" {
		return runFullLinux(args[2:], stdout, stderr)
	}
	flags := flag.NewFlagSet("testselection", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	policyPath := flags.String("policy", "", "selection policy path")
	staged := flags.Bool("staged", false, "use staged changes only")
	rangeSpec := flags.String("range", "", "use an explicit base..head range")
	execute := flags.Bool("execute", false, "execute selected Go packages")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || (*staged && *rangeSpec != "") {
		fmt.Fprintln(stderr, "usage: testselection [--root DIR] [--policy FILE] [--staged | --range BASE..HEAD] [--execute]")
		return 2
	}
	absoluteRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, "testselection:", err)
		return 2
	}
	repo, err := awfgit.Open(absoluteRoot)
	if err != nil {
		return selectionFailure(stdout, stderr, fmt.Errorf("open repository: %w", err))
	}

	selectionContext, cancelSelection := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelSelection()
	evaluationRoot := absoluteRoot
	var changed []string
	switch {
	case *rangeSpec != "":
		base, head, parseErr := awfgit.ParseRange(*rangeSpec, false)
		if parseErr != nil {
			return selectionFailure(stdout, stderr, fmt.Errorf("read changed paths: %w", parseErr))
		}
		changed, err = repo.RangeChangedPaths(selectionContext, base, head)
		if err == nil {
			evaluationRoot, err = materializeRevision(selectionContext, repo, head)
		}
		if err != nil {
			return selectionFailure(stdout, stderr, fmt.Errorf("read requested revision tree: %w", err))
		}
		defer os.RemoveAll(evaluationRoot)
	case *staged:
		changed, err = repo.ChangedPaths(selectionContext, true, "")
	default:
		changed, err = repo.WorktreeChangedPaths(selectionContext)
	}
	if err != nil {
		return selectionFailure(stdout, stderr, fmt.Errorf("read changed paths: %w", err))
	}

	if *policyPath == "" {
		*policyPath = filepath.Join(evaluationRoot, "test-selection.json")
	}
	policy, err := testselection.Load(*policyPath)
	if err != nil {
		return selectionFailure(stdout, stderr, err)
	}
	result, err := testselection.Select(selectionContext, evaluationRoot, policy, changed)
	if writeErr := write(result, stdout); writeErr != nil {
		fmt.Fprintln(stderr, "testselection: write selection evidence:", writeErr)
		return 2
	}
	if err != nil || result.Outcome == "refused" {
		if err != nil {
			fmt.Fprintln(stderr, "testselection:", err)
		}
		return 2
	}
	if *execute {
		executionContext, cancelExecution := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancelExecution()
		if err := executeSelection(executionContext, evaluationRoot, result, stderr); err != nil {
			fmt.Fprintln(stderr, "testselection:", err)
			return 1
		}
	}
	return 0
}

func selectionFailure(stdout, stderr io.Writer, err error) int {
	_ = write(testselection.Result{
		Version:     testselection.PolicyVersion,
		Outcome:     "refused",
		Lanes:       []testselection.Lane{},
		Packages:    []testselection.Package{},
		Diagnostics: []string{err.Error()},
	}, stdout)
	fmt.Fprintln(stderr, "testselection:", err)
	return 2
}

// materializeRevision creates a private source tree from the requested commit.
// Package discovery, policy reads, source inspection, and executed tests all use
// this tree rather than whichever worktree invoked the command. Commit reads go
// through the repository Git seam; no second native-Git boundary is introduced.
func materializeRevision(ctx context.Context, repo *awfgit.Repo, revision string) (string, error) {
	entries, err := repo.CommitEntries(ctx, revision)
	if err != nil {
		return "", fmt.Errorf("read requested revision entries: %w", err)
	}
	paths := make([]string, len(entries))
	for index, entry := range entries {
		paths[index] = entry.Path
	}
	blobs, err := repo.CommitBlobsAt(ctx, revision, paths)
	if err != nil {
		return "", fmt.Errorf("read requested revision blobs: %w", err)
	}
	destination, err := os.MkdirTemp("", "testselection-tree-")
	if err != nil {
		return "", fmt.Errorf("create revision tree: %w", err)
	}
	remove := true
	defer func() {
		if remove {
			_ = os.RemoveAll(destination)
		}
	}()
	for _, blob := range blobs {
		filename := filepath.Join(destination, filepath.FromSlash(blob.Path))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			return "", fmt.Errorf("create requested revision directory: %w", err)
		}
		switch blob.Mode {
		case awfgit.BlobRegular, awfgit.BlobExecutable:
			mode := os.FileMode(0o644)
			if blob.Mode == awfgit.BlobExecutable {
				mode = 0o755
			}
			if err := os.WriteFile(filename, blob.Bytes, mode); err != nil {
				return "", fmt.Errorf("write requested revision path %q: %w", blob.Path, err)
			}
		case awfgit.BlobSymlink:
			target := string(blob.Bytes)
			if !safeMaterializedSymlink(destination, filename, target) {
				return "", fmt.Errorf("requested revision path %q has an unsafe symlink target", blob.Path)
			}
			if err := os.Symlink(target, filename); err != nil {
				return "", fmt.Errorf("write requested revision symlink %q: %w", blob.Path, err)
			}
		default:
			return "", fmt.Errorf("requested revision path %q has an unsupported mode", blob.Path)
		}
	}
	remove = false
	return destination, nil
}

func safeMaterializedSymlink(root, filename, target string) bool {
	if target == "" || filepath.IsAbs(target) {
		return false
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(filename), target))
	relative, err := filepath.Rel(root, resolved)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

var runSelectedGoTests = func(ctx context.Context, root string, args, environment []string, output io.Writer) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Env = environment
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

var readGoCaches = func(ctx context.Context, root string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "go", "env", "-json", "GOCACHE", "GOMODCACHE")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	return parseGoCaches(output)
}

func parseGoCaches(data []byte) (string, string, error) {
	var values struct {
		GoCache    string `json:"GOCACHE"`
		GoModCache string `json:"GOMODCACHE"`
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return "", "", fmt.Errorf("decode go env cache paths: %w", err)
	}
	if values.GoCache == "" || values.GoModCache == "" {
		return "", "", fmt.Errorf("go env returned an empty cache path")
	}
	return values.GoCache, values.GoModCache, nil
}

func executeSelection(ctx context.Context, root string, result testselection.Result, output io.Writer) error {
	packagePaths := make([]string, 0, len(result.Packages))
	for _, selected := range result.Packages {
		packagePaths = append(packagePaths, selected.Path)
	}
	if len(packagePaths) == 0 {
		return nil
	}
	sort.Strings(packagePaths)
	goCache, moduleCache, err := readGoCaches(ctx, root)
	if err != nil {
		return fmt.Errorf("read shared Go caches: %w", err)
	}
	temporary, err := os.MkdirTemp("/tmp", "awft-")
	if err != nil {
		return fmt.Errorf("create isolated test root: %w", err)
	}
	defer os.RemoveAll(temporary)
	environment := environmentWithout(os.Environ(), "HOME", "TMPDIR", "GOTMPDIR", "GOCACHE", "GOMODCACHE", "GOMAXPROCS")
	environment = append(environment,
		"HOME="+temporary,
		"TMPDIR="+temporary,
		"GOTMPDIR="+temporary,
		"GOCACHE="+goCache,
		"GOMODCACHE="+moduleCache,
	)
	if err := runSelectedGoTests(ctx, root, goTestArgs(packagePaths), environment, output); err != nil {
		return fmt.Errorf("selected Go tests: %w", err)
	}
	return nil
}

func goTestArgs(targets []string) []string {
	return append([]string{"test", "-count=1"}, targets...)
}

func environmentWithout(environment []string, names ...string) []string {
	removed := make(map[string]bool, len(names))
	for _, name := range names {
		removed[name] = true
	}
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		name, _, _ := strings.Cut(value, "=")
		if !removed[name] {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func write(result testselection.Result, output io.Writer) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal selection result: %w", err)
	}
	_, err = fmt.Fprintln(output, string(data))
	return err
}

type timingArtifact struct {
	Version          int      `json:"version"`
	Mode             string   `json:"mode"`
	Platform         string   `json:"platform"`
	Command          []string `json:"command"`
	DurationMS       int64    `json:"duration_ms"`
	WarningCeilingMS int64    `json:"warning_ceiling_ms"`
	HardTimeoutMS    int64    `json:"hard_timeout_ms"`
	Status           string   `json:"status"`
}

type fullRunner func(context.Context, string, []string, io.Writer) (time.Duration, error)

var runFullSuite fullRunner = func(ctx context.Context, root string, args []string, output io.Writer) (time.Duration, error) {
	started := time.Now()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Env = environmentWithout(os.Environ(), "AWF_PI_RUNTIME_SMOKE")
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	return time.Since(started), err
}

var currentPlatform = func() (string, string) { return runtime.GOOS, runtime.GOARCH }
var fullLinuxBudgetContext = context.WithTimeout

func runFullLinux(args []string, _ io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("testselection full-linux", flag.ContinueOnError)
	flags.SetOutput(stderr)
	mode := flags.String("mode", "", "calibrate or budget")
	root := flags.String("root", ".", "repository root")
	artifactPath := flags.String("artifact", os.Getenv("AWF_FULL_LINUX_TIMING_ARTIFACT"), "timing artifact path")
	ceilingText := flags.String("ceiling", os.Getenv("AWF_FULL_LINUX_CEILING"), "evidence-derived warning ceiling")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || (*mode != "calibrate" && *mode != "budget") {
		fmt.Fprintln(stderr, "usage: testselection full-linux --mode calibrate|budget [--ceiling DURATION] [--artifact FILE]")
		return 2
	}
	goos, goarch := currentPlatform()
	if goos != "linux" || goarch != "amd64" {
		fmt.Fprintln(stderr, "testselection: full-linux requires linux/amd64")
		return 2
	}
	if *artifactPath == "" {
		*artifactPath = filepath.Join(*root, ".cache", "full-linux-timing.json")
	}

	var ceiling time.Duration
	if *mode == "budget" {
		var err error
		ceiling, err = time.ParseDuration(*ceilingText)
		if err != nil || ceiling <= 0 {
			fmt.Fprintln(stderr, "testselection: budget mode requires an explicit evidence-derived --ceiling")
			return 2
		}
	}
	timeout := hardTimeout(ceiling)
	ctx, cancel := fullLinuxBudgetContext(context.Background(), timeout)
	defer cancel()
	command := goTestArgs([]string{"./..."})
	duration, runErr := runFullSuite(ctx, *root, command, stderr)

	status := "passed"
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		status = "timeout"
	case runErr != nil:
		status = "failed"
	case *mode == "budget" && duration > ceiling:
		status = "warning"
	}
	artifact := timingArtifact{
		Version:          1,
		Mode:             *mode,
		Platform:         "linux/amd64",
		Command:          append([]string{"go"}, command...),
		DurationMS:       duration.Milliseconds(),
		WarningCeilingMS: ceiling.Milliseconds(),
		HardTimeoutMS:    timeout.Milliseconds(),
		Status:           status,
	}
	if err := writeTiming(*artifactPath, artifact); err != nil {
		fmt.Fprintln(stderr, "testselection:", err)
		return 2
	}
	switch status {
	case "timeout":
		fmt.Fprintf(stderr, "ERROR: full Linux suite exceeded hard timeout %s\n", timeout)
		return 1
	case "failed":
		fmt.Fprintln(stderr, "testselection: full Linux suite:", runErr)
		return 1
	case "warning":
		fmt.Fprintf(stderr, "WARNING: full Linux suite duration %s exceeds evidence-derived budget %s\n", duration, ceiling)
	}
	return 0
}

func hardTimeout(ceiling time.Duration) time.Duration {
	return max(3*ceiling, 10*time.Minute)
}

func writeTiming(filename string, artifact timingArtifact) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("create timing artifact directory: %w", err)
	}
	data, err := json.Marshal(artifact)
	if err != nil {
		return fmt.Errorf("marshal timing artifact: %w", err)
	}
	if err := os.WriteFile(filename, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write timing artifact: %w", err)
	}
	return nil
}
