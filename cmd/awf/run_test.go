package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// minimalYAML is a valid tree-config for a scaffolded fixture project.
const minimalYAML = `prefix: example
vars: {testCmd: go test ./..., gateCmd: make gate}
skills: [tdd]
agents: []
`

// scaffoldProject writes a minimal tree config under a git-backed root and syncs
// it, leaving a drift-clean project. The base commit gives the working Tree a
// HEAD, which the commands that read one (check, invariants) require.
func initializeProject(root string, out io.Writer) error {
	return runSyncInitialized(root, project.InitAuthority{InitializedWithVersion: project.Version}, out)
}

func scaffoldProject(t *testing.T) string {
	t.Helper()
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	if err := initializeProject(root, io.Discard); err != nil {
		t.Fatalf("scaffold sync: %v", err)
	}
	return root
}

// invariant: tooling/upgrade-runtime:initial-adoption-version-immutable
func TestInitialAdoptionAuthorityImmutableAcrossCommands(t *testing.T) {
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, root, map[string]string{"docs/decisions/0001-existing.md": testsupport.ADR("Accepted", testsupport.WithTitle("0001: Existing"))})
	gitfixture.Commit(t, repo, root, "existing ADR", nil)
	testsupport.SwapVar(t, &isInteractive, func() bool { return false })
	// The gateCmd answer keeps the scaffold's enabled hooks singleton valid
	// for the ordinary syncs below (ADR-0156 Decision 5).
	if err := runInit(root, false, false, []string{"gateCmd=make gate"}, "", io.Discard); err != nil {
		t.Fatal(err)
	}
	initial, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	assertAuthority := func(step string) {
		t.Helper()
		got, err := manifest.Load(config.LockPath(root))
		if err != nil {
			t.Fatal(err)
		}
		if got.InitializedWithVersion != initial.InitializedWithVersion || got.ADRFormatV1From != initial.ADRFormatV1From || got.ADRFormatV2From != initial.ADRFormatV2From || !slices.Equal(got.LegacyADRGaps, initial.LegacyADRGaps) {
			t.Fatalf("%s changed authority: initial=%#v got=%#v", step, initial, got)
		}
	}
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	assertAuthority("ordinary sync")
	if err := runUpgrade(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	assertAuthority("zero-migration upgrade")
	if err := runInit(root, true, false, []string{"gateCmd=make gate"}, "", io.Discard); err != nil {
		t.Fatal(err)
	}
	assertAuthority("forced init")

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("initialize", &git.CommitOptions{Author: gitfixture.Sig, Committer: gitfixture.Sig}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*manifest.Lock)
	}{
		{"initializedWithVersion", func(lock *manifest.Lock) {
			lock.InitializedWithVersion = "0.18.0"
			if lock.InitializedWithVersion == initial.InitializedWithVersion {
				lock.InitializedWithVersion = "0.17.0"
			}
		}},
		{"adrFormatV1From", func(lock *manifest.Lock) { lock.ADRFormatV1From++; lock.ADRFormatV2From++ }},
		{"adrFormatV2From", func(lock *manifest.Lock) { lock.ADRFormatV2From++ }},
		{"legacyAdrGaps", func(lock *manifest.Lock) { lock.LegacyADRGaps = []int{1} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := *initial
			mutated.LegacyADRGaps = slices.Clone(initial.LegacyADRGaps)
			tc.mutate(&mutated)
			if err := mutated.Save(config.LockPath(root)); err != nil {
				t.Fatal(err)
			}
			if _, err := wt.Add(".awf/awf.lock"); err != nil {
				t.Fatal(err)
			}
			if err := runCheck(root, true, io.Discard); err == nil || !strings.Contains(err.Error(), "immutable") || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("staged %s mutation error = %v", tc.name, err)
			}
			if err := initial.Save(config.LockPath(root)); err != nil {
				t.Fatal(err)
			}
			if _, err := wt.Add(".awf/awf.lock"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInitSeedsEmptyAuthority(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &isInteractive, func() bool { return false })
	if err := runInit(root, false, false, nil, "", io.Discard); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if lock.InitializedWithVersion != project.Version || lock.ADRFormatV1From != 1 || lock.ADRFormatV2From != 1 || lock.LegacyADRGaps == nil || len(lock.LegacyADRGaps) != 0 {
		t.Fatalf("authority = version %q cutoffs %d/%d gaps %v", lock.InitializedWithVersion, lock.ADRFormatV1From, lock.ADRFormatV2From, lock.LegacyADRGaps)
	}
}

func TestInitSealsBrownfieldAuthority(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &isInteractive, func() bool { return false })
	one := testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0001: One"))
	three := testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0003: Three"))
	onePath := filepath.Join(root, "docs/decisions/0001-one.md")
	threePath := filepath.Join(root, "docs/decisions/0003-three.md")
	testsupport.WriteFile(t, onePath, one)
	testsupport.WriteFile(t, threePath, three)
	if err := runInit(root, false, false, nil, "", io.Discard); err != nil {
		t.Fatal(err)
	}
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if lock.ADRFormatV1From != 4 || lock.ADRFormatV2From != 4 || len(lock.LegacyADRGaps) != 1 || lock.LegacyADRGaps[0] != 2 {
		t.Fatalf("cutoffs/gaps = %d/%d/%v", lock.ADRFormatV1From, lock.ADRFormatV2From, lock.LegacyADRGaps)
	}
	for path, want := range map[string]string{onePath: one, threePath: three} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("brownfield ADR changed: %s err=%v", path, err)
		}
	}
}

