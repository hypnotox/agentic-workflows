package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// The drift and state children each run alone on a clean tree and print their own
// clean line, so neither borrows the bare form's verdict.
func TestCheckChildrenCleanLines(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	for _, tc := range []struct{ sub string }{
		{"repo drift"},
		{"repo state"},
	} {
		t.Run(tc.sub, func(t *testing.T) {
			root := scaffoldProject(t)
			var out, errb bytes.Buffer
			args := append([]string{"awf", "check"}, strings.Fields(tc.sub)...)
			if code := runAt(t, root, args, &out, &errb); code != 0 {
				t.Fatalf("exit = %d, stderr=%q", code, errb.String())
			}
			if out.String() != completedCheckReport {
				t.Errorf("output = %q, want structured completed report", out.String())
			}
			// Neither child prints the bare form's version-ahead note or advisories.
			if strings.Contains(out.String(), "is ahead of this project") {
				t.Errorf("check %s must not print the version-ahead note:\n%s", tc.sub, out.String())
			}
		})
	}
}

// invariant: invariants/current-state-authority:invariants-zero-slugs-clean (TestCheckRepoStateNoInvariantClaims)
func TestCheckRepoStateNoInvariantClaims(t *testing.T) {
	root := scaffoldProject(t)
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "check", "repo", "state"}, &out, &errb); code != 0 {
		t.Fatalf("check repo state with no invariant claims exited %d: %s", code, errb.String())
	}
	if got := out.String(); got != completedCheckReport || strings.Contains(got, "backing") {
		t.Fatalf("check repo state with no invariant claims = %q, want structured completed report", got)
	}
}

// invariant: tooling/cli:check-universe-groups (TestCheckStatePathsDispatchDistinctly)
func TestCheckStatePathsDispatchDistinctly(t *testing.T) {
	root := syncedGitProject(t, checkYAML)
	for _, tc := range []struct {
		args []string
	}{
		{[]string{"awf", "check", "repo", "state"}},
		{[]string{"awf", "check", "staged", "state"}},
	} {
		var out, errb bytes.Buffer
		if code := runAt(t, root, tc.args, &out, &errb); code != 0 {
			t.Fatalf("%v exited %d: %s", tc.args, code, errb.String())
		}
		if out.String() != completedCheckReport {
			t.Errorf("%v output = %q, want structured completed report", tc.args, out.String())
		}
	}
}

// invariant: tooling/quality-gates:gates-always-run (TestCheckScannersAlwaysRun)
// invariant: tooling/cli:repo-check-capability-plan (TestCheckScannersAlwaysRun)
func TestCheckScannersAlwaysRun(t *testing.T) {
	const prosePath = "docs/prose.md"
	const memoryPath = "docs/decisions/0001-memory.md"
	violations := map[string]string{
		prosePath:  "an en dash \u2013 here\n",
		memoryPath: testsupport.ADR("Proposed", testsupport.WithBody("## Context\n\n"+cite()+"\n")),
	}
	root := syncedGitProjectFiles(t, checkYAML, violations)
	for _, tc := range []struct {
		args  []string
		paths []string
	}{
		{[]string{"awf", "check", "repo"}, []string{prosePath, memoryPath}},
		{[]string{"awf", "check", "repo", "prose"}, []string{prosePath}},
		{[]string{"awf", "check", "repo", "memory"}, []string{memoryPath}},
	} {
		var out, errb bytes.Buffer
		code := runAt(t, root, tc.args, &out, &errb)
		proseOnly := tc.args[len(tc.args)-1] == "prose"
		if (proseOnly && code != 0) || (!proseOnly && code == 0) {
			t.Fatalf("%v exit = %d, want prose-only warning success and memory-bearing failure:\n%s%s", tc.args, code, out.String(), errb.String())
		}
		report := out.String() + errb.String()
		for _, path := range tc.paths {
			if !strings.Contains(report, path) {
				t.Fatalf("%v did not report %s:\n%s", tc.args, path, report)
			}
		}
		if strings.Contains(out.String(), "disabled") {
			t.Fatalf("%v exposed a retired disabled state:\n%s", tc.args, out.String())
		}
	}

	exemptYAML := checkYAML + "proseGate:\n  exemptions:\n    - path: " + prosePath + "\n      codepoint: U+2013\n      count: 1\nmemoryCite:\n  exemptions:\n    - path: " + memoryPath + "\n      count: 1\n"
	exemptRoot := syncedGitProjectFiles(t, exemptYAML, violations)
	for _, args := range [][]string{{"awf", "check", "repo"}, {"awf", "check", "repo", "prose"}, {"awf", "check", "repo", "memory"}} {
		var out, errb bytes.Buffer
		if code := runAt(t, exemptRoot, args, &out, &errb); code != 0 {
			t.Fatalf("%v ignored its exemption list (exit %d): %s%s", args, code, out.String(), errb.String())
		}
	}
}

