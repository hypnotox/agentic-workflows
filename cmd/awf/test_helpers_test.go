package main

import (
	"context"
	"io"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// minimalYAML is a valid tree-config for a scaffolded fixture project.
const minimalYAML = `prefix: example
profile: full
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
	state, cfg, err := loader.OpenForOperation(ctx, root)
	if err != nil {
		return err
	}
	prepared, err := composePublisher(state, cfg).Prepare()
	if err != nil {
		return err
	}
	result, err := prepared.Initialize(publisher.InitAuthority{InitializedWithVersion: project.Version})
	if err != nil {
		return err
	}
	mutation, err := result.Mutation()
	if err != nil {
		return err
	}
	return renderSyncMutation(out, mutation)
}

func scaffoldProject(t *testing.T) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("scaffold sync: %v", err)
	}
	// Repository drift now requires the rendered transaction to be indexed.
	gitfixture.AddAll(t, repo)
	return root
}
