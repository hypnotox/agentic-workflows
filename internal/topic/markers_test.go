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

// invariant: invariants/topics-and-markers:invariants-marker-whitespace (TestBuildMarkerIndex)
// invariant: invariants/topics-and-markers:invariants-multilang-scan (TestBuildMarkerIndex)
// invariant: invariants/topics-and-markers:touches-marker-advisory (TestBuildMarkerIndex)
func TestBuildMarkerIndex(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/a.go"), "// an ordinary comment\n// state machine transition\n// invariant checking helper\n// touches-stateful code\n // state: alpha/contracts:rule\n// touches-state: alpha/contracts:stable - reviewed here\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n")
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

// invariant: invariants/topics-and-markers:proof-marker-test-scoped (TestBuildMarkerIndexRejected)
func TestBuildMarkerIndexRejected(t *testing.T) {
	// Every proof marker below carries a name and a matching declaration line, so
	// each case still fails for the reason it is named after rather than newly
	// failing on the name rule. want pins that.
	cases := map[string]struct {
		back       Backing
		path, line string
		mutate     func(*config.CurrentStateConfig)
		want       string
	}{
		"malformed":       {TestBacking, "internal/a.go", "// state: nope\n// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n", nil, "malformed current-state marker"},
		"unknown":         {TestBacking, "internal/a.go", "// state: alpha/contracts:missing\n// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n", nil, "unknown claim ID"},
		"out of scope":    {TestBacking, "web/out.html", "<!-- state: alpha/contracts:rule -->\n", nil, "outside effective topic scope"},
		"proof test glob": {TestBacking, "internal/a.go", "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n", nil, "proof marker is outside currentState.testGlobs"},
		"proof rule":      {TestBacking, "internal/a_test.go", "// invariant: alpha/contracts:rule (TestRule)\nfunc TestRule() {}\n", nil, "proof marker targets non-test-backed invariant"},
		"proof unbacked":  {Unbacked, "internal/a_test.go", "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n", nil, "proof marker targets non-test-backed invariant"},
		"touches empty":   {TestBacking, "internal/a.go", "// touches-state: alpha/contracts:rule - \n// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n", nil, "malformed current-state marker"},
		"missing close":   {TestBacking, "web/out.html", "<!-- state: alpha/contracts:rule\n", nil, "missing closing token"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(root, tc.path), tc.line)
			cfg := markerConfig()
			if tc.mutate != nil {
				tc.mutate(cfg)
			}
			_, err := BuildMarkerIndex(root, markerCorpus(tc.back), cfg)
			if err == nil {
				t.Fatal("wanted error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// invariant: invariants/topics-and-markers:backed-requires-proof (TestBuildMarkerIndexBackingObligations)
// invariant: invariants/topics-and-markers:invariants-three-state (TestBuildMarkerIndexBackingObligations)
// invariant: invariants/topics-and-markers:invariants-unbacked-detected (TestBuildMarkerIndexBackingObligations)
// invariant: invariants/topics-and-markers:unbacked-refuses-proof (TestBuildMarkerIndexBackingObligations)
func TestBuildMarkerIndexBackingObligations(t *testing.T) {
	if _, err := BuildMarkerIndex(t.TempDir(), markerCorpus(TestBacking), nil); err == nil {
		t.Fatal("missing proof accepted")
	}
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n")
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

// invariant: invariants/topics-and-markers:invariant-marker-close-token (TestMarkerPayloadClosingToken)
// invariant: invariants/topics-and-markers:invariants-marker-literal (TestMarkerPayloadClosingToken)
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

// A proof marker must carry a trailing name; a state marker may not. The accepted
// bodies pin that the name is free text rather than an identifier, which keeps the
// rule portable to an adopter whose tests are string literals, not named functions.
func TestBuildMarkerIndexRequiresAProofName(t *testing.T) {
	for _, body := range []string{
		"// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n",
		"// invariant: alpha/contracts:stable (it('strips the header'))\nit('strips the header')\n",
		"// invariant: alpha/contracts:stable (T)\nfunc T() {}\n",
	} {
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), body)
		idx, err := BuildMarkerIndex(root, markerCorpus(TestBacking), markerConfig())
		if err != nil {
			t.Fatalf("body %q rejected: %v", body, err)
		}
		sites := idx.ForClaim("alpha/contracts:stable")
		if len(sites) != 1 || sites[0].Kind != ProofMarker {
			t.Errorf("body %q resolved to %+v, want one proof site", body, sites)
		}
	}

	// A bare payload, and the padded and empty parentheticals, all reach the named
	// diagnostic rather than falling through to the generic malformed-marker error.
	// The declaration is present in every case, so only the payload is on trial.
	for _, payload := range []string{
		"// invariant: alpha/contracts:stable\n",
		"// invariant: alpha/contracts:stable ( TestStable )\n",
		"// invariant: alpha/contracts:stable ()\n",
	} {
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), payload+"func TestStable() {}\n")
		_, err := BuildMarkerIndex(root, markerCorpus(TestBacking), markerConfig())
		if err == nil || !strings.Contains(err.Error(), "does not name a proving unit") {
			t.Fatalf("payload %q: err = %v, want it to report no proving unit", payload, err)
		}
	}

	// A named state marker is not a state marker with a note: the name group belongs
	// to the proof expression alone, so this falls through to malformed.
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/a.go"), "// state: alpha/contracts:rule (TestThing)\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n")
	_, err := BuildMarkerIndex(root, markerCorpus(TestBacking), markerConfig())
	if err == nil || !strings.Contains(err.Error(), "malformed current-state marker") {
		t.Fatalf("named state marker: err = %v, want malformed current-state marker", err)
	}
}

// The name must actually occur in the file, on a line that is neither a comment
// nor the marker's own. Each case below is one load-bearing mechanism of that
// rule; without any one of them a stranded marker stays green.
func TestProofNameMustOccurInTheFile(t *testing.T) {
	build := func(body string) error {
		t.Helper()
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), body)
		_, err := BuildMarkerIndex(root, markerCorpus(TestBacking), markerConfig())
		return err
	}

	// The named unit appears nowhere in the file: the plain stranding.
	err := build("// invariant: alpha/contracts:stable (TestStable)\nfunc TestOther() {}\n")
	if err == nil || !strings.Contains(err.Error(), `names "TestStable", which does not occur in this file`) {
		t.Fatalf("absent name: err = %v", err)
	}

	// The name's sole occurrence is inside a comment. This is the regression test
	// for the class the comment exclusion closes: this repo's convention puts a doc
	// comment naming the test directly above the marker block, and without the
	// exclusion that comment alone would satisfy a marker whose test is gone.
	err = build("// TestStable pins the contract.\n// invariant: alpha/contracts:stable (TestStable)\nfunc TestOther() {}\n")
	if err == nil || !strings.Contains(err.Error(), "does not occur in this file") {
		t.Fatalf("comment-only name: err = %v", err)
	}

	// Flanking, trailing side. A rename leaves a longer identifier behind; without
	// the flanking condition it would satisfy the marker, and a rename is exactly
	// the drift this check exists to catch.
	err = build("// invariant: alpha/contracts:stable (TestStable)\nfunc TestStableAndMore() {}\n")
	if err == nil || !strings.Contains(err.Error(), "does not occur in this file") {
		t.Fatalf("trailing-flanked name: err = %v", err)
	}

	// Flanking, leading side. Both halves of the condition are load-bearing, so a
	// prefix rename must fail too; testing only the trailing half lets the leading
	// half be deleted with the suite green.
	err = build("// invariant: alpha/contracts:stable (TestStable)\nfunc XTestStable() {}\n")
	if err == nil || !strings.Contains(err.Error(), "does not occur in this file") {
		t.Fatalf("leading-flanked name: err = %v", err)
	}

	// Flanking is over runes, not bytes: a non-ASCII letter abutting the name is an
	// identifier character too, or an adopter whose labels carry accented letters
	// loses the rename protection an ASCII one gets.
	err = build("// invariant: alpha/contracts:stable (TestStable)\nfunc TestStableé() {}\n")
	if err == nil || !strings.Contains(err.Error(), "does not occur in this file") {
		t.Fatalf("rune-flanked name: err = %v", err)
	}

	// A flanked hit must not abandon the line: the same name can occur twice on one
	// line, once inside a longer identifier and once on its own. Scanning only the
	// first occurrence would reject this live test as deleted.
	if err = build("// invariant: alpha/contracts:stable (TestStable)\nfunc TestStableWrapper() { TestStable() }\n"); err != nil {
		t.Fatalf("second unflanked occurrence on the same line rejected: %v", err)
	}

	// Stacked markers naming the same absent unit do not satisfy each other, and
	// the first one is what fails. They must name the SAME unit: two different
	// absent names would fail because neither name occurs at all and would merely
	// duplicate the first case, whereas this pins that the sibling marker LINE is
	// excluded from the search. The second marker is unreachable in this run by
	// design, since the scan returns on its first error.
	err = build("// invariant: alpha/contracts:stable (TestStable)\n// invariant: alpha/contracts:stable (TestStable)\nfunc TestOther() {}\n")
	if err == nil || !strings.Contains(err.Error(), "a_test.go:1:") {
		t.Fatalf("stacked markers: err = %v, want the first marker's line", err)
	}
}
