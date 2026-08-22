// Package plancheck owns semantic validation and advisory results for prepared plans.
package plancheck

import (
	"errors"
	"fmt"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

const (
	// PropertyAuthority is the protected property for plan validity findings.
	PropertyAuthority checkresult.Property = "authority"
	// PropertyDetail is the protected property for plan detail warnings.
	PropertyDetail checkresult.Property = "plan-detail-quality"
)

// Diagnostics adapts one prepared parser outcome without reopening plan sources.
func Diagnostics(plansErr error) (checkresult.Result, error) {
	if plansErr == nil {
		return checkresult.New(nil, nil)
	}
	var diagnostics *plan.DiagnosticsError
	if !errors.As(plansErr, &diagnostics) {
		return checkresult.Result{}, plansErr
	}
	findings := make([]checkresult.Finding, 0, len(diagnostics.Diagnostics))
	for _, diagnostic := range diagnostics.Diagnostics {
		if !knownDiagnostic(diagnostic.Category) {
			return checkresult.Result{}, fmt.Errorf("unknown plan diagnostic category %q", diagnostic.Category)
		}
		findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-"+diagnostic.Category, "docs/plans/"+diagnostic.Path, diagnostic.Detail))
	}
	return checkresult.New(findings, nil)
}

// Validity evaluates plan status, ADR links, and planned commit subjects.
func Validity(plans []plan.Plan, corpus adr.Corpus, scopes []config.ScopeSpec, enabled bool) (checkresult.Result, error) {
	if !enabled {
		return checkresult.New(nil, nil)
	}
	settings := audit.Resolve(scopes)
	var findings []checkresult.Finding
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		path := "docs/plans/" + pl.Filename
		if !plan.ValidStatuses[pl.Status] {
			findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-frontmatter", path, fmt.Sprintf("status %q not in {Proposed, Implemented}", pl.Status)))
		}
		for _, link := range pl.ADRs {
			if _, ok := corpus.ByIdentity(link.Identity()); !ok {
				findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-adr-link", path, "ADR-"+link.Identity()))
			}
		}
		for _, subject := range pl.CommitSubjects {
			for _, issue := range audit.CheckPlannedSubject(subject, settings) {
				if issue.Severity == severity.Error {
					findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-commit-subject", path, issue.Detail))
				}
			}
		}
	}
	return checkresult.New(findings, nil)
}

// ScopeInformation returns unknown-scope planned-subject notes in authored order.
func ScopeInformation(plans []plan.Plan, scopes []config.ScopeSpec) ([]checkresult.Information, error) {
	settings := audit.Resolve(scopes)
	var information []checkresult.Information
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		for _, subject := range pl.CommitSubjects {
			for _, issue := range audit.CheckPlannedSubject(subject, settings) {
				if issue.Severity == severity.Warn {
					information = append(information, checkresult.Information{Evidence: checkresult.Evidence{Kind: "advisory", Detail: fmt.Sprintf("docs/plans/%s: planned commit %s", pl.Filename, issue.Detail)}})
				}
			}
		}
	}
	return information, nil
}

// Artifact evaluates plan-v2 Decision references and assignment advisories.
func Artifact(plans []plan.Plan, corpus adr.Corpus) (checkresult.Result, error) {
	var findings []checkresult.Finding
	for _, p := range plans {
		if p.Format != "plan-v2" {
			continue
		}
		assigned := map[string]bool{}
		allowed, missingLinks := allowedADRIdentities(p, corpus)
		for _, identity := range missingLinks {
			findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-reference", p.Path, fmt.Sprintf("%s adrs %q: ADR not found", p.Filename, identity)))
		}
		for _, phase := range p.Phases {
			for _, task := range phase.Tasks {
				refs := append(append([]plan.DecisionRef{}, task.Fields.Applying...), task.Fields.Context...)
				for _, ref := range refs {
					record, ok := corpus.ByIdentity(ref.ADR)
					where := fmt.Sprintf("%s task %d.%d %s %q", p.Filename, phase.Number, task.Number, ref.Kind, ref.Authored)
					if !ok {
						findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-reference", p.Path, where+": ADR not found"))
						continue
					}
					if ref.Kind == "Applying" && !allowed[record.Identity()] {
						findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-reference", p.Path, where+": Applying ADR is absent from adrs"))
						continue
					}
					if ref.Kind == "Context" && record.IsContentAmendable() {
						findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-reference", p.Path, where+": Context requires frozen ADR"))
						continue
					}
					item, err := record.LookupDecision(ref.Selector)
					if err != nil {
						findings = append(findings, finding(severity.Error, PropertyAuthority, "plan-reference", p.Path, where+": "+err.Error()))
						continue
					}
					if ref.Kind == "Applying" {
						assigned[item.Key] = true
					}
				}
			}
		}
		if !p.IsProposed() {
			continue
		}
		if len(p.ADRs) > 0 {
			for _, phase := range p.Phases {
				for _, task := range phase.Tasks {
					if task.Fields.Kind != plan.TaskSpike && len(task.Fields.Applying) == 0 {
						findings = append(findings, finding(severity.Warn, PropertyDetail, "plan-advisory", "", fmt.Sprintf("%s task %d.%d has no Applying assignment", p.Filename, phase.Number, task.Number)))
					}
				}
			}
			for _, link := range p.ADRs {
				if record, ok := corpus.ByIdentity(link.Identity()); ok {
					for _, item := range record.Decisions() {
						if !assigned[item.Key] {
							findings = append(findings, finding(severity.Warn, PropertyDetail, "plan-advisory", "", fmt.Sprintf("%s Decision %s has no Applying assignment", p.Filename, item.Key)))
						}
					}
				}
			}
		}
		completed := map[string]bool{}
		advanced := map[string]bool{}
		for _, phase := range p.Phases {
			for _, slug := range phase.Advances {
				advanced[slug] = true
			}
			for _, slug := range phase.Completes {
				completed[slug] = true
			}
		}
		for _, item := range p.DoD {
			if !advanced[item.Slug] && !completed[item.Slug] {
				findings = append(findings, finding(severity.Warn, PropertyDetail, "plan-advisory", "", fmt.Sprintf("%s DoD %s has no outcome assignment", p.Filename, item.Slug)))
			} else if advanced[item.Slug] && !completed[item.Slug] {
				findings = append(findings, finding(severity.Warn, PropertyDetail, "plan-advisory", "", fmt.Sprintf("%s DoD %s is advanced but has no Completes owner", p.Filename, item.Slug)))
			}
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Rank != findings[j].Rank {
			return findings[i].Rank == severity.Error
		}
		if findings[i].Rank == severity.Warn {
			return findings[i].Evidence.Detail < findings[j].Evidence.Detail
		}
		return false
	})
	return checkresult.New(findings, nil)
}

