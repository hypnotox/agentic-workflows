// Command testselection emits conservative affected-Go-package test evidence.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testselection"
)

func main() { // coverage-ignore: os.Exit wrapper; run is unit-tested
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("testselection", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	policyPath := flags.String("policy", "", "selection policy path")
	staged := flags.Bool("staged", false, "use staged changes only")
	rangeSpec := flags.String("range", "", "use an explicit base..head range")
	execute := flags.Bool("execute", false, "execute the selected behavioral targets")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || (*staged && *rangeSpec != "") {
		fmt.Fprintln(stderr, "usage: testselection [--root DIR] [--policy FILE] [--staged | --range BASE..HEAD] [--execute]")
		return 2
	}
	absoluteRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, "testselection:", err)
		return 2
	}
	if *policyPath == "" {
		*policyPath = filepath.Join(absoluteRoot, "test-selection.json")
	}
	policy, err := testselection.Load(*policyPath)
	if err != nil {
		fmt.Fprintln(stderr, "testselection:", err)
		return 2
	}
	repo, err := awfgit.Open(absoluteRoot)
	if err != nil {
		fmt.Fprintln(stderr, "testselection: open repository:", err)
		return 2
	}
	selectionContext, cancelSelection := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelSelection()
	var changed []string
	switch {
	case *staged:
		changed, err = repo.ChangedPaths(selectionContext, true, "")
	case *rangeSpec != "":
		base, head, parseErr := awfgit.ParseRange(*rangeSpec, false)
		if parseErr != nil {
			err = parseErr
		} else {
			changed, err = repo.RangeChangedPaths(selectionContext, base, head)
		}
	default:
		changed, err = repo.WorktreeChangedPaths(selectionContext)
	}
	if err != nil {
		if writeErr := writeRefused(stdout, policy.Version, fmt.Errorf("read changed paths: %w", err)); writeErr != nil {
			fmt.Fprintln(stderr, "testselection: write selection evidence:", writeErr)
		} else {
			fmt.Fprintln(stderr, "testselection: read changed paths:", err)
		}
		return 2
	}
	result, err := testselection.Select(selectionContext, absoluteRoot, policy, changed)
	cancelSelection()
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
		if err := executeSelection(executionContext, absoluteRoot, result, stdout); err != nil {
			fmt.Fprintln(stderr, "testselection:", err)
			return 1
		}
	}
	return 0
}

const feedbackWorkers = 2

type testTarget struct {
	label         string
	args          []string
	expectedTests []string
}

type testRun struct {
	output []byte
	err    error
}

func executeSelection(ctx context.Context, root string, result testselection.Result, stdout io.Writer) error {
	targets := make([]testTarget, 0, len(result.Packages)+len(result.Suites))
	for _, pkg := range result.Packages {
		targets = append(targets, testTarget{label: pkg.Path, args: []string{pkg.Path}})
	}
	for _, suite := range result.Suites {
		targets = append(targets, testTarget{
			label:         "suite:" + suite.ID,
			args:          []string{"-json", "-run", exactPattern(suite.Tests), suite.Package},
			expectedTests: append([]string(nil), suite.Tests...),
		})
	}
	if len(targets) == 0 {
		return nil
	}

	cacheOutput, err := exec.CommandContext(ctx, "go", "env", "GOCACHE", "GOMODCACHE").Output()
	if err != nil {
		return fmt.Errorf("read shared Go caches: %w", err)
	}
	caches := strings.Fields(string(cacheOutput))
	if len(caches) != 2 {
		return fmt.Errorf("read shared Go caches: expected two paths")
	}

	runs := make([]testRun, len(targets))
	jobs := make(chan int)
	workers := feedbackWorkers
	if workers > len(targets) {
		workers = len(targets)
	}
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				runs[index] = runTarget(ctx, root, targets[index], caches)
			}
		}()
	}
	for index := range targets {
		jobs <- index
	}
	close(jobs)
	group.Wait()

	var firstErr error
	for index, run := range runs {
		if len(run.output) > 0 {
			if _, err := stdout.Write(run.output); err != nil {
				return fmt.Errorf("write behavioral output: %w", err)
			}
		}
		if run.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("behavioral target %s failed: %w", targets[index].label, run.err)
		}
	}
	return firstErr
}

func exactPattern(names []string) string {
	quoted := make([]string, len(names))
	for index, name := range names {
		quoted[index] = regexp.QuoteMeta(name)
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

var feedbackTemporarySequence atomic.Uint64

func runTarget(ctx context.Context, root string, target testTarget, caches []string) testRun {
	// Unix-domain socket fixtures must fit the platform path limit beneath
	// testing.T.TempDir, so keep the isolated worker root deliberately short.
	temporary := filepath.Join("/tmp", fmt.Sprintf("%05s%02s", strconv.FormatInt(int64(os.Getpid()), 36), strconv.FormatUint(feedbackTemporarySequence.Add(1), 36)))
	if err := os.Mkdir(temporary, 0o700); err != nil {
		return testRun{err: fmt.Errorf("create isolated roots: %w", err)}
	}
	defer os.RemoveAll(temporary)
	args := []string{"test", "-p=1", "-timeout=5m", "-count=1"}
	args = append(args, target.args...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+temporary, "TMPDIR="+temporary, "GOTMPDIR="+temporary, "GOMAXPROCS=1", "GOCACHE="+caches[0], "GOMODCACHE="+caches[1])
	output, err := cmd.CombinedOutput()
	if err != nil || len(target.expectedTests) == 0 {
		return testRun{output: output, err: err}
	}
	observed := map[string]bool{}
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
		var event struct {
			Action string `json:"Action"`
			Test   string `json:"Test"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			return testRun{output: output, err: fmt.Errorf("parse suite execution evidence: %w", err)}
		}
		if event.Action == "run" && event.Test != "" {
			observed[event.Test] = true
		}
	}
	missing := []string{}
	for _, test := range target.expectedTests {
		if !observed[test] {
			missing = append(missing, test)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return testRun{output: output, err: fmt.Errorf("unavailable proving units: %s", strings.Join(missing, ", "))}
	}
	return testRun{output: output}
}

func writeRefused(out io.Writer, version int, err error) error {
	return write(testselection.Result{Version: version, Outcome: "refused", Packages: []testselection.Package{}, Suites: []testselection.Suite{}, Reasons: []string{err.Error()}}, out)
}

func write(result testselection.Result, out io.Writer) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}
