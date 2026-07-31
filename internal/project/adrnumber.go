package project

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// NumberAssignment is one pending record's slug and the number numbering gave
// it.
type NumberAssignment struct {
	Slug   string
	Number string
}

// NumberingReport is the mapping one numbering run assigned, in assignment
// order, so it can be pasted into the integration commit message.
type NumberingReport struct {
	Assignments []NumberAssignment
}

// String renders one `<slug> -> NNNN` line per assignment.
func (r NumberingReport) String() string {
	var b strings.Builder
	for _, a := range r.Assignments {
		fmt.Fprintf(&b, "%s -> %s\n", a.Slug, a.Number)
	}
	return b.String()
}

// duplicateNumbersRecipe is the reset-remake recipe a duplicate-number corpus
// with no pending record is offered verbatim (ADR-0194 item 12). It is a hint,
// never an action: numbering unmakes a stale numbering commit by reset, and
// reads no git provenance to decide that for the caller.
const duplicateNumbersRecipe = "duplicate ADR numbers with no pending record: if a stale numbering commit collided, " +
	"run: git reset --hard HEAD~1 && git merge <integration branch> && awf adr number, then gate and merge back"

// NumberPendingADRs numbers the corpus's pending records at integration
// (ADR-0194 item 8). It runs in the effort worktree after the integration
// branch has been merged in and before the merge back: with exactly one pending
// record a bare call numbers it, and with several the caller must name every
// pending slug in the intended add-before-revise order.
//
// The effect surface is exhaustive and matches item 9: each named record is
// renamed and its heading rewritten through internal/adr's seam, the authored
// claim parts take the slug-to-number substitution, and the project re-renders
// so the generated topic docs and the decision INDEX match. No status-history
// event, no already-numbered record, and no plan is touched.
//
// It deliberately does not precondition on a green check. A green check between
// merge-in and numbering is the norm now that ADR-0191 removed the global state
// sequence, but an unrelated merge finding must not deadlock the one command
// that can resolve the corpus.
//
// Every refusal happens before the first rename, so a refused run leaves the
// corpus exactly as it found it. Past that point the run is partial-completion:
// the renames and the substitution are already on disk, so a failing re-render
// returns the assignments alongside its error rather than an empty report. The
// mapping is what the integration commit message needs, and the caller cannot
// reconstruct it once the pending files are gone.
func (p *Project) NumberPendingADRs(ctx context.Context, slugs []string) (NumberingReport, error) {
	corpus, duplicates, err := p.numberingCorpus()
	if err != nil {
		return NumberingReport{}, err
	}
	order, err := numberingOrder(pendingSlugs(corpus), slugs, duplicates)
	if err != nil {
		return NumberingReport{}, err
	}
	if err := checkAddBeforeRevise(corpus, order); err != nil {
		return NumberingReport{}, err
	}
	next, err := corpus.NextIdentity()
	if err != nil { // coverage-ignore: the corpus seam rejected any record whose number is not the four-digit group FilenameRe captured
		return NumberingReport{}, err
	}
	report := NumberingReport{}
	renames := make(map[string]string, len(order))
	for i, slug := range order {
		// Highest-plus-one at assignment time: every prior assignment in this
		// run has already raised the corpus's highest number by exactly one.
		number := next + i
		if err := adr.RenumberPending(p.decisionsDir(), slug, number); err != nil { // coverage-ignore: every named slug came from the corpus, whose pending records all parsed from a `<slug>.md` file carrying the slug-form heading
			return NumberingReport{}, err
		}
		renames[slug] = fmt.Sprintf("%04d", number)
		report.Assignments = append(report.Assignments, NumberAssignment{Slug: slug, Number: renames[slug]})
	}
	if _, err := topic.SubstituteProvenance(p.Root, renames); err != nil { // coverage-ignore: SubstituteProvenance's own error paths are unreachable, so this propagation cannot be driven from here
		return report, err
	}
	if _, _, _, err := p.SyncReport(ctx); err != nil {
		return report, err
	}
	return report, nil
}

