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
		"nested metadata path": func(t *testing.T, r string) {
			testsupport.WriteFile(t, filepath.Join(r, ".awf/topics/metadata/alpha/nested/x.yaml"), "title: X\nsummary: X.\npaths: [x]\n")
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

func TestOperationSpecificProvenance(t *testing.T) {
	claimID := "alpha/x:x"
	op := func(verb adr.OpVerb) adr.Operation { return adr.Operation{Verb: verb, ID: claimID, Slug: "x"} }
	status := func(value string) adr.HistoryEvent { return adr.HistoryEvent{Kind: adr.HistoryStatus, Status: value} }
	applied := func(sequence int, operation adr.Operation) adr.HistoryEvent {
		return adr.HistoryEvent{Kind: adr.HistoryApplied, Sequence: sequence, HasSequence: true, Operations: []adr.Operation{operation}}
	}
	assemble := func(t *testing.T, records []adr.ADR, origin string, revised ...string) error {
		t.Helper()
		revisedLine := ""
		if len(revised) != 0 {
			revisedLine = "Revised-by: ADR-" + strings.Join(revised, ", ADR-") + "\n"
		}
		_, err := assembleCorpus(
			map[string]metaEntry{"alpha/x": {meta: Metadata{Title: "X", Summary: "X.", Paths: []string{"x"}}, path: "meta"}},
			map[string]partEntry{"alpha/x": {data: []byte("Intro.\n\n## Claims\n\n### `rule: x`\nRule.\nOrigin: ADR-" + origin + "\n" + revisedLine), path: "part"}},
			[]string{"alpha"}, map[string][]string{"alpha": {"x"}}, adr.NewCorpus(records),
		)
		return err
	}

	legacy := adr.ADR{Number: "0001", Format: adr.Legacy, Status: "Implemented"}
	updating := adr.ADR{Number: "0002", Format: adr.CurrentStateV2, Status: "Implementing", Operations: []adr.Operation{op(adr.OpUpdate), {Verb: adr.OpAdd, ID: "alpha/x:later", Slug: "later"}}, History: []adr.HistoryEvent{status("Proposed"), status("Implementing"), applied(1, op(adr.OpUpdate))}}
	if err := assemble(t, []adr.ADR{legacy, updating}, "0001", "0002"); err != nil {
		t.Fatalf("applied Implementing revision rejected: %v", err)
	}
	pending := updating
	pending.Number, pending.Status, pending.History = "0003", "Proposed", []adr.HistoryEvent{status("Proposed")}
	if err := assemble(t, []adr.ADR{legacy, pending}, "0001", "0003"); err == nil || !strings.Contains(err.Error(), "without an applied update operation") {
		t.Fatalf("remaining provenance error = %v", err)
	}
	canceled := pending
	canceled.Number, canceled.Status, canceled.History = "0004", "Abandoned", []adr.HistoryEvent{status("Proposed"), status("Abandoned")}
	if err := assemble(t, []adr.ADR{legacy, canceled}, "0001", "0004"); err == nil || !strings.Contains(err.Error(), "without an applied update operation") {
		t.Fatalf("canceled provenance error = %v", err)
	}
	bad := updating
	bad.Number, bad.Status, bad.History = "0005", "Implemented", nil
	if err := assemble(t, []adr.ADR{legacy, bad}, "0001", "0005"); err == nil || !strings.Contains(err.Error(), "invalid ADR-0005 application") {
		t.Fatalf("invalid provenance projection error = %v", err)
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
	if got := metadata[id.String()].path; got != "first.yaml" {
		t.Fatalf("duplicate overwrote first path: %q", got)
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