func TestInitRejectsAmbiguousBrownfieldAuthority(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"malformed", map[string]string{"docs/decisions/0001-bad.md": "---\nstatus: [bad\n---\n"}},
		{"duplicate", map[string]string{
			"docs/decisions/0001-one.md": testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0001: One")),
			"docs/decisions/0001-two.md": testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0001: Two")),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.SwapVar(t, &isInteractive, func() bool { return false })
			for path, body := range tc.files {
				testsupport.WriteFile(t, filepath.Join(root, path), body)
			}
			before := snapshotTree(t, root)
			var out bytes.Buffer
			if err := runInit(root, false, false, nil, "", &out); err == nil {
				t.Fatal("expected refusal")
			}
			if after := snapshotTree(t, root); after != before {
				t.Fatal("ambiguous first adoption mutated the repository tree")
			}
			if out.Len() != 0 {
				t.Fatalf("ambiguous first adoption wrote output: %q", out.String())
			}
		})
	}
}

func TestInitForcePreservesAuthority(t *testing.T) {
	root := t.TempDir()
	testsupport.SwapVar(t, &isInteractive, func() bool { return false })
	// A forced re-init over an existing config runs an ordinary sync, so the
	// gateCmd answer keeps its enabled hooks singleton valid (ADR-0156).
	if err := runInit(root, false, false, []string{"gateCmd=make gate"}, "", io.Discard); err != nil {
		t.Fatal(err)
	}
	before, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, true, false, []string{"gateCmd=make gate"}, "", io.Discard); err != nil {
		t.Fatal(err)
	}
	after, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if before.InitializedWithVersion != after.InitializedWithVersion || before.ADRFormatV1From != after.ADRFormatV1From || before.ADRFormatV2From != after.ADRFormatV2From || !slices.Equal(before.LegacyADRGaps, after.LegacyADRGaps) {
		t.Fatalf("authority changed: before=%#v after=%#v", before, after)
	}
}

