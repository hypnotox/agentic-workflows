package topic

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func markerCorpus(backing Backing) Corpus {
	t := Topic{ID: TopicID{"alpha", "contracts"}, Metadata: Metadata{Paths: []string{"internal/**"}}, Claims: []Claim{{ID: "alpha/contracts:rule", Slug: "rule", Type: Rule}, {ID: "alpha/contracts:stable", Slug: "stable", Type: Invariant, Backing: backing}}}
	return Corpus{all: []Topic{t}, byTopic: map[string]*Topic{"alpha/contracts": &t}, byClaim: map[string]*Claim{"alpha/contracts:rule": &t.Claims[0], "alpha/contracts:stable": &t.Claims[1]}, DomainPaths: map[string][]string{"alpha": {"internal/**"}}}
}
func markerConfig() *config.CurrentStateConfig {
	return &config.CurrentStateConfig{Sources: []config.CurrentStateSource{{Globs: []string{"internal/**"}, Marker: "//"}, {Globs: []string{"web/**"}, Marker: "<!--", Close: "-->"}}, TestGlobs: []string{"internal/**/*_test.go"}}
}
func TestBuildMarkerIndex(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/a.go"), "// an ordinary comment\n// state machine transition\n// invariant checking helper\n// touches-stateful code\n // state: alpha/contracts:rule\n// touches-state: alpha/contracts:stable - reviewed here\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable\n")
	testsupport.WriteFile(t, filepath.Join(root, "web/x.html"), "<!-- ordinary comment without close\n<!-- state machine comment without close\n<!-- state: alpha/contracts:rule -->\n")
	testsupport.WriteFile(t, filepath.Join(root, "README.md"), "unmatched\n")
	testsupport.WriteFile(t, filepath.Join(root, ".git/ignored.go"), "// state: alpha/contracts:missing\n")
	c := markerCorpus(TestBacking)
	c.all[0].Metadata.Applies = "global"
	c.all[0].Metadata.Paths = nil
	c.byTopic["alpha/contracts"] = &c.all[0]
	idx, err := BuildMarkerIndex(root, c, markerConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.All()) != 4 || len(idx.ForClaim("alpha/contracts:rule")) != 2 || idx.ForClaim("none") != nil {
		t.Fatalf("sites %#v", idx.All())
	}
	if got := idx.All()[0]; got.Line != 5 || got.Path == "" {
		t.Fatalf("first site = %#v", got)
	}
}
func TestBuildMarkerIndexPrunesForeignTrees(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"internal/git-directory/.git/config",
		"internal/git-directory/ignored.go",
		"internal/gitfile/ignored.go",
		"internal/adopter/.awf/config.yaml",
		"internal/adopter/ignored.go",
		"internal/vendor/ignored.go",
		"internal/node_modules/ignored.go",
	} {
		body := "ignored\n"
		if strings.HasSuffix(path, ".go") {
			body = "// state: alpha/contracts:missing\n"
		}
		testsupport.WriteFile(t, filepath.Join(root, path), body)
	}
	testsupport.WriteFile(t, filepath.Join(root, "internal/gitfile/.git"), "gitdir: elsewhere\n")
	// Hidden directories other than the explicitly reserved .git tree follow
	// the current-state scanner's existing stance and remain eligible sources.
	testsupport.WriteFile(t, filepath.Join(root, "internal/.cache/kept.go"), "// state: alpha/contracts:rule\n")
	idx, err := BuildMarkerIndex(root, markerCorpus(Unbacked), markerConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := idx.ForClaim("alpha/contracts:rule"); len(got) != 1 || got[0].Path != "internal/.cache/kept.go" {
		t.Fatalf("sites %#v", idx.All())
	}
}

func TestBuildMarkerIndexWrapsDescendantWalkError(t *testing.T) {
	root := t.TempDir()
	want := errors.New("descendant unavailable")
	walk := func(path string, fn fs.WalkDirFunc) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err := fn(path, fs.FileInfoToDirEntry(info), nil); err != nil {
			return err
		}
		return fn(filepath.Join(path, "internal"), nil, want)
	}
	_, err := buildMarkerIndex(root, markerCorpus(Unbacked), markerConfig(), walk)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "scan current-state markers") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildMarkerIndexRejected(t *testing.T) {
	cases := map[string]struct {
		back       Backing
		path, line string
		mutate     func(*config.CurrentStateConfig)
	}{
		"malformed":       {TestBacking, "internal/a.go", "// state: nope\n// invariant: alpha/contracts:stable\n", nil},
		"unknown":         {TestBacking, "internal/a.go", "// state: alpha/contracts:missing\n// invariant: alpha/contracts:stable\n", nil},
		"out of scope":    {TestBacking, "web/out.html", "<!-- state: alpha/contracts:rule -->\n", nil},
		"proof test glob": {TestBacking, "internal/a.go", "// invariant: alpha/contracts:stable\n", nil},
		"proof rule":      {TestBacking, "internal/a_test.go", "// invariant: alpha/contracts:rule\n// invariant: alpha/contracts:stable\n", nil},
		"proof unbacked":  {Unbacked, "internal/a_test.go", "// invariant: alpha/contracts:stable\n", nil},
		"touches empty":   {TestBacking, "internal/a.go", "// touches-state: alpha/contracts:rule - \n// invariant: alpha/contracts:stable\n", nil},
		"missing close":   {TestBacking, "web/out.html", "<!-- state: alpha/contracts:rule\n", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(root, tc.path), tc.line)
			cfg := markerConfig()
			if tc.mutate != nil {
				tc.mutate(cfg)
			}
			if _, err := BuildMarkerIndex(root, markerCorpus(tc.back), cfg); err == nil {
				t.Fatal("wanted error")
			}
		})
	}
}
func TestBuildMarkerIndexBackingObligations(t *testing.T) {
	if _, err := BuildMarkerIndex(t.TempDir(), markerCorpus(TestBacking), nil); err == nil {
		t.Fatal("missing proof accepted")
	}
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable\n")
	if _, err := BuildMarkerIndex(root, markerCorpus(Unbacked), markerConfig()); err == nil {
		t.Fatal("unbacked proof accepted")
	}
	if idx, err := BuildMarkerIndex(t.TempDir(), markerCorpus(Unbacked), nil); err != nil || len(idx.All()) != 0 {
		t.Fatalf("unbacked no marker: %#v %v", idx, err)
	}
}
func TestSortSitesKindTie(t *testing.T) {
	sites := []MarkerSite{{Path: "x", Line: 1, Kind: TouchesMarker}, {Path: "x", Line: 1, Kind: StateMarker}}
	sortSites(sites)
	if sites[0].Kind != StateMarker {
		t.Fatalf("%#v", sites)
	}
}

func TestMarkerPayloadClosingToken(t *testing.T) {
	src := config.CurrentStateSource{Marker: "/*", Close: "*/"}
	if got, ok := markerPayload("/* state: alpha/contracts:rule */", src); !ok || got != "state: alpha/contracts:rule" {
		t.Fatalf("%q %v", got, ok)
	}
	if _, ok := markerPayload("// state: x", src); ok {
		t.Fatal("wrong opener")
	}
	if _, ok := markerPayload("/* state: x", src); ok {
		t.Fatal("missing close")
	}
}