// An unrecognized positional lists the valid subcommands. MaxPos is -1 so the
// handler owns this message rather than a generic arity error.
func TestCheckUniverseUnknownSubcommand(t *testing.T) {
	root := syncedGitProject(t, checkYAML)
	for _, universe := range []string{"repo", "staged"} {
		var out, errb bytes.Buffer
		code := runAt(t, root, []string{"awf", "check", universe, "bogus"}, &out, &errb)
		if code != 2 || !strings.Contains(errb.String(), `unknown subcommand "bogus"`) {
			t.Errorf("check %s bogus = code %d, stderr %q", universe, code, errb.String())
		}
	}
}

func TestCheckUnknownSubcommand(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := scaffoldProject(t)
	var out, errb bytes.Buffer
	code := runAt(t, root, []string{"awf", "check", "bogus"}, &out, &errb)
	if code == 0 {
		t.Fatal("check bogus exited 0")
	}
	if !strings.Contains(errb.String(), `unknown subcommand "bogus"`) {
		t.Errorf("diagnostic = %q, want the unknown-subcommand message", errb.String())
	}
	for _, want := range []string{"repo", "staged"} {
		if !strings.Contains(errb.String(), want) {
			t.Errorf("diagnostic omits the valid subcommand %q: %s", want, errb.String())
		}
	}
}

// aheadSchemaGitProject builds a git-backed project whose lock records a schema
// generation ahead of this binary, so every gated command refuses it. It must be
// git-backed: the prose and memory scans read the index and refuse outside a
// repository, before consulting their own knob.
func aheadSchemaGitProject(t *testing.T) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	lock := &manifest.Lock{AWFVersion: "0.4.0", SchemaVersion: migrate.Current() + 1, Files: map[string]manifest.Entry{}}
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	gitfixture.Stage(t, repo, map[string]string{
		".awf/config.yaml": "prefix: ex\nintegrationBranch: main\n",
		".awf/awf.lock":    string(b),
	})
	gitfixture.Commit(t, repo, "head", nil)
	return root
}

// The whole check family inherits its gate from the top-level command, so a
// direct prose invocation refuses before scanning when the project schema is
// ahead of the binary.
func TestCheckProseRefusesOnSchemaAheadProject(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := aheadSchemaGitProject(t)
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "check", "repo", "prose"}, &out, &errb); code != 1 {
		t.Fatalf("check repo prose exit = %d on an ahead-schema project, stderr=%q", code, errb.String())
	}
	if all := out.String() + errb.String(); !strings.Contains(all, "update your pinned awf") {
		t.Fatalf("check repo prose refused without the version-gate message: %s", all)
	}
}

// The per-child project-state exemption keeps the hook-wired commit child
// runnable under a committed current-state journal, where bare check refuses.
// Without this a commit-msg hook would refuse to validate a message mid-upgrade.

// The two new entry points surface their own failures: a drifted rendered file
// and a current-state finding each exit non-zero with a count-naming error.
func TestCheckChildrenReportFindings(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	t.Run("drift", func(t *testing.T) {
		root := scaffoldProject(t)
		// Hand-edit a rendered file so the drift oracle flags it.
		lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
		if err != nil {
			t.Fatal(err)
		}
		var target string
		for path := range lock.Files {
			if strings.HasSuffix(path, ".md") {
				if target == "" || path < target {
					target = path
				}
			}
		}
		if target == "" {
			t.Fatal("no rendered Markdown file in the lock to tamper with")
		}
		testsupport.WriteFile(t, filepath.Join(root, target), "hand-edited\n")
		var out, errb bytes.Buffer
		code := runAt(t, root, []string{"awf", "check", "repo", "drift"}, &out, &errb)
		if code != 1 {
			t.Fatalf("exit = %d, want 1; stdout=%q", code, out.String())
		}
		if !strings.Contains(out.String(), "hand-edited") || errb.Len() != 0 {
			t.Errorf("produced drift report stdout=%q stderr=%q", out.String(), errb.String())
		}
	})

	t.Run("state", func(t *testing.T) {
		// An owned path with no scoped topic is an error-severity coverage finding.
		root := syncedGitProjectFiles(t, coverageYAML(), coverageFiles())
		var out, errb bytes.Buffer
		code := runAt(t, root, []string{"awf", "check", "repo", "state"}, &out, &errb)
		if code != 1 {
			t.Fatalf("exit = %d, want 1; stdout=%q", code, out.String())
		}
		if !strings.Contains(out.String(), "uncovered") || errb.Len() != 0 {
			t.Errorf("produced state report stdout=%q stderr=%q", out.String(), errb.String())
		}
	})
}

