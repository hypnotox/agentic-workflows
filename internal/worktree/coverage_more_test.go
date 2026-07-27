package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeErrorsAndTopologyMatrix(t *testing.T) {
	if (&RefusalError{Category: "x"}).Error() != "worktree refusal (x)" {
		t.Fatal("refusal formatting")
	}
	if (&PartialMutationError{EffortID: "id", Repair: "fix"}).Error() == "" {
		t.Fatal("partial formatting")
	}
	bad := func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("fault") }
	if _, err := nativeRunner(t.Context(), filepath.Join(t.TempDir(), "missing"), "status"); err == nil {
		t.Fatal("native fault hidden")
	}
	if _, err := resolve(t.Context(), func(context.Context, string, ...string) ([]byte, error) { return []byte("short\n"), nil }, ".", "HEAD"); err == nil {
		t.Fatal("short revision accepted")
	}
	if err := operationFreeForTest(t, func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("fault") }); err == nil {
		t.Fatal("operation fault hidden")
	}
	if err := status(t.Context(), func(context.Context, string, ...string) ([]byte, error) { return []byte(" M file\x00"), nil }, "."); err == nil {
		t.Fatal("dirty accepted")
	}
	if err := exactRegistration(t.Context(), bad, ".", "/x", "refs/heads/x"); err == nil {
		t.Fatal("registration fault hidden")
	}
	for _, p := range []string{filepath.Join(t.TempDir(), "file"), filepath.Join(t.TempDir(), "link")} {
		if strings.HasSuffix(p, "link") {
			target := filepath.Join(filepath.Dir(p), "target")
			_ = os.Mkdir(target, 0700)
			_ = os.Symlink(target, p)
		} else {
			_ = os.WriteFile(p, []byte("x"), 0600)
		}
		if err := safeManagedPath(p); err == nil {
			t.Fatalf("unsafe path accepted: %s", p)
		}
	}
	if (&RefusalError{Category: "x", Risk: "risk"}).Error() == "" || (&PartialMutationError{EffortID: "id", Repair: "repair"}).Error() == "" {
		t.Fatal("error formatting")
	}
	valid := []byte("worktree /x\x00HEAD abc\x00branch refs/heads/x\x00detached\x00bare\x00prunable reason\x00\x00")
	regs, err := registrations(t.Context(), func(context.Context, string, ...string) ([]byte, error) { return valid, nil }, ".")
	if err != nil || len(regs) != 1 || !regs[0].detached || !regs[0].bare {
		t.Fatalf("valid registration: %#v %v", regs, err)
	}
	for _, out := range []string{"", "worktree /x\x00", "HEAD abc\x00\x00", "worktree\x00\x00", "worktree /x\x00unknown x\x00\x00"} {
		if _, err := registrations(t.Context(), func(context.Context, string, ...string) ([]byte, error) { return []byte(out), nil }, "."); err == nil {
			t.Fatalf("malformed registration accepted: %q", out)
		}
	}
	for _, out := range []string{
		"worktree /other\x00HEAD abc\x00branch refs/heads/x\x00\x00",
		"worktree /x\x00HEAD abc\x00branch refs/heads/y\x00\x00",
	} {
		if err := exactRegistration(t.Context(), func(context.Context, string, ...string) ([]byte, error) { return []byte(out), nil }, ".", "/x", "refs/heads/x"); err == nil {
			t.Fatal("invalid exact registration accepted")
		}
	}
}

func operationFreeForTest(t *testing.T, run Runner) error {
	root := t.TempDir()
	return (&Manager{ctx: t.Context(), run: run}).operationFree(root)
}
