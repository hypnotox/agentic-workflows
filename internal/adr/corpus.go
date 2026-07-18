package adr

import (
	"fmt"
	"os"
)

// Corpus is the parsed decisions directory: one parse, threaded to every
// consumer that needs an ADR fact (ADR-0130 item 1). It answers questions
// rather than exposing fields for a caller to re-derive an answer from
// (item 2), which is what collapsed the three-way "is live" and the twice-built
// supersession relation into one place.
//
// The zero value is not useful; construct with NewCorpus.
type Corpus struct {
	all   []ADR
	byNum map[string]ADR
}

// NewCorpus builds the view over an already-parsed slice. Construction is the
// single seam where derived structure is built, so nothing downstream rebuilds
// it (corpus-model-not-rebuilt).
func NewCorpus(adrs []ADR) Corpus {
	byNum := make(map[string]ADR, len(adrs))
	for _, a := range adrs {
		byNum[a.Number] = a
	}
	return Corpus{all: adrs, byNum: byNum}
}

// LoadCorpus parses a decisions directory into the view. It is the single
// construction seam: adr.ParseDir has no production caller outside this
// package, so every consumer - the *Project that threads the view to the
// checks, and the schema migrations, which run before a Project can be opened
// and so cannot be handed one - enters through here.
func LoadCorpus(dir string) (Corpus, error) {
	adrs, err := ParseDir(dir)
	if err != nil {
		return Corpus{}, err
	}
	return NewCorpus(adrs), nil
}

// All returns every parsed ADR in directory order.
func (c Corpus) All() []ADR { return c.all }

// ByNumber returns the ADR with the given four-digit number. The ADR number is
// the sole identity key (ADR-0130 item 4).
func (c Corpus) ByNumber(num string) (ADR, bool) {
	a, ok := c.byNum[num]
	return a, ok
}

// Has reports whether the corpus contains an ADR with the given number.
func (c Corpus) Has(num string) bool {
	_, ok := c.byNum[num]
	return ok
}

// DecisionItems returns the Decision item numbers the named ADR enumerates.
// An absent ADR yields no items rather than an error: every caller is already
// validating existence separately, and a token into a missing target is that
// check's finding to report, not this one's.
func (c Corpus) DecisionItems(num string) []int {
	a, ok := c.byNum[num]
	if !ok {
		return nil
	}
	return a.DecisionItems()
}

// DeclaredSlugs returns the invariant slugs the named ADR declares, backed and
// unbacked alike, in declaration order.
func (c Corpus) DeclaredSlugs(num string) []string {
	a, ok := c.byNum[num]
	if !ok {
		return nil
	}
	return a.DeclaredSlugs()
}

// RefsOf returns the supersession tokens the named ADR carries, in document
// order. This is the "what does this ADR claim" question consumers previously
// answered by ranging over ADR.Refs themselves (corpus-owns-field-reads).
//
// The mirror question - "who claims this anchor" - is not here yet: nothing
// asks it until the coverage model needs it, and the dead-code gate refuses a
// production method no main can reach.
func (c Corpus) RefsOf(num string) []SupersessionRef {
	a, ok := c.byNum[num]
	if !ok {
		return nil
	}
	return a.Refs
}

// Raw returns the ADR file's bytes. Raw access is enumerated and closed
// (ADR-0130 item 6): the migration's offset surgery and the retired-key
// frontmatter scan are the only two legitimate consumers below the semantic
// layer. A third caller means the view is missing a question.
func (c Corpus) Raw(num string) ([]byte, error) {
	a, ok := c.byNum[num]
	if !ok {
		return nil, fmt.Errorf("no ADR %s in corpus", num)
	}
	return os.ReadFile(a.Path)
}
