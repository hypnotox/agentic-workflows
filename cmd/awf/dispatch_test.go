package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestRunRefusesFullOnlyCommandForCoreProfileAfterStateAdmission(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\n")
	var stdout, stderr bytes.Buffer
	if code := runFrom(root, []string{"awf", "audit", "HEAD"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "awf audit is unavailable in the selected core governance footprint") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	for _, configYAML := range []string{
		"prefix: example\nprofile: core\nintegrationBranch: main\ndomains: [rendering]\n",
		"profile: [\n",
	} {
		testsupport.WriteAwfConfig(t, root, configYAML)
		stderr.Reset()
		if code := runFrom(root, []string{"awf", "audit", "HEAD"}, &stdout, &stderr); code != 1 {
			t.Fatalf("invalid capability config exit = %d, stderr = %q", code, stderr.String())
		}
	}
}

func TestRunFullOnlyCapabilityFollowsLiveAuthorityAdmission(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.WriteAwfConfig(t, root, "profile: [\n")
	testsupport.WriteFile(t, config.LockPath(root), `{"awfVersion":"0.39.2","schemaVersion":45,"files":{},"bridgeAttestation":{"version":1,"adrFormatV1From":1,"legacyADRGaps":null}}`)

	var stdout, stderr bytes.Buffer
	if code := runAt(t, root, []string{"awf", "audit", "HEAD"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "below live floor") || strings.Contains(got, "parse config") {
		t.Fatalf("Full-only admission order = %q", got)
	}
}

func TestRequireCommandCapabilityPreservesProjectPresenceFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(config.RootDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	lockPath := config.LockPath(root)
	if err := os.Symlink(filepath.Base(lockPath), lockPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	err := requireCommandCapability(context.Background(), root, clispec.Command{Name: "audit"}, "", invocation{})
	if !errors.Is(err, syscall.ELOOP) {
		t.Fatalf("capability project-presence error = %v, want symlink loop", err)
	}
}

func TestRunStagedFullOnlyCapabilityUsesIndexConfig(t *testing.T) {
	root := syncedGitProject(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\n")

	var stdout, stderr bytes.Buffer
	if code := runAt(t, root, []string{"awf", "check", "staged", "state"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "selected core governance footprint") {
		t.Fatalf("staged capability consulted working config: %q", stderr.String())
	}
}

func TestRunContextStagedCapabilityUsesIndexConfig(t *testing.T) {
	t.Run("staged Full overrides working Core", func(t *testing.T) {
		root := syncedGitProject(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
		testsupport.WriteFile(t, filepath.Join(root, "README.md"), "staged\n")
		gitfixture.Add(t, gitfixture.At(root), "README.md")
		testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: core\nintegrationBranch: main\n")

		var stdout, stderr bytes.Buffer
		if code := runAt(t, root, []string{"awf", "context", "--staged"}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if strings.Contains(stderr.String(), "selected core governance footprint") {
			t.Fatalf("staged capability consulted working config: %q", stderr.String())
		}
	})

	t.Run("staged Core overrides working Full", func(t *testing.T) {
		root := syncedGitProject(t, "prefix: example\nprofile: core\nintegrationBranch: main\n")
		testsupport.WriteFile(t, filepath.Join(root, "README.md"), "staged\n")
		gitfixture.Add(t, gitfixture.At(root), "README.md")
		testsupport.WriteAwfConfig(t, root, "prefix: example\nprofile: full\nintegrationBranch: main\n")

		var stdout, stderr bytes.Buffer
		if code := runAt(t, root, []string{"awf", "context", "--staged"}, &stdout, &stderr); code != 1 {
			t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "selected core governance footprint") {
			t.Fatalf("staged Core capability refusal = %q", stderr.String())
		}
	})
}

func TestRunStagedFullOnlyCapabilityAllowsPreAdoptionIndex(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})

	var stdout, stderr bytes.Buffer
	if code := runAt(t, repo.Root(), []string{"awf", "check", "staged", "state"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "no staged .awf/awf.lock") || strings.Contains(got, "governance footprint") {
		t.Fatalf("pre-adoption staged capability = %q", got)
	}
}

func TestRunGetwdError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := newRunner(func() (string, error) { return "", errors.New("boom") }, os.Stdin, func() bool { return false }).run([]string{"awf", "render"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on getwd error, got %d", code)
	}
	if out.Len() != 0 || errb.String() != "condition: awf: boom\n" {
		t.Errorf("streams stdout=%q stderr=%q", out.String(), errb.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "bogus"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for unknown command, got %d", code)
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("missing unknown-command text: %q", errb.String())
	}
}

func TestRunDispatchError(t *testing.T) {
	// render in a bare dir: project.Open fails -> handler error -> exit 1.
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "render"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on dispatch error, got %d", code)
	}
	if !strings.HasPrefix(errb.String(), "condition: awf:") {
		t.Errorf("expected typed diagnostic, got %q", errb.String())
	}
}
