package coverage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const baselineVersion = 1

// Position identifies a line and column in a Go source file.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Identity is the exact identity of one coverprofile block.
type Identity struct {
	File       string   `json:"file"`
	Start      Position `json:"start"`
	End        Position `json:"end"`
	Statements int      `json:"statements"`
}

// Directive describes one coverage-ignore directive and its relationship to the
// measured whole-module profile.
type Directive struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	TargetLine int    `json:"targetLine"`
	Reason     string `json:"reason"`
	Mapped     bool   `json:"mapped"`
	Executed   bool   `json:"executed"`
}

// Selector is one critical path set derived from the whole-module profile.
type Selector struct {
	Name   string     `json:"name"`
	Roots  []string   `json:"roots"`
	Misses []Identity `json:"misses"`
}

// Analysis is the policy view of one merged whole-module profile and its source
// tree.
type Analysis struct {
	Raw                  Report
	Filtered             Report
	RawMisses            []Identity
	UniverseSHA256       string
	Selectors            []Selector
	ProductionDirectives []Directive
	TestDirectives       []Directive
}

// MissAdmission records why an exact raw miss may remain in the baseline.
type MissAdmission struct {
	Identity  Identity  `json:"identity"`
	Reason    string    `json:"reason"`
	MovedFrom *Identity `json:"movedFrom,omitempty"`
}

// IgnoreClass is one admitted production coverage-ignore class.
type IgnoreClass string

const (
	IgnoreProcessExit        IgnoreClass = "tested-process-exit"
	IgnoreImpossibleState    IgnoreClass = "revalidated-impossible-state"
	IgnoreDeterministicFault IgnoreClass = "uninducible-deterministic-fault"
	IgnorePlatformOnly       IgnoreClass = "platform-only"
)

// DirectiveAdmission records the reviewed class and evidence for one retained
// production directive.
type DirectiveAdmission struct {
	Directive Directive   `json:"directive"`
	Class     IgnoreClass `json:"class"`
	Evidence  string      `json:"evidence"`
}

// PlatformDirective records a source directive that the host profile cannot
// measure.
type PlatformDirective struct {
	Directive Directive   `json:"directive"`
	Platforms []string    `json:"platforms"`
	Class     IgnoreClass `json:"class"`
	Evidence  string      `json:"evidence"`
}

// EquivalentMutant is an exact independently reviewed equivalent survivor.
type EquivalentMutant struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Mutator string `json:"mutator"`
	Reason  string `json:"reason"`
}

// SelectorBaseline is the canonical baseline view of a critical selector.
type SelectorBaseline struct {
	Name   string     `json:"name"`
	Roots  []string   `json:"roots"`
	Misses []Identity `json:"misses"`
}

// Baseline is the canonical generated coverage policy artifact.
type Baseline struct {
	Version              int                  `json:"version"`
	UniverseSHA256       string               `json:"universeSHA256"`
	Repository           []MissAdmission      `json:"repository"`
	Selectors            []SelectorBaseline   `json:"selectors"`
	ProductionDirectives []DirectiveAdmission `json:"productionDirectives"`
	TestDirectives       []Directive          `json:"testDirectives"`
	PlatformDirectives   []PlatformDirective  `json:"platformDirectives"`
	EquivalentMutants    []EquivalentMutant   `json:"equivalentMutants"`
}

// Review supplies independently reviewed evidence for baseline changes.
type Review struct {
	Misses             []MissAdmission      `json:"misses"`
	Directives         []DirectiveAdmission `json:"directives"`
	PlatformDirectives []PlatformDirective  `json:"platformDirectives"`
	EquivalentMutants  []EquivalentMutant   `json:"equivalentMutants"`
}

// Finding is one blocking policy diagnostic.
type Finding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var selectorPolicy = []Selector{
	{Name: "hard-safety", Roots: []string{"cmd/covercheck", "cmd/mutants", "internal/commitpolicy", "internal/coverage", "internal/filepublication"}},
	{Name: "state-authority", Roots: []string{"internal/adr", "internal/currentstate", "internal/currentstatecoord", "internal/topic"}},
	{Name: "repository-effort-lifecycle", Roots: []string{"internal/effort", "internal/git", "internal/worktree"}},
	{Name: "migration-recovery", Roots: []string{"internal/config", "internal/migrate", "internal/upgrade"}},
	{Name: "publication-application", Roots: []string{"internal/project", "internal/publisher"}},
	{Name: "command-boundary", Roots: []string{"cmd/awf"}},
}

