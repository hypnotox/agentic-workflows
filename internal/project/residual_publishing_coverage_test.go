package project

import (
	"context"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func TestResidualPitfallLoaderReadsSnapshotTree(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/docs/pitfalls/example.md", Mode: snapshot.Regular, Bytes: []byte("---\ntitle: Example\n---\nbody\n")}})
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := loadPitfallCorpusFrom(snapshotTreeReader{tree: tree})
	if err != nil || corpus.Len() != 1 {
		t.Fatalf("snapshot pitfall corpus = %#v, %v", corpus, err)
	}
}

func TestResidualSingletonProjectionAndTopicOpenFailure(t *testing.T) {
	entries := plainSingletons(catalog.Standard)
	if len(entries) == 0 {
		t.Fatal("standard catalog has no plain singleton")
	}
	layout := Layout{DocsDir: config.DocsDir}
	if got := entries[0].outPath(layout); got == "" {
		t.Fatal("plain singleton projected an empty output path")
	}
	if _, err := queryTopic(t.TempDir(), nil, context.Background(), "missing", topic.QueryOptions{}); err == nil {
		t.Fatal("queryTopic accepted a directory outside a repository")
	}
}
