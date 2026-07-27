package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeClosureFaultAndRefusalBranches(t *testing.T) {
	if _, err := Open(t.Context(), filepath.Join(t.TempDir(), "missing"), Options{}); err == nil {
		t.Fatal("open accepted missing repository")
	}
	m, id := attachedManager(t)
	if _, err := m.managed(string(id) + "/"); err != nil {
		t.Fatal(err)
	}
	badRoots := *m
	badRoots.roots.PrimaryRoot = "relative-root"
	if _, err := badRoots.managed(string(id)); err == nil {
		t.Fatal("managed accepted invalid resident root")
	}

	markerRoot := t.TempDir()
	marker := filepath.Join(markerRoot, "MERGE_HEAD")
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (&Manager{ctx: t.Context(), run: func(context.Context, string, ...string) ([]byte, error) { return []byte(marker + "\n"), nil }}).operationFree(markerRoot); err == nil {
		t.Fatal("operation marker was ignored")
	}
	if err := (&Manager{ctx: t.Context(), run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(filepath.Join(markerRoot, "missing") + "\n"), nil
	}}).operationFree(markerRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeRunner(context.Background(), filepath.Join(t.TempDir(), "missing"), "status"); err == nil {
		t.Fatal("native command failure hidden")
	}

	fresh, err := m.efforts.New("collision", false)
	if err != nil {
		t.Fatal(err)
	}
	path, err := m.managed(fresh.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Add(fresh.ID, "HEAD"); err == nil {
		t.Fatal("directory collision accepted")
	}
	fileEffort, err := m.efforts.New("file collision", false)
	if err != nil {
		t.Fatal(err)
	}
	filePath, _ := m.managed(fileEffort.ID)
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Add(fileEffort.ID, "HEAD"); err == nil {
		t.Fatal("file collision accepted")
	}

	// A runner that reaches every post-registration integration decision while
	// keeping Git topology observations deterministic.
	baseRunner := func(dirty, detached, branchTarget, ff, mergeFault bool) Runner {
		return func(ctx context.Context, root string, args ...string) ([]byte, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(joined, "worktree list"):
				return nativeRunner(ctx, m.roots.InvokingRoot, args...)
			case strings.HasPrefix(joined, "status") && dirty:
				return []byte(" M file\x00"), nil
			case strings.HasPrefix(joined, "status"):
				return nil, nil
			case strings.HasPrefix(joined, "rev-parse --git-path"):
				return []byte(filepath.Join(root, "missing") + "\n"), nil
			case strings.HasPrefix(joined, "symbolic-ref"):
				if detached {
					return nil, errors.New("detached")
				}
				if branchTarget {
					return []byte("awf/" + string(id) + "\n"), nil
				}
				return []byte("main\n"), nil
			case strings.HasPrefix(joined, "rev-parse --verify"):
				return []byte(strings.Repeat("a", 40) + "\n"), nil
			case strings.HasPrefix(joined, "merge-base"):
				if ff {
					return nil, nil
				}
				return nil, &fakeExit{code: 1}
			case strings.HasPrefix(joined, "merge") && mergeFault:
				return nil, errors.New("conflict")
			default:
				return nil, nil
			}
		}
	}
	for name, run := range map[string]Runner{
		"detached":               baseRunner(false, true, false, true, false),
		"branch target":          baseRunner(false, false, true, true, false),
		"dirty without approval": baseRunner(true, false, false, true, false),
		"merge conflict":         baseRunner(false, false, false, true, true),
	} {
		t.Run(name, func(t *testing.T) {
			m.run = run
			if _, err := m.Integrate(string(id), false, ""); err == nil {
				t.Fatal("refusal branch accepted")
			}
		})
	}
	m.run = baseRunner(true, false, false, true, false)
	if _, err := m.Integrate(string(id), true, "discarded tracked change"); err != nil {
		t.Fatalf("approved dirty integration: %v", err)
	}

	manual, err := m.efforts.New("manual faults", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Add(manual.ID, "HEAD"); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]Runner{
		"ancestor fault":       baseRunner(false, false, false, true, false),
		"target resolve fault": baseRunner(false, false, false, true, false),
	} {
		_ = name
		_ = run
	}
	m.run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.HasPrefix(joined, "worktree list") {
			return nativeRunner(ctx, m.roots.InvokingRoot, args...)
		}
		if strings.HasPrefix(joined, "rev-parse --verify") {
			return []byte(strings.Repeat("b", 40) + "\n"), nil
		}
		if strings.HasPrefix(joined, "merge-base") {
			return nil, errors.New("probe fault")
		}
		return baseRunner(false, false, false, true, false)(ctx, root, args...)
	}
	if _, err := m.RecordManualIntegration(manual.ID, "HEAD", false, ""); err == nil {
		t.Fatal("manual ancestor fault hidden")
	}
}

type fakeExit struct{ code int }

func (e *fakeExit) Error() string { return fmt.Sprintf("exit %d", e.code) }
func (e *fakeExit) ExitCode() int { return e.code }
