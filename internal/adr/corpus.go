package adr

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Corpus is the parsed decisions directory: one parse, threaded to every
// consumer that needs an ADR fact (ADR-0130 item 1). It answers questions
// rather than exposing fields for a caller to re-derive an answer from
// (item 2), which is what collapsed the three-way "is live" and the twice-built
// supersession relation into one place.
//
// The zero value is not useful; construct with NewCorpus.
type Corpus struct {
	all    []ADR
	byNum  map[string]ADR
	bySlug map[string]ADR
}

// DuplicateIdentityError reports corpus identity keys declared by more than one
// file. It is typed because exactly one consumer needs the detail rather than
// the message: the numbering command refuses on a duplicate-number corpus and
// hints the reset-remake recipe (ADR-0202 item 12). Every other consumer treats
// it as fatal.
type DuplicateIdentityError struct {
	Numbers []string
	Slugs   []string
}

func (e *DuplicateIdentityError) Error() string {
	parts := make([]string, 0, len(e.Numbers)+len(e.Slugs))
	for _, n := range e.Numbers {
		parts = append(parts, fmt.Sprintf("ADR number %s is declared by more than one file", n))
	}
	for _, s := range e.Slugs {
		parts = append(parts, fmt.Sprintf("ADR slug %q is declared by more than one file", s))
	}
	return strings.Join(parts, "; ")
}

// Status is an ADR lifecycle status as presented by semantic corpus queries.
type Status = string

// OperationRecord is the ADR identity for one claim operation: the number for a
// numbered record and the slug for a pending one. Ascending ADR number is the
// per-claim provenance order (ADR-0191), with pending records after every number
// (ADR-0202 item 10).
type OperationRecord struct {
	Identity string
	Title    string
	Status   Status
}

// ClaimOperationHistory is the implemented add/update/remove history for one
// qualified claim identity. LegacyBaseline is true when a retained removal has
// no recorded add, so the history has an unrecorded pre-operation baseline.
type ClaimOperationHistory struct {
	Origin         *OperationRecord
	LegacyBaseline bool
	RevisedBy      []OperationRecord
	RemovedBy      *OperationRecord
}

// NewCorpus builds the view over an already-parsed slice, indexing both
// identity keys: the four-digit number of a numbered record and the retained
// slug of every slug-carrying record, pending or numbered (ADR-0202 item 3).
//
// A duplicate in either index is a *DuplicateIdentityError. The returned Corpus
// is still populated, last-wins, so the numbering command can read the pending
// set out of a colliding corpus to build its refusal; every other caller treats
// the error as fatal.
func NewCorpus(adrs []ADR) (Corpus, error) {
	byNum := make(map[string]ADR, len(adrs))
	bySlug := make(map[string]ADR, len(adrs))
	var duplicate DuplicateIdentityError
	for _, a := range adrs {
		if a.Number != "" {
			if _, seen := byNum[a.Number]; seen && !slices.Contains(duplicate.Numbers, a.Number) {
				duplicate.Numbers = append(duplicate.Numbers, a.Number)
			}
			byNum[a.Number] = a
		}
		if a.Slug != "" {
			if _, seen := bySlug[a.Slug]; seen && !slices.Contains(duplicate.Slugs, a.Slug) {
				duplicate.Slugs = append(duplicate.Slugs, a.Slug)
			}
			bySlug[a.Slug] = a
		}
	}
	c := Corpus{all: adrs, byNum: byNum, bySlug: bySlug}
	if len(duplicate.Numbers) != 0 || len(duplicate.Slugs) != 0 {
		return c, &duplicate
	}
	return c, nil
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
	for i, a := range adrs {
		data, err := os.ReadFile(a.Path)
		if err != nil { // coverage-ignore: ParseDir just read the same discovered file
			return Corpus{}, err
		}
		parsed, err := ParseRecord(a.Filename, data)
		if err != nil {
			return Corpus{}, fmt.Errorf("parse %s: %w", a.Filename, err)
		}
		parsed.Path = a.Path
		adrs[i] = parsed
	}
	return NewCorpus(adrs)
}

