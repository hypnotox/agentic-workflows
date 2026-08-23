package project

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// stagedHeadFiles is the HEAD content the staged/range fixtures share: a config
// with a currentState policy, a domain, a one-claim topic scoped to
// internal/foo/**, and the Implemented ADR the claim cites.
func stagedHeadFiles() map[string]string {
	return map[string]string{
		".awf/awf.lock":                                `{"awfVersion":"0.18.0","schemaVersion":14,"files":{},"adrFormatV1From":2,"legacyAdrGaps":[]}`,
		".awf/config.yaml":                             csYAML,
		".awf/domains/alpha.yaml":                      "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths:\n  - internal/foo/**\n",
		".awf/topics/parts/alpha/one/current-state.md": csRuleTopic,
		"docs/decisions/0001-first.md": testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First"),
			testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
	}
}

// attestedLock returns the permanent cutoff used by staged fixtures.
func attestedLock() *manifest.Lock {
	return &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14}
}

func lockJSON(t *testing.T, lock *manifest.Lock) string {
	t.Helper()
	data, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// writeLock writes and stages the project's awf.lock.
func writeLock(t *testing.T, p *ProjectState, lock *manifest.Lock) {
	t.Helper()
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, lockFile(p.Root()), string(b))
	gitfixture.Add(t, gitfixture.At(p.Root()), ".awf/awf.lock")
}

// openStaged opens a project whose config is on disk (staged or untracked),
// failing the test on error.
func openStaged(t *testing.T, dir string) *ProjectState {
	t.Helper()
	p, err := Open(testContext(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