func TestInitForceRefusesMissingAuthority(t *testing.T) {
	for _, tc := range []struct{ name, lock, want string }{
		{"missing", "", "use the bridge release to attest"},
		{"bridge", `{"awfVersion":"0.19.0","schemaVersion":14,"files":{},"bridgeAttestation":{"version":1,"preparedHead":"x","treeDigest":"sha256:x","adrFormatV1From":1,"legacyADRGaps":[]}}`, "use the bridge release to attest"},
		{"invalid", `{"awfVersion":"0.19.0","schemaVersion":14,"files":{},"adrFormatV1From":1}`, "restore .awf/awf.lock from version control"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteAwfConfig(t, root, minimalYAML)
			if tc.lock != "" {
				testsupport.WriteFile(t, config.LockPath(root), tc.lock)
			}
			before := snapshotTree(t, root)
			var out bytes.Buffer
			err := runInit(root, true, false, nil, "", &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
			if after := snapshotTree(t, root); after != before {
				t.Fatal("forced init authority refusal mutated the repository tree")
			}
			if out.Len() != 0 {
				t.Fatalf("forced init authority refusal wrote output: %q", out.String())
			}
		})
	}
}

func testInitFirstADRChecksClean(t *testing.T) {
	for _, tc := range []struct {
		name   string
		legacy []string
		cutoff int
		gaps   []int
	}{
		{name: "fresh", cutoff: 1, gaps: []int{}},
		{name: "brownfield", legacy: []string{"0001-old.md", "0003-old.md"}, cutoff: 4, gaps: []int{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, root := gitfixture.InitRepo(t)
			gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
			for _, name := range tc.legacy {
				testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", name), testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle(name[:4]+": Old")))
			}
			testsupport.SwapVar(t, &isInteractive, func() bool { return false })
			// The gateCmd answer keeps the scaffold's enabled hooks singleton
			// valid for the post-init syncs (ADR-0156 Decision 5).
			if err := runInit(root, false, false, []string{"gateCmd=make gate"}, "", io.Discard); err != nil {
				t.Fatal(err)
			}
			lock, err := manifest.Load(config.LockPath(root))
			if err != nil {
				t.Fatal(err)
			}
			if lock.ADRFormatV1From != tc.cutoff || lock.ADRFormatV2From != tc.cutoff || !slices.Equal(lock.LegacyADRGaps, tc.gaps) {
				t.Fatalf("initial authority = cutoffs %d/%d gaps %v, want %d/%d gaps %v", lock.ADRFormatV1From, lock.ADRFormatV2From, lock.LegacyADRGaps, tc.cutoff, tc.cutoff, tc.gaps)
			}
			wt, err := repo.Worktree()
			if err != nil {
				t.Fatal(err)
			}
			if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := wt.Commit("initialize", &git.CommitOptions{Author: gitfixture.Sig, Committer: gitfixture.Sig}); err != nil {
				t.Fatal(err)
			}
			if err := runNew(root, "adr", []string{"First", "Current"}, io.Discard); err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf("%04d-", tc.cutoff)
			entries, err := os.ReadDir(filepath.Join(root, "docs/decisions"))
			if err != nil {
				t.Fatal(err)
			}
			var created string
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), want) {
					created = filepath.Join(root, "docs/decisions", entry.Name())
				}
			}
			if created == "" {
				t.Fatalf("new ADR not created at cutoff %d", tc.cutoff)
			}
			body, err := os.ReadFile(created)
			if err != nil {
				t.Fatal(err)
			}
			text := string(body)
			if !strings.Contains(text, "format: current-state-v2\n") {
				t.Fatalf("new ADR at cutoff %d is not current-state-v2", tc.cutoff)
			}
			start, end := strings.Index(text, "## State changes\n"), strings.Index(text, "## Consequences\n")
			if start < 0 || end < 0 || end <= start {
				t.Fatal("scaffold lacks state-change section")
			}
			text = text[:start] + "## State changes\n\nNone.\n\n" + text[end:]
			history := strings.Index(text, "## Status history\n")
			if history < 0 {
				t.Fatal("scaffold lacks status history")
			}
			text = text[:history] + "## Status history\n\n- 2026-07-21: Proposed\n"
			if err := os.WriteFile(created, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := runSync(root, io.Discard); err != nil {
				t.Fatal(err)
			}
			if err := runCheck(root, false, io.Discard); err != nil {
				t.Fatalf("check: %v", err)
			}
		})
	}
}