// All returns every parsed ADR in directory order.
func (c Corpus) All() []ADR { return c.all }

// NextIdentity returns one more than the highest ADR identity, or 1 for an
// empty corpus. Migration code uses this semantic query rather than adding a
// raw decisions-directory reader.
func (c Corpus) NextIdentity() (int, error) {
	max := 0
	for _, a := range c.all {
		if a.Number == "" {
			continue // a pending record holds no numeric identity to succeed
		}
		n, err := strconv.Atoi(a.Number)
		if err != nil { // coverage-ignore: past the numberless guard, Number is the four-digit group FilenameRe captured
			return 0, err
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// ByNumber returns the ADR with the given four-digit number.
func (c Corpus) ByNumber(num string) (ADR, bool) {
	a, ok := c.byNum[num]
	return a, ok
}

// Has reports whether the corpus contains an ADR with the given number.
func (c Corpus) Has(num string) bool {
	_, ok := c.byNum[num]
	return ok
}

// BySlug returns the record carrying the given retained slug, pending or
// numbered.
func (c Corpus) BySlug(slug string) (ADR, bool) {
	a, ok := c.bySlug[slug]
	return a, ok
}

// ByIdentity returns the record whose identity key equals id: a four-digit
// number resolves against the numbered index, anything else against the slug
// index (ADR-0202 item 4).
func (c Corpus) ByIdentity(id string) (ADR, bool) {
	if IsSlugIdentity(id) {
		return c.BySlug(id)
	}
	return c.ByNumber(id)
}

// OperationProgress returns the operation partition for one ADR identity.
// Missing and invalid-present records are deliberately distinct.
func (c Corpus) OperationProgress(identity string) (OperationProgress, bool, error) {
	a, ok := c.ByIdentity(identity)
	if !ok {
		return OperationProgress{}, false, nil
	}
	progress, err := a.OperationProgress()
	if err != nil {
		return OperationProgress{}, true, err
	}
	progress.Applied = append([]AppliedOperation(nil), progress.Applied...)
	progress.Remaining = append([]Operation(nil), progress.Remaining...)
	progress.Canceled = append([]Operation(nil), progress.Canceled...)
	return progress, true, nil
}

// ClaimOperationHistory returns applied operation history for claimID in
// ascending ADR-number order. Remaining and canceled operations are excluded,
// and every returned slice is fresh.
func (c Corpus) ClaimOperationHistory(claimID string) (ClaimOperationHistory, bool) {
	type recordedOperation struct {
		verb   OpVerb
		record OperationRecord
	}
	var records []recordedOperation
	for _, a := range c.all {
		progress, err := a.OperationProgress()
		if err != nil {
			continue
		}
		for _, applied := range progress.Applied {
			if applied.Operation.ID == claimID {
				records = append(records, recordedOperation{verb: applied.Operation.Verb, record: OperationRecord{
					Identity: a.Identity(), Title: a.Title, Status: a.Status,
				}})
			}
		}
	}
	if len(records) == 0 {
		return ClaimOperationHistory{}, false
	}
	sort.SliceStable(records, func(i, j int) bool {
		return IdentityOrder(records[i].record.Identity) < IdentityOrder(records[j].record.Identity)
	})
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
	history.LegacyBaseline = history.Origin == nil && history.RemovedBy != nil
	history.RevisedBy = append([]OperationRecord(nil), history.RevisedBy...)
	return history, true
}

// Raw returns the ADR file's bytes. Raw access is enumerated and closed
// (ADR-0130 item 6): the retirement-token offset surgery, the retired-key
// frontmatter scan, and the ADR-0191 state-sequence retrofit are the only
// three legitimate consumers below the semantic layer. A fourth caller means
// the view is missing a question.
func (c Corpus) Raw(num string) ([]byte, error) {
	a, ok := c.byNum[num]
	if !ok {
		return nil, fmt.Errorf("no ADR %s in corpus", num)
	}
	return os.ReadFile(a.Path)
}