// AnalyzeProfile resolves the module root and analyzes one whole-module profile.
func AnalyzeProfile(profilePath string) (Analysis, error) {
	root, err := moduleRoot()
	if err != nil {
		return Analysis{}, err
	}
	modPath, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return Analysis{}, err
	}
	return Analyze(profilePath, root, modPath)
}

// Analyze parses one whole-module profile into the canonical raw policy model.
func Analyze(profilePath, srcRoot, modPath string) (Analysis, error) {
	blocks, err := parseProfile(profilePath)
	if err != nil {
		return Analysis{}, err
	}
	blocks = mergeBlocks(blocks)
	slices.SortFunc(blocks, compareBlock)

	directives, err := scanDirectives(srcRoot, blocks, modPath)
	if err != nil {
		return Analysis{}, err
	}
	ignored := make(map[string]map[int]bool)
	for _, directive := range directives {
		qualified := modPath + "/" + directive.File
		if ignored[qualified] == nil {
			ignored[qualified] = make(map[int]bool)
		}
		ignored[qualified][directive.TargetLine] = true
	}

	analysis := Analysis{}
	var universe []Identity
	for _, current := range blocks {
		id, err := identityOf(current, modPath)
		if err != nil {
			return Analysis{}, err
		}
		if _, err := os.Stat(filepath.Join(srcRoot, filepath.FromSlash(id.File))); err != nil {
			return Analysis{}, fmt.Errorf("coverage: inspect profile source %s: %w", id.File, err)
		}
		universe = append(universe, id)
		analysis.Raw.Total += current.numStmt
		if current.count > 0 {
			analysis.Raw.Covered += current.numStmt
		} else {
			analysis.RawMisses = append(analysis.RawMisses, id)
		}
		if ignored[current.file][current.startLine] {
			continue
		}
		analysis.Filtered.Total += current.numStmt
		if current.count > 0 {
			analysis.Filtered.Covered += current.numStmt
		}
	}
	analysis.UniverseSHA256 = hashIdentities(universe)
	analysis.ProductionDirectives, analysis.TestDirectives = splitDirectives(directives)
	analysis.Selectors = deriveSelectors(analysis.RawMisses)
	return analysis, nil
}

func compareBlock(a, b block) int {
	if a.file != b.file {
		return strings.Compare(a.file, b.file)
	}
	if a.startLine != b.startLine {
		return a.startLine - b.startLine
	}
	if a.startColumn != b.startColumn {
		return a.startColumn - b.startColumn
	}
	if a.endLine != b.endLine {
		return a.endLine - b.endLine
	}
	if a.endColumn != b.endColumn {
		return a.endColumn - b.endColumn
	}
	return a.numStmt - b.numStmt
}

func identityOf(current block, modPath string) (Identity, error) {
	prefix := modPath + "/"
	if !strings.HasPrefix(current.file, prefix) {
		return Identity{}, fmt.Errorf("coverage: profile file %q is outside module %q", current.file, modPath)
	}
	return Identity{
		File:       strings.TrimPrefix(current.file, prefix),
		Start:      Position{Line: current.startLine, Column: current.startColumn},
		End:        Position{Line: current.endLine, Column: current.endColumn},
		Statements: current.numStmt,
	}, nil
}

