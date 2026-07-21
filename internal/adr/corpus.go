package adr

import (
	"fmt"
	"os"
	"sort"
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

// Status is an ADR lifecycle status as presented by semantic corpus queries.
type Status = string

// OperationRecord is the ADR identity and implementation order for one claim
// operation. StateSequence orders implemented mutations independently of ADR
// number.
type OperationRecord struct {
	Number        string
	Title         string
	Status        Status
	StateSequence int
}

// ClaimOperationHistory is the implemented add/update/remove history for one
// qualified claim identity.
type ClaimOperationHistory struct {
	Origin    *OperationRecord
	RevisedBy []OperationRecord
	RemovedBy *OperationRecord
}

// NewCorpus builds the view over an already-parsed slice.
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

// ClaimOperationHistory returns the implemented operation history for claimID
// in state-sequence order. The corpus-level current-state checker validates the
// add/update/remove lifecycle; this semantic view only projects that validated
// history and returns a fresh revision slice on every call.
func (c Corpus) ClaimOperationHistory(claimID string) (ClaimOperationHistory, bool) {
	type recordedOperation struct {
		verb   OpVerb
		record OperationRecord
	}
	var records []recordedOperation
	for _, a := range c.all {
		if !a.IsImplemented() || len(a.Operations) == 0 {
			continue
		}
		sequence := a.History[len(a.History)-1].Sequence
		for _, op := range a.Operations {
			if op.ID == claimID {
				records = append(records, recordedOperation{verb: op.Verb, record: OperationRecord{
					Number: a.Number, Title: a.Title, Status: a.Status, StateSequence: sequence,
				}})
			}
		}
	}
	if len(records) == 0 {
		return ClaimOperationHistory{}, false
	}
	sort.Slice(records, func(i, j int) bool { return records[i].record.StateSequence < records[j].record.StateSequence })
	history := ClaimOperationHistory{RevisedBy: []OperationRecord{}}
	for _, operation := range records {
		record := operation.record
		switch operation.verb {
		case OpAdd:
			history.Origin = &record
		case OpUpdate:
			history.RevisedBy = append(history.RevisedBy, record)
		case OpRemove:
			history.RemovedBy = &record
		}
	}
	history.RevisedBy = append([]OperationRecord(nil), history.RevisedBy...)
	return history, true
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
