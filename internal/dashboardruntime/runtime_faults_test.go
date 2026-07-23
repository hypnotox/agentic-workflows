package dashboardruntime

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetRuntimeSeams(t *testing.T) {
	t.Helper()
	oldGit, oldPublish := runtimeGitOutput, runtimePublish
	oldLookPath, oldAbs, oldCacheDir, oldRand := runtimeLookPath, runtimeAbs, runtimeCacheDir, runtimeRandRead
	t.Cleanup(func() {
		runtimeGitOutput, runtimePublish = oldGit, oldPublish
		runtimeLookPath, runtimeAbs, runtimeCacheDir, runtimeRandRead = oldLookPath, oldAbs, oldCacheDir, oldRand
	})
}

func fixedEnvironment(t *testing.T) BuildEnvironment {
	t.Helper()
	return BuildEnvironment{GOOS: "linux", GOARCH: "amd64", GoBinary: filepath.Join(t.TempDir(), "go"), XDGCacheHome: t.TempDir(), Stderr: io.Discard}
}

func TestResolveStateMachineFaults(t *testing.T) {
	boom := errors.New("injected")
	head := strings.Repeat("a", 40)
	cases := []struct {
		name string
		git  func(string, ...string) (string, error)
		pub  func(string, string, string, BuildEnvironment) (Launcher, error)
		want bool
	}{
		{"object format", func(string, ...string) (string, error) { return "", boom }, nil, true},
		{"head absent", scriptedResolveGit("", "", boom), nil, true},
		{"initialize loses without winner", scriptedResolveGit(head, "", boom), nil, true},
		{"initialize loses to different winner", scriptedResolveGit(head, strings.Repeat("b", 40), boom), nil, true},
		{"publish", scriptedResolveGit("", head, nil), func(string, string, string, BuildEnvironment) (Launcher, error) { return Launcher{}, boom }, true},
		{"initialize converges", scriptedResolveGit(head, head, boom), func(_ string, commit, _ string, _ BuildEnvironment) (Launcher, error) {
			return Launcher{Commit: commit}, nil
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetRuntimeSeams(t)
			runtimeGitOutput = tc.git
			if tc.pub != nil {
				runtimePublish = tc.pub
			}
			_, err := Resolve("root", fixedEnvironment(t))
			if (err != nil) != tc.want {
				t.Fatalf("Resolve error = %v, wantError=%v", err, tc.want)
			}
		})
	}
}

func scriptedResolveGit(head, winner string, updateErr error) func(string, ...string) (string, error) {
	refCalls := 0
	return func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "--show-object-format"):
			return "sha1", nil
		case strings.Contains(joined, "HEAD^{commit}"):
			if head == "" {
				return "", errors.New("absent")
			}
			return head, nil
		case strings.Contains(joined, runtimeRef+"^{commit}"):
			refCalls++
			if refCalls == 1 || winner == "" {
				return "", errors.New("absent")
			}
			return winner, nil
		case len(args) > 0 && args[0] == "update-ref":
			return "", updateErr
		default:
			return "", errors.New("unexpected git call: " + joined)
		}
	}
}

func TestAdvanceStateMachineFaults(t *testing.T) {
	boom := errors.New("injected")
	oldCommit, newCommit := strings.Repeat("a", 40), strings.Repeat("b", 40)
	baseGit := func(_ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "--show-object-format"):
			return "sha1", nil
		case strings.Contains(joined, runtimeRef+"^{commit}"):
			return oldCommit, nil
		case strings.Contains(joined, "candidate^{commit}"):
			return newCommit, nil
		case args[0] == "update-ref":
			return "", boom
		default:
			return "", boom
		}
	}
	for _, scenario := range []string{"object", "publish", "cas"} {
		t.Run(scenario, func(t *testing.T) {
			resetRuntimeSeams(t)
			runtimeGitOutput = baseGit
			runtimePublish = func(string, string, string, BuildEnvironment) (Launcher, error) {
				return Launcher{Path: "launcher"}, nil
			}
			switch scenario {
			case "object":
				runtimeGitOutput = func(string, ...string) (string, error) { return "", boom }
			case "publish":
				runtimePublish = func(string, string, string, BuildEnvironment) (Launcher, error) { return Launcher{}, boom }
			}
			if _, err := Advance("root", "candidate", fixedEnvironment(t)); err == nil {
				t.Fatal("fault was not returned")
			}
		})
	}
}

func TestEnvironmentAndNonceFaultSeams(t *testing.T) {
	boom := errors.New("injected")
	if _, err := Resolve("root", BuildEnvironment{GoBinary: "/go", XDGCacheHome: "relative"}); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Resolve normalization error = %v", err)
	}
	if _, err := Advance("root", "HEAD", BuildEnvironment{GoBinary: "/go", XDGCacheHome: "relative"}); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Advance normalization error = %v", err)
	}
	t.Run("default cache", func(t *testing.T) {
		resetRuntimeSeams(t)
		want := t.TempDir()
		runtimeCacheDir = func() (string, error) { return want, nil }
		env, err := normalizeEnvironment(BuildEnvironment{GoBinary: "/go"})
		if err != nil || env.XDGCacheHome != want {
			t.Fatalf("environment = %+v, %v", env, err)
		}
	})
	t.Run("look path", func(t *testing.T) {
		resetRuntimeSeams(t)
		runtimeLookPath = func(string) (string, error) { return "", boom }
		if _, err := normalizeEnvironment(BuildEnvironment{XDGCacheHome: t.TempDir()}); !errors.Is(err, ErrBuild) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("absolute go", func(t *testing.T) {
		resetRuntimeSeams(t)
		runtimeAbs = func(string) (string, error) { return "", boom }
		if _, err := normalizeEnvironment(BuildEnvironment{GoBinary: "go", XDGCacheHome: t.TempDir()}); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("cache dir", func(t *testing.T) {
		resetRuntimeSeams(t)
		runtimeCacheDir = func() (string, error) { return "", boom }
		if _, err := normalizeEnvironment(BuildEnvironment{GoBinary: "/go"}); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("nonce", func(t *testing.T) {
		resetRuntimeSeams(t)
		runtimeRandRead = func([]byte) (int, error) { return 0, boom }
		if _, err := randomStagingSuffix(); !errors.Is(err, boom) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestRunGitOutputSuccess(t *testing.T) {
	root := t.TempDir()
	if output, err := runGitOutput(root, "init", "--bare"); err != nil || !strings.Contains(output, "Initialized") {
		t.Fatalf("runGitOutput = %q, %v", output, err)
	}
	if err := os.WriteFile(filepath.Join(root, "not-used"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}
