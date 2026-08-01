package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

func TestEffortProtocol2CLIFromPrimaryAndLinkedWorktrees(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// invariant: tooling/cli:effort-command-contract (TestEffortProtocol2CLIFromPrimaryAndLinkedWorktrees)
	// invariant: tooling/effort-management:effort-record-authority (TestEffortProtocol2CLIFromPrimaryAndLinkedWorktrees)
	primary := filepath.Join(t.TempDir(), "primary with spaces")
	fixture := gitfixture.InitNativeAt(t, primary)
	if err := os.WriteFile(filepath.Join(primary, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, fixture, "tracked.txt")
	gitfixture.NativeCommit(t, fixture, "base")
	linked := filepath.Join(filepath.Dir(primary), "linked with spaces")
	gitfixture.NativeWorktreeAddDetached(t, fixture, linked, "HEAD")

	createdJSON := runEffortCommand(t, primary, "new", []string{"CLI outcome"}, map[string]bool{"--json": true, "--no-worktree": true})
	var created struct {
		SchemaVersion int                  `json:"schemaVersion"`
		Effort        effort.Record        `json:"effort"`
		Worktree      *effortWorktreeFacts `json:"worktree"`
	}
	if err := json.Unmarshal([]byte(createdJSON), &created); err != nil {
		t.Fatal(err)
	}
	if created.SchemaVersion != 2 || created.Effort.Slug != "cli-outcome" || created.Effort.Title != "CLI outcome" || created.Effort.MemoryPath != ".awf/efforts/cli-outcome/memory.md" || created.Worktree != nil {
		t.Fatalf("new JSON = %#v", created)
	}
	resident := filepath.Join(primary, ".awf", "efforts", "cli-outcome")
	if _, err := os.Stat(filepath.Join(resident, "state.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(resident, "memory.md")); err != nil {
		t.Fatal(err)
	}
	shownJSON := runEffortCommand(t, linked, "show", []string{"cli-outcome"}, map[string]bool{"--json": true})
	var shown struct {
		SchemaVersion int           `json:"schemaVersion"`
		Effort        effort.Record `json:"effort"`
	}
	if err := json.Unmarshal([]byte(shownJSON), &shown); err != nil {
		t.Fatal(err)
	}
	primaryMemory := filepath.Join(primary, ".awf", "efforts", "cli-outcome", "memory.md")
	qualified := created.Effort
	qualified.MemoryPath = primaryMemory
	if shown.SchemaVersion != 2 || shown.Effort != qualified {
		t.Fatalf("show JSON from the linked checkout = %#v, want the created effort with memoryPath %q", shown, primaryMemory)
	}
	if text := runEffortCommand(t, linked, "show", []string{"cli-outcome"}, nil); !strings.Contains(text, "memory="+primaryMemory) {
		t.Fatalf("linked show text = %q, want the primary-root-qualified memory fact", text)
	}
	listedJSON := runEffortCommand(t, linked, "list", nil, map[string]bool{"--json": true})
	var listed struct {
		SchemaVersion int             `json:"schemaVersion"`
		Efforts       []effort.Record `json:"efforts"`
	}
	if err := json.Unmarshal([]byte(listedJSON), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.SchemaVersion != 2 || len(listed.Efforts) != 1 || listed.Efforts[0].Slug != "cli-outcome" || listed.Efforts[0].MemoryPath != primaryMemory {
		t.Fatalf("list JSON = %#v", listed)
	}
	if text := runEffortCommand(t, primary, "show", []string{"cli-outcome"}, nil); !strings.Contains(text, "effort cli-outcome") || !strings.Contains(text, "memory=.awf/efforts/cli-outcome/memory.md") {
		t.Fatalf("show text = %q", text)
	}

	finished := runEffortCommand(t, primary, "finish", []string{"cli-outcome"}, nil)
	if !strings.Contains(finished, "changed active rename: yes") || !strings.Contains(finished, "changed cleanup: yes") {
		t.Fatalf("finish output = %q", finished)
	}
	if _, err := os.Stat(resident); !os.IsNotExist(err) {
		t.Fatalf("finished resident remains: %v", err)
	}
}

func TestEffortJSONFailuresWriteNoStdoutAndRejectProtocol1(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := commandRepo(t)
	for _, test := range []struct {
		sub string
		pos []string
	}{
		{sub: "new", pos: []string{"界🙂"}},
		{sub: "show", pos: []string{"missing-effort"}},
	} {
		var stdout bytes.Buffer
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: test.sub, inv: invocation{positionals: test.pos, bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &stdout}, openEffortComposition)
		if err == nil || stdout.Len() != 0 || !strings.Contains(err.Error(), "next action") {
			t.Fatalf("%s error=%v stdout=%q", test.sub, err, stdout.String())
		}
	}

	dir := filepath.Join(root, ".awf", "efforts", "legacy-effort")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("Effort: legacy-effort\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"schemaVersion":1,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"legacy-effort","title":"Legacy effort","createdAt":"2026-07-29T12:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "show", inv: invocation{positionals: []string{"legacy-effort"}, bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &stdout}, openEffortComposition)
	if err == nil || stdout.Len() != 0 || !strings.Contains(err.Error(), "unsupported schemaVersion 1") {
		t.Fatalf("protocol-1 error=%v stdout=%q", err, stdout.String())
	}
}

type effortErrorWriter struct{}

func (effortErrorWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestEffortCommandUsageAndOutputErrors(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := commandRepo(t)
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: filepath.Join(root, "missing"), sub: "list", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("invalid repository accepted")
	}
	for _, bools := range []map[string]bool{{}, {"--no-worktree": true}} {
		if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{" "}, bools: bools, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
			t.Fatalf("blank title accepted with bools %v", bools)
		}
	}
	runEffortCommand(t, root, "new", []string{"Output errors"}, map[string]bool{"--no-worktree": true})
	for _, test := range []struct {
		sub   string
		pos   []string
		bools map[string]bool
	}{
		{sub: "new", pos: []string{"Another output"}, bools: map[string]bool{"--no-worktree": true}},
		{sub: "new", pos: []string{"JSON output"}, bools: map[string]bool{"--json": true, "--no-worktree": true}},
		{sub: "show", pos: []string{"output-errors"}},
		{sub: "show", pos: []string{"output-errors"}, bools: map[string]bool{"--json": true}},
		{sub: "list"},
		{sub: "list", bools: map[string]bool{"--json": true}},
	} {
		if test.bools == nil {
			test.bools = map[string]bool{}
		}
		err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: test.sub, inv: invocation{positionals: test.pos, bools: test.bools, values: map[string]string{}}, stdout: effortErrorWriter{}}, openEffortComposition)
		if err == nil {
			t.Errorf("%s output error ignored", test.sub)
		}
	}
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "finish", inv: invocation{positionals: []string{"output-errors"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: effortErrorWriter{}}, openEffortComposition); err == nil {
		t.Fatal("finish output error ignored")
	}
	if err := writeWorktreeResult(&bytes.Buffer{}, worktree.Result{}, os.ErrInvalid); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("worktree error = %v", err)
	}
	if err := writeWorktreeResult(effortErrorWriter{}, worktree.Result{Condition: "done"}, nil); err == nil {
		t.Fatal("worktree output error ignored")
	}
	if got := yesNo(false); got != "no" {
		t.Fatalf("yesNo(false) = %q", got)
	}
	for _, ctx := range []*cmdCtx{
		{root: root, sub: "worktree", inv: invocation{positionals: []string{"add"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}},
		{root: root, sub: "worktree", inv: invocation{positionals: []string{"remove", "output-errors"}, bools: map[string]bool{}, values: map[string]string{"--base": "HEAD"}}, stdout: &bytes.Buffer{}},
		{root: root, sub: "worktree", inv: invocation{positionals: []string{"other", "output-errors"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}},
	} {
		if err := runEffort(ctx, openEffortComposition); err == nil {
			t.Fatal("invalid worktree grammar accepted")
		}
	}
	corruptRoot := commandRepo(t)
	if err := os.MkdirAll(filepath.Join(corruptRoot, ".awf", "efforts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corruptRoot, ".awf", "efforts", "foreign"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: corruptRoot, sub: "list", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("corrupt list accepted")
	}
	emptyRoot := commandRepo(t)
	if output := runEffortCommand(t, emptyRoot, "list", nil, nil); output != "" {
		t.Fatalf("empty text list = %q", output)
	}
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "finish", inv: invocation{positionals: []string{"missing-effort"}, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("missing finish accepted")
	}
	if err := runEffort(&cmdCtx{ctx: testContext(t), root: root, sub: "unknown", inv: invocation{bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}, openEffortComposition); err == nil {
		t.Fatal("unknown effort child accepted")
	}
}

func runEffortCommand(t *testing.T, root, sub string, positionals []string, bools map[string]bool) string {
	t.Helper()
	if bools == nil {
		bools = map[string]bool{}
	}
	var out bytes.Buffer
	ctx := &cmdCtx{ctx: testContext(t), root: root, sub: sub, inv: invocation{positionals: positionals, bools: bools, values: map[string]string{}}, stdout: &out}
	if err := runEffort(ctx, openEffortComposition); err != nil {
		t.Fatalf("awf effort %s: %v", sub, err)
	}
	return out.String()
}

func commandRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "command repo")
	fixture := gitfixture.InitNativeAt(t, root)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, fixture, "tracked.txt")
	// An adopted project commits the resident .gitignore files awf renders, which
	// is what keeps owned effort and worktree state out of the cleanliness
	// oracle's view; the fixture carries them for the same reason.
	for _, resident := range []string{"efforts", "worktrees"} {
		ignore := filepath.Join(root, ".awf", resident, ".gitignore")
		if err := os.MkdirAll(filepath.Dir(ignore), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ignore, []byte("*\n!.gitignore\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitfixture.NativeAdd(t, fixture, ".awf/"+resident+"/.gitignore")
	}
	gitfixture.NativeCommit(t, fixture, "base")
	return root
}
