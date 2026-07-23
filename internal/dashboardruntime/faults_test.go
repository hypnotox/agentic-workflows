package dashboardruntime

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func resetCacheSeams(t *testing.T) {
	t.Helper()
	oldMkdir, oldMkdirTemp, oldChmod := cacheMkdir, cacheMkdirTemp, cacheChmod
	oldRemoveAll, oldRename := cacheRemoveAll, cacheRename
	oldReadFile, oldReadDir, oldLstat := cacheReadFile, cacheReadDir, cacheLstat
	oldOpenFile, oldOpen, oldEval := cacheOpenFile, cacheOpen, cacheEvalSymlinks
	oldAcquire, oldPathExists, oldRemoveStaging := cacheAcquireLock, cachePathExists, cacheRemoveStaging
	oldSuffix, oldMaterialize, oldPolicy := cacheRandomSuffix, cacheMaterialize, cacheReadPolicy
	oldBuild, oldWrite, oldDigest := cacheBuildArtifact, cacheWriteSynced, cacheDigestFile
	oldSyncFile, oldSyncDirectory := cacheSyncFile, cacheSyncDirectory
	t.Cleanup(func() {
		cacheMkdir, cacheMkdirTemp, cacheChmod = oldMkdir, oldMkdirTemp, oldChmod
		cacheRemoveAll, cacheRename = oldRemoveAll, oldRename
		cacheReadFile, cacheReadDir, cacheLstat = oldReadFile, oldReadDir, oldLstat
		cacheOpenFile, cacheOpen, cacheEvalSymlinks = oldOpenFile, oldOpen, oldEval
		cacheAcquireLock, cachePathExists, cacheRemoveStaging = oldAcquire, oldPathExists, oldRemoveStaging
		cacheRandomSuffix, cacheMaterialize, cacheReadPolicy = oldSuffix, oldMaterialize, oldPolicy
		cacheBuildArtifact, cacheWriteSynced, cacheDigestFile = oldBuild, oldWrite, oldDigest
		cacheSyncFile, cacheSyncDirectory = oldSyncFile, oldSyncDirectory
	})
}

