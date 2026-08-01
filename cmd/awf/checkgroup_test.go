package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// The drift and state children each run alone on a clean tree and print their own
// clean line, so neither borrows the bare form's verdict.
func TestCheckChildrenCleanLines(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	for _, tc := range []struct{ sub, want string }{
		{"repo drift", "awf check repo drift: clean"},
		{"repo state", "awf check repo state: clean"},
	} {
		t.Run(tc.sub, func(t *testing.T) {
			root := scaffoldProject(t)
			var out, errb bytes.Buffer
			args := append([]string{"awf", "check"}, strings.Fields(tc.sub)...)
			if code := runAt(t, root, args, &out, &errb); code != 0 {
				t.Fatalf("exit = %d, stderr=%q", code, errb.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Errorf("output = %q, want %q", out.String(), tc.want)
			}
			// Neither child prints the bare form's version-ahead note or advisories.
			if strings.Contains(out.String(), "is ahead of this project") {
				t.Errorf("check %s must not print the version-ahead note:\n%s", tc.sub, out.String())
			}
		})
	}
}

// An unrecognized positional lists the valid subcommands. MaxPos is -1 so the
// handler owns this message rather than a generic arity error.
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
	for _, want := range []string{"repo", "staged", "invariants"} {
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

// The per-child project-state exemption: the three hook-wired children stay
// runnable under a committed current-state journal and under an attested lock,
// where bare check refuses. Without this a commit-msg hook would refuse to
// validate a message mid-upgrade.
// invariant: tooling/cli:group-child-project-guard-exemption (TestCheckExemptChildrenRunUnderGuardedProjectState)
func TestCheckExemptChildrenRunUnderGuardedProjectState(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	lockText := func(t *testing.T, attested bool) string {
		t.Helper()
		lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
		if attested {
			lock.BridgeAttestation = &manifest.BridgeAttestation{Version: 1, PreparedHead: "head", TreeDigest: "sha256:x", ADRFormatV1From: 2, LegacyADRGaps: []int{}}
		}
		b, err := lock.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	const journal = `{"version":1,"phase":"prepared","finalLockSHA256":"sha256:x","operations":[{"path":".awf/awf.lock","prior":{"present":false,"mode":0,"content":null},"replacement":{"present":false,"mode":0,"content":null}}]}`
	configText := "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\n"

	// guarded builds a git-backed project in the named guarded state.
	guarded := func(t *testing.T, journaled bool) string {
		t.Helper()
		repo := gitfixture.InitRepo(t)
		root := repo.Root()
		files := map[string]string{
			".awf/config.yaml": configText,
			".awf/awf.lock":    lockText(t, !journaled),
		}
		if journaled {
			files[".awf/current-state-upgrade.journal"] = journal
		}
		gitfixture.Stage(t, repo, files)
		gitfixture.Commit(t, repo, "head", nil)
		return root
	}

	for _, state := range []struct {
		name      string
		journaled bool
		refusal   string
	}{
		{"journal", true, "upgrade journal is present"},
		{"attestation", false, "committed current-state attestation"},
	} {
		t.Run(state.name, func(t *testing.T) {
			// Bare check refuses: the guard applies to the group's own node.
			root := guarded(t, state.journaled)
			var out, errb bytes.Buffer
			if code := runAt(t, root, []string{"awf", "check"}, &out, &errb); code != 1 {
				t.Fatalf("bare check exit = %d, want the guard refusal", code)
			}
			if !strings.Contains(errb.String(), state.refusal) {
				t.Fatalf("bare check diagnostic = %q, want %q", errb.String(), state.refusal)
			}
			// Only staged commit remains exempt so a commit-msg hook works mid-upgrade.
			for _, sub := range []string{"commit"} {
				args := []string{"awf", "check", "staged", sub}
				msg := filepath.Join(t.TempDir(), "MSG")
				testsupport.WriteFile(t, msg, "feat(awf): a conventional subject\n")
				args = append(args, msg)
				out.Reset()
				errb.Reset()
				if code := runAt(t, guarded(t, state.journaled), args, &out, &errb); code != 0 {
					t.Errorf("check %s exit = %d under a %s, stderr=%q", sub, code, state.name, errb.String())
				}
			}
		})
	}
}

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
		if !strings.Contains(errb.String(), "awf check repo drift:") || !strings.Contains(errb.String(), "drift(s)") {
			t.Errorf("diagnostic = %q, want a drift count", errb.String())
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
		if !strings.Contains(errb.String(), "awf check repo state:") || !strings.Contains(errb.String(), "current-state issue(s)") {
			t.Errorf("diagnostic = %q, want a current-state count", errb.String())
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
		// No .awf/ at all, so project.Open fails in both entry points.
		if err := runCheckDrift(ctx, t.TempDir(), io.Discard); err == nil {
			t.Error("expected an Open error from check repo drift")
		}
		if err := runCheckState(ctx, t.TempDir(), io.Discard); err == nil {
			t.Error("expected an Open error from check repo state")
		}
	})

	t.Run("drift render error", func(t *testing.T) {
		// A data value spelling the no-value token makes the in-memory re-render
		// fail, so p.Check() returns an error rather than a drift list.
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nvars: {}\nskills: [tdd]\nagents: []\n")
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "tdd.yaml"),
			"data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n")
		if err := runCheckDrift(ctx, root, io.Discard); err == nil {
			t.Fatal("expected check repo drift to surface the render error from p.Check()")
		}
	})

	t.Run("state working-tree error", func(t *testing.T) {
		// A drift-clean but non-git project fails the working-tree read inside
		// CheckCurrentState.
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, checkYAML)
		if err := initializeProject(testContext(t), root, io.Discard); err != nil {
			t.Fatalf("render: %v", err)
		}
		if err := runCheckState(ctx, root, io.Discard); err == nil {
			t.Fatal("expected a working-tree error from CheckCurrentState outside a git repository")
		}
	})

	t.Run("state prints warn notes", func(t *testing.T) {
		// A warn-ranked fan-out finding rides the non-failing note: channel, so the
		// command stays clean while still printing the note.
		root := syncedGitProjectFiles(t, coverageYAML(), fanoutFiles())
		var out bytes.Buffer
		if err := runCheckState(ctx, root, &out); err != nil {
			t.Fatalf("a warn-ranked finding must not fail check repo state: %v", err)
		}
		if !strings.Contains(out.String(), "note: ") {
			t.Errorf("expected a warn note on stdout:\n%s", out.String())
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

// awf help lists the check group's six children, extending the group-child
// assertion to the newly grouped command.
func TestHelpListsCheckChildren(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	var out, errb bytes.Buffer
	run([]string{"awf", "help"}, &out, &errb)
	got := out.String()
	for _, sub := range []string{"repo", "staged", "invariants"} {
		if !strings.Contains(got, "\n    "+sub+" ") {
			t.Errorf("awf help omits the check child %q:\n%s", sub, got)
		}
	}
	// The four retired top-level names are gone from the overview.
	for _, retired := range []string{"\n  invariants ", "\n  prose-gate ", "\n  memory-gate ", "\n  commit-gate "} {
		if strings.Contains(got, retired) {
			t.Errorf("awf help still lists the retired top-level %q", strings.TrimSpace(retired))
		}
	}
}
