package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testselection"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func selectionRepository(t *testing.T) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	for path, content := range map[string]string{
		"go.mod":                "module example.test/selection\ngo 1.26\n",
		"test-selection.json":   `{"version":1,"meta_suites":[{"id":"composition","package":"./cmd/meta","tests":["TestComposition"]}],"shared_path_patterns":["templates/**","*.json","x"],"generated_go_patterns":["*_generated.go"]}`,
		"internal/leaf/leaf.go": "package leaf\n",
		"internal/leaf/leaf_test.go": `package leaf
import (
	"net"
	"os"
	"path/filepath"
	"testing"
)
func TestFilesystemProjectReaderPathsExcludeUnsupportedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".codegraph", "daemon.sock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
	listener, err := net.Listen("unix", path)
	if err != nil { t.Fatal(err) }
	_ = listener.Close()
}
`,
		"internal/user/user.go": "package user\nimport _ \"example.test/selection/internal/leaf\"\n",
		"cmd/meta/main.go":      "package main\nfunc main() {}\n",
		"cmd/meta/main_test.go": "package main\nimport (\"os\"; \"testing\")\nfunc TestMain(m *testing.M) { f, _ := os.OpenFile(\"lifecycle.log\", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); _, _ = f.WriteString(\"run\\n\"); _ = f.Close(); os.Exit(m.Run()) }\nfunc TestComposition(t *testing.T) {}\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Merge(t, repo, "base")
	return root
}

func decodeResult(t *testing.T, output string) testselection.Result {
	t.Helper()
	var result testselection.Result
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("machine output %q: %v", output, err)
	}
	return result
}

func TestRunEmitsDefaultWorkingTreeSelection(t *testing.T) {
	root := selectionRepository(t)
	path := filepath.Join(root, "internal/leaf/leaf.go")
	if err := os.WriteFile(path, []byte("package leaf\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"testselection", "--root", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	result := decodeResult(t, stdout.String())
	if result.Outcome != "selected" || len(result.Packages) != 2 || len(result.Suites) != 1 || result.Suites[0].ID != "composition" {
		t.Fatalf("result = %#v", result)
	}
}

// invariant: tooling/quality-gates:affected-package-feedback (TestRunExecutesSelectedPackagesAndDeclaredSuites)
func TestRunExecutesSelectedPackagesAndDeclaredSuites(t *testing.T) {
	root := selectionRepository(t)
	path := filepath.Join(root, "internal/leaf/leaf.go")
	if err := os.WriteFile(path, []byte("package leaf\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"testselection", "--root", root, "--execute"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	lines := bytes.SplitN(stdout.Bytes(), []byte("\n"), 2)
	result := decodeResult(t, string(lines[0]))
	if result.Outcome != "selected" || len(result.Packages) != 2 || len(result.Suites) != 1 {
		t.Fatalf("result=%#v", result)
	}
	if len(lines) != 2 || !bytes.Contains(lines[1], []byte("example.test/selection/internal/leaf")) || !bytes.Contains(lines[1], []byte("example.test/selection/internal/user")) || !bytes.Contains(lines[1], []byte(`"Test":"TestComposition"`)) {
		t.Fatalf("behavioral output=%q", lines[1])
	}
	lifecycle, err := os.ReadFile(filepath.Join(root, "cmd", "meta", "lifecycle.log"))
	if err != nil || string(lifecycle) != "run\n" {
		t.Fatalf("suite lifecycle = %q, %v", lifecycle, err)
	}
}

func TestRunRefusesUnavailableDeclaredSuiteWhenOwnerRunsFully(t *testing.T) {
	root := selectionRepository(t)
	if err := os.WriteFile(filepath.Join(root, "cmd", "meta", "main.go"), []byte("package main\n// changed\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "meta", "main_test.go"), []byte("package main\nimport (\"os\"; \"testing\")\nfunc TestMain(m *testing.M) { os.Exit(m.Run()) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"testselection", "--root", root, "--execute"}, &stdout, &stderr); code != 1 || !bytes.Contains(stderr.Bytes(), []byte("unavailable proving units: TestComposition")) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestExecuteSelectionRefusesUnavailableDeclaredSuite(t *testing.T) {
	root := selectionRepository(t)
	result := testselection.Result{
		Version:  1,
		Outcome:  "selected",
		Packages: []testselection.Package{},
		Suites: []testselection.Suite{{
			ID:      "missing",
			Package: "./cmd/meta",
			Tests:   []string{"TestMissing"},
		}},
	}
	if err := executeSelection(t.Context(), root, result, &bytes.Buffer{}); err == nil || !bytes.Contains([]byte(err.Error()), []byte("unavailable proving units: TestMissing")) {
		t.Fatalf("error = %v", err)
	}
}

func TestRunSupportsStagedAndRangeAndRefusesBadUsage(t *testing.T) {
	root := selectionRepository(t)
	gitfixture.Stage(t, gitfixture.At(root), map[string]string{"internal/leaf/leaf.go": "package leaf\n// staged\n"})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"testselection", "--root", root, "--staged"}, &stdout, &stderr); code != 0 {
		t.Fatalf("staged code=%d stderr=%s", code, stderr.String())
	}
	if result := decodeResult(t, stdout.String()); result.Outcome != "selected" {
		t.Fatalf("staged result=%#v", result)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"testselection", "--root", root, "--range", "HEAD..HEAD"}, &stdout, &stderr); code != 0 {
		t.Fatalf("range code=%d stderr=%s", code, stderr.String())
	}
	if result := decodeResult(t, stdout.String()); result.Outcome != "empty" {
		t.Fatalf("range result=%#v", result)
	}
	if code := run([]string{"testselection", "--staged", "--range", "HEAD..HEAD"}, &stdout, &stderr); code != 2 {
		t.Fatalf("usage code=%d", code)
	}
}