// Check evaluates every plan policy group for staged compatibility consumers.
type selectedDecisionRef struct {
	ref         plan.DecisionRef
	phase, task int
}

// Select resolves one executable phase or task selector.
func Select(p plan.Plan, selector string) (plan.Phase, plan.Task, error) {
	return plan.Select(p, selector)
}

// ResolveSelectedDecisions resolves and partitions the selected Applying and Context Decisions.
func ResolveSelectedDecisions(p plan.Plan, corpus adr.Corpus, phase plan.Phase, task plan.Task) ([]plan.ResolvedDecision, []plan.ResolvedDecision, error) {
	var refs []selectedDecisionRef
	appendTask := func(candidate plan.Task) {
		for _, ref := range candidate.Fields.Applying {
			refs = append(refs, selectedDecisionRef{ref: ref, phase: phase.Number, task: candidate.Number})
		}
		for _, ref := range candidate.Fields.Context {
			refs = append(refs, selectedDecisionRef{ref: ref, phase: phase.Number, task: candidate.Number})
		}
	}
	if task.Number != 0 {
		appendTask(task)
	} else {
		for _, candidate := range phase.Tasks {
			appendTask(candidate)
		}
	}

	allowed, _ := allowedADRIdentities(p, corpus)
	order := make([]string, 0, len(refs))
	resolved := make(map[string]plan.ResolvedDecision, len(refs))
	applyingKeys := make(map[string]bool, len(refs))
	for _, located := range refs {
		ref := located.ref
		prefix := fmt.Sprintf("plan %s task %d.%d %s %q", p.Filename, located.phase, located.task, ref.Kind, ref.Authored)
		record, ok := corpus.ByIdentity(ref.ADR)
		if !ok {
			return nil, nil, fmt.Errorf("%s: ADR not found", prefix)
		}
		if ref.Kind == "Applying" && !allowed[record.Identity()] {
			return nil, nil, fmt.Errorf("%s: Applying ADR is absent from adrs", prefix)
		}
		if ref.Kind == "Context" && record.IsContentAmendable() {
			return nil, nil, fmt.Errorf("%s: requires frozen ADR", prefix)
		}
		item, err := record.LookupDecision(ref.Selector)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", prefix, err)
		}
		if _, seen := resolved[item.Key]; !seen {
			order = append(order, item.Key)
			resolved[item.Key] = plan.ResolvedDecision{Key: item.Key, ADRIdentity: item.ADRIdentity, Title: item.Title, Status: item.Status, Markdown: item.Markdown}
		}
		if ref.Kind == "Applying" {
			applyingKeys[item.Key] = true
		}
	}

	var applying, context []plan.ResolvedDecision
	for _, key := range order {
		if applyingKeys[key] {
			applying = append(applying, resolved[key])
		} else {
			context = append(context, resolved[key])
		}
	}
	return applying, context, nil
}

func allowedADRIdentities(p plan.Plan, corpus adr.Corpus) (map[string]bool, []string) {
	allowed := make(map[string]bool, len(p.ADRs))
	var missing []string
	for _, link := range p.ADRs {
		record, ok := corpus.ByIdentity(link.Identity())
		if !ok {
			missing = append(missing, link.Identity())
			continue
		}
		allowed[record.Identity()] = true
	}
	return allowed, missing
}

func finding(rank severity.Rank, property checkresult.Property, kind, path, detail string) checkresult.Finding {
	return checkresult.Finding{Rank: rank, Property: property, Evidence: checkresult.Evidence{Kind: kind, Path: path, Detail: detail}}
}

func knownDiagnostic(category string) bool {
	switch category {
	case "field", "frontmatter", "numbering", "path", "paths", "phase-close", "projection", "relationship", "structure":
		return true
	}
	return false
}
