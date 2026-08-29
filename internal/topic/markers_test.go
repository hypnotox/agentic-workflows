package topic

import (
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

func markerIndexForTest(t *testing.T, root string, corpus Corpus, cfg *config.CurrentStateConfig) (MarkerIndex, error) {
	t.Helper()
	return markerIndexFromTreeFiles(treeFromDir(t, root).List(), corpus, cfg)
}

func TestByteLineSourceAcceptsExactMarkerBoundary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		length  int
		wantErr bool
	}{
		{name: "exact", length: maxMarkerLineBytes},
		{name: "over", length: maxMarkerLineBytes + 1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			visits := 0
			_, err := byteLineSource([]byte(strings.Repeat("x", tc.length)))(func(line string) error {
				visits++
				if len(line) != tc.length {
					t.Fatalf("line length = %d, want %d", len(line), tc.length)
				}
				return nil
			})
			if tc.wantErr {
				if err == nil || visits != 0 {
					t.Fatalf("over-bound read visits=%d error=%v", visits, err)
				}
				return
			}
			if err != nil || visits != 1 {
				t.Fatalf("exact-bound read visits=%d error=%v", visits, err)
			}
		})
	}
}

// invariant: invariants/topics-and-markers:invariants-marker-whitespace (TestBuildMarkerIndex)
// invariant: invariants/topics-and-markers:invariants-multilang-scan (TestBuildMarkerIndex)
// invariant: invariants/topics-and-markers:proof-only-marker-grammar (TestBuildMarkerIndex)
func TestBuildMarkerIndex(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/a.go"), "// state: alpha/contracts:rule\n// touches-state: alpha/contracts:stable - inert\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), " // invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n")
	testsupport.WriteFile(t, filepath.Join(root, "web/x.html"), "<!-- state: alpha/contracts:rule\n")
	idx, err := markerIndexForTest(t, root, markerCorpus(TestBacking), markerConfig())
	if err != nil {
		t.Fatal(err)
	}
	sites := idx.All()
	if len(sites) != 1 || sites[0].Kind != ProofMarker || sites[0].ClaimID != "alpha/contracts:stable" {
		t.Fatalf("sites %#v", sites)
	}
}

