package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestControlRootInternalParserAndHelpers(t *testing.T) {
	valid := []byte("worktree /a\x00HEAD abc\x00branch refs/heads/main\x00\x00worktree /b\x00HEAD def\x00detached\x00prunable missing administrative files\x00\x00worktree /bare\x00bare\x00\x00")
	records, err := parseWorktreePorcelain(valid)
	if err != nil || len(records) != 3 || records[0].branch != "refs/heads/main" || records[0].head != "abc" || !records[1].detached || !records[1].prunable || !records[2].bare {
		t.Fatalf("valid parse = %#v, %v", records, err)
	}
	cases := [][]byte{
		nil,
		[]byte("worktree /a"),
		[]byte("\x00"),
		[]byte("HEAD x\x00\x00"),
		[]byte("worktree\x00\x00"),
		[]byte("worktree /a\x00worktree /b\x00"),
		[]byte("worktree /a\x00HEAD x\x00HEAD y\x00branch b\x00\x00"),
		[]byte("worktree /a\x00HEAD\x00branch b\x00\x00"),
		[]byte("worktree /a\x00HEAD x\x00branch\x00\x00"),
		[]byte("worktree /a\x00HEAD x\x00detached value\x00\x00"),
		[]byte("worktree /a\x00HEAD x\x00branch b\x00bare value\x00\x00"),
		[]byte("worktree /a\x00HEAD x\x00branch b\x00unknown\x00\x00"),
		[]byte("worktree /a\x00HEAD x\x00branch b\x00"),
	}
	for i, raw := range cases {
		if _, err := parseWorktreePorcelain(raw); err == nil {
			t.Errorf("invalid parser case %d accepted", i)
		}
	}
	if !lexicallyContained("/a", "/a/b") || lexicallyContained("/a", "/b") || !sameCleanPath("/a/.", "/a") {
		t.Fatal("path helper mismatch")
	}
	if got, err := cleanAbsolute("."); err != nil || !filepath.IsAbs(got) {
		t.Fatalf("cleanAbsolute = %q, %v", got, err)
	}
}

