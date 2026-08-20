// Package configspec is the compile-time, adopter-facing description authority
// for the .awf configuration surface: every config.yaml key, sidecar field,
// var, and per-artifact data key an adopter can set. Descriptions are
// publication prose: they state effect and availability in the adopter's terms
// - never internal rationale, concrete ADR citations, or repo-identity
// literals (the residue rules, test-enforced).
package configspec

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// Entry describes one adopter-writable configuration key.
type Entry struct {
	Path         string // dotted YAML path: "audit.diffThreshold", "sidecar.sections.<name>.drop"
	Type         string // value shape as prose: "string", "bool", "string list", "key → value map"
	Default      string // effective default as prose: "docs", "accept any scope", "none"
	Description  string // full adopter-voiced description
	Availability string // when the key has effect: "always", "domain sidecars only", ...
}

// LiveStateClass declares whether a configuration-reference key has a
// project-specific current-value projection. Static keys deliberately carry no
// current value: sidecar fields and structural list leaves have no one project
// value to display.
type LiveStateClass uint8

const (
	StaticNotApplicable LiveStateClass = iota
	LiveStateProjection
)

// LiveStateClassifications derives the exhaustive classification from the
// config-spec authority. Sidecar fields and item-schema leaves have no
// singular project value; every other project config path does.
func LiveStateClassifications() map[string]LiveStateClass {
	classes := make(map[string]LiveStateClass, len(Keys()))
	for _, entry := range Keys() {
		classes[entry.Path] = liveStateClass(entry.Path)
	}
	return classes
}

func liveStateClass(path string) LiveStateClass {
	if strings.HasPrefix(path, "sidecar.") || strings.Contains(path, "[]") || strings.Contains(path, "<name>") {
		return StaticNotApplicable
	}
	return LiveStateProjection
}

// VarEntry describes one config var. Description text is carried verbatim
// from the catalog descriptor - the catalog stays the sole var authority;
// configspec attaches only the availability clause.
type VarEntry struct {
	Key          string
	Description  string
	Availability string
}

// DataKey describes one adopter-settable sidecar data: key of one artifact.
type DataKey struct {
	Kind        string // "skills", "agents", "docs"
	Artifact    string // artifact name; "agents-doc" uses kind "docs"
	Key         string
	Fields      []string // declared record fields when the value is a list of mappings
	Description string
}

// Keys returns every described config.yaml and sidecar key. Sidecar fields
// carry the "sidecar." path prefix.
func Keys() []Entry { return keys }

// DataKeys returns the per-artifact sidecar data-key descriptions.
func DataKeys() []DataKey { return dataKeys }

// VarEntries derives the var descriptions from the catalog's config-var
// descriptors (empty or "var" Target - the init-routing descriptors are not
// vars: keys), description text verbatim, availability clause attached here.
// touches-state: config/configspec-and-reference:configspec-var-derivation - var entries derived from catalog descriptors; proof in spec_test.go
func VarEntries() []VarEntry {
	var out []VarEntry
	for _, d := range catalog.Standard.Vars {
		if d.Target != "" && d.Target != "var" {
			continue
		}
		out = append(out, VarEntry{Key: d.Key, Description: d.Description, Availability: varAvailability[d.Key]})
	}
	return out
}

// varAvailability holds the configspec-owned availability clause per config
// var; the parity test pins its key set to the config-var descriptors.
var varAvailability = map[string]string{
	"gateCmd":           "Consumed while a rendered artifact's template references it, by the `{{=awf:gateCmd}}` placeholder in convention parts (including the rendered pre-push hook payload's part channel), and by divergent effort-integration guidance.",
	"gateCmdFull":       "Consumed while a rendered artifact's template references it.",
	"checkCmd":          "Consumed while a rendered artifact's template references it, and by the `{{=awf:checkCmd}}` placeholder in convention parts.",
	"commitGateCmd":     "Consumed by the always-rendered commit-msg hook payload.",
	"testCmd":           "Consumed while a rendered artifact's template references it.",
	"activeMdRegenCmd":  "Consumed while a rendered artifact's template references it (the decision-index regeneration steps in the chain skills).",
	"invariantTestPath": "Consumed while a rendered artifact's template references it (the invariant-backing guidance in the decision docs and skills).",
}

