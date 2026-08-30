package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestControlRootPrimaryAndLinkedWorktreeShareAuthority(t *testing.T) {
	for _, commonForm := range []string{"relative", "absolute"} {
		t.Run(commonForm+"-commondir-detached-head-and-spaces", func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "repository fixtures with spaces")
			primary := filepath.Join(base, " primary checkout ")
			initNativeRepo(t, primary)
			linked := filepath.Join(base, " linked checkout ")
			runGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")

			gitdir := gitdirFromFile(t, filepath.Join(linked, ".git"))
			commondirPath := filepath.Join(gitdir, "commondir")
			commondir := trimGitOutputLine(readFile(t, commondirPath))
			if filepath.IsAbs(commondir) {
				t.Fatalf("native linked-worktree commondir = %q, want relative", commondir)
			}
			if commonForm == "absolute" {
				common := trimGitOutputLine(runGit(t, "-C", linked, "rev-parse", "--path-format=absolute", "--git-common-dir"))
				writeFile(t, commondirPath, common+"\n")
				if rewritten := trimGitOutputLine(readFile(t, commondirPath)); !filepath.IsAbs(rewritten) {
					t.Fatalf("rewritten linked-worktree commondir = %q, want absolute", rewritten)
				}
			}

			primaryRoots, err := awfgit.ResolveControlRoots(testContext(t), primary)
			if err != nil {
				t.Fatalf("resolve primary checkout: %v", err)
			}
			linkedRoots, err := awfgit.ResolveControlRoots(testContext(t), linked)
			if err != nil {
				t.Fatalf("resolve linked checkout: %v", err)
			}

			wantCommon := trimGitOutputLine(runGit(t, "-C", primary, "rev-parse", "--path-format=absolute", "--git-common-dir"))
			assertRoots(t, primaryRoots, primary, wantCommon, primary)
			assertRoots(t, linkedRoots, linked, wantCommon, primary)
			if primaryRoots.PrimaryRoot != linkedRoots.PrimaryRoot {
				t.Fatalf("primary roots differ: primary call %q, linked call %q", primaryRoots.PrimaryRoot, linkedRoots.PrimaryRoot)
			}
			if primaryRoots.InvokingRoot == linkedRoots.InvokingRoot {
				t.Fatalf("invoking roots collapsed to %q", primaryRoots.InvokingRoot)
			}
		})
	}
}

// User-authorized parity: a default --separate-git-dir linked worktree has no authoritative reverse primary mapping, so it is non-forceable missing-primary; the direct primary remains resolvable.
func TestControlRootSeparateGitDirRepository(t *testing.T) {
	base := filepath.Join(t.TempDir(), "separate git dir fixture")
	primary := filepath.Join(base, " checkout with spaces ")
	common := filepath.Join(base, " metadata with spaces.git ")
	initNativeSeparateGitDirRepo(t, primary, common)

	if got := trimGitOutputLine(runGit(t, "-C", primary, "rev-parse", "--show-toplevel")); got != cleanAbsolute(t, primary) {
		t.Fatalf("separate-git-dir show-toplevel = %q, want %q", got, cleanAbsolute(t, primary))
	}
	if got := trimGitOutputLine(runGit(t, "-C", primary, "rev-parse", "--path-format=absolute", "--git-dir")); got != cleanAbsolute(t, common) {
		t.Fatalf("separate-git-dir absolute git-dir = %q, want %q", got, cleanAbsolute(t, common))
	}
	if got := trimGitOutputLine(runGit(t, "-C", primary, "rev-parse", "--path-format=absolute", "--git-common-dir")); got != cleanAbsolute(t, common) {
		t.Fatalf("separate-git-dir absolute common-dir = %q, want %q", got, cleanAbsolute(t, common))
	}
	assertSingleListedNonBarePrimary(t, primary, primary, common)
	linked := filepath.Join(base, " linked checkout with spaces ")
	runGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")

	roots, err := awfgit.ResolveControlRoots(testContext(t), primary)
	if err != nil {
		t.Fatalf("resolve separate-git-dir checkout: %v", err)
	}
	assertRoots(t, roots, primary, common, primary)
	_, err = awfgit.ResolveControlRoots(testContext(t), linked)
	requireNonForceableHardSafety(t, err, "missing-primary", common)
	resident, err := roots.ResidentRoot(awfgit.ResidentEfforts)
	if err != nil {
		t.Fatalf("resolve efforts resident root: %v", err)
	}
	if want := filepath.Join(cleanAbsolute(t, primary), ".awf", "efforts"); resident != want {
		t.Fatalf("resident root = %q, want %q", resident, want)
	}
}

