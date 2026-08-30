package currentstate_test

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// treeFrom builds a snapshot Tree from an in-memory path->content map so a load
// case can shape an exact current-state universe without touching the filesystem.
func treeFrom(t *testing.T, files map[string]string) *snapshot.Tree {
	t.Helper()
	var fl []snapshot.File
	for p, c := range files {
		fl = append(fl, snapshot.File{Path: p, Mode: snapshot.Regular, Bytes: []byte(c)})
	}
	tree, err := snapshot.NewTree(fl)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func loadCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Parse("/nonexistent", []byte("prefix: test\ndomains: [alpha]\n"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func ruleTopicPart() string {
	return "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\n"
}

// TestLoadFromTreeUsesOnlyCurrentStateInputs proves historical decisions may be
// absent or arbitrary Markdown without affecting topic loading.
func TestLoadFromTreeUsesOnlyCurrentStateInputs(t *testing.T) {
	for name, decision := range map[string]string{
		"absent":    "",
		"malformed": "---\nformat: unknown\n---\n# Not a current-state input\n",
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{
				".awf/topics/metadata/alpha/one.yaml":          "title: One\nsummary: O.\npaths: [\"internal/**\"]\n",
				".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart(),
			}
			if decision != "" {
				files["docs/decisions/0001-history.md"] = decision
			}
			loaded, err := currentstate.LoadFromTree(treeFrom(t, files), loadCfg(t))
			if err != nil {
				t.Fatal(err)
			}
			if len(loaded.Topics.All()) != 1 {
				t.Fatalf("topics = %d, want 1", len(loaded.Topics.All()))
			}
			if _, ok := loaded.Topics.ByClaimID("alpha/one:r"); !ok {
				t.Fatal("claim alpha/one:r missing from current-state corpus")
			}
		})
	}
}

func TestLoadFromTreePropagatesCurrentStateFailures(t *testing.T) {
	tree := treeFrom(t, map[string]string{
		"docs/decisions/0001-history.md":               "not parseable as a legacy lifecycle record\n",
		".awf/topics/metadata/alpha/one.yaml":          "title: [unterminated\n",
		".awf/topics/parts/alpha/one/current-state.md": ruleTopicPart(),
	})
	if _, err := currentstate.LoadFromTree(tree, loadCfg(t)); err == nil {
		t.Fatal("malformed topic metadata was accepted")
	}
}