// keys is the hand-authored description table for config.yaml and sidecar
// keys; the reflection parity test keeps it bidirectionally matched to the
// config structs.
var keys = []Entry{
	{
		Path: "profile", Type: "enum: core or full", Default: "core for fresh init; existing repositories migrate to full",
		Description:  "Selects one closed governance footprint. Core includes the operational workflow. Full adds decision records, plans, current-state authority, context, and governance audit. The footprints use the same correctness, autonomy, maintainability, and review-quality bar.",
		Availability: "Always; required and visible.",
	},
	{
		Path: "prefix", Type: "string", Default: "none: required, set at init",
		Description:  "The name prefix for rendered skills: a skill renders to `<prefix>-<name>` (directory and frontmatter name), and rendered prose references skills by that prefixed name. Must be non-empty, without path separators.",
		Availability: "Always.",
	},
	{
		Path: "integrationBranch", Type: "string", Default: "none: required; the schema migration writes integrationBranch: main",
		Description:  "The branch effort work integrates into. `awf new adr` scaffolds a numbered decision record on this branch and a pending slug-identified one anywhere else, and the check refuses a pending record while the checkout is positively identified as being on it. Must be non-empty and free of whitespace, must not start with `-`; slashes are legal, so `release/1.0` is accepted. There is no in-code default, and audit range resolution never reads it.",
		Availability: "Always.",
	},
	{
		Path: "render.templateSourceRoot", Type: "normalized repository-relative directory", Default: "none (template source symbols disabled)",
		Description:  "The repository directory containing the implementation template tree. When set, generated Markdown carries compact `awf:template-source` comments identifying the root template, included partials, and structural sections. The directory and every referenced source file must exist in the selected repository state.",
		Availability: "Markdown outputs rendered from templates; omitted leaves output unchanged.",
	},
	{
		Path: "vars", Type: "key → value map", Default: "seeded with every catalog-referenced var as an empty string at init",
		Description:  "Freeform values templates interpolate. A key with a value renders it; a present-but-empty key is an open to-do (rendered artifacts referencing it degrade to generic prose and a non-failing note nudges you); a deleted key is the deliberate, git-auditable decline of that var; the generic prose renders silently. A non-empty key no rendered artifact references is unranked Information and exits zero.",
		Availability: "Each key is consumed only while a rendered artifact's template (or a `gateCmd`/`checkCmd` part placeholder) references it, except that `gateCmd` is also consumed by divergent effort-integration guidance.",
	},
	{
		Path: "localDocs", Type: "list of {name, title, description} mappings", Default: "none",
		Description:  "Additive repository-local documents. Each name is a lowercase kebab-case path below docs without .md; decisions, plans, domains, topics, and pitfalls are reserved. Title and description are nonblank one-line metadata.",
		Availability: "Always; each entry renders one managed in-place document.",
	},
	{
		Path: "localDocs[].name", Type: "lowercase kebab-case path", Default: "required", Description: "The docs-relative path without .md.", Availability: "Within each localDocs entry.",
	},
	{
		Path: "localDocs[].title", Type: "string", Default: "required", Description: "The awf-owned document heading.", Availability: "Within each localDocs entry.",
	},
	{
		Path: "localDocs[].description", Type: "string", Default: "required", Description: "The one-line document-map description.", Availability: "Within each localDocs entry.",
	},
	{
		Path: "domains", Type: "string list", Default: "none",
		Description:  "Freeform domain keys. Each renders a generated `docs/domains/<name>.md` doc (a compact topic list plus your `current-state` convention part) and can declare a file territory via the domain sidecar's `paths:`.",
		Availability: "Always.",
	},
	{
		Path: "tags", Type: "key → value map", Default: "none",
		Description:  "A governed vocabulary of cross-cutting keyword tags, each mapping a tag name to a one-line meaning. Authored pitfall `tags:` are validated against it: with a non-empty vocabulary, a used tag that is not a declared member is failing drift, as is a member with an empty meaning. Parsed legacy ADR tags remain historical metadata outside current membership validation. An empty or absent vocabulary disables the check (pitfall tags are then free-form). Declaring a member no pitfall uses is allowed.",
		Availability: "Always; the membership check is inert until the vocabulary is non-empty.",
	},
	{
		Path: "contextIgnore", Type: "string list", Default: "none",
		Description:  "Anchored doublestar globs for tracked paths that context and coverage should exclude (config source, docs, top-level non-code files). Matching paths are ineligible for directory expansion and coverage, including staged queries, alongside awf's own generated outputs. An empty or absent list adds no exclusion.",
		Availability: "Always; consulted by working and staged context path expansion and coverage.",
	},
	{
		Path: "commitPolicy.grandfatheredThrough", Type: "lowercase full object ID", Default: "none: required when commitPolicy is present",
		Description:  "The exact full SHA-1 or SHA-256 commit object ID whose reachable ancestry is tolerated. Abbreviations, uppercase IDs, and non-object IDs are invalid; repository resolution happens when policy enforcement runs. Omitting the complete commitPolicy block preserves existing behavior and activates no policy.",
		Availability: "Required when the optional commitPolicy block is present; the block itself may be absent.",
	},
	{
		Path: "commitPolicy.allowedIdentities", Type: "list of {name, email} mappings", Default: "none (identity matching disabled)",
		Description:  "Optional nonempty allowlist of exact author and committer identity pairs. Identity matching is disabled when this key is omitted; an explicitly empty or null key and duplicate pairs are invalid.",
		Availability: "Within commitPolicy.",
	},
	{
		Path: "commitPolicy.allowedIdentities[].name", Type: "string", Default: "required within each identity record",
		Description:  "The exact author and committer name. It is nonempty valid UTF-8 without controls or leading/trailing whitespace, and pairs with email for byte-for-byte matching.",
		Availability: "Within each optional commitPolicy.allowedIdentities record.",
	},
	{
		Path: "commitPolicy.allowedIdentities[].email", Type: "string", Default: "required within each identity record",
		Description:  "The exact author and committer email. It is nonempty valid UTF-8 without controls or leading/trailing whitespace, and pairs with name for byte-for-byte matching.",
		Availability: "Within each optional commitPolicy.allowedIdentities record.",
	},
	{
		Path: "commitPolicy.requireSignedCommits", Type: "bool", Default: "false",
		Description:  "Requires policy-era commits to carry a verified SSH signature from an allowed signer. False with allowedSigners, or true without allowedSigners, is invalid.",
		Availability: "Within commitPolicy.",
	},
	{
		Path: "commitPolicy.allowedSigners", Type: "list of {principal, key} mappings", Default: "none: required when requireSignedCommits is true",
		Description:  "Nonempty exact SSH signer records when signing is required. A present empty or null list, signers while signing is disabled, and duplicate records are invalid.",
		Availability: "Within commitPolicy while requireSignedCommits is true.",
	},
	{
		Path: "commitPolicy.allowedSigners[].principal", Type: "ASCII authorization token", Default: "required within each signer record",
		Description:  "The signer authorization principal, containing only letters, digits, `.`, `_`, `@`, `+`, or `-`. It authorizes its associated key and is not an asserted commit identity.",
		Availability: "Within each commitPolicy.allowedSigners record while requireSignedCommits is true.",
	},
	{
		Path: "commitPolicy.allowedSigners[].key", Type: "OpenSSH public-key record", Default: "required within each signer record",
		Description:  "An exact option-free, comment-free single OpenSSH public-key record accepted by ssh-keygen and using a supported SSH algorithm. Newlines, trailing records, and unsupported keys are invalid.",
		Availability: "Within each commitPolicy.allowedSigners record while requireSignedCommits is true.",
	},
	{
		Path: "currentState.sources", Type: "list of {globs, marker, close} mappings", Default: "none",
		Description:  "Source families scanned for qualified current-state relevance, advisory, and invariant proof markers during topic validation.",
		Availability: "Consumed by current-state topic validation, coverage, context, and the staged check.",
	},
	{
		Path: "currentState.sources[].globs", Type: "string list", Default: "none",
		Description:  "Non-empty, duplicate-free anchored path globs matched against slash-separated repository-relative paths for one current-state marker source.",
		Availability: "Within each `currentState.sources` entry during topic validation.",
	},
	{
		Path: "currentState.sources[].marker", Type: "string", Default: "none",
		Description:  "Non-empty literal opening comment token that prefixes a qualified current-state marker line.",
		Availability: "Within each `currentState.sources` entry during topic validation.",
	},
	{
		Path: "currentState.sources[].close", Type: "string", Default: "none: no close token stripped",
		Description:  "Optional non-empty literal closing comment token stripped from a matched current-state marker line.",
		Availability: "Within each `currentState.sources` entry during topic validation.",
	},
	{
		Path: "currentState.testGlobs", Type: "string list", Default: "none",
		Description:  "Duplicate-free anchored path globs identifying proof-eligible test files for current-state invariant claims.",
		Availability: "Consumed by current-state topic validation, coverage, context, and the staged check.",
	},
	{
		Path: "audit.allowedScopes", Type: "list of scope entries (bare string, or {name, meaning})", Default: "accept any scope",
		Description:  "The project's Conventional Commits scope taxonomy: the single home for commit scopes; rendered prose quotes it from here. Absent = accept any scope; entries are enforced by `awf check staged commit`/`awf audit` and editing them reflags referencing rendered artifacts.",
		Availability: "Read by `awf check staged commit`, `awf audit`, and every rendered artifact quoting the scope list.",
	},
	{
		Path: "audit.allowedScopes[].name", Type: "string", Default: "none",
		Description:  "The scope token as it appears in a commit subject (`feat(<name>): ...`). A bare-string list entry is shorthand for a name-only entry.",
		Availability: "Within each `audit.allowedScopes` entry.",
	},
	{
		Path: "audit.allowedScopes[].meaning", Type: "string", Default: "empty",
		Description:  "Optional human meaning for the scope, shown wherever the taxonomy is rendered for people choosing a scope.",
		Availability: "Within each `audit.allowedScopes` entry.",
	},
	{
		Path: "bootstrap.enabled", Type: "bool", Default: "false (key absent); awf init scaffolds it true",
		Description:  "Renders the self-pinning `.awf/bootstrap.sh` installer (pinned to the rendering awf version, checksum-verified) and the `.awf/upgrade.sh` porcelain. Absent and false both mean: do not render.",
		Availability: "Always.",
	},
	{
		Path: "proseGate.exemptions", Type: "list of {path, codepoint, count} mappings", Default: "empty (nothing is exempt)",
		Description:  "Places where a guarded en dash or em dash is permitted, typically a quotation, frozen record, or text discussing the character it contains. An entry exempts one guarded codepoint in one path before punctuation restraint is evaluated.",
		Availability: "Always.",
	},
	{
		Path: "proseGate.exemptions[].path", Type: "string", Default: "required",
		Description:  "The repo-relative path the exemption covers. A rendered file and the source it renders from each need their own entry, because each holds its own copy of the character.",
		Availability: "Always.",
	},
	{
		Path: "proseGate.exemptions[].codepoint", Type: "string", Default: "required",
		Description:  "The exempted codepoint, spelled `U+2014`, never the character itself. Use `U+2013` for an en dash or `U+2014` for an em dash. Former ellipsis and curly-quote codepoints remain accepted as inert compatibility input; other values are errors.",
		Availability: "Always.",
	},
	{
		Path: "proseGate.exemptions[].count", Type: "int", Default: "unset (any number is permitted)",
		Description:  "The exact whole-file occurrence count expected for the guarded codepoint. Set, a count change in an exempt file still fails, which suits a frozen record; unset, any number is permitted. The value is ignored for an inert compatibility entry.",
		Availability: "Always.",
	},
	{
		Path: "memoryCite.exemptions", Type: "list of {path, count} mappings", Default: "empty (nothing is exempt)",
		Description:  "Decision records permitted to name a specific working-memory file, typically prose that is genuinely about one particular file. An entry exempts one path. Prefer rewording to the placeholder form over adding an entry.",
		Availability: "Always.",
	},
	{
		Path: "memoryCite.exemptions[].path", Type: "string", Default: "required",
		Description:  "The repo-relative path the exemption covers. Only a path under the decisions or plans directory can carry a finding, so only such a path is worth exempting.",
		Availability: "Always.",
	},
	{
		Path: "memoryCite.exemptions[].count", Type: "int", Default: "unset (any number is permitted)",
		Description:  "The exact number of citations expected. Set, an added citation in an exempt file still fails, which suits a frozen record; unset, any number is permitted, which suits a living file that may gain another mention.",
		Availability: "Always.",
	},
	{
		Path: "sidecar.data", Type: "key → value map", Default: "empty: catalog defaults apply",
		Description:  "Per-artifact structured render data. A same-key catalog-backed list layers the catalog default followed by project entries; an empty project list keeps the complete default, and null or a non-list value is invalid. Non-list catalog data retains shallow top-level project replacement. Project-only and specialized data retain their owning behavior; see the per-artifact list below.",
		Availability: "Keys must be referenced by the artifact's template. An unreferenced key is unranked Information and exits zero; rejected entirely on domain sidecars (paths-only) and on the config-reference sidecar (its tables are generated).",
	},
	{
		Path: "sidecar.dataDefaults", Type: "data-key → bool map", Default: "empty: catalog-backed list defaults remain enabled",
		Description:  "Controls same-key catalog-backed list defaults. An absent key or true keeps the catalog default; false suppresses it so effective content is only the authored project list, or an empty list when none is authored. Explicit true differs from absence only as configuration presence, not effective content.",
		Availability: "Every entry must name a same-key list default declared by that catalog artifact. Unknown, non-list, and differently keyed specialized values are invalid.",
	},
	{
		Path: "sidecar.sections", Type: "section-name → override map", Default: "empty",
		Description:  "Per-section overrides for the artifact's declared sections. Body replacement is by convention part (a file at the section's parts path); this map holds the structured overrides: currently `drop`.",
		Availability: "Section names must be catalog-declared for the artifact; unknown names refuse at open. Rejected on domain sidecars.",
	},
	{
		Path: "sidecar.sections.<name>.drop", Type: "bool", Default: "false",
		Description:  "Omits the named section from the rendered artifact entirely. A drop beats a convention part; a data key referenced only inside a dropped section counts as unused.",
		Availability: "Within a declared section's override entry.",
	},
	{
		Path: "sidecar.paths", Type: "string list (anchored path globs)", Default: "none",
		Description:  "A domain's file territory, matched against slash-separated repo-relative paths. Current-state topic ownership, context, and coverage use these selectors; empty, duplicate, or malformed selectors are rejected.",
		Availability: "Domain sidecars only; validated during working-tree and staged loading, and rejected at open on any other kind.",
	},
}

