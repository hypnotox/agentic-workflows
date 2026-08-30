package publisher

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// TestResidentGitignoresAlwaysOn asserts RenderAll unconditionally emits one
// self-ignoring .gitignore with a #-comment banner (ADR-0069) for every
// resident root awf owns - no config gate, unlike bootstrap/hooks. The
// standalone memory root is not among them: schema 22 stopped owning it, so no
// render may bring it back.
// invariant: rendering/singletons-and-payloads:memory-gitignore-always-on (TestResidentGitignoresAlwaysOn)
func TestResidentGitignoresAlwaysOn(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	rendered := map[string]string{}
	for i := range out {
		rendered[out[i].Path] = out[i].Content
	}
	want := "# " + bannerText + "\n*\n!.gitignore\n"
	for _, name := range resident.RootNames() {
		path := ".awf/" + name + "/.gitignore"
		content, found := rendered[path]
		if !found {
			t.Fatalf("expected %s in every RenderAll output", path)
		}
		if content != want {
			t.Errorf("%s content = %q, want %q", path, content, want)
		}
		if !strings.HasPrefix(content, "# ") {
			t.Errorf("%s banner must be a #-comment, got %q", path, content)
		}
	}
	if _, found := rendered[".awf/memory/.gitignore"]; found {
		t.Error("render reintroduced the standalone memory resident root")
	}
}
