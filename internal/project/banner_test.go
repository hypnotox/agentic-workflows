package project

import (
	"strings"
	"testing"
)

// TestInjectBannerShebang covers the shebang branch: the banner becomes a
// `#`-comment second line, after the shebang, so the script stays executable.
func TestInjectBannerShebang(t *testing.T) {
	got := injectBanner("#!/usr/bin/env bash\nset -e\n")
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
	got := injectBanner("# Title\n\nbody\n")
	if !strings.HasPrefix(got, "<!-- "+bannerText+" -->\n") {
		t.Fatalf("plain content missing leading HTML banner: %q", got)
	}
}

// TestInjectBannerFrontmatter covers the unchanged frontmatter branch: the banner
// lands after the closing `---`.
func TestInjectBannerFrontmatter(t *testing.T) {
	got := injectBanner("---\nname: x\n---\nbody\n")
	want := "---\nname: x\n---\n<!-- " + bannerText + " -->\nbody\n"
	if got != want {
		t.Fatalf("frontmatter banner = %q, want %q", got, want)
	}
}
