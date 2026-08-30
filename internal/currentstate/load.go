package currentstate

import (
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Loaded is one immutable current-state snapshot view.
type Loaded struct {
	Topics topic.Corpus
}

// LoadFromTree assembles the current-state topic corpus from one snapshot tree.
// cfg supplies configured domains and marker-source families.
func LoadFromTree(tree *snapshot.Tree, cfg *config.Config) (Loaded, error) {
	topics, err := topic.LoadCorpusFromTree(tree, cfg)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{Topics: topics}, nil
}
