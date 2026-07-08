package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func wfFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for name, content := range files {
		m[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return m
}

func runOn(t *testing.T, fsys fs.FS) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(fsys, &out, &errb)
	return code, out.String(), errb.String()
}

const pinnedWorkflow = `jobs:
  gate:
    steps:
      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
      - uses: ./.github/actions/local-helper
      - uses: docker://alpine@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      - uses: codecov/codecov-action@0fb7174895f61a3b6b78fc075e0cd60383518dac # v5.5.5
        with:
          token: x
      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          version: 'v2.17.0'
`

func TestRunPassesPinned(t *testing.T) {
	code, out, errb := runOn(t, wfFS(map[string]string{"ci.yml": pinnedWorkflow}))
	if code != 0 {
		t.Fatalf("want exit 0, got %d, stderr:\n%s", code, errb)
	}
	if !strings.Contains(out, "all workflow references pinned") {
		t.Errorf("expected confirmation on stdout, got:\n%s", out)
	}
}

func TestRunFailsTagPinnedAction(t *testing.T) {
	code, _, errb := runOn(t, wfFS(map[string]string{"ci.yml": "      - uses: actions/checkout@v6\n"}))
	if code != 1 || !strings.Contains(errb, "full 40-hex commit SHA") {
		t.Fatalf("want exit 1 with SHA error, got %d:\n%s", code, errb)
	}
}

func TestRunFailsUndigestedDocker(t *testing.T) {
	code, _, errb := runOn(t, wfFS(map[string]string{"ci.yml": "      - uses: docker://alpine:3.20\n"}))
	if code != 1 || !strings.Contains(errb, "image digest") {
		t.Fatalf("want exit 1 with digest error, got %d:\n%s", code, errb)
	}
}

func TestRunFailsFloatedGoreleaserVersion(t *testing.T) {
	wf := "      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3\n        with:\n          version: '~> v2'\n"
	code, _, errb := runOn(t, wfFS(map[string]string{"release.yaml": wf}))
	if code != 1 || !strings.Contains(errb, "exact vX.Y.Z") {
		t.Fatalf("want exit 1 with version error (and .yaml scanned), got %d:\n%s", code, errb)
	}
}

func TestRunFailsGoreleaserWithoutVersion(t *testing.T) {
	goreleaser := "      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3\n        with:\n          args: check\n"
	for name, wf := range map[string]string{
		"at EOF":           goreleaser,
		"before next step": goreleaser + "      - uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3\n",
	} {
		code, _, errb := runOn(t, wfFS(map[string]string{"ci.yml": wf}))
		if code != 1 || !strings.Contains(errb, "no version: input") {
			t.Fatalf("%s: a goreleaser-action step without version: must fail, got %d:\n%s", name, code, errb)
		}
	}
}

func TestRunIgnoresVersionKeysOfOtherActions(t *testing.T) {
	wf := "      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0\n        with:\n          version: not-semver-but-not-goreleaser\n"
	if code, _, errb := runOn(t, wfFS(map[string]string{"ci.yml": wf})); code != 0 {
		t.Fatalf("version under a non-goreleaser action must be ignored, got %d:\n%s", code, errb)
	}
}

func TestRunSkipsDirectoriesAndNonYAML(t *testing.T) {
	fsys := wfFS(map[string]string{
		"ci.yml":    pinnedWorkflow,
		"sub/x.yml": "      - uses: actions/checkout@v6\n",
		"README.md": "      - uses: actions/checkout@v6\n",
	})
	if code, _, errb := runOn(t, fsys); code != 0 {
		t.Fatalf("subdirectories and non-YAML files must be skipped, got %d:\n%s", code, errb)
	}
}

func TestRunFailsNoWorkflowFiles(t *testing.T) {
	code, _, errb := runOn(t, wfFS(nil))
	if code != 1 || !strings.Contains(errb, "no workflow files") {
		t.Fatalf("want exit 1 with no-files error, got %d:\n%s", code, errb)
	}
}

type readDirErrFS struct{ fstest.MapFS }

func (readDirErrFS) ReadDir(string) ([]fs.DirEntry, error) { return nil, errors.New("boom") }

func TestRunFailsUnreadableDir(t *testing.T) {
	code, _, errb := runOn(t, readDirErrFS{})
	if code != 1 || !strings.Contains(errb, "read .github/workflows") {
		t.Fatalf("want exit 1 with readdir error, got %d:\n%s", code, errb)
	}
}

type readFileErrFS struct{ fstest.MapFS }

func (readFileErrFS) ReadFile(string) ([]byte, error) { return nil, errors.New("boom") }

func TestRunFailsUnreadableFile(t *testing.T) {
	code, _, errb := runOn(t, readFileErrFS{wfFS(map[string]string{"ci.yml": pinnedWorkflow})})
	if code != 1 || !strings.Contains(errb, "boom") {
		t.Fatalf("want exit 1 with read error, got %d:\n%s", code, errb)
	}
}

// TestRepoWorkflowsPinned runs the check against the repo's real workflows, so
// an unpinned reference fails the test suite even without the ./x gate wiring.
func TestRepoWorkflowsPinned(t *testing.T) {
	code, _, errb := runOn(t, os.DirFS("../../.github/workflows"))
	if code != 0 {
		t.Fatalf("repo workflows are not pin-clean:\n%s", errb)
	}
}
