package topic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func corpusFixture(t *testing.T) (string, *config.Config) {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: test\nintegrationBranch: main\ndomains: [alpha, beta]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/alpha.yaml"), "paths: [\"internal/**\"]\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/domains/beta.yaml"), "paths: [\"pkg/**\"]\n")
	cfg, err := config.Load(filepath.Join(root, ".awf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	return root, cfg
}
func writeTopic(t *testing.T, root, domain, slug, meta, part string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/metadata", domain, slug+".yaml"), meta)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts", domain, slug, "current-state.md"), part)
}
func rulePart(slug, refs string) string {
	refsLine := ""
	if refs != "" {
		refsLine = "References: " + refs + "\n"
	}
	return "Intro.\n\n## Claims\n\n### `rule: " + slug + "`\nRule prose.\n" + refsLine
}
func loadCorpusForTest(t *testing.T, root string, cfg *config.Config) (Corpus, error) {
	t.Helper()
	return LoadCorpusFromTree(treeFromDir(t, root), cfg)
}

func TestLoadCorpusAndIndexesWithoutADRCorpus(t *testing.T) {
	root, cfg := corpusFixture(t)
	writeTopic(t, root, "alpha", "one", "title: Zed\nsummary: Z.\npaths: [\"internal/**\"]\n", rulePart("same", "beta/two:same"))
	writeTopic(t, root, "beta", "two", "title: Alpha\nsummary: A.\napplies: global\n", rulePart("same", "alpha/one:same"))
	c, err := loadCorpusForTest(t, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.All()) != 2 || len(c.ForDomain("alpha")) != 1 {
		t.Fatalf("all/domain: %#v", c.All())
	}
	if _, ok := c.ByTopicID("alpha/one"); !ok {
		t.Fatal("topic lookup")
	}
	if _, ok := c.ByClaimID("beta/two:same"); !ok {
		t.Fatal("claim lookup")
	}
	if got := strings.Join(c.Outgoing("alpha/one:same"), ","); got != "beta/two:same" {
		t.Fatal(got)
	}
	if got := strings.Join(c.Incoming("alpha/one:same"), ","); got != "beta/two:same" {
		t.Fatal(got)
	}
}

func TestLoadCorpusRejected(t *testing.T) {
	cases := map[string]func(*testing.T, string){
		"orphan metadata": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/metadata/alpha/x.yaml"), "title: X\nsummary: X.\npaths: [x]\n")
		},
		"orphan part": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/parts/alpha/x/current-state.md"), rulePart("x", ""))
		},
		"bad part path": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/parts/Bad/x/current-state.md"), rulePart("x", ""))
		},
		"unconfigured": func(t *testing.T, r string) {
			writeTopic(t, r, "other", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", ""))
		},
		"dangling": func(t *testing.T, r string) {
			writeTopic(t, r, "alpha", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", "alpha/y:z"))
		},
		"self": func(t *testing.T, r string) {
			writeTopic(t, r, "alpha", "x", "title: X\nsummary: X.\npaths: [x]\n", rulePart("x", "alpha/x:x"))
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			root, cfg := corpusFixture(t)
			setup(t, root)
			if _, err := loadCorpusForTest(t, root, cfg); err == nil {
				t.Fatal("wanted error")
			}
		})
	}
}

func TestRecordMetaRejectsDuplicateID(t *testing.T) {
	metadata := map[string]metaEntry{}
	id := TopicID{"alpha", "x"}
	if err := recordMeta(metadata, id, metaEntry{path: "first.yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := recordMeta(metadata, id, metaEntry{path: "second.yaml"}); err == nil {
		t.Fatal("duplicate topic ID accepted")
	}
}
func TestLoadCorpusPropagatesMarkerFailure(t *testing.T) {
	root, _ := corpusFixture(t)
	writeTopic(t, root, "alpha", "x", "title: X\nsummary: X.\npaths: [\"internal/**\"]\n", "Intro.\n\n## Claims\n### `invariant: stable`\nStable.\nBacking: test\n")
	cfg := parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"internal/**\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n")
	if _, err := loadCorpusForTest(t, root, cfg); err == nil {
		t.Fatal("marker failure was not propagated")
	}
}
func TestLoadCorpusNoTopicTree(t *testing.T) {
	root, cfg := corpusFixture(t)
	c, err := loadCorpusForTest(t, root, cfg)
	if err != nil || len(c.All()) != 0 {
		t.Fatalf("%#v %v", c, err)
	}
}
func TestLoadCorpusIgnoresNonCurrentStateSiblingPart(t *testing.T) {
	root := t.TempDir()
	metadata := filepath.Join(root, ".awf/topics/metadata/alpha")
	parts := filepath.Join(root, ".awf/topics/parts/alpha/one")
	if err := os.MkdirAll(metadata, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadata, "one.yaml"), []byte("title: One\nsummary: One.\npaths: [\"internal/**\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parts, "current-state.md"), []byte(rulePart("stable", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parts, "draft.md"), []byte("malformed sibling"), 0o644); err != nil {
		t.Fatal(err)
	}
	corpus, err := loadCorpusForTest(t, root, parseCfg(t, "prefix: test\nintegrationBranch: main\ndomains: [alpha]\n"))
	if err != nil || len(corpus.All()) != 1 {
		t.Fatalf("corpus = %#v, %v", corpus.All(), err)
	}
}