func TestRunUpgradeAddsExploringAtSchemaThirteen(t *testing.T) {
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {}\nskills: [debugging]\nagents: []\n")
	lock := &manifest.Lock{AWFVersion: "0.17.0", SchemaVersion: 12, Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, LegacyADRGaps: []int{}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runUpgrade(root, &out); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	for _, want := range []string{
		`close-enabled-set: enabled skill "exploring" (required by "debugging")`,
		"awf upgrade: applied exploring-skill-closure",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("upgrade output missing %q:\n%s", want, out.String())
		}
	}
	upgradedLock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if upgradedLock.SchemaVersion != 18 {
		t.Errorf("lock schema = %d, want 18", upgradedLock.SchemaVersion)
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, skill := range cfg.Skills {
		if skill == "exploring" {
			found = true
		}
	}
	if !found {
		t.Errorf("upgraded skills missing exploring: %v", cfg.Skills)
	}
	if err := runCheck(root, false, io.Discard); err != nil {
		t.Errorf("post-upgrade check: %v", err)
	}
}

func TestRunSyncPrintsPrunedFiles(t *testing.T) {
	root := scaffoldProject(t)
	// Disable the only skill; the re-sync prunes its rendered file and says so.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "skills: [tdd]", "skills: []", 1))
	var out bytes.Buffer
	if err := runSync(root, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "awf sync: pruned .claude/skills/example-tdd/SKILL.md\n") {
		t.Errorf("missing prune line:\n%s", out.String())
	}
	// A drift-clean re-sync prints no prune lines.
	out.Reset()
	if err := runSync(root, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "pruned") {
		t.Errorf("routine re-sync must not report prunes:\n%s", out.String())
	}
}

func TestRunSyncPrintsChangedFiles(t *testing.T) {
	root := scaffoldProject(t)
	// A var edit moves the config hash of every artifact referencing it; the
	// re-sync attributes the changed output to the project's own inputs.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: ./x gate", 1))
	var out bytes.Buffer
	if err := runSync(root, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "awf sync: changed .claude/skills/example-tdd/SKILL.md (config)\n") {
		t.Errorf("missing config-cause change line:\n%s", out.String())
	}
	// A drift-clean re-sync prints no change lines.
	out.Reset()
	if err := runSync(root, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "changed") || strings.Contains(out.String(), "added") {
		t.Errorf("routine re-sync must not report changes:\n%s", out.String())
	}
	// Enabling an artifact reports its files as added.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: ./x gate", 1)+"docs: [pitfalls]\n")
	out.Reset()
	if err := runSync(root, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "awf sync: added docs/pitfalls.md\n") {
		t.Errorf("missing added line:\n%s", out.String())
	}
}

func TestRunNoArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for no args, got %d", code)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("missing usage text: %q", errb.String())
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		var out, errb bytes.Buffer
		if code := run([]string{"awf", arg}, &out, &errb); code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", arg, code)
		}
		if !strings.Contains(out.String(), "Commands:") || !strings.Contains(out.String(), "uninstall") {
			t.Errorf("%s: help text missing content:\n%s", arg, out.String())
		}
	}
}

func TestRunGetwdError(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return "", errors.New("boom") })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "sync"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on getwd error, got %d", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for unknown command, got %d", code)
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("missing unknown-command text: %q", errb.String())
	}
}

func TestRunAddMissingSkillArg(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "enable"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for add without skill, got %d", code)
	}
}

func TestRunArgValidation(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"awf", "check", "--bogus"}, "unknown flag"},
		{"unexpected positional", []string{"awf", "sync", "extra"}, "unexpected arguments"},
		{"value flag without value", []string{"awf", "context", "--range"}, "needs a value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(c.args, &out, &errb); code != 2 {
				t.Fatalf("expected exit 2, got %d", code)
			}
			if !strings.Contains(errb.String(), c.want) {
				t.Errorf("missing %q in stderr: %q", c.want, errb.String())
			}
		})
	}
}