func TestPublishFaultBoundaries(t *testing.T) {
	fixture := newRuntimeFixture(t)
	boom := errors.New("injected")
	cases := []struct {
		name string
		set  func()
	}{
		{"lock", func() { cacheAcquireLock = func(string) (*advisoryLock, error) { return nil, boom } }},
		{"entry inspection", func() { cachePathExists = func(string) (bool, error) { return false, boom } }},
		{"staging cleanup", func() { cacheRemoveStaging = func(string, string) error { return boom } }},
		{"staging nonce", func() { cacheRandomSuffix = func() (string, error) { return "", boom } }},
		{"staging mkdir", func() {
			cacheRandomSuffix = func() (string, error) { return strings.Repeat("a", 32), nil }
			cacheMkdir = func(string, os.FileMode) error { return boom }
		}},
		{"build temp", func() { cacheMkdirTemp = func(string, string) (string, error) { return "", boom } }},
		{"build chmod", func() { cacheChmod = func(string, os.FileMode) error { return boom } }},
		{"materialize", func() { cacheMaterialize = func(string, string, string) error { return boom } }},
		{"policy", func() {
			cacheMaterialize = func(string, string, string) error { return nil }
			cacheReadPolicy = func(string) ([]byte, error) { return nil, boom }
		}},
		{"first build", func() {
			cacheMaterialize = func(string, string, string) error { return nil }
			cacheReadPolicy = func(string) ([]byte, error) { return []byte(fixturePolicy), nil }
			cacheBuildArtifact = func(string, string, string, BuildEnvironment) error { return boom }
		}},
		{"second build", func() {
			cacheMaterialize = func(string, string, string) error { return nil }
			cacheReadPolicy = func(string) ([]byte, error) { return []byte(fixturePolicy), nil }
			calls := 0
			cacheBuildArtifact = func(_, _, output string, _ BuildEnvironment) error {
				calls++
				if calls == 2 {
					return boom
				}
				return os.WriteFile(output, []byte("binary"), 0o700)
			}
		}},
		{"write policy", func() {
			configureSuccessfulPublishSeams()
			cacheWriteSynced = func(string, []byte, os.FileMode) error { return boom }
		}},
		{"digest awf", func() {
			configureSuccessfulPublishSeams()
			cacheDigestFile = func(string) (string, error) { return "", boom }
		}},
		{"digest launcher", func() {
			configureSuccessfulPublishSeams()
			calls := 0
			cacheDigestFile = func(path string) (string, error) {
				calls++
				if calls == 2 {
					return "", boom
				}
				return digestFile(path)
			}
		}},
		{"write metadata", func() {
			configureSuccessfulPublishSeams()
			calls := 0
			cacheWriteSynced = func(path string, content []byte, mode os.FileMode) error {
				calls++
				if calls == 2 {
					return boom
				}
				return writeSyncedFile(path, content, mode)
			}
		}},
		{"sync awf", func() { configureSuccessfulPublishSeams(); cacheSyncFile = func(string) error { return boom } }},
		{"sync launcher", func() {
			configureSuccessfulPublishSeams()
			calls := 0
			cacheSyncFile = func(string) error {
				calls++
				if calls == 2 {
					return boom
				}
				return nil
			}
		}},
		{"sync staging", func() { configureSuccessfulPublishSeams(); cacheSyncDirectory = func(string) error { return boom } }},
		{"rename", func() { configureSuccessfulPublishSeams(); cacheRename = func(string, string) error { return boom } }},
		{"sync cache", func() {
			configureSuccessfulPublishSeams()
			calls := 0
			cacheSyncDirectory = func(string) error {
				calls++
				if calls == 2 {
					return boom
				}
				return nil
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetCacheSeams(t)
			env := fixture.env
			env.XDGCacheHome = filepath.Join(t.TempDir(), "cache")
			tc.set()
			if _, err := publish(fixture.root, fixture.head.String(), "sha1", env); err == nil {
				t.Fatal("fault was not returned")
			}
		})
	}
}

func configureSuccessfulPublishSeams() {
	cacheMaterialize = func(string, string, string) error { return nil }
	cacheReadPolicy = func(string) ([]byte, error) { return []byte(fixturePolicy), nil }
	cacheBuildArtifact = func(_, _, output string, _ BuildEnvironment) error {
		return os.WriteFile(output, []byte("binary"), 0o700)
	}
}

func TestPublishRenameWinnerIsReused(t *testing.T) {
	fixture := newRuntimeFixture(t)
	resetCacheSeams(t)
	configureSuccessfulPublishSeams()
	cacheRename = func(oldPath, newPath string) error {
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
		return errors.New("injected post-rename race")
	}
	env := fixture.env
	env.XDGCacheHome = filepath.Join(t.TempDir(), "cache")
	launcher, err := publish(fixture.root, fixture.head.String(), "sha1", env)
	if err != nil || launcher.Path == "" {
		t.Fatalf("winner reuse = %+v, %v", launcher, err)
	}
}

func TestRepositoryGoAndBuildFaults(t *testing.T) {
	env := BuildEnvironment{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoBinary: goBinary(t), XDGCacheHome: t.TempDir(), Stderr: os.Stderr}
	if _, err := publish(filepath.Join(t.TempDir(), "missing"), strings.Repeat("a", 40), "sha1", env); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("repository error = %v", err)
	}
	fixture := newRuntimeFixture(t)
	resetCacheSeams(t)
	cacheEvalSymlinks = func(string) (string, error) { return "", errors.New("injected") }
	if _, err := repositoryIdentity(fixture.root); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("canonical identity error = %v", err)
	}
	cacheEvalSymlinks = filepath.EvalSymlinks
	gitDir := filepath.Join(fixture.root, ".git")
	moved := gitDir + ".moved"
	if err := os.Rename(gitDir, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := repositoryIdentity(fixture.root); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("identity error = %v", err)
	}
	if err := os.Rename(moved, gitDir); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		for _, scenario := range []string{"env-fail", "env-malformed", "build-fail"} {
			t.Run(scenario, func(t *testing.T) {
				wrapper := filepath.Join(t.TempDir(), "go")
				script := "#!/bin/sh\ncase \"$1\" in version) echo go-version;; env) "
				switch scenario {
				case "env-fail":
					script += "echo failed >&2; exit 1"
				case "env-malformed":
					script += "echo only-one-line"
				default:
					script += "printf 'exp\\ntoolchain\\n'"
				}
				if scenario == "build-fail" {
					script += ";; build) echo build-failed >&2; exit 1"
				}
				script += ";; esac\n"
				if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
				testEnv := fixture.env
				testEnv.GoBinary = wrapper
				if scenario == "build-fail" {
					if err := buildArtifact(t.TempDir(), "./missing", filepath.Join(t.TempDir(), "out"), testEnv); !errors.Is(err, ErrBuild) {
						t.Fatalf("build error = %v", err)
					}
				} else if _, _, _, err := inspectGo(testEnv); !errors.Is(err, ErrBuild) {
					t.Fatalf("inspect error = %v", err)
				}
			})
		}
	}
}

