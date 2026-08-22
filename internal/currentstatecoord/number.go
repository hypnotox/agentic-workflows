package currentstatecoord

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// NumberAssignment is one pending record's slug and the number numbering gave
// it.
type NumberAssignment struct {
	Slug   string
	Number string
}

// NumberingReport is the mapping one numbering run assigned, in assignment
// order, so its caller can present it after a partial completion.
type NumberingReport struct {
	Assignments []NumberAssignment
}

// duplicateNumbersRecipe is the reset-remake recipe a duplicate-number corpus
// with no pending record is offered verbatim (ADR-0202 item 12). It is a hint,
// never an action: numbering unmakes a stale numbering commit by reset, and
// reads no git provenance to decide that for the caller.
const duplicateNumbersRecipe = "duplicate ADR numbers with no pending record: if a stale numbering commit collided, " +
	"run: git reset --hard HEAD~1 && git merge <integration branch> && awf adr number, then gate and merge back"

// NumberPendingADRs loads the pre-mutation authority universe, numbers its
// pending records, substitutes their topic provenance, and then runs publish
// against the separately selected post-mutation universe.
//
// The callback keeps Publisher composition with its caller. Refusals happen
// before the first rename and return no assignments. Once a rename succeeds,
// later substitution or publication errors retain the assignments that describe
// the partial completion.
func NumberPendingADRs(root string, slugs []string, publish func() error) (NumberingReport, error) {
	corpus, duplicates, err := numberingCorpus(root)
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
		if err := adr.RenumberPending(decisionsDir(root), slug, number); err != nil { // coverage-ignore: every named slug came from the corpus, whose pending records all parsed from a `<slug>.md` file carrying the slug-form heading
			return NumberingReport{}, err
		}
		renames[slug] = fmt.Sprintf("%04d", number)
		report.Assignments = append(report.Assignments, NumberAssignment{Slug: slug, Number: renames[slug]})
	}
	if err := topic.SubstituteProvenance(root, renames); err != nil { // coverage-ignore: SubstituteProvenance's own error paths are unreachable, so this propagation cannot be driven from here
		return report, err
	}
	if err := publish(); err != nil {
		return report, err
	}
	return report, nil
}

func decisionsDir(root string) string {
	return filepath.Join(root, config.DocsDir, "decisions")
}

// numberingCorpus loads the corpus through the construction seam, keeping a
// duplicate-identity corpus rather than aborting on it: duplicate numbers are
// the input to a refusal the caller needs, not a reason to refuse to look. A
// duplicate slug is different - it makes the pending set itself ambiguous, so
// there is nothing coherent to refuse with and the error stands.
func numberingCorpus(root string) (adr.Corpus, *adr.DuplicateIdentityError, error) {
	corpus, err := adr.LoadCorpus(decisionsDir(root))
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
// substitution's canonicalization total (ADR-0202 item 8).
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
// run assigns (ADR-0202 items 8 and 10).
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