func TestRunGitIsolatesHostileConfigurationEnvironment(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_0", "hostile-inherited-branch")
	t.Setenv("GIT_CONFIG_KEY_1", "commit.gpgSign")
	t.Setenv("GIT_CONFIG_VALUE_1", "true")
	t.Setenv("GIT_CONFIG_KEY_99", "malformed.stray-key")
	t.Setenv("GIT_CONFIG_VALUE_99", "malformed-stray-value")
	t.Setenv("GIT_CONFIG", filepath.Join(t.TempDir(), "missing-hostile-repository-config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "missing-hostile-global"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "0")
	t.Setenv("GIT_CONFIG_PARAMETERS", "'init.defaultBranch'='hostile-parameter-branch'")
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "missing-hostile-system"))
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GIT_ASKPASS", "false")
	t.Setenv("SSH_ASKPASS", "false")
	t.Setenv("GCM_INTERACTIVE", "Always")

	primary := filepath.Join(t.TempDir(), "hostile environment checkout")
	initNativeRepo(t, primary)
	if got := strings.TrimSpace(runGit(t, "-C", primary, "symbolic-ref", "--short", "HEAD")); got == "hostile-inherited-branch" {
		t.Fatalf("inherited command-scope Git configuration selected branch %q", got)
	}
	roots, err := awfgit.ResolveControlRoots(testContext(t), primary)
	if err != nil {
		t.Fatalf("resolve with hostile Git environment: %v", err)
	}
	common := strings.TrimSpace(runGit(t, "-C", primary, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	assertRoots(t, roots, primary, common, primary)
}

func TestControlRootRejectsMalformedWorktreeRecords(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake native Git fixture requires a POSIX script")
	}
	cases := map[string]string{
		"stream-without-NUL-termination": "worktree %s\\000HEAD abcdef\\000branch refs/heads/main",
		"record-without-NUL-delimiter":   "worktree %s\\000HEAD abcdef\\000branch refs/heads/main\\000",
		"prunable-without-reason":        "worktree %s\\000HEAD abcdef\\000branch refs/heads/main\\000prunable\\000\\000",
		"prunable-with-blank-reason":     "worktree %s\\000HEAD abcdef\\000branch refs/heads/main\\000prunable   \\000\\000",
		"bare-and-prunable":              "worktree %s\\000bare\\000prunable administrative-reason\\000\\000",
		"non-bare-without-HEAD":          "worktree %s\\000branch refs/heads/main\\000\\000",
		"non-bare-without-state":         "worktree %s\\000HEAD abcdef\\000\\000",
		"non-bare-with-two-states":       "worktree %s\\000HEAD abcdef\\000branch refs/heads/main\\000detached\\000\\000",
		"bare-with-HEAD":                 "worktree %s\\000HEAD abcdef\\000bare\\000\\000",
		"bare-with-branch":               "worktree %s\\000branch refs/heads/main\\000bare\\000\\000",
		"bare-with-detached":             "worktree %s\\000detached\\000bare\\000\\000",
	}
	for name, topology := range cases {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "malformed repository")
			if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(t.TempDir(), "bin")
			git := filepath.Join(bin, "git")
			script := "#!/bin/sh\n" +
				"if [ \"$3\" = rev-parse ]; then\n" +
				"  case \"$4\" in\n" +
				"    --is-bare-repository) printf 'false\\n' ;;\n" +
				"    --show-toplevel) printf '%s\\n' \"$2\" ;;\n" +
				"    --path-format=absolute) printf '%s/.git\\n' \"$2\" ;;\n" +
				"  esac\n" +
				"elif [ \"$3\" = worktree ]; then\n" +
				"  printf '" + topology + "' \"$2\"\n" +
				"fi\n"
			writeFile(t, git, script)
			if err := os.Chmod(git, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

			_, err := awfgit.ResolveControlRoots(testContext(t), root)
			requireNonForceableHardSafety(t, err, "unconfined", filepath.Join(root, ".git"))
		})
	}
}

