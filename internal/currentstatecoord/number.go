package currentstatecoord

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
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

// PublicationOutcome is the owner-rendered Publisher result retained by ADR
// numbering when publication crosses its own mutation boundary.
type PublicationOutcome interface {
	HasCommittedEffects() bool
	PartialMutation() (presentation.Mutation, error)
}

// NumberingEffect is one exact root-relative path fact committed before a
// later numbering failure.
type NumberingEffect struct {
	Kind string
	Path string
}

// PartialNumberingError retains exact numbering, provenance, and Publisher
// effects after numbering has crossed its first mutation. It preserves the
// mechanism cause and supplies a residue-first recovery boundary.
type PartialNumberingError struct {
	Report      NumberingReport
	Effects     []NumberingEffect
	Publication PublicationOutcome
	Cause       error
	Recovery    []string
}

func (e *PartialNumberingError) Error() string { return e.Cause.Error() }
func (e *PartialNumberingError) Unwrap() error { return e.Cause }
func (e *PartialNumberingError) Document() (presentation.Document, error) {
	assignments := make([]presentation.Value, 0, len(e.Report.Assignments))
	for _, assignment := range e.Report.Assignments {
		value, err := presentation.Literal(assignment.Slug + " -> " + assignment.Number)
		if err != nil {
			return presentation.Document{}, err
		}
		assignments = append(assignments, value)
	}
	effects := make([]presentation.Value, 0, len(e.Effects))
	for _, effect := range e.Effects {
		value, err := presentation.Literal(effect.Kind + " " + effect.Path)
		if err != nil {
			return presentation.Document{}, err
		}
		effects = append(effects, value)
	}
	changes := make([]presentation.MutationChange, 0, 3)
	if len(assignments) != 0 {
		changes = append(changes, presentation.MutationChange{Label: "assignments", Values: assignments})
	}
	if len(effects) != 0 {
		changes = append(changes, presentation.MutationChange{Label: "numbering and provenance effects", Values: effects})
	}
	next := []presentation.Value{}
	if e.Publication != nil && e.Publication.HasCommittedEffects() {
		publication, err := e.Publication.PartialMutation()
		if err != nil {
			return presentation.Document{}, err
		}
		changes = append(changes, publication.Changes...)
		next = append(next, publication.NextActions...)
	}
	for _, action := range e.Recovery {
		value, err := presentation.Prose(action)
		if err != nil {
			return presentation.Document{}, err
		}
		next = append(next, value)
	}
	return (presentation.Mutation{Status: "ADR numbering partially committed", Changes: changes, NextActions: next}).Document()
}

