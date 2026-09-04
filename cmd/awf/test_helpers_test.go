package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func runNonInteractive(args []string, stdout, stderr io.Writer) int {
	return newRunner(os.Getwd, os.Stdin, func() bool { return false }).run(args, stdout, stderr)
}

func runFrom(root string, args []string, stdout, stderr io.Writer) int {
	return newRunner(func() (string, error) { return root, nil }, os.Stdin, func() bool { return false }).run(args, stdout, stderr)
}

func mustReadCLIFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// runInit supplies real process inputs only for focused operation tests. CLI
// dispatch tests construct an instance-owned runner with their own inputs.
func runInit(ctx context.Context, root string, describe bool, sets []string, answersFile string, stdout io.Writer) error {
	return runInitWithProjectLoader(ctx, root, describe, sets, answersFile, os.Stdin, false, stdout, newProjectLoader, gate)
}

// minimalYAML is a valid tree-config for a scaffolded fixture project.
const minimalYAML = `prefix: example
integrationBranch: master
vars: {testCmd: go test ./..., gateCmd: make gate}
`

// scaffoldProject writes a minimal tree config under a git-backed root and syncs
// it, leaving a drift-clean project. The base commit gives the working Tree a
// HEAD, which the commands that read one (check, invariants) require.
func initializeProject(ctx context.Context, root string, out io.Writer) error {
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	state, err := loader.Load(ctx, root)
	if err != nil {
		return err
	}
	result, err := composePublisher(state).Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version})
	if err != nil {
		return err
	}
	mutation, err := result.Mutation()
	if err != nil {
		return err
	}
	return renderSyncMutation(out, mutation)
}

var (
	scaffoldSeedOnce sync.Once
	scaffoldSeed     testsupport.TreeSeed
	scaffoldSeedErr  error
)

func scaffoldProject(t *testing.T) string {
	t.Helper()
	scaffoldSeedOnce.Do(func() {
		repo := gitfixture.InitRepo(t)
		root := repo.Root()
		gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
		testsupport.WriteAwfConfig(t, root, minimalYAML)
		if err := initializeProject(testContext(t), root, io.Discard); err != nil {
			scaffoldSeedErr = err
			return
		}
		// Repository drift requires the rendered transaction to be indexed.
		gitfixture.AddAll(t, repo)
		scaffoldSeed, scaffoldSeedErr = testsupport.CaptureTree(root)
	})
	if scaffoldSeedErr != nil {
		t.Fatalf("prepare scaffold seed: %v", scaffoldSeedErr)
	}
	root := filepath.Join(t.TempDir(), "project")
	if err := scaffoldSeed.Clone(root); err != nil {
		t.Fatalf("clone scaffold seed: %v", err)
	}
	return root
}