// dataKeys is the hand-authored per-artifact data-key description table; the
// parity test derives the expected set from the catalog and the embedded
// templates (include-expanded), so an undescribed key cannot ship.
var dataKeys = []DataKey{
	{Kind: "skills", Artifact: "brainstorming", Key: "errorBoundaries", Description: "The error-handling boundaries the design-sections step walks (list); unset, the section keeps its generic boundary prose."},
	{Kind: "skills", Artifact: "brainstorming", Key: "loadBearingExamples", Description: "Project-specific examples of load-bearing decisions for the definitions section (list); unset, the generic examples render."},
	{Kind: "skills", Artifact: "tdd", Key: "testSurfaces", Description: "The project's test surfaces (list of {name, kind, location}) the skill routes new tests to; the default names generic unit/integration/e2e surfaces."},
	{Kind: "skills", Artifact: "adr-lifecycle", Key: "adrStates", Description: "The decision-record lifecycle states (list of {name, meaning, mutability}) the skill's state table renders; the default is the five-state current-state-v2 lifecycle."},
	{Kind: "skills", Artifact: "proposing-adr", Key: "adrSections", Description: "The required decision-record section names, in order (list); the default is Context through Alternatives Considered."},
	{Kind: "skills", Artifact: "proposing-adr", Key: "adrTriggers", Description: "The project's load-bearing triggers that warrant a decision record (list); the default names the generic boundary/dependency/format/workflow triggers."},
	{Kind: "skills", Artifact: "executing-plans", Key: "e2eSuitePaths", Description: "Where the project's end-to-end suites live (prose or list) for the gate-tier guidance; unset, the generic tier prose renders."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "focusItems", Description: "The reviewer's project-focus lens items (list of {name, description}); the defaults cover decision clarity, consequences honesty, and claim-topic cohesion."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "reviewSubject", Description: "The one-word subject label the review spine addresses (default: the decision record)."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "readStep", Description: "The reviewer's opening read instruction: what to read in full before applying lenses."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "digestLabel", Description: "The label heading the reviewer's returned digest."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "digestSummary", Description: "The digest's summary skeleton: the bullet template the reviewer fills per review."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "focusItems", Description: "The reviewer's project-focus lens items (list of {name, description}); the defaults cover change-specific executability, dependency order, snapshot-scoped verification, and check-authority taxonomy."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "docCurrencyItems", Description: "The doc-currency checks the reviewer applies (list of {check}); the default checks that the plan schedules every doc update its changes invalidate."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "reviewSubject", Description: "The one-word subject label the review spine addresses (default: the plan)."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "readStep", Description: "The reviewer's opening read instruction: what to read in full before applying lenses."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "digestLabel", Description: "The label heading the reviewer's returned digest."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "digestSummary", Description: "The digest's summary skeleton: the bullet template the reviewer fills per review."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "correctnessTraps", Description: "The correctness traps the reviewer checks first (list of {description}); the default names error paths and boundary conditions."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "focusItems", Description: "The reviewer's project-focus lens items (list of {name, description}); the defaults cover plan adherence, test coverage, verification-instrument falsifiability, and check-authority taxonomy."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "docCurrencyItems", Description: "The doc-currency checks the reviewer applies (list of {check}); the default checks same-commit updates of every doc stating the old behaviour."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "reviewSubject", Description: "The one-word subject label the review spine addresses (default: the diff)."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "readStep", Description: "The reviewer's opening read instruction: what to read in full before applying lenses."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "digestLabel", Description: "The label heading the reviewer's returned digest."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "digestSummary", Description: "The digest's summary skeleton: the bullet template the reviewer fills per review."},
	{Kind: "agents", Artifact: "implementer", Key: "prohibitedShortcuts", Description: "The bolt-on shortcuts the implementer must never take (list of {description}); the default names speculative abstraction and misplaced responsibility. Unset, the body omits the list and the rest of the contract renders unchanged."},
	{Kind: "docs", Artifact: "glossary", Key: "terms", Description: "The glossary's terms as an ordered list of `{term, meaning, domains}` records; the table renders always sorted (case-insensitive, pipes escaped), and an empty term or meaning, an interior newline, an unknown record key, or a case-insensitive duplicate term fails the render naming the offending term. `domains` (optional) must resolve to configured domains. A term here overrides the standard vocabulary awf ships of the same case-insensitive name, which is how you replace or retire one. Unset, the doc renders the standard vocabulary alone; the pointer telling you where to add terms renders only when neither layer supplies a term. A meaning longer than the terseness guideline raises a non-failing advisory rather than failing the render."},
	{Kind: "docs", Artifact: "agents-doc", Key: "commands", Fields: []string{"cmd", "desc"}, Description: "Extra command entries for the agent guide's Commands section (list of {cmd, desc}-shaped mappings rendered as lines); unset, only the built-in command list renders."},
	{Kind: "docs", Artifact: "agents-doc", Key: "docMap", Fields: []string{"path", "desc"}, Description: "Extra document-map entries for the agent guide (list rendered after the managed docs); unset, only the managed docs render."},
	{Kind: "docs", Artifact: "agents-doc", Key: "invariants", Fields: []string{"kind", "ref", "text"}, Description: "The project's hard-rules list for the agent guide's Invariants section (list of {ref, text} mappings); unset, the section renders its generic invariants prose."},
}