func TestRunDispatchError(t *testing.T) {
	// sync in a bare dir: project.Open fails -> handler error -> exit 1.
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "sync"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on dispatch error, got %d", code)
	}
	if !strings.HasPrefix(errb.String(), "awf:") {
		t.Errorf("expected awf-prefixed error, got %q", errb.String())
	}
}

// TestRunDispatchArms drives every switch arm through run() against a scaffolded
// project, covering the dispatch statements.
func TestRunDispatchArms(t *testing.T) {
	for _, cmd := range []string{"sync", "check", "invariants", "list", "upgrade"} {
		t.Run(cmd, func(t *testing.T) {
			root := scaffoldProject(t)
			testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
			var out, errb bytes.Buffer
			if code := run([]string{"awf", cmd}, &out, &errb); code != 0 {
				t.Fatalf("%s: expected exit 0, got %d (%s)", cmd, code, errb.String())
			}
		})
	}
	t.Run("enable", func(t *testing.T) {
		root := t.TempDir()
		awf := filepath.Join(root, ".awf")
		if err := os.MkdirAll(awf, 0o755); err != nil {
			t.Fatal(err)
		}
		// skills: [] so a fresh skill can be added.
		cfg := strings.Replace(minimalYAML, "skills: [tdd]", "skills: []", 1)
		if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initializeProject(root, io.Discard); err != nil {
			t.Fatal(err)
		}
		testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "enable", "skill", "tdd"}, &out, &errb); code != 0 {
			t.Fatalf("add: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
	t.Run("init", func(t *testing.T) {
		root := t.TempDir()
		testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
			t.Fatalf("init: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
}

// TestHandlersOnBareDirError covers each handler's project.Open error return.
func TestHandlersOnBareDirError(t *testing.T) {
	bare := func(t *testing.T) string { return t.TempDir() }
	t.Run("check", func(t *testing.T) {
		if err := runCheck(bare(t), false, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("invariants", func(t *testing.T) {
		if err := runInvariants(bare(t), io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("list", func(t *testing.T) {
		if err := runList(bare(t), "", io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("new", func(t *testing.T) {
		if err := runNew(bare(t), "adr", []string{"x"}, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("enable", func(t *testing.T) {
		if err := runEnable(bare(t), "skill", "tdd", false, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("disable", func(t *testing.T) {
		if err := runDisable(bare(t), "skill", "tdd", false, false, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
}

func TestRunInvariantsLoadFault(t *testing.T) {
	// A malformed ADR makes the working-tree corpus load error out of runInvariants.
	root := scaffoldProject(t)
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte("---\n: : bad yaml : :\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInvariants(root, io.Discard); err == nil {
		t.Error("expected a corpus load error on a malformed ADR")
	}
}

func TestRunCheckErrorPaths(t *testing.T) {
	t.Run("stale-schema", func(t *testing.T) {
		root := t.TempDir()
		claude := filepath.Join(root, ".claude")
		if err := os.MkdirAll(claude, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// check is Gated: the driver refuses a stale schema before the handler.
		var out, errb bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check"}, &out, &errb); code != 1 {
			t.Errorf("expected the driver to refuse check on stale schema, got %d", code)
		}
	})
	t.Run("check-error-malformed-adr", func(t *testing.T) {
		// A malformed ADR makes p.Check() (INDEX.md generation) error.
		root := scaffoldProject(t)
		adrDir := filepath.Join(root, "docs", "decisions")
		if err := os.MkdirAll(adrDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte("---\n: : bad yaml : :\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runCheck(root, false, io.Discard); err == nil {
			t.Error("expected check error on a malformed ADR")
		}
	})
}

func TestRunListSidecarError(t *testing.T) {
	// A malformed sidecar for the enabled skill makes Sidecar() error.
	root := scaffoldProject(t)
	skillsDir := filepath.Join(root, ".awf", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "tdd.yaml"), []byte("data: [not, a, map]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runList(root, "", io.Discard); err == nil {
		t.Error("expected Sidecar parse error")
	}
}

func TestRunSyncSyncError(t *testing.T) {
	// A directory squatting on a rendered output path makes p.SyncReport() fail.
	root := scaffoldProject(t)
	out := filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil { // SKILL.md is now a directory
		t.Fatal(err)
	}
	if err := runSync(root, io.Discard); err == nil {
		t.Error("expected Sync error when an output path is a directory")
	}
}

func TestRunInitSyncError(t *testing.T) {
	// Config exists (skip scaffold); a squatting output dir makes the inner
	// runSync fail, covering runInit's runSync error return.
	root := scaffoldProject(t)
	out := filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, false, false, nil, "", io.Discard); err == nil {
		t.Error("expected runInit to surface the sync error")
	}
}

func TestRunUpgradeAppliesLegacyMigration(t *testing.T) {
	// A legacy single-file project migrates to the tree layout, covering the
	// applied-migrations loop and the terminal sync.
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "prefix: example\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\nskills: {}\nagents: {}\n"
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: "0.1.0", Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, LegacyADRGaps: []int{}}).Save(filepath.Join(claude, "awf.lock")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runUpgrade(root, &out); err != nil {
		t.Fatalf("runUpgrade legacy: %v", err)
	}
	if !strings.Contains(out.String(), "applied") {
		t.Errorf("expected an applied migration, got %q", out.String())
	}
}

// A schema-7 config the ADR-0081 closure validation refuses is repaired by
// awf upgrade: close-enabled-set closes the enabled set, then the terminal
// sync opens it cleanly.
func TestRunUpgradeRepairsUnclosedConfig(t *testing.T) {
	repo, root := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, root, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {}\nskills: [brainstorming]\nagents: []\n")
	lock := &manifest.Lock{SchemaVersion: 7, Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, LegacyADRGaps: []int{}}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root, false, io.Discard); err == nil {
		t.Fatal("pre-upgrade check should refuse (schema gate)")
	}
	var out bytes.Buffer
	if err := runUpgrade(root, &out); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if !strings.Contains(out.String(), `close-enabled-set: enabled skill "proposing-adr" (required by "brainstorming")`) {
		t.Errorf("expected closure additions printed, got %q", out.String())
	}
	if err := runCheck(root, false, io.Discard); err != nil {
		t.Errorf("post-upgrade check should pass, got %v", err)
	}
}

func TestRunUpgradeMigrationError(t *testing.T) {
	// A legacy config that fails to parse makes the migration error.
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte(": : not valid : :\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: "0.1.0", Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, LegacyADRGaps: []int{}}
	if err := lock.Save(filepath.Join(claude, "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runUpgrade(root, io.Discard); err == nil {
		t.Error("expected migration error for a malformed legacy config")
	}
}

// invariant: tooling/cli:single-os-exit
func TestNoOsExitOutsideMain(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "os.Exit") {
				continue
			}
			// The sole permitted os.Exit is main's one-line wrapper.
			if f == "main.go" && strings.Contains(line, "func main()") {
				continue
			}
			t.Errorf("%s:%d: os.Exit outside main's wrapper: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}

func TestGateRejectsStaleSchema(t *testing.T) {
	// A legacy single-file layout (.claude/awf.yaml, no tree config) reports
	// generation 0 -> GateState "gate".
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gate(root); err == nil {
		t.Fatal("expected gate to reject stale schema")
	}
	// sync is Gated: the driver surfaces the same gate error before the handler.
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "sync"}, &out, &errb); code != 1 {
		t.Errorf("expected the driver to fail sync on stale schema, got %d", code)
	}
}

func TestProbeCollisionsOpenError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: [bad\n")
	if _, err := probeCollisions(root); err == nil {
		t.Fatal("expected config open error")
	}
	if err := runInit(root, false, false, nil, "", io.Discard); err == nil {
		t.Fatal("expected init probe error")
	}
}

func TestRunInitOnExistingConfigSkipsScaffold(t *testing.T) {
	// Pre-existing config -> scaffold branch is skipped; init still syncs.
	root := scaffoldProject(t)
	if err := runInit(root, false, false, nil, "", io.Discard); err != nil {
		t.Fatalf("runInit on existing config: %v", err)
	}
}

func TestRunInvariantsNoClaims(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runInvariants(root, &out); err != nil {
		t.Fatalf("runInvariants: %v", err)
	}
	if !strings.Contains(out.String(), "no invariant claims") {
		t.Errorf("expected the no-claims report, got %q", out.String())
	}
}

func TestRunListPrintsSkills(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runList(root, "", &out); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(out.String(), "tdd") {
		t.Errorf("expected tdd in listing, got %q", out.String())
	}
}

// invariant: tooling/cli:upgrade-always-syncs
func TestRunUpgradeAlreadyCurrentStillSyncs(t *testing.T) {
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runUpgrade(root, &out); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if !strings.Contains(out.String(), "already at schema") {
		t.Errorf("expected already-at-schema report, got %q", out.String())
	}
	// The zero-migrations path must still sync: a same-schema binary bump
	// re-renders every managed file and re-pins the bootstrap (ADR-0085).
	if !strings.Contains(out.String(), "awf sync: done") {
		t.Errorf("expected the no-op upgrade to run a sync, got %q", out.String())
	}
}

// invariant: tooling/init-and-enablement:init-collision-guard
func TestInitGuardBlocksAndForceOverrides(t *testing.T) {
	root := t.TempDir()
	// A pre-existing, non-awf CLAUDE.md is a collision.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail on collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	// Nothing written: the scaffolded config tree was rolled back.
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("expected .awf to be rolled back")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) != "mine\n" {
		t.Fatalf("CLAUDE.md clobbered: %q", b)
	}
	// --force backs up the colliding file, then overwrites and completes.
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("init --force failed: %s", errb.String())
	}
	// The original is preserved at <path>.awf-bak.
	// invariant: tooling/init-and-enablement:init-force-backs-up
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak")); string(b) != "mine\n" {
		t.Fatalf("CLAUDE.md.awf-bak = %q, want original %q", b, "mine\n")
	}
	// And the live file was overwritten with managed content.
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) == "mine\n" {
		t.Fatalf("CLAUDE.md should have been overwritten, still %q", b)
	}
	if !strings.Contains(out.String(), "backed up CLAUDE.md") {
		t.Errorf("expected backup report on stdout, got %q", out.String())
	}
	// Regression: init delegates its backup to the chained sync (one BackupFile path,
	// ADR-0035), so the colliding file is backed up exactly once - no double-backup.
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md.awf-bak.1")); !os.IsNotExist(err) {
		t.Error("expected exactly one backup; CLAUDE.md.awf-bak.1 should not exist")
	}
}

func TestInitRollbackPreservesExistingAwf(t *testing.T) {
	root := t.TempDir()
	// Pre-existing authored .awf/ content but no config.yaml -> init scaffolds config,
	// then a collision (non-managed CLAUDE.md) forces a refusal + rollback.
	part := filepath.Join(root, ".awf", "skills", "parts", "foo", "extra.md")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("hand-authored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, false, false, nil, "", io.Discard); err == nil {
		t.Fatal("expected init to refuse on collision")
	}
	// The scaffolded config.yaml is rolled back...
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Error("config.yaml should have been removed on rollback")
	}
	// ...but the pre-existing authored content survives.
	if _, err := os.Stat(part); err != nil {
		t.Errorf("pre-existing .awf content must be preserved, got: %v", err)
	}
}

func TestInitForceBackupDoesNotClobberPriorBak(t *testing.T) {
	root := t.TempDir()
	// A colliding CLAUDE.md plus a pre-existing backup from an earlier --force.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md.awf-bak"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("init --force: %s", errb.String())
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak")); string(b) != "v1\n" {
		t.Errorf("prior .awf-bak clobbered: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak.1")); string(b) != "v2\n" {
		t.Errorf("CLAUDE.md.awf-bak.1 = %q, want v2", b)
	}
}

func TestInitIdempotentReinitNoCollision(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("first init failed: %s", errb.String())
	}
	// Re-init over the now-managed tree: every planned path is in the prior lock,
	// so p.InitCollisions skips them all and init proceeds without --force.
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("re-init failed: %s", errb.String())
	}
}

func TestInitCollisionsOpenError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".awf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unknown field → strict config.Load fails → project.Open errors inside runInit.
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("bogusField: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: 14, Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, LegacyADRGaps: []int{}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail when project.Open errors")
	}
	// --force skips the probe, so the same malformed config now fails at
	// runInit's own post-scaffold project.Open - keeping that branch covered.
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code == 0 {
		t.Fatal("expected init --force to fail when project.Open errors")
	}
}

func TestInitAbortsWhenInitCollisionsFails(t *testing.T) {
	root := t.TempDir()
	// An existing permanent project can still have a malformed ADR. --force skips
	// the probe so runInit's own p.InitCollisions call forwards that deterministic
	// planning error.
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	if err := (&manifest.Lock{AWFVersion: project.Version, SchemaVersion: 14, Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, LegacyADRGaps: []int{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	dd := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(dd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dd, "0099-bad.md"), []byte("---\nstatus: [unclosed\n---\n# Bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail when p.InitCollisions errors")
	}
}

func TestSyncReportsIndexOwnershipTakeover(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	// Foreign ADR index present before any sync (no lock yet).
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "INDEX.md"), []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: project.Version, SchemaVersion: 18, Files: map[string]manifest.Entry{}, ADRFormatV1From: 1, ADRFormatV2From: 1, LegacyADRGaps: []int{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "sync"}, &out, &errb); code != 0 {
		t.Fatalf("sync: %s", errb.String())
	}
	if !strings.Contains(out.String(), "backed up docs/decisions/INDEX.md") {
		t.Errorf("missing backup line: %q", out.String())
	}
	if !strings.Contains(out.String(), "note: awf now generates") {
		t.Errorf("missing ownership-takeover note: %q", out.String())
	}
}

// A collision refuses BEFORE any prompt: with a colliding AGENTS.md and an
// interactive stdin, init exits without emitting a single prompt line and
// without creating .awf/.
func TestInitCollisionProbeRefusesBeforePrompts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	testsupport.SwapVar(t, &isInteractive, func() bool { return true })
	testsupport.SwapVar(t, &stdin, io.Reader(strings.NewReader("SHOULD-NOT-BE-CONSUMED\n")))
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to refuse on collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	if out.String() != "" {
		t.Errorf("prompt text emitted before the collision refusal:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf")); !os.IsNotExist(err) {
		t.Errorf(".awf/ should not exist after a probe refusal (err=%v)", err)
	}
}

// A trim answer can enable a non-core artifact the curated-core probe set does
// not cover: the probe passes, and the accurate post-answer check still
// refuses and rolls the scaffolded config back. (The leaves-only trim derives
// zero agents under ADR-0081, so the selection is closure-valid.)
func TestInitPostAnswerCollisionAfterProbePasses(t *testing.T) {
	root := t.TempDir()
	skillPath := filepath.Join(root, ".claude", "skills", filepath.Base(root)+"-tdd", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--set", "skills=tdd"}, &out, &errb); code == 0 {
		t.Fatal("expected init to refuse on the post-answer collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Error("scaffolded config should have been rolled back")
	}
}
