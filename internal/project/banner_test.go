package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// TestInjectBannerShebang covers the shebang branch: the banner becomes a
// `#`-comment second line, after the shebang, so the script stays executable.
func TestInjectBannerShebang(t *testing.T) {
	got := injectBanner("#!/usr/bin/env bash\nset -e\n", "")
	lines := strings.Split(got, "\n")
	if lines[0] != "#!/usr/bin/env bash" {
		t.Fatalf("first line = %q, want the shebang", lines[0])
	}
	want := "# " + bannerText
	if lines[1] != want {
		t.Fatalf("second line = %q, want %q", lines[1], want)
	}
	if lines[2] != "set -e" {
		t.Fatalf("third line = %q, want the body", lines[2])
	}
}

// TestInjectBannerPlain covers the unchanged non-frontmatter HTML-comment branch.
func TestInjectBannerPlain(t *testing.T) {
	got := injectBanner("# Title\n\nbody\n", "")
	// invariant: rendering/sync-and-drift:provenance-banner (TestInjectBannerPlain)
	if !strings.HasPrefix(got, "<!-- "+bannerText+" -->\n") {
		t.Fatalf("plain content missing leading HTML banner: %q", got)
	}
}

func TestInjectBannerExplicitCommentStyles(t *testing.T) {
	for _, tc := range []struct {
		style render.CommentStyle
		want  string
	}{
		{render.HashComment, "# " + bannerText + "\n"},
		{render.TOMLComment, "# " + bannerText + "\n"},
		{render.SlashComment, "// " + bannerText + "\n"},
	} {
		if got := injectBanner("body\n", "", tc.style); got != tc.want+"body\n" {
			t.Errorf("style %v banner = %q", tc.style, got)
		}
	}
}

// TestInjectBannerFrontmatter covers the unchanged frontmatter branch: the banner
// lands after the closing `---`.
func TestInjectBannerFrontmatter(t *testing.T) {
	got := injectBanner("---\nname: x\n---\nbody\n", "")
	want := "---\nname: x\n---\n<!-- " + bannerText + " -->\nbody\n"
	if got != want {
		t.Fatalf("frontmatter banner = %q, want %q", got, want)
	}
}

// A resident gitignore is neither markdown nor a shebang script: its banner is a
// leading #-comment keyed on the template id (ADR-0069).
func TestInjectBannerResidentGitignore(t *testing.T) {
	want := "# " + bannerText + "\n*\n!.gitignore\n"
	names := resident.RootNames()
	if len(names) == 0 {
		t.Fatal("no resident roots to check")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got := injectBanner("*\n!.gitignore\n", residentGitignoreTID(name))
			if got != want {
				t.Errorf("gitignore banner:\ngot  %q\nwant %q", got, want)
			}
			if strings.Contains(got, "<!--") {
				t.Errorf("gitignore contains an HTML comment: %q", got)
			}
		})
	}
}

// TestIndexMdCarriesCanonicalBanner regresses a banner drift: generateIndexMD
// (like the former generateActiveMD) must call injectBanner rather than return
// adr.RenderIndexMD's content as-is, so INDEX.md's banner matches every other
// rendered artifact's canonical bannerText.
func TestIndexMdCarriesCanonicalBanner(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ndomains: []\n", nil)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "INDEX.md"))
	if err != nil {
		t.Fatalf("read INDEX.md: %v", err)
	}
	want := "<!-- " + bannerText + " -->\n"
	if !strings.HasPrefix(string(got), want) {
		t.Fatalf("INDEX.md banner = %q, want prefix %q", got[:min(60, len(got))], want)
	}
}
