package topic

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func corpusFixture(t *testing.T) (string, *config.Config, adr.Corpus) {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: test\ndomains: [alpha, beta]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/alpha.yaml"), "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/beta.yaml"), "paths: [\"pkg/**\"]\n")
	for n, status := range map[string]string{"0001": "Implemented", "0002": "Implemented", "0003": "Proposed"} {
		testsupport.WriteFile(t, filepath.Join(root, "docs/decisions/"+n+"-x.md"), testsupport.ADR(status, testsupport.WithTitle(n+": X"), testsupport.WithBody("## Decision\n\n1. X.\n")))
	}
	cfg, err := config.Load(filepath.Join(root, ".awf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	ac, err := adr.LoadCorpus(filepath.Join(root, "docs/decisions"))
	if err != nil {
		t.Fatal(err)
	}
	return root, cfg, ac
}
func writeTopic(t *testing.T, root, domain, slug, meta, part string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata", domain, slug+".yaml"), meta)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts", domain, slug, "current-state.md"), part)
}
func rulePart(slug, origin, refs string) string {
	r := ""
	if refs != "" {
		r = "References: " + refs + "\n"
	}
	return "Intro.\n\n## Claims\n\n### `rule: " + slug + "`\nRule prose.\nOrigin: ADR-" + origin + "\n" + r
}
func TestLoadCorpusAndIndexes(t *testing.T) {
	root, cfg, adrs := corpusFixture(t)
	writeTopic(t, root, "alpha", "one", "title: Zed\nsummary: Z.\npaths: [\"internal/**\"]\n", rulePart("same", "0001", "beta/two:same"))
	writeTopic(t, root, "beta", "two", "title: Alpha\nsummary: A.\napplies: global\n", rulePart("same", "0001", "alpha/one:same"))
	c, err := LoadCorpus(root, cfg, adrs)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.All()) != 2 || len(c.ForDomain("alpha")) != 1 {
		t.Fatalf("all/domain: %#v", c.All())
	}
	if _, ok := c.ByTopicID("alpha/one"); !ok {
		t.Fatal("topic lookup")
	}
	if _, ok := c.ByTopicID("none"); ok {
		t.Fatal("unexpected topic")
	}
	if _, ok := c.ByClaimID("beta/two:same"); !ok {
		t.Fatal("claim lookup")
	}
	if _, ok := c.ByClaimID("none"); ok {
		t.Fatal("unexpected claim")
	}
	if got := strings.Join(c.Outgoing("alpha/one:same"), ","); got != "beta/two:same" {
		t.Fatal(got)
	}
	if got := strings.Join(c.Incoming("alpha/one:same"), ","); got != "beta/two:same" {
		t.Fatal(got)
	}
	if c.Outgoing("none") != nil || c.Incoming("none") != nil {
		t.Fatal("missing refs")
	}
}
func TestLoadCorpusRejected(t *testing.T) {
	cases := map[string]func(*testing.T, string){
		"orphan metadata": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/metadata/alpha/x.yaml"), "title: X\nsummary: X.\npaths: [x]\n")
		},
		"orphan part": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/parts/alpha/x/current-state.md"), rulePart("x", "0001", ""))
		},
		"bad part path": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/parts/Bad/x/current-state.md"), rulePart("x", "0001", ""))
		},
		"unconfigured": func(t *testing.T, r string) {
			writeTopic(t, r, "other", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", "0001", ""))
		},
		"missing adr": func(t *testing.T, r string) {
			writeTopic(t, r, "alpha", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", "9999", ""))
		},
		"proposed adr": func(t *testing.T, r string) {
			writeTopic(t, r, "alpha", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", "0003", ""))
		},
		"dangling": func(t *testing.T, r string) {
			writeTopic(t, r, "alpha", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", "0001", "alpha/y:z"))
		},
		"ignored extension": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/metadata/note.txt"), "ignored\n")
			writeTopic(t, r, "alpha", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", "0001", ""))
		},
		"malformed metadata": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/metadata/alpha/x.yaml"), "title: [\n")
		},
		"malformed part": func(t *testing.T, r string) {
			writeTopic(t, r, "alpha", "x", "title: X\nsummary: X.\npaths: [x]\n", "no claims\n")
		},
		"self": func(t *testing.T, r string) {
			writeTopic(t, r, "alpha", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", "0001", "alpha/x:x"))
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			root, cfg, adrs := corpusFixture(t)
			setup(t, root)
			_, err := LoadCorpus(root, cfg, adrs)
			if name == "ignored extension" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("wanted error")
			}
		})
	}
}

func TestLoadCorpusPropagatesMarkerFailure(t *testing.T) {
	root, _, adrs := corpusFixture(t)
	writeTopic(t, root, "alpha", "x", "title: X\nsummary: X.\npaths: [\"internal/**\"]\n", "Intro.\n\n## Claims\n### `invariant: stable`\nStable.\nOrigin: ADR-0001\nBacking: test\n")
	cfg, err := config.Parse(filepath.Join(root, ".awf"), []byte("prefix: test\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"internal/**\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCorpus(root, cfg, adrs); err == nil {
		t.Fatal("marker failure was not propagated")
	}
}
func TestLoadCorpusNoTopicTree(t *testing.T) {
	root, cfg, adrs := corpusFixture(t)
	c, err := LoadCorpus(root, cfg, adrs)
	if err != nil || len(c.All()) != 0 {
		t.Fatalf("%#v %v", c, err)
	}
}
