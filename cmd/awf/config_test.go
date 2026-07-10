package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// Outside an adopted tree the command prints the static catalog reference and
// succeeds — pre-adoption discovery never refuses.
// invariant: config-command-static-fallback (backing test)
func TestRunConfigStaticFallback(t *testing.T) {
	var out bytes.Buffer
	if err := runConfig(t.TempDir(), "", &out); err != nil {
		t.Fatalf("static mode errored: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"static — not inside an awf project",
		"## config.yaml keys",
		"audit.diffThreshold (int)",
		"gateCmd (var)",
		"Catalog consumers:",
		"sidecar.local (bool)",
		"data.testSurfaces",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("static reference missing %q", want)
		}
	}
	if strings.Contains(got, "current:") || strings.Contains(got, "state:") {
		t.Error("static reference must carry no live state")
	}
}

func TestRunConfigLiveAndSingleKey(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runConfig(root, "", &out); err != nil {
		t.Fatalf("live mode errored: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"live state for this project",
		"current: `example`", // prefix
		"state: set (`make gate`)",
		"Consumed by:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("live reference missing %q", want)
		}
	}

	for key, want := range map[string]string{
		"audit.diffThreshold": "current: 400 (default)",
		"gateCmd":             "state: set (`make gate`)",
		"sidecar.local":       "renders nothing",
		"testSurfaces":        "skill tdd · data.testSurfaces",
	} {
		out.Reset()
		if err := runConfig(root, key, &out); err != nil {
			t.Fatalf("single-key %q errored: %v", key, err)
		}
		if !strings.Contains(out.String(), want) {
			t.Errorf("single-key %q output missing %q:\n%s", key, want, out.String())
		}
	}

	if err := runConfig(root, "nonsense", io.Discard); err == nil ||
		!strings.Contains(err.Error(), `unknown key or var "nonsense"`) {
		t.Errorf("unknown key: got %v", err)
	}

	// Static single-key works too (no live fields printed).
	out.Reset()
	if err := runConfig(t.TempDir(), "gateCmd", &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "state:") {
		t.Error("static single-key output must carry no state")
	}
}

// The dispatch path: `awf config [<key>]` routes through run() with the
// optional positional, exit 0 static and exit 1 on an unknown key.
func TestRunConfigDispatch(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "config"}, &out, &errb); code != 0 {
		t.Fatalf("bare config: exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "static") {
		t.Errorf("bare config output missing static header:\n%s", out.String())
	}
	out.Reset()
	if code := run([]string{"awf", "config", "gateCmd"}, &out, &errb); code != 0 {
		t.Fatalf("single-key config: exit %d", code)
	}
	if code := run([]string{"awf", "config", "nope"}, &out, &errb); code != 1 {
		t.Errorf("unknown key should exit 1, got %d", code)
	}
}

// An invalid config fails project open like every gated command.
func TestRunConfigOpenError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: \"\"\n")
	if err := runConfig(root, "", io.Discard); err == nil ||
		!strings.Contains(err.Error(), "prefix") {
		t.Errorf("expected the open-time validation error, got %v", err)
	}
}

// A render fault after a successful open (an unreadable convention part)
// surfaces as the command's error rather than a panic or silence.
func TestRunConfigRenderFault(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	// A directory where a part file may sit makes the part read fail non-ErrNotExist.
	if err := os.MkdirAll(filepath.Join(root, ".awf/skills/parts/tdd/surfaces.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runConfig(root, "", io.Discard); err == nil {
		t.Error("expected a render fault to surface")
	}
}

// Inside an adopted tree the command is gated: a binary behind the project's
// schema refuses like every gated command.
func TestRunConfigGatedInsideProject(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, checkYAML)
	l := &manifest.Lock{AWFVersion: awfVersion(), SchemaVersion: migrate.Current() + 1, Files: map[string]manifest.Entry{}}
	if err := l.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	if err := runConfig(root, "", io.Discard); err == nil ||
		!strings.Contains(err.Error(), "update your pinned awf") {
		t.Errorf("expected the schema-ahead gate refusal, got %v", err)
	}
}