func TestMarkerIndexFromTreeSkipsNestedAdoptedProject(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/adopter/.awf/config.yaml"), "prefix: nested\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/adopter/ignored_test.go"), "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n")
	idx, err := markerIndexForTest(t, root, markerCorpus(Unbacked), markerConfig())
	if err != nil || len(idx.All()) != 0 {
		t.Fatalf("sites %#v err=%v", idx.All(), err)
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
		"malformed proof": {TestBacking, "internal/a_test.go", "// invariant: nope\n", nil, "malformed current-state marker"},
		"unknown proof":   {TestBacking, "internal/a_test.go", "// invariant: alpha/contracts:missing (TestMissing)\nfunc TestMissing() {}\n", nil, "unknown claim ID"},
		"proof test glob": {TestBacking, "internal/a.go", "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n", nil, "proof marker is outside currentState.testGlobs"},
		"proof rule":      {TestBacking, "internal/a_test.go", "// invariant: alpha/contracts:rule (TestRule)\nfunc TestRule() {}\n", nil, "proof marker targets non-test-backed invariant"},
		"proof unbacked":  {Unbacked, "internal/a_test.go", "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n", nil, "proof marker targets non-test-backed invariant"},
		"missing close":   {TestBacking, "web/out.html", "<!-- invariant: alpha/contracts:stable (TestStable)\nTestStable\n", func(c *config.CurrentStateConfig) { c.TestGlobs = append(c.TestGlobs, "web/**") }, "missing closing token"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(root, tc.path), tc.line)
			cfg := markerConfig()
			if tc.mutate != nil {
				tc.mutate(cfg)
			}
			_, err := markerIndexForTest(t, root, markerCorpus(tc.back), cfg)
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
	if _, err := markerIndexForTest(t, t.TempDir(), markerCorpus(TestBacking), nil); err == nil {
		t.Fatal("missing proof accepted")
	}
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n")
	if _, err := markerIndexForTest(t, root, markerCorpus(Unbacked), markerConfig()); err == nil {
		t.Fatal("unbacked proof accepted")
	}
	if idx, err := markerIndexForTest(t, t.TempDir(), markerCorpus(Unbacked), nil); err != nil || len(idx.All()) != 0 {
		t.Fatalf("unbacked no marker: %#v %v", idx, err)
	}
}

// invariant: invariants/topics-and-markers:invariant-marker-close-token (TestMarkerPayloadClosingToken)
// invariant: invariants/topics-and-markers:invariants-marker-literal (TestMarkerPayloadClosingToken)
func TestMarkerPayloadClosingToken(t *testing.T) {
	src := config.CurrentStateSource{Marker: "/*", Close: "*/"}
	if got, ok := markerPayload("/* invariant: alpha/contracts:stable (TestStable) */", src); !ok || got != "invariant: alpha/contracts:stable (TestStable)" {
		t.Fatalf("%q %v", got, ok)
	}
	if _, ok := markerPayload("// invariant: x", src); ok {
		t.Fatal("wrong opener")
	}
	if _, ok := markerPayload("/* invariant: x", src); ok {
		t.Fatal("missing close")
	}
}

// A proof marker must carry a trailing name; a state marker may not. The accepted
// bodies pin that the name is free text rather than an identifier, which keeps the
// rule portable to an adopter whose tests are string literals, not named functions.
// invariant: invariants/topics-and-markers:proof-marker-names-its-unit (TestBuildMarkerIndexRequiresAProofName)
func TestBuildMarkerIndexRequiresAProofName(t *testing.T) {
	for _, body := range []string{
		"// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n",
		"// invariant: alpha/contracts:stable (it('strips the header'))\nit('strips the header')\n",
		"// invariant: alpha/contracts:stable (T)\nfunc T() {}\n",
	} {
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), body)
		idx, err := markerIndexForTest(t, root, markerCorpus(TestBacking), markerConfig())
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
		_, err := markerIndexForTest(t, root, markerCorpus(TestBacking), markerConfig())
		if err == nil || !strings.Contains(err.Error(), "does not name a proving unit") {
			t.Fatalf("payload %q: err = %v, want it to report no proving unit", payload, err)
		}
	}

	// A named state marker is not a state marker with a note: the name group belongs
	// to the proof expression alone, so this falls through to malformed.
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/a.go"), "// state: alpha/contracts:rule (TestThing)\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n")
	idx, err := markerIndexForTest(t, root, markerCorpus(TestBacking), markerConfig())
	if err != nil || len(idx.All()) != 1 {
		t.Fatalf("named state comment was not inert: sites=%#v err=%v", idx.All(), err)
	}
}

// The name must actually occur in the file, on a line that does not itself open
// with the marker token. Each case below is one load-bearing mechanism of that
// rule; without any one of them a stranded marker stays green.
// invariant: invariants/topics-and-markers:proof-marker-names-its-unit (TestProofNameMustOccurInTheFile)
func TestProofNameMustOccurInTheFile(t *testing.T) {
	build := func(body string) error {
		t.Helper()
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), body)
		_, err := markerIndexForTest(t, root, markerCorpus(TestBacking), markerConfig())
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
		t.Fatalf("trailing rune-flanked name: err = %v", err)
	}

	// Both sides decode runes, so the leading side needs its own non-ASCII case:
	// testing only the trailing one lets identBefore revert to a byte test unseen.
	err = build("// invariant: alpha/contracts:stable (TestStable)\nfunc éTestStable() {}\n")
	if err == nil || !strings.Contains(err.Error(), "does not occur in this file") {
		t.Fatalf("leading rune-flanked name: err = %v", err)
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
