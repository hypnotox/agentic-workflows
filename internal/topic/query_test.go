package topic

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func loadedQueryFixture(t *testing.T) Corpus {
	t.Helper()
	root, _ := corpusFixture(t)
	cfg, err := config.Parse(filepath.Join(root, ".awf"), []byte(`prefix: test
integrationBranch: main
domains: [alpha, beta]
currentState:
  sources:
    - globs: ["internal/**", "pkg/**"]
      marker: "//"
  testGlobs: ["internal/**/*_test.go"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	writeTopic(t, root, "alpha", "contracts", "title: Contracts\nsummary: Current contracts.\npaths: [\"internal/**\"]\n", "Intro.\n\n## Claims\n\n### `rule: order`\nDeterministic order.\nReferences: beta/global:shared\n\n### `invariant: stable`\nStable output.\nBacking: unbacked\nVerify: compare snapshots.\n")
	writeTopic(t, root, "beta", "global", "title: Global\nsummary: Global contracts.\napplies: global\n", rulePart("shared", "alpha/contracts:stable"))
	testsupport.WriteFile(t, filepath.Join(root, "internal/schedule.go"), "package schedule\n// touches-state: alpha/contracts:order - scheduler entry point\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal/stable_test.go"), "package schedule\n// touches-state: alpha/contracts:stable - snapshot boundary\n")
	testsupport.WriteFile(t, filepath.Join(root, "pkg/global.go"), "package global\n// state: beta/global:shared\n")
	corpus, err := loadCorpusForTest(t, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return corpus
}

// invariant: invariants/current-state-authority:current-state-adr-independent (TestQueryCurrentStateWithoutADRCorpus)
func TestQueryCurrentStateWithoutADRCorpus(t *testing.T) {
	corpus := loadedQueryFixture(t)
	topicResult, err := Query(corpus, "alpha/contracts", QueryOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if topicResult.Kind != "topic" || len(topicResult.Claims) != 2 || topicResult.Claims[0].Backing != ExplicitNoBacking || topicResult.References != nil || topicResult.Coverage != nil {
		t.Fatalf("topic result = %#v", topicResult)
	}
	claimResult, err := Query(corpus, "alpha/contracts:stable", QueryOptions{}, nil)
	if err != nil || claimResult.Kind != "claim" || len(claimResult.Claims) != 1 {
		t.Fatalf("claim result = %#v %v", claimResult, err)
	}
}
func TestQueryReferencesAndCoverage(t *testing.T) {
	corpus := loadedQueryFixture(t)
	currentPaths := []string{"internal/schedule.go", "internal/stable_test.go", "pkg/global.go"}
	got, err := Query(corpus, "alpha/contracts", QueryOptions{References: true, Coverage: true}, currentPaths)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.References) != 2 || got.Coverage == nil || !reflect.DeepEqual(got.References[0].Outgoing, []string{"beta/global:shared"}) {
		t.Fatalf("result = %#v", got)
	}
	claim, err := Query(corpus, "alpha/contracts:stable", QueryOptions{Coverage: true}, currentPaths)
	if err != nil || len(claim.Coverage.Applicability.MarkerSites) != 0 {
		t.Fatalf("claim = %#v %v", claim, err)
	}
}
func TestQueryMissingActiveClaimIsNotFound(t *testing.T) {
	corpus := loadedQueryFixture(t)
	if _, err := Query(corpus, "alpha/contracts:missing", QueryOptions{References: true, Coverage: true}, nil); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Query missing claim = %v", err)
	}
}
func TestQuerySelectorsAndStableJSON(t *testing.T) {
	for _, selector := range []string{"", "alpha", "Alpha/topic", "alpha/topic:", "alpha/topic:bad:more"} {
		if _, _, err := ParseSelector(selector); err == nil {
			t.Errorf("ParseSelector(%q) accepted", selector)
		}
	}
	if _, err := Query(Corpus{}, "bad", QueryOptions{}, nil); err == nil || !strings.Contains(err.Error(), "invalid topic selector") {
		t.Fatalf("Query malformed selector = %v", err)
	}
	result, err := Query(loadedQueryFixture(t), "alpha/contracts", QueryOptions{References: true, Coverage: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	one, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := json.Marshal(result)
	if !reflect.DeepEqual(one, two) || !strings.Contains(string(one), `"claimId"`) || !strings.Contains(string(one), `"backing":"none"`) || strings.Contains(string(one), "history") {
		t.Fatalf("unstable or legacy JSON: %s", one)
	}
}