func TestControlRootRejectsBareRepositoryAsNonForceable(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare repository with spaces.git")
	runGit(t, "init", "--bare", bare)
	_, err := awfgit.ResolveControlRoots(testContext(t), bare)
	requireNonForceableHardSafety(t, err, "bare-repository", bare)
}

func TestControlRootRejectsAmbiguousOrMissingPrimaryEntry(t *testing.T) {
	t.Run("ambiguous-primary-identity", func(t *testing.T) {
		base := t.TempDir()
		primary := filepath.Join(base, "primary")
		initNativeRepo(t, primary)
		linked := filepath.Join(base, "linked")
		runGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")
		common := strings.TrimSpace(runGit(t, "-C", primary, "rev-parse", "--path-format=absolute", "--git-common-dir"))
		writeFile(t, filepath.Join(linked, ".git"), "gitdir: "+common+"\n")

		_, err := awfgit.ResolveControlRoots(testContext(t), primary)
		requireNonForceableHardSafety(t, err, "ambiguous-primary", common)
	})

	t.Run("missing-primary-identity", func(t *testing.T) {
		base := t.TempDir()
		primary := filepath.Join(base, "primary")
		common := filepath.Join(base, "metadata.git")
		initNativeSeparateGitDirRepo(t, primary, common)
		linked := filepath.Join(base, "linked")
		runGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")
		if err := os.Remove(filepath.Join(primary, ".git")); err != nil {
			t.Fatal(err)
		}

		_, err := awfgit.ResolveControlRoots(testContext(t), linked)
		requireNonForceableHardSafety(t, err, "missing-primary", common)
	})
}