func TestPublishedEntryReadFaults(t *testing.T) {
	fixture := newRuntimeFixture(t)
	launcher, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Dir(launcher.Path)
	var expected runtimeMetadata
	decodeJSONFile(t, filepath.Join(entry, "metadata.json"), &expected)
	expected.BinarySHA256, expected.LauncherSHA256, expected.PolicySHA256 = "", "", ""
	t.Run("read directory", func(t *testing.T) {
		resetCacheSeams(t)
		cacheReadDir = func(string) ([]os.DirEntry, error) { return nil, errors.New("injected") }
		if err := validatePublishedEntry(entry, expected, fixture.env); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("read metadata", func(t *testing.T) {
		resetCacheSeams(t)
		cacheReadFile = func(path string) ([]byte, error) {
			if filepath.Base(path) == "metadata.json" {
				return nil, errors.New("injected")
			}
			return os.ReadFile(path)
		}
		if err := validatePublishedEntry(entry, expected, fixture.env); !errors.Is(err, ErrSnapshot) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("metadata mismatch", func(t *testing.T) {
		bad := expected
		bad.Commit = strings.Repeat("0", len(bad.Commit))
		if err := validatePublishedEntry(entry, bad, fixture.env); !errors.Is(err, ErrCacheCollision) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestCacheHelperFaults(t *testing.T) {
	boom := errors.New("injected")
	t.Run("path lstat", func(t *testing.T) {
		resetCacheSeams(t)
		cacheLstat = func(string) (os.FileInfo, error) { return nil, boom }
		if _, err := pathExists("x"); !errors.Is(err, boom) {
			t.Fatalf("pathExists error = %v", err)
		}
	})
	t.Run("staging read", func(t *testing.T) {
		resetCacheSeams(t)
		cacheReadDir = func(string) ([]os.DirEntry, error) { return nil, boom }
		if err := removeIncompleteStaging(t.TempDir(), "key"); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("invalid staging suffix", func(t *testing.T) {
		root, key := t.TempDir(), "key"
		name := "." + key + ".tmp-" + strings.Repeat("z", 32)
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := removeIncompleteStaging(root, key); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("invalid staging removed: %v", err)
		}
	})
	t.Run("staging remove", func(t *testing.T) {
		resetCacheSeams(t)
		root, key := t.TempDir(), "key"
		name := "." + key + ".tmp-" + strings.Repeat("a", 32)
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
		cacheRemoveAll = func(string) error { return boom }
		if err := removeIncompleteStaging(root, key); !errors.Is(err, ErrBuild) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("file open", func(t *testing.T) {
		resetCacheSeams(t)
		cacheOpenFile = func(string, int, os.FileMode) (*os.File, error) { return nil, boom }
		if err := writeSyncedFile("x", nil, 0o600); !errors.Is(err, boom) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("sync open", func(t *testing.T) {
		resetCacheSeams(t)
		cacheOpen = func(string) (*os.File, error) { return nil, boom }
		if err := syncFile("x"); !errors.Is(err, boom) {
			t.Fatalf("syncFile = %v", err)
		}
		if err := syncDirectory("x"); !errors.Is(err, boom) {
			t.Fatalf("syncDirectory = %v", err)
		}
	})
}