// numberingCorpus loads the corpus through the construction seam, keeping a
// duplicate-identity corpus rather than aborting on it: duplicate numbers are
// the input to a refusal the caller needs, not a reason to refuse to look. A
// duplicate slug is different - it makes the pending set itself ambiguous, so
// there is nothing coherent to refuse with and the error stands.
func (p *Project) numberingCorpus() (adr.Corpus, *adr.DuplicateIdentityError, error) {
	corpus, err := adr.LoadCorpus(p.decisionsDir())
	if err == nil {
		return corpus, nil, nil
	}
	var duplicates *adr.DuplicateIdentityError
	if !errors.As(err, &duplicates) || len(duplicates.Slugs) != 0 {
		return adr.Corpus{}, nil, err
	}
	return corpus, duplicates, nil
}

// pendingSlugs lists the corpus's pending records in directory order.
func pendingSlugs(corpus adr.Corpus) []string {
	var slugs []string
	for _, a := range corpus.All() {
		if a.IsPending() {
			slugs = append(slugs, a.Slug)
		}
	}
	return slugs
}

// numberingOrder resolves the assignment order from the pending set and the
// caller's arguments, or names why numbering cannot run. An explicit list must
// name every pending record: the integration-branch block leaves partial
// numbering no legal destination, and completeness is what keeps the
// substitution's canonicalization total (ADR-0194 item 8).
func numberingOrder(pending, args []string, duplicates *adr.DuplicateIdentityError) ([]string, error) {
	switch {
	case duplicates != nil && len(pending) != 0:
		return nil, errors.New("duplicate ADR numbers present; resolve the corpus before numbering")
	case duplicates != nil:
		return nil, errors.New(duplicateNumbersRecipe)
	case len(pending) == 0:
		return nil, errors.New("no pending ADR to number")
	case len(args) == 0 && len(pending) > 1:
		return nil, fmt.Errorf("several pending ADRs require an explicit list naming every pending slug:\n%s", strings.Join(pending, "\n"))
	case len(args) == 0:
		return pending, nil
	}
	for i, slug := range args {
		if !slices.Contains(pending, slug) {
			return nil, fmt.Errorf("%q names no pending ADR", slug)
		}
		if slices.Contains(args[:i], slug) {
			return nil, fmt.Errorf("%q is named more than once", slug)
		}
	}
	var omitted []string
	for _, slug := range pending {
		if !slices.Contains(args, slug) {
			omitted = append(omitted, slug)
		}
	}
	if len(omitted) != 0 {
		return nil, fmt.Errorf("the explicit list must name every pending ADR; omitted: %s", strings.Join(omitted, ", "))
	}
	return args, nil
}

// reviseVerbs names the dependency an operation carries on the add that
// established the claim, for the refusal message.
var reviseVerbs = map[adr.OpVerb]string{adr.OpUpdate: "revises", adr.OpRemove: "removes"}

// checkAddBeforeRevise refuses an assignment order that would number a pending
// record before another pending record whose claim-add it revises. The check is
// corpus-local and topological over the pending set alone, which is what
// guarantees no slug Origin can end up inverted against its slug revisions: an
// already-numbered adder necessarily holds a smaller number than anything this
// run assigns (ADR-0194 items 8 and 10).
func checkAddBeforeRevise(corpus adr.Corpus, order []string) error {
	addedBy := map[string]string{}
	for _, slug := range order {
		record, _ := corpus.BySlug(slug)
		for _, operation := range record.Operations {
			if operation.Verb == adr.OpAdd {
				addedBy[operation.ID] = slug
			}
		}
	}
	position := make(map[string]int, len(order))
	for i, slug := range order {
		position[slug] = i
	}
	for i, slug := range order {
		record, _ := corpus.BySlug(slug)
		for _, operation := range record.Operations {
			verb, dependent := reviseVerbs[operation.Verb]
			adder, added := addedBy[operation.ID]
			if !dependent || !added || position[adder] < i {
				continue
			}
			return fmt.Errorf("%s %s a claim added by %s; number %s first", slug, verb, adder, adder)
		}
	}
	return nil
}