func TestControlRootRejectsMissingNonPrunableWorktreeGitFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake native Git fixture requires a POSIX script")
	}
	base := t.TempDir()
	primary := filepath.Join(base, "primary")
	initNativeRepo(t, primary)
	linked := filepath.Join(base, "linked")
	if err := os.Mkdir(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	git := filepath.Join(bin, "git")
	script := "#!/bin/sh\n" +
		"if [ \"$3\" = rev-parse ]; then\n" +
		"  case \"$4\" in\n" +
		"    --is-bare-repository) printf 'false\\n' ;;\n" +
		"    --show-toplevel) printf '%s\\n' \"$2\" ;;\n" +
		"    --path-format=absolute) printf '%s/.git\\n' \"$2\" ;;\n" +
		"  esac\n" +
		"elif [ \"$3\" = worktree ]; then\n" +
		"  printf 'worktree %s\\000HEAD abcdef\\000branch refs/heads/main\\000\\000worktree %s\\000HEAD abcdef\\000detached\\000\\000' \"$2\" '" + linked + "'\n" +
		"fi\n"
	writeFile(t, git, script)
	if err := os.Chmod(git, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := awfgit.ResolveControlRoots(testContext(t), primary)
	requireNonForceableHardSafety(t, err, "repository-identity", linked)
}

// A registered worktree whose directory is swapped for a symlink is still
// listed by native Git with no prunable marker, so the identity ladder is the
// only thing standing between the swap and a resolution through it.
func TestControlRootRejectsSymlinkedRegisteredWorktree(t *testing.T) {
	base := t.TempDir()
	primary := filepath.Join(base, "primary")
	initNativeRepo(t, primary)
	linked := filepath.Join(base, "linked")
	runGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")
	moved := filepath.Join(base, "linked.moved")
	if err := os.Rename(linked, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, linked); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if strings.Contains(runGit(t, "-C", primary, "worktree", "list", "--porcelain"), "prunable") {
		t.Fatal("fixture no longer reaches the non-prunable identity refusal: Git now prunes the symlinked registration")
	}

	_, err := awfgit.ResolveControlRoots(testContext(t), primary)
	requireNonForceableHardSafety(t, err, "symlink", linked)
}

// A bare primary emits a bare worktree record even when the invoking checkout
// is a non-bare linked worktree, so the bare record must be skipped and the
// call must end in the missing-primary refusal rather than adopting it.
func TestControlRootSkipsBareRecordFromLinkedCheckout(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	initNativeRepo(t, source)
	bare := filepath.Join(base, "bare repository with spaces.git")
	runGit(t, "clone", "--bare", source, bare)
	linked := filepath.Join(base, "linked checkout with spaces")
	runGit(t, "-C", bare, "worktree", "add", "--detach", linked, "HEAD")

	if got := trimGitOutputLine(runGit(t, "-C", linked, "rev-parse", "--is-bare-repository")); got != "false" {
		t.Fatalf("linked checkout of a bare primary reports is-bare-repository = %q, want false", got)
	}
	if !strings.Contains(runGit(t, "-C", linked, "worktree", "list", "--porcelain"), "\nbare\n") {
		t.Fatal("fixture no longer lists the bare primary record from the linked checkout")
	}

	common := trimGitOutputLine(runGit(t, "-C", linked, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	_, err := awfgit.ResolveControlRoots(testContext(t), linked)
	requireNonForceableHardSafety(t, err, "missing-primary", common)
}

func TestControlRootRejectsMissingListedGitdirPointerTarget(t *testing.T) {
	base := t.TempDir()
	primary := filepath.Join(base, "primary")
	initNativeRepo(t, primary)
	linked := filepath.Join(base, "linked")
	runGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")
	missing := filepath.Join(base, "missing-gitdir-target")
	writeFile(t, filepath.Join(linked, ".git"), "gitdir: "+missing+"\n")

	_, err := awfgit.ResolveControlRoots(testContext(t), primary)
	requireNonForceableHardSafety(t, err, "repository-identity", linked)
}

func TestControlRootRejectsRepositoryIdentityMismatch(t *testing.T) {
	base := t.TempDir()
	primary := filepath.Join(base, "first-primary")
	initNativeRepo(t, primary)
	linked := filepath.Join(base, "first-linked")
	runGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")

	foreign := filepath.Join(base, "foreign-primary")
	initNativeRepo(t, foreign)
	foreignCommon := strings.TrimSpace(runGit(t, "-C", foreign, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	writeFile(t, filepath.Join(linked, ".git"), "gitdir: "+foreignCommon+"\n")

	_, err := awfgit.ResolveControlRoots(testContext(t), linked)
	requireNonForceableHardSafety(t, err, "repository-identity", linked)
}

func TestControlRootResidentNameClosedSet(t *testing.T) {
	primary := filepath.Join(t.TempDir(), "primary with spaces")
	roots := awfgit.ControlRoots{PrimaryRoot: primary}
	for name, leaf := range map[awfgit.ResidentName]string{
		awfgit.ResidentEfforts:   "efforts",
		awfgit.ResidentWorktrees: "worktrees",
	} {
		t.Run("accepts-"+leaf, func(t *testing.T) {
			got, err := roots.ResidentRoot(name)
			if err != nil {
				t.Fatalf("resolve %q resident root: %v", name, err)
			}
			if want := filepath.Join(cleanAbsolute(t, primary), ".awf", leaf); got != want {
				t.Fatalf("resident root = %q, want %q", got, want)
			}
		})
	}
	for _, name := range []awfgit.ResidentName{"", "logs"} {
		t.Run("rejects-"+string(name), func(t *testing.T) {
			_, err := roots.ResidentRoot(name)
			requireNonForceableHardSafety(t, err, "unknown-resident", string(name))
		})
	}
}

func TestControlRootRejectsSymlinkedOrUnconfinedResidentRoots(t *testing.T) {
	t.Run("unconfined-resident-name", func(t *testing.T) {
		roots := resolvedNativeRepo(t)
		_, err := roots.ResidentRoot(awfgit.ResidentName("../escape"))
		requireNonForceableHardSafety(t, err, "unknown-resident", "../escape")
	})

	t.Run("symlinked-awf", func(t *testing.T) {
		roots := resolvedNativeRepo(t)
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(roots.PrimaryRoot, ".awf")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := roots.ResidentRoot(awfgit.ResidentWorktrees)
		requireNonForceableHardSafety(t, err, "symlink", filepath.Join(roots.PrimaryRoot, ".awf"))
	})

	t.Run("symlinked-resident-root", func(t *testing.T) {
		roots := resolvedNativeRepo(t)
		if err := os.Mkdir(filepath.Join(roots.PrimaryRoot, ".awf"), 0o700); err != nil {
			t.Fatal(err)
		}
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(roots.PrimaryRoot, ".awf", "worktrees")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := roots.ResidentRoot(awfgit.ResidentWorktrees)
		requireNonForceableHardSafety(t, err, "symlink", filepath.Join(roots.PrimaryRoot, ".awf", "worktrees"))
	})

	t.Run("symlinked-invoking-ancestor", func(t *testing.T) {
		base := t.TempDir()
		primary := filepath.Join(base, "real", "primary")
		initNativeRepo(t, primary)
		alias := filepath.Join(base, "alias")
		if err := os.Symlink(filepath.Join(base, "real"), alias); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := awfgit.ResolveControlRoots(testContext(t), filepath.Join(alias, "primary"))
		requireNonForceableHardSafety(t, err, "symlink", alias)
	})
}

func TestControlRootRejectsForeignOwnedResidentAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX uid ownership")
	}
	if os.Geteuid() != 0 {
		t.Skip("creating a foreign-owned fixture requires chown privilege")
	}
	roots := resolvedNativeRepo(t)
	awfDir := filepath.Join(roots.PrimaryRoot, ".awf")
	if err := os.Mkdir(awfDir, 0o700); err != nil {
		t.Fatal(err)
	}
	foreignUID := 1
	if os.Geteuid() == foreignUID {
		foreignUID = 2
	}
	if err := os.Chown(awfDir, foreignUID, -1); err != nil {
		t.Skipf("platform exposes ownership but does not permit foreign chown: %v", err)
	}
	t.Cleanup(func() { _ = os.Chown(awfDir, os.Geteuid(), -1) })

	_, err := roots.ResidentRoot(awfgit.ResidentEfforts)
	requireNonForceableHardSafety(t, err, "foreign-owner", awfDir)
}

func resolvedNativeRepo(t *testing.T) awfgit.ControlRoots {
	t.Helper()
	primary := filepath.Join(t.TempDir(), "primary")
	initNativeRepo(t, primary)
	roots, err := awfgit.ResolveControlRoots(testContext(t), primary)
	if err != nil {
		t.Fatalf("resolve fixture roots: %v", err)
	}
	return roots
}

func initNativeSeparateGitDirRepo(t *testing.T, primary, common string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(primary), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, "init", "--separate-git-dir", common, primary)
	writeFile(t, filepath.Join(primary, "tracked.txt"), "base\n")
	runGit(t, "-C", primary, "add", "tracked.txt")
	runGit(t, "-C", primary, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "base")
}

func initNativeRepo(t *testing.T, primary string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(primary), 0o755); err != nil {
		t.Fatal(err)
	}
	args := []string{"init"}
	args = append(args, primary)
	runGit(t, args...)
	writeFile(t, filepath.Join(primary, "tracked.txt"), "base\n")
	runGit(t, "-C", primary, "add", "tracked.txt")
	runGit(t, "-C", primary, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "base")
}

func runGit(t *testing.T, args ...string) string {
	t.Helper()
	globalConfig := filepath.Join(t.TempDir(), "empty-global.gitconfig")
	writeFile(t, globalConfig, "")
	commandArgs := append([]string{
		"-c", "commit.gpgSign=false",
		"-c", "tag.gpgSign=false",
		"-c", "core.askPass=",
	}, args...)
	cmd := exec.CommandContext(testContext(t), "git", commandArgs...)
	cmd.Env = isolatedGitEnvironment(os.Environ(), []string{
		"GIT_CONFIG_GLOBAL=" + globalConfig,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
		"SSH_ASKPASS=true",
		"GCM_INTERACTIVE=Never",
	})
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func isolatedGitEnvironment(inherited, explicit []string) []string {
	controlled := map[string]bool{
		"GIT_CONFIG":            true,
		"GIT_CONFIG_COUNT":      true,
		"GIT_CONFIG_GLOBAL":     true,
		"GIT_CONFIG_NOSYSTEM":   true,
		"GIT_CONFIG_PARAMETERS": true,
		"GIT_CONFIG_SYSTEM":     true,
		"GIT_TERMINAL_PROMPT":   true,
		"GIT_ASKPASS":           true,
		"SSH_ASKPASS":           true,
		"GCM_INTERACTIVE":       true,
	}
	filtered := make([]string, 0, len(inherited)+len(explicit))
	for _, entry := range inherited {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if controlled[key] || strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, explicit...)
}

func gitdirFromFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := strings.CutPrefix(trimGitOutputLine(string(raw)), "gitdir: ")
	if !ok {
		t.Fatalf("%s is not a gitdir pointer: %q", path, raw)
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(filepath.Dir(path), value)
	}
	return filepath.Clean(value)
}

func trimGitOutputLine(value string) string {
	value = strings.TrimSuffix(value, "\n")
	return strings.TrimSuffix(value, "\r")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSingleListedNonBarePrimary(t *testing.T, repo, checkout, common string) {
	t.Helper()
	output := runGit(t, "-C", repo, "worktree", "list", "--porcelain", "-z")
	var records [][]string
	var record []string
	for _, field := range strings.Split(output, "\x00") {
		if field == "" {
			if len(record) > 0 {
				records = append(records, record)
				record = nil
			}
			continue
		}
		record = append(record, field)
	}
	if len(record) > 0 {
		records = append(records, record)
	}
	if len(records) != 1 {
		t.Fatalf("worktree list record count = %d, want 1: %q", len(records), output)
	}
	record = records[0]
	if len(record) == 0 || !strings.HasPrefix(record[0], "worktree ") {
		t.Fatalf("worktree list primary record = %q, want leading worktree field", record)
	}
	hasHEAD := false
	hasCheckoutState := false
	for _, field := range record[1:] {
		switch {
		case strings.HasPrefix(field, "HEAD "):
			hasHEAD = true
		case strings.HasPrefix(field, "branch "), field == "detached":
			hasCheckoutState = true
		case field == "bare":
			t.Fatalf("worktree list identifies %q as bare: %q", cleanAbsolute(t, checkout), record)
		}
	}
	if !hasHEAD || !hasCheckoutState {
		t.Fatalf("worktree list does not identify a non-bare checkout for %q: %q", cleanAbsolute(t, checkout), record)
	}
	gitdir := filepath.Clean(strings.TrimPrefix(record[0], "worktree "))
	if gitdir != cleanAbsolute(t, common) {
		t.Fatalf("listed primary gitdir for checkout %q = %q, want CommonDir %q", cleanAbsolute(t, checkout), gitdir, cleanAbsolute(t, common))
	}
}

func assertRoots(t *testing.T, got awfgit.ControlRoots, invoking, common, primary string) {
	t.Helper()
	want := awfgit.ControlRoots{
		InvokingRoot: cleanAbsolute(t, invoking),
		CommonDir:    cleanAbsolute(t, common),
		PrimaryRoot:  cleanAbsolute(t, primary),
	}
	if got != want {
		t.Fatalf("roots = %+v, want %+v", got, want)
	}
	if !filepath.IsAbs(got.CommonDir) {
		t.Fatalf("common directory is not absolute: %q", got.CommonDir)
	}
}

func cleanAbsolute(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return filesystem.NormalizePlatformPath(absolute)
}

func requireNonForceableHardSafety(t *testing.T, err error, category, path string) {
	t.Helper()
	if err == nil {
		t.Fatal("unsafe state accepted")
	}
	var hard *awfgit.HardSafetyError
	if !errors.As(err, &hard) {
		t.Fatalf("error %T (%v) is not a HardSafetyError", err, err)
	}
	var forceable interface{ Forceable() bool }
	if !errors.As(err, &forceable) {
		t.Fatalf("hard safety error %T does not expose forceability", err)
	}
	if forceable.Forceable() {
		t.Fatalf("hard safety error is forceable: %v", err)
	}
	if hard.Category != category {
		t.Fatalf("hard safety category = %q, want %q (error: %v)", hard.Category, category, err)
	}
	wantPath := filesystem.NormalizePlatformPath(path)
	if path != "" && filepath.Clean(hard.Path) != wantPath {
		t.Fatalf("hard safety path = %q, want %q (error: %v)", hard.Path, wantPath, err)
	}
	diagnostic := hard.Error()
	if !strings.Contains(diagnostic, category) {
		t.Fatalf("hard safety diagnostic %q omits category %q", diagnostic, category)
	}
	if path != "" && !strings.Contains(diagnostic, wantPath) {
		t.Fatalf("hard safety diagnostic %q omits path %q", diagnostic, wantPath)
	}
}
