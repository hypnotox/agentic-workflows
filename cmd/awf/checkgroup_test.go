package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
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
		{"drift", "awf check drift: clean"},
		{"state", "awf check state: clean"},
	} {
		t.Run(tc.sub, func(t *testing.T) {
			root := scaffoldProject(t)
			var out, errb bytes.Buffer
			if code := runAt(t, root, []string{"awf", "check", tc.sub}, &out, &errb); code != 0 {
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

// --staged is bare-check only. Every child declares the flag in clispec purely so
// this handler-owned diagnostic is reachable; an undeclared flag would die in
// parseArgs with a generic unknown-flag error instead.
func TestCheckChildrenRejectStaged(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	spec, ok := clispec.Lookup("check")
	if !ok {
		t.Fatal("Lookup(check) missing")
	}
	for _, child := range spec.Children {
		t.Run(child.Name, func(t *testing.T) {
			root := scaffoldProject(t)
			var out, errb bytes.Buffer
			code := runAt(t, root, []string{"awf", "check", child.Name, "--staged"}, &out, &errb)
			if code == 0 {
				t.Fatalf("check %s --staged exited 0", child.Name)
			}
			if !strings.Contains(errb.String(), "--staged applies to the bare form only") {
				t.Errorf("diagnostic = %q, want the bare-form-only message", errb.String())
			}
		})
	}
}

// guardProjectState's staged predicate carries the same `sub == ""` narrowing as
// the driver's gate switch, and this is what pins it. Without the narrowing a
// child invoked with --staged sends the guard down its staged path, which reads
// the git index: in a non-git adopted tree that fails with a snapshot error at
// exit 1 before the handler can produce the bare-form-only diagnostic. A git-backed
// fixture cannot tell the two apart, which is why this one deliberately is not.
func TestCheckChildStagedRejectionPrecedesStateGuard(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("render: %v", err)
	}
	var out, errb bytes.Buffer
	code := runAt(t, root, []string{"awf", "check", "drift", "--staged"}, &out, &errb)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (CLI misuse); stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "--staged applies to the bare form only") {
		t.Errorf("diagnostic = %q, want the bare-form-only message, not a git failure", errb.String())
	}
}

// `awf check --staged drift` puts the subcommand after the flag, so resolve never
// sees it as a child and it arrives as a positional. That earns the ordering
// message, not the unknown-subcommand one.
func TestCheckSubcommandAfterFlag(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	// The staged gate runs before the handler, so the fixture needs a committed
	// lock that satisfies it for the handler's diagnostic to be the one reached.
	lock := &manifest.Lock{
		AWFVersion: project.Version, SchemaVersion: migrate.Current(),
		Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, ADRFormatV2From: 1, ADRFormatV3From: 1, LegacyADRGaps: []int{},
	}
	b, err := lock.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	root := stagedCheckProject(t, map[string]string{
		".awf/config.yaml": "prefix: example\nskills: [tdd]\nagents: []\n",
		".awf/awf.lock":    string(b),
	}, nil)
	var out, errb bytes.Buffer
	code := runAt(t, root, []string{"awf", "check", "--staged", "drift"}, &out, &errb)
	if code == 0 {
		t.Fatal("check --staged drift exited 0")
	}
	if !strings.Contains(errb.String(), "the subcommand must come first") {
		t.Errorf("diagnostic = %q, want the ordering message", errb.String())
	}
	if strings.Contains(errb.String(), "unknown subcommand") {
		t.Errorf("a valid child must not be reported as unknown: %q", errb.String())
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
	for _, want := range []string{"drift", "state", "invariants", "prose", "memory", "commit"} {
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
		".awf/config.yaml": "prefix: ex\n",
		".awf/awf.lock":    string(b),
	})
	gitfixture.Commit(t, repo, "head", nil)
	return root
}

// The per-child gating property: `check prose` and `check memory` resolve to
// Ungated under a Gated parent, so they run against a project whose lock is
// behind this binary where bare `check` refuses. This is the half of the claim
// the driver owns; the clispec resolver's half is proved in that package.
// invariant: tooling/cli:group-child-gating-honored
func TestCheckUngatedChildrenRunOnSchemaAheadProject(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	for _, sub := range []string{"prose", "memory"} {
		t.Run(sub, func(t *testing.T) {
			root := aheadSchemaGitProject(t)
			var out, errb bytes.Buffer
			if code := runAt(t, root, []string{"awf", "check", sub}, &out, &errb); code != 0 {
				t.Fatalf("check %s exit = %d on an ahead-schema project, stderr=%q", sub, code, errb.String())
			}
			// Bare check refuses the same tree, which is what makes the child's
			// exemption meaningful rather than vacuous.
			out.Reset()
			errb.Reset()
			if code := runAt(t, root, []string{"awf", "check"}, &out, &errb); code != 1 {
				t.Fatalf("bare check exit = %d, want the version-gate refusal", code)
			}
		})
	}
}

// The per-child project-state exemption: the three hook-wired children stay
// runnable under a committed current-state journal and under an attested lock,
// where bare check refuses. Without this a commit-msg hook would refuse to
// validate a message mid-upgrade.
// invariant: tooling/cli:group-child-project-guard-exemption
func TestCheckExemptChildrenRunUnderGuardedProjectState(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	lockText := func(t *testing.T, attested bool) string {
		t.Helper()
		lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
		if attested {
			lock.BridgeAttestation = &manifest.BridgeAttestation{Version: 1, PreparedHead: "head", TreeDigest: "sha256:x", ADRFormatV1From: 2, LegacyADRGaps: []int{}}
		} else {
			lock.ADRFormatV1From = 1
			lock.ADRFormatV2From = 1
			lock.ADRFormatV3From = 1
			lock.LegacyADRGaps = []int{}
		}
		b, err := lock.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	const journal = `{"version":1,"phase":"prepared","finalLockSHA256":"sha256:x","operations":[{"path":".awf/awf.lock","prior":{"present":false,"mode":0,"content":null},"replacement":{"present":false,"mode":0,"content":null}}]}`
	configText := "prefix: example\nskills: [tdd]\nagents: []\n"

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
			// The three exempt children run anyway.
			for _, sub := range []string{"prose", "memory", "commit"} {
				args := []string{"awf", "check", sub}
				if sub == "commit" {
					msg := filepath.Join(t.TempDir(), "MSG")
					testsupport.WriteFile(t, msg, "feat(awf): a conventional subject\n")
					args = append(args, msg)
				}
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
		code := runAt(t, root, []string{"awf", "check", "drift"}, &out, &errb)
		if code != 1 {
			t.Fatalf("exit = %d, want 1; stdout=%q", code, out.String())
		}
		if !strings.Contains(errb.String(), "awf check drift:") || !strings.Contains(errb.String(), "drift(s)") {
			t.Errorf("diagnostic = %q, want a drift count", errb.String())
		}
	})

	t.Run("state", func(t *testing.T) {
		// An owned path with no scoped topic is an error-severity coverage finding.
		root := syncedGitProjectFiles(t, coverageYAML(), coverageFiles())
		var out, errb bytes.Buffer
		code := runAt(t, root, []string{"awf", "check", "state"}, &out, &errb)
		if code != 1 {
			t.Fatalf("exit = %d, want 1; stdout=%q", code, out.String())
		}
		if !strings.Contains(errb.String(), "awf check state:") || !strings.Contains(errb.String(), "current-state issue(s)") {
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
			t.Error("expected an Open error from check drift")
		}
		if err := runCheckState(ctx, t.TempDir(), io.Discard); err == nil {
			t.Error("expected an Open error from check state")
		}
	})

	t.Run("drift render error", func(t *testing.T) {
		// A data value spelling the no-value token makes the in-memory re-render
		// fail, so p.Check() returns an error rather than a drift list.
		root := t.TempDir()
		testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {}\nskills: [tdd]\nagents: []\n")
		testsupport.WriteFile(t, filepath.Join(root, ".awf", "skills", "tdd.yaml"),
			"data:\n  testSurfaces:\n    - {name: \"<no value>\", kind: k, location: l}\n")
		if err := runCheckDrift(ctx, root, io.Discard); err == nil {
			t.Fatal("expected check drift to surface the render error from p.Check()")
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
			t.Fatalf("a warn-ranked finding must not fail check state: %v", err)
		}
		if !strings.Contains(out.String(), "note: ") {
			t.Errorf("expected a warn note on stdout:\n%s", out.String())
		}
	})
}

// awf help lists the check group's six children, extending the group-child
// assertion to the newly grouped command.
func TestHelpListsCheckChildren(t *testing.T) {
	ctx := testContext(t)
	_ = ctx
	var out, errb bytes.Buffer
	run([]string{"awf", "help"}, &out, &errb)
	got := out.String()
	for _, sub := range []string{"drift", "state", "invariants", "prose", "memory", "commit"} {
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