func hashIdentities(identities []Identity) string {
	h := sha256.New()
	for _, id := range identities {
		fmt.Fprintf(h, "%s:%d.%d,%d.%d %d\n", id.File, id.Start.Line, id.Start.Column, id.End.Line, id.End.Column, id.Statements)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func scanDirectives(root string, blocks []block, modPath string) ([]Directive, error) {
	measured := make(map[string][]block)
	for _, current := range blocks {
		rel := strings.TrimPrefix(current.file, modPath+"/")
		if rel == current.file {
			continue
		}
		measured[filepath.ToSlash(rel)] = append(measured[filepath.ToSlash(rel)], current)
	}
	var directives []Directive
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(path, filepath.Clean(root)), string(filepath.Separator)))
		source, err := os.ReadFile(path)
		if err != nil { // coverage-ignore: WalkDir just read this regular entry; failure requires a concurrent namespace, permission, or storage change
			return err
		}
		found, err := sourceDirectives(rel, source, measured[rel])
		if err != nil {
			return err
		}
		directives = append(directives, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(directives, compareDirective)
	return directives, nil
}

func sourceDirectives(rel string, source []byte, measured []block) ([]Directive, error) {
	var directives []Directive
	for index, line := range strings.Split(string(source), "\n") {
		markerIndex := strings.Index(line, marker)
		if markerIndex < 0 {
			continue
		}
		reason := strings.TrimSpace(line[markerIndex+len(marker):])
		if !strings.HasPrefix(reason, ":") || strings.TrimSpace(reason[1:]) == "" {
			return nil, fmt.Errorf("%s:%d: %s requires a non-empty reason (use %q)", rel, index+1, marker, marker+": <why>")
		}
		target := index + 1
		if strings.TrimSpace(line[:markerIndex]) == "" {
			target++
		}
		directive := Directive{File: rel, Line: index + 1, TargetLine: target, Reason: strings.TrimSpace(reason[1:])}
		for _, current := range measured {
			if current.startLine != target {
				continue
			}
			directive.Mapped = true
			directive.Executed = directive.Executed || current.count > 0
		}
		directives = append(directives, directive)
	}
	return directives, nil
}

func blocksForFile(blocks []block, file string) []block {
	var selected []block
	for _, current := range blocks {
		if current.file == file {
			selected = append(selected, current)
		}
	}
	return selected
}

func compareDirective(a, b Directive) int {
	if a.File != b.File {
		return strings.Compare(a.File, b.File)
	}
	return a.Line - b.Line
}

func splitDirectives(all []Directive) (production, tests []Directive) {
	for _, directive := range all {
		if strings.HasSuffix(directive.File, "_test.go") {
			tests = append(tests, directive)
		} else {
			production = append(production, directive)
		}
	}
	return production, tests
}

func deriveSelectors(misses []Identity) []Selector {
	selectors := make([]Selector, len(selectorPolicy))
	for index, policy := range selectorPolicy {
		selectors[index] = Selector{Name: policy.Name, Roots: slices.Clone(policy.Roots)}
		for _, miss := range misses {
			if pathInRoots(miss.File, policy.Roots) {
				selectors[index].Misses = append(selectors[index].Misses, miss)
			}
		}
	}
	return selectors
}

func pathInRoots(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// Regenerate creates the next canonical policy while preserving unchanged
// admissions and requiring explicit review for every added or moved identity.
func Regenerate(analysis Analysis, previous *Baseline, review Review) (Baseline, error) {
	baseline := Baseline{Version: baselineVersion, UniverseSHA256: analysis.UniverseSHA256}
	previousMisses := make(map[Identity]MissAdmission)
	previousDirectives := make(map[string]DirectiveAdmission)
	if previous != nil {
		for _, admission := range previous.Repository {
			previousMisses[admission.Identity] = admission
		}
		for _, admission := range previous.ProductionDirectives {
			previousDirectives[directiveKey(admission.Directive)] = admission
		}
		baseline.PlatformDirectives = slices.Clone(previous.PlatformDirectives)
		baseline.EquivalentMutants = slices.Clone(previous.EquivalentMutants)
	}
	reviewMisses := make(map[Identity]MissAdmission)
	for _, admission := range review.Misses {
		reviewMisses[admission.Identity] = admission
	}
	for _, id := range analysis.RawMisses {
		if admission, ok := previousMisses[id]; ok {
			baseline.Repository = append(baseline.Repository, admission)
			continue
		}
		admission, ok := reviewMisses[id]
		if !ok || strings.TrimSpace(admission.Reason) == "" {
			return Baseline{}, fmt.Errorf("coverage: raw miss %s requires review", formatIdentity(id))
		}
		if admission.MovedFrom != nil {
			if _, existed := previousMisses[*admission.MovedFrom]; !existed || slices.Contains(analysis.RawMisses, *admission.MovedFrom) {
				return Baseline{}, fmt.Errorf("coverage: moved miss %s has invalid previous identity", formatIdentity(id))
			}
		}
		baseline.Repository = append(baseline.Repository, admission)
	}

	reviewDirectives := make(map[string]DirectiveAdmission)
	for _, admission := range review.Directives {
		reviewDirectives[directiveKey(admission.Directive)] = admission
	}
	for _, directive := range analysis.ProductionDirectives {
		key := directiveKey(directive)
		if admission, ok := previousDirectives[key]; ok {
			admission.Directive = directive
			baseline.ProductionDirectives = append(baseline.ProductionDirectives, admission)
			continue
		}
		admission, ok := reviewDirectives[key]
		if !ok {
			return Baseline{}, fmt.Errorf("coverage: production directive %s:%d requires review", directive.File, directive.Line)
		}
		admission.Directive = directive
		baseline.ProductionDirectives = append(baseline.ProductionDirectives, admission)
	}
	baseline.TestDirectives = slices.Clone(analysis.TestDirectives)
	if review.PlatformDirectives != nil {
		baseline.PlatformDirectives = slices.Clone(review.PlatformDirectives)
	}
	if review.EquivalentMutants != nil {
		baseline.EquivalentMutants = slices.Clone(review.EquivalentMutants)
	}
	for _, selector := range analysis.Selectors {
		baseline.Selectors = append(baseline.Selectors, SelectorBaseline{Name: selector.Name, Roots: slices.Clone(selector.Roots), Misses: slices.Clone(selector.Misses)})
	}
	if err := validateBaseline(baseline); err != nil {
		return Baseline{}, err
	}
	return baseline, nil
}

func directiveKey(directive Directive) string {
	return fmt.Sprintf("%s:%d:%d:%s", directive.File, directive.Line, directive.TargetLine, directive.Reason)
}

func formatIdentity(id Identity) string {
	return fmt.Sprintf("%s:%d.%d,%d.%d %d", id.File, id.Start.Line, id.Start.Column, id.End.Line, id.End.Column, id.Statements)
}

// Evaluate compares exact current identities and directive evidence with a
// canonical baseline. Percentages never participate in the decision.
func Evaluate(analysis Analysis, baseline Baseline) []Finding {
	var findings []Finding
	if analysis.UniverseSHA256 != baseline.UniverseSHA256 {
		findings = append(findings, Finding{Code: "profile-universe-mismatch", Message: "profile identity universe differs from the reviewed whole-module profile"})
	}
	admitted := make(map[Identity]bool)
	for _, miss := range baseline.Repository {
		admitted[miss.Identity] = true
	}
	for _, miss := range analysis.RawMisses {
		if !admitted[miss] {
			findings = append(findings, Finding{Code: "raw-identity-added", Message: formatIdentity(miss)})
		}
	}
	selectorBaselines := make(map[string]SelectorBaseline)
	for _, selector := range baseline.Selectors {
		selectorBaselines[selector.Name] = selector
	}
	for _, selector := range analysis.Selectors {
		allowed := make(map[Identity]bool)
		for _, miss := range selectorBaselines[selector.Name].Misses {
			allowed[miss] = true
		}
		for _, miss := range selector.Misses {
			if !allowed[miss] {
				findings = append(findings, Finding{Code: "selector-identity-added", Message: selector.Name + ": " + formatIdentity(miss)})
			}
		}
	}
	admittedDirectives := make(map[string]bool)
	for _, admission := range baseline.ProductionDirectives {
		admittedDirectives[directiveKey(admission.Directive)] = true
	}
	for _, directive := range analysis.ProductionDirectives {
		if directive.Executed {
			findings = append(findings, Finding{Code: "executed-ignore", Message: fmt.Sprintf("%s:%d ignored body executed", directive.File, directive.Line)})
		}
		if !admittedDirectives[directiveKey(directive)] {
			findings = append(findings, Finding{Code: "production-directive-changed", Message: fmt.Sprintf("%s:%d is not admitted", directive.File, directive.Line)})
		}
	}
	currentDirectives := make(map[string]bool)
	for _, directive := range analysis.ProductionDirectives {
		currentDirectives[directiveKey(directive)] = true
	}
	for _, admission := range baseline.ProductionDirectives {
		if !currentDirectives[directiveKey(admission.Directive)] {
			findings = append(findings, Finding{Code: "production-directive-removed", Message: fmt.Sprintf("%s:%d is absent", admission.Directive.File, admission.Directive.Line)})
		}
	}
	return findings
}

// CanonicalBaseline validates and renders a byte-stable baseline.
func CanonicalBaseline(baseline Baseline) ([]byte, error) {
	baseline = normalizeBaseline(baseline)
	if err := validateBaseline(baseline); err != nil {
		return nil, err
	}
	raw, _ := json.MarshalIndent(baseline, "", "  ") // Baseline contains only JSON-native fields.
	return append(raw, '\n'), nil
}

// LoadBaseline loads strict, canonical baseline evidence.
func LoadBaseline(path string) (Baseline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Baseline{}, fmt.Errorf("coverage: read baseline: %w", err)
	}
	var baseline Baseline
	if err := decodeStrict(raw, &baseline); err != nil {
		return Baseline{}, fmt.Errorf("coverage: parse baseline: %w", err)
	}
	canonical, err := CanonicalBaseline(baseline)
	if err != nil {
		return Baseline{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Baseline{}, errors.New("coverage: baseline is not canonical")
	}
	return baseline, nil
}

// LoadReview loads strict review evidence.
func LoadReview(path string) (Review, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Review{}, fmt.Errorf("coverage: read review: %w", err)
	}
	var review Review
	if err := decodeStrict(raw, &review); err != nil {
		return Review{}, fmt.Errorf("coverage: parse review: %w", err)
	}
	return review, nil
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func normalizeBaseline(baseline Baseline) Baseline {
	baseline.Repository = nonNil(slices.Clone(baseline.Repository))
	slices.SortFunc(baseline.Repository, func(a, b MissAdmission) int { return compareIdentity(a.Identity, b.Identity) })
	baseline.Selectors = nonNil(append([]SelectorBaseline(nil), baseline.Selectors...))
	for index := range baseline.Selectors {
		baseline.Selectors[index].Roots = nonNil(slices.Clone(baseline.Selectors[index].Roots))
		slices.Sort(baseline.Selectors[index].Roots)
		baseline.Selectors[index].Misses = nonNil(slices.Clone(baseline.Selectors[index].Misses))
		slices.SortFunc(baseline.Selectors[index].Misses, compareIdentity)
	}
	slices.SortFunc(baseline.Selectors, func(a, b SelectorBaseline) int { return strings.Compare(a.Name, b.Name) })
	baseline.ProductionDirectives = nonNil(slices.Clone(baseline.ProductionDirectives))
	slices.SortFunc(baseline.ProductionDirectives, func(a, b DirectiveAdmission) int { return compareDirective(a.Directive, b.Directive) })
	baseline.TestDirectives = nonNil(slices.Clone(baseline.TestDirectives))
	slices.SortFunc(baseline.TestDirectives, compareDirective)
	baseline.PlatformDirectives = nonNil(append([]PlatformDirective(nil), baseline.PlatformDirectives...))
	for index := range baseline.PlatformDirectives {
		baseline.PlatformDirectives[index].Platforms = nonNil(slices.Clone(baseline.PlatformDirectives[index].Platforms))
		slices.Sort(baseline.PlatformDirectives[index].Platforms)
	}
	slices.SortFunc(baseline.PlatformDirectives, func(a, b PlatformDirective) int { return compareDirective(a.Directive, b.Directive) })
	baseline.EquivalentMutants = nonNil(slices.Clone(baseline.EquivalentMutants))
	slices.SortFunc(baseline.EquivalentMutants, compareEquivalentMutant)
	return baseline
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

func compareIdentity(a, b Identity) int {
	if a.File != b.File {
		return strings.Compare(a.File, b.File)
	}
	if a.Start.Line != b.Start.Line {
		return a.Start.Line - b.Start.Line
	}
	if a.Start.Column != b.Start.Column {
		return a.Start.Column - b.Start.Column
	}
	if a.End.Line != b.End.Line {
		return a.End.Line - b.End.Line
	}
	if a.End.Column != b.End.Column {
		return a.End.Column - b.End.Column
	}
	return a.Statements - b.Statements
}

func compareEquivalentMutant(a, b EquivalentMutant) int {
	if a.File != b.File {
		return strings.Compare(a.File, b.File)
	}
	if a.Line != b.Line {
		return a.Line - b.Line
	}
	if a.Column != b.Column {
		return a.Column - b.Column
	}
	return strings.Compare(a.Mutator, b.Mutator)
}

func validateBaseline(baseline Baseline) error {
	if baseline.Version != baselineVersion {
		return fmt.Errorf("coverage: unsupported baseline version %d", baseline.Version)
	}
	decodedHash, err := hex.DecodeString(baseline.UniverseSHA256)
	if err != nil || len(decodedHash) != sha256.Size {
		return errors.New("coverage: invalid profile universe hash")
	}
	seenMisses := make(map[Identity]bool)
	for _, admission := range baseline.Repository {
		if err := validateIdentity(admission.Identity); err != nil {
			return err
		}
		if strings.TrimSpace(admission.Reason) == "" {
			return fmt.Errorf("coverage: miss %s has no review reason", formatIdentity(admission.Identity))
		}
		if seenMisses[admission.Identity] {
			return fmt.Errorf("coverage: duplicate miss %s", formatIdentity(admission.Identity))
		}
		seenMisses[admission.Identity] = true
	}
	if len(baseline.Selectors) != len(selectorPolicy) {
		return fmt.Errorf("coverage: got %d selectors, want %d", len(baseline.Selectors), len(selectorPolicy))
	}
	policies := make(map[string][]string)
	for _, selector := range selectorPolicy {
		policies[selector.Name] = selector.Roots
	}
	seenSelectors := make(map[string]bool)
	for _, selector := range baseline.Selectors {
		roots, ok := policies[selector.Name]
		if !ok || seenSelectors[selector.Name] {
			return fmt.Errorf("coverage: invalid selector %q", selector.Name)
		}
		seenSelectors[selector.Name] = true
		if !slices.Equal(selector.Roots, roots) {
			return fmt.Errorf("coverage: selector %q roots do not match policy", selector.Name)
		}
		seen := make(map[Identity]bool)
		for _, miss := range selector.Misses {
			if !seenMisses[miss] || !pathInRoots(miss.File, roots) || seen[miss] {
				return fmt.Errorf("coverage: selector %q has invalid miss %s", selector.Name, formatIdentity(miss))
			}
			seen[miss] = true
		}
	}
	seenDirectives := make(map[string]bool)
	for _, admission := range baseline.ProductionDirectives {
		if !validIgnoreClass(admission.Class) || strings.TrimSpace(admission.Evidence) == "" {
			return fmt.Errorf("coverage: production directive %s:%d has invalid class or evidence", admission.Directive.File, admission.Directive.Line)
		}
		key := directiveKey(admission.Directive)
		if seenDirectives[key] {
			return fmt.Errorf("coverage: duplicate production directive %s:%d", admission.Directive.File, admission.Directive.Line)
		}
		seenDirectives[key] = true
	}
	for _, platform := range baseline.PlatformDirectives {
		if platform.Directive.Mapped || platform.Directive.Executed || platform.Class != IgnorePlatformOnly || len(platform.Platforms) == 0 || strings.TrimSpace(platform.Evidence) == "" {
			return fmt.Errorf("coverage: invalid platform directive %s:%d", platform.Directive.File, platform.Directive.Line)
		}
	}
	seenMutants := make(map[string]bool)
	for _, mutant := range baseline.EquivalentMutants {
		key := fmt.Sprintf("%s:%d:%d:%s", mutant.File, mutant.Line, mutant.Column, mutant.Mutator)
		if mutant.File == "" || mutant.Line <= 0 || mutant.Column <= 0 || mutant.Mutator == "" || strings.TrimSpace(mutant.Reason) == "" || seenMutants[key] {
			return fmt.Errorf("coverage: invalid equivalent mutant %q", key)
		}
		seenMutants[key] = true
	}
	return nil
}

func validateIdentity(identity Identity) error {
	if identity.File == "" || filepath.IsAbs(identity.File) || strings.Contains(identity.File, "\\") || identity.Start.Line <= 0 || identity.Start.Column <= 0 || identity.End.Line <= 0 || identity.End.Column <= 0 || identity.Statements <= 0 {
		return fmt.Errorf("coverage: invalid identity %s", formatIdentity(identity))
	}
	return nil
}

func validIgnoreClass(class IgnoreClass) bool {
	switch class {
	case IgnoreProcessExit, IgnoreImpossibleState, IgnoreDeterministicFault, IgnorePlatformOnly:
		return true
	default:
		return false
	}
}