// Document maps numbering assignments to their semantic presentation.
func (r NumberingReport) Document() (presentation.Document, error) {
	records := make([]presentation.Record, 0, len(r.Assignments))
	for _, assignment := range r.Assignments {
		slug, err := presentation.Literal(assignment.Slug)
		if err != nil {
			return presentation.Document{}, err
		}
		number, err := presentation.Literal(assignment.Number)
		if err != nil {
			return presentation.Document{}, err
		}
		record, err := presentation.NewRecord(slug, number)
		if err != nil { // coverage-ignore: validated nonempty literals always form a valid record
			return presentation.Document{}, err
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return presentation.Document{}, nil
	}
	return (presentation.Collection{Status: "ADR numbering completed", Categories: []presentation.CollectionCategory{{Label: "assignments", Schema: []string{"slug", "number"}, Records: records}}}).Document()
}

// duplicateNumbersRecipe is the reset-remake recipe a duplicate-number corpus
// with no pending record is offered verbatim (ADR-0202 item 12). It is a hint,
// never an action: numbering unmakes a stale numbering commit by reset, and
// reads no git provenance to decide that for the caller.
const duplicateNumbersRecipe = "duplicate ADR numbers with no pending record: if a stale numbering commit collided, " +
	"run: git reset --hard HEAD~1 && git merge <integration branch> && awf adr number, then gate and merge back"

// NumberPendingADRsLeased loads the pre-mutation authority universe through
// the selected-root handle, numbers its pending records, substitutes topic
// provenance, and then runs publish against the separately selected
// post-mutation universe. The caller retains the covering lease through
// presentation of the returned complete or partial outcome.
func NumberPendingADRsLeased(root string, slugs []string, publish func() (PublicationOutcome, error), lease *filesystem.Lease) (NumberingReport, error) {
	if !lease.CoversTracked(root) {
		return NumberingReport{}, errors.New("ADR numbering requires a covering tracked lease")
	}
	files, err := filesystem.Open(root)
	if err != nil {
		return NumberingReport{}, err
	}
	defer files.Close()
	return numberPendingADRs(slugs, publish, files)
}

func numberPendingADRs(slugs []string, publish func() (PublicationOutcome, error), files *filesystem.Handle) (NumberingReport, error) {
	corpus, duplicates, err := numberingCorpus(files)
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
	if err != nil {
		return NumberingReport{}, err
	}
	report := NumberingReport{}
	effects := []NumberingEffect{}
	renames := make(map[string]string, len(order))
	decisions := filepath.ToSlash(filepath.Join(config.DocsDir, "decisions"))
	for i, slug := range order {
		// Highest-plus-one at assignment time: every prior assignment in this
		// run has already raised the corpus's highest number by exactly one.
		number := next + i
		numbered := fmt.Sprintf("%04d", number)
		if err := adr.RenumberPendingConfined(files, decisions, slug, number); err != nil {
			var partial *adr.PartialRenumberError
			if errors.As(err, &partial) && partial.DestinationPublished {
				report.Assignments = append(report.Assignments, NumberAssignment{Slug: slug, Number: numbered})
				effects = append(effects,
					NumberingEffect{Kind: "destination-published", Path: partial.Destination},
					NumberingEffect{Kind: "source-retained", Path: partial.Source},
				)
				return report, &PartialNumberingError{Report: report, Effects: effects, Cause: err, Recovery: []string{"verify the listed numbered destination, remove only the listed retained pending source, then run awf render"}}
			}
			return report, err
		}
		renames[slug] = numbered
		report.Assignments = append(report.Assignments, NumberAssignment{Slug: slug, Number: numbered})
		effects = append(effects,
			NumberingEffect{Kind: "destination-published", Path: filepath.ToSlash(filepath.Join(decisions, numbered+"-"+slug+".md"))},
			NumberingEffect{Kind: "source-retired", Path: filepath.ToSlash(filepath.Join(decisions, slug+".md"))},
		)
	}
	provenance, err := topic.SubstituteProvenanceConfined(files, renames)
	for _, path := range provenance.Paths {
		effects = append(effects, NumberingEffect{Kind: "provenance-replaced", Path: path})
	}
	if err != nil {
		return report, &PartialNumberingError{Report: report, Effects: effects, Cause: err, Recovery: []string{"inspect the listed provenance replacements, complete remaining assigned-slug replacements under .awf/topics/parts, then run awf render"}}
	}
	publication, err := publish()
	if err != nil {
		return report, &PartialNumberingError{Report: report, Effects: effects, Publication: publication, Cause: err, Recovery: []string{"repair the reported publication fault, then run awf render"}}
	}
	return report, nil
}

type confinedCorpusReader struct {
	files *filesystem.Handle
}

func (r confinedCorpusReader) ReadFile(path string) ([]byte, bool, error) {
	data, err := r.files.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func (r confinedCorpusReader) Paths(prefix string) ([]string, error) {
	root := strings.TrimSuffix(prefix, "/")
	paths := []string{}
	err := r.files.Walk(root, func(path string, info fs.FileInfo) (bool, error) {
		if info.Mode()&fs.ModeSymlink != 0 {
			return false, fmt.Errorf("ADR authority path %q is a symlink", path)
		}
		if !info.IsDir() {
			paths = append(paths, path)
		}
		return true, nil
	})
	return paths, err
}

// numberingCorpus loads authority through the selected-root handle, keeping a
// duplicate-identity corpus rather than aborting on it: duplicate numbers are
// the input to a refusal the caller needs, not a reason to refuse to look. A
// duplicate slug is different because it makes the pending set ambiguous.
func numberingCorpus(files *filesystem.Handle) (adr.Corpus, *adr.DuplicateIdentityError, error) {
	corpus, err := adr.LoadCorpusFromTree(confinedCorpusReader{files: files}, filepath.ToSlash(filepath.Join(config.DocsDir, "decisions")))
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