func TestNativeGitFailuresRetainWrappedCauseAndContext(t *testing.T) {
	nonRepository := filepath.Join(t.TempDir(), "not-a-repository")
	if err := os.Mkdir(nonRepository, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveControlRoots(t.Context(), nonRepository)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || !strings.Contains(err.Error(), "inspect bare-repository state") || !strings.Contains(err.Error(), nonRepository) {
		t.Fatalf("non-repository error lost exit cause or context: %T %v", err, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = runGitBytes(ctx, t.TempDir(), "version")
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "git -C") {
		t.Fatalf("canceled Git error lost cause or context: %T %v", err, err)
	}

	t.Setenv("PATH", t.TempDir())
	_, err = runGitBytes(t.Context(), t.TempDir(), "version")
	if !errors.Is(err, exec.ErrNotFound) || !strings.Contains(err.Error(), "git -C") {
		t.Fatalf("failed Git execution lost cause or context: %T %v", err, err)
	}
}

func TestNativeGitStageFailuresAndStrictScalarParsing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake native Git fixture requires a POSIX script")
	}
	for _, stage := range []string{"show", "common", "worktree"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			script := filepath.Join(bin, "git")
			body := "#!/bin/sh\n" +
				"if [ \"$3\" = rev-parse ] && [ \"$4\" = --is-bare-repository ]; then printf 'false\\n'; exit 0; fi\n" +
				"if [ \"$3\" = rev-parse ] && [ \"$4\" = --show-toplevel ]; then " + shellStageResult(stage, "show", "printf '%s\\n' \"$2\"") + "; fi\n" +
				"if [ \"$3\" = rev-parse ] && [ \"$4\" = --path-format=absolute ]; then " + shellStageResult(stage, "common", "printf '%s/.git\\n' \"$2\"") + "; fi\n" +
				"if [ \"$3\" = worktree ]; then " + shellStageResult(stage, "worktree", "printf 'worktree %s\\000HEAD abc\\000branch refs/heads/main\\000\\000' \"$2\"") + "; fi\n"
			if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			_, err := ResolveControlRoots(t.Context(), root)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || !strings.Contains(err.Error(), root) {
				t.Fatalf("%s-stage error lost exit cause or path context: %T %v", stage, err, err)
			}
		})
	}

	t.Run("path-output", func(t *testing.T) {
		bin := t.TempDir()
		script := filepath.Join(bin, "git")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'first\\nsecond\\n'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if _, err := runGitPath(t.Context(), t.TempDir(), "path"); err == nil || !strings.Contains(err.Error(), "invalid path response") {
			t.Fatalf("multiline Git path accepted: %v", err)
		}
	})

	t.Run("scalar-output", func(t *testing.T) {
		bin := t.TempDir()
		script := filepath.Join(bin, "git")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf ' false \\n'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if _, err := runGitText(t.Context(), t.TempDir(), "scalar"); err == nil || !strings.Contains(err.Error(), "invalid scalar response") {
			t.Fatalf("space-padded Git scalar accepted: %v", err)
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := runGitPath(ctx, t.TempDir(), "path"); !errors.Is(err, context.Canceled) {
		t.Fatalf("path command error lost cancellation cause: %v", err)
	}
}

func shellStageResult(current, target, success string) string {
	if current == target {
		return "printf 'stage failure\\n' >&2; exit 7"
	}
	return success
}

func TestListWorktreeRegistrationsErrors(t *testing.T) {
	if _, err := ListWorktreeRegistrations(t.Context(), filepath.Join(t.TempDir(), "not-a-repository")); err == nil {
		t.Fatal("non-repository registration list succeeded")
	}
	bin := t.TempDir()
	script := filepath.Join(bin, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'malformed\\000'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := ListWorktreeRegistrations(t.Context(), t.TempDir()); err == nil {
		t.Fatal("malformed registration topology accepted")
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'worktree relative\\000HEAD abc\\000branch refs/heads/main\\000\\000'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ListWorktreeRegistrations(t.Context(), t.TempDir()); err == nil {
		t.Fatal("relative registration path accepted")
	}
}

func TestControlRootInternalGitDirAndSafetyErrors(t *testing.T) {
	root := t.TempDir()
	dotGit := filepath.Join(root, ".git")
	if err := os.Mkdir(dotGit, 0o700); err != nil {
		t.Fatal(err)
	}
	if got, err := worktreeGitDir(root); err != nil || got != dotGit {
		t.Fatalf("directory gitdir = %q, %v", got, err)
	}
	if err := os.Remove(dotGit); err != nil {
		t.Fatal(err)
	}
	metadata := filepath.Join(root, "metadata")
	if err := os.Mkdir(metadata, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dotGit, []byte("gitdir: metadata\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := worktreeGitDir(root); err != nil || got != metadata {
		t.Fatalf("pointer gitdir = %q, %v", got, err)
	}
	if err := os.WriteFile(dotGit, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := worktreeGitDir(root); err == nil {
		t.Fatal("bad pointer accepted")
	}
	if err := os.Remove(dotGit); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(metadata, dotGit); err == nil {
		if _, err := worktreeGitDir(root); err == nil {
			t.Fatal("symlinked gitdir accepted")
		}
	}
	if _, err := worktreeGitDir(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing gitdir accepted")
	}

	base := errors.New("base")
	hard := &HardSafetyError{Category: "identity", Path: root, Err: base}
	if hard.Forceable() || !errors.Is(hard, base) || !strings.Contains(hard.Error(), "identity") || (&HardSafetyError{}).Error() != "hard safety refusal" {
		t.Fatalf("hard safety contract: %v", hard)
	}
	var nilHard *HardSafetyError
	if nilHard.Error() != "hard safety refusal" || nilHard.Unwrap() != nil {
		t.Fatal("nil hard safety methods")
	}
	if got := identityError(root, hard); !errors.Is(got, hard) {
		t.Fatal("identityError rewrapped hard error")
	}
	if got := identityError(root, base); !errors.Is(got, base) {
		t.Fatal("identityError did not retain plain error")
	}
	wrapped := errors.New("outer: " + base.Error())
	if got := unwrappedError(wrapped); got.Error() != wrapped.Error() {
		t.Fatal("unwrapped plain error changed")
	}
}

type controlRootFileInfo struct{ sys any }

func (f controlRootFileInfo) Name() string       { return "x" }
func (f controlRootFileInfo) Size() int64        { return 0 }
func (f controlRootFileInfo) Mode() os.FileMode  { return 0 }
func (f controlRootFileInfo) ModTime() time.Time { return time.Time{} }
func (f controlRootFileInfo) IsDir() bool        { return false }
func (f controlRootFileInfo) Sys() any           { return f.sys }

func TestControlRootOwnershipReflectionFallbacks(t *testing.T) {
	for _, sys := range []any{nil, "not-struct", struct{ Other uint32 }{1}, &struct{ Other uint32 }{1}} {
		if !ownedByCurrentUser(controlRootFileInfo{sys: sys}) {
			t.Fatalf("unknown ownership representation %T refused", sys)
		}
	}
	var pointer *struct{ Uid uint32 }
	if !ownedByCurrentUser(controlRootFileInfo{sys: pointer}) {
		t.Fatal("nil ownership pointer refused")
	}
	if !ownedByCurrentUser(controlRootFileInfo{sys: struct{ Uid uint32 }{uint32(os.Geteuid())}}) {
		t.Fatal("current owner refused")
	}
	foreign := uint32(os.Geteuid() + 1)
	if ownedByCurrentUser(controlRootFileInfo{sys: struct{ Uid uint32 }{foreign}}) {
		t.Fatal("foreign owner accepted")
	}
}