// The two entry points' error returns, called directly because the driver would
// refuse most of these trees before the handler ran. Each fixture is the one the
// equivalent runCheck path already uses.
func TestCheckChildrenErrorPaths(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	t.Run("open error", func(t *testing.T) {
		// No .awf/ or repository, so every direct child fails before checking.
		root := t.TempDir()
		if err := runCheckStagedState(ctx, root, io.Discard); err == nil {
			t.Error("expected an Open error from check staged state")
		}
		if err := runCheckStagedDrift(ctx, root, io.Discard); err == nil {
			t.Error("expected an Open error from check staged drift")
		}
		if err := runCheckDrift(ctx, root, io.Discard); err == nil {
			t.Error("expected an Open error from check repo drift")
		}
		if err := runCheckState(ctx, t.TempDir(), io.Discard); err == nil {
			t.Error("expected an Open error from check repo state")
		}
	})

	t.Run("drift render error", func(t *testing.T) {
		// A data value spelling the no-value token makes the in-memory re-render
		// fail, so Project.CheckReport returns an error rather than a drift list.
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nvars: {}\n")
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "brainstorming.yaml"),
			"data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n")
		if err := runCheckDrift(ctx, root, io.Discard); err == nil {
			t.Fatal("expected check repo drift to surface the render error from Project.CheckReport")
		}
	})

	t.Run("state filesystem fallback", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, checkYAML)
		if err := initializeProject(testContext(t), root, io.Discard); err != nil {
			t.Fatalf("render: %v", err)
		}
		if err := runCheckState(ctx, root, io.Discard); err != nil {
			t.Fatalf("filesystem current-state fallback: %v", err)
		}
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		if err := runCheckState(canceled, root, io.Discard); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled filesystem current-state fallback = %v", err)
		}
	})

	t.Run("state prints warn notes", func(t *testing.T) {
		// The fixed fan-out budget is eight, so this two-topic fixture stays clean.
		root := syncedGitProjectFiles(t, coverageYAML(), fanoutFiles())
		var out bytes.Buffer
		if err := runCheckState(ctx, root, &out); err != nil {
			t.Fatalf("a warn-ranked finding must not fail check repo state: %v", err)
		}
		if !strings.Contains(out.String(), "findings: 0 errors, 0 warnings") {
			t.Errorf("expected a clean fixed-budget report:\n%s", out.String())
		}
	})
}

func TestCheckSubcommandEnumerationPaths(t *testing.T) {
	if got := checkSubcommands("repo"); got != "drift, state, prose, memory" {
		t.Fatalf("repo children = %q", got)
	}
	if got := checkSubcommands("repo unknown"); got != "drift, state, prose, memory" {
		t.Fatalf("unknown nested path should retain the addressed group, got %q", got)
	}
}

func TestRunCheckGroupDirectEdges(t *testing.T) {
	root := syncedGitProject(t, checkYAML)
	c := &cmdCtx{ctx: testContext(t), root: root, sub: "repo", inv: invocation{}, stdout: io.Discard}
	if err := runCheckGroup(c); err != nil {
		t.Fatalf("repo aggregate: %v", err)
	}
	c.sub = "unreachable"
	if err := runCheckGroup(c); err == nil || !strings.Contains(err.Error(), "unknown subcommand") {
		t.Fatalf("direct unknown path = %v", err)
	}
}

// awf help lists both check-universe children, extending the group-child
// assertion to the nested command structure.
func TestHelpListsCheckChildren(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	var out, errb bytes.Buffer
	run([]string{"awf", "help"}, &out, &errb)
	got := out.String()
	for _, sub := range []string{"repo", "staged"} {
		if !strings.Contains(got, "    "+sub+" |") {
			t.Errorf("awf help omits the check child %q:\n%s", sub, got)
		}
	}
	// The overview contains no entries for these four top-level names.
	for _, retired := range []string{"invariants |", "prose-gate |", "memory-gate |", "commit-gate |"} {
		if strings.Contains(got, retired) {
			t.Errorf("awf help still lists the retired top-level %q", strings.TrimSpace(retired))
		}
	}
}

// invariant: tooling/cli:group-child-project-guard-exemption (TestCheckExemptChildrenRunUnderGuardedSession)
func TestCheckExemptChildrenRunUnderGuardedSession(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "current-state-upgrade.journal"), `{"version":1,"phase":"prepared","finalLockSHA256":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881","operations":[{"path":".awf/awf.lock","prior":{"present":false,"mode":0,"content":null},"replacement":{"present":true,"mode":420,"content":"eA=="}}]}`)
	repo := gitfixture.At(root)
	gitfixture.AddAll(t, repo)

	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "check", "staged"}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "current-state upgrade journal") {
		t.Fatalf("non-exempt staged check did not prove the journal guard: code=%d stderr=%q", code, errb.String())
	}

	out.Reset()
	errb.Reset()
	message := writeMsg(t, "test(awf): exercise commit hook\n")
	if code := runAt(t, root, []string{"awf", "check", "staged", "commit", message}, &out, &errb); code != 0 {
		t.Fatalf("exempt commit child exit = %d, stderr=%q", code, errb.String())
	}
	if strings.Contains(errb.String(), "current-state upgrade journal") {
		t.Fatalf("commit child was blocked by the journal guard: %q", errb.String())
	}
}
