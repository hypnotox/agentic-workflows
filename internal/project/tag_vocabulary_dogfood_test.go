package project

import (
	"maps"
	"slices"
	"testing"
)

func TestTagVocabularyMatchesPitfallTags(t *testing.T) {
	root := repoRootDir(t)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := loadPitfallCorpus(p)
	if err != nil {
		t.Fatal(err)
	}

	pitfallTagSet := map[string]bool{}
	for _, entry := range corpus.All() {
		for _, tag := range entry.Tags {
			pitfallTagSet[tag] = true
		}
	}
	configured := slices.Sorted(maps.Keys(p.Cfg.Tags))
	pitfallTags := slices.Sorted(maps.Keys(pitfallTagSet))
	if surplus, missing := difference(configured, pitfallTags), difference(pitfallTags, configured); len(surplus) != 0 || len(missing) != 0 {
		t.Fatalf("tag vocabulary does not match current pitfall tags\nconfigured without pitfall consumers=%v\npitfall tags missing from config=%v", surplus, missing)
	}
}
