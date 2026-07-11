// Package configspec is the compile-time, adopter-facing description authority
// for the .awf configuration surface: every config.yaml key, sidecar field,
// var, and per-artifact data key an adopter can set. Descriptions are
// publication prose: they state effect and availability in the adopter's terms
// — never internal rationale, concrete ADR citations, or repo-identity
// literals (the residue rules, test-enforced).
package configspec

import "github.com/hypnotox/agentic-workflows/internal/catalog"

// Entry describes one adopter-writable configuration key.
type Entry struct {
	Path         string // dotted YAML path: "audit.diffThreshold", "sidecar.sections.<name>.drop"
	Type         string // value shape as prose: "string", "bool", "string list", "key → value map"
	Default      string // effective default as prose: "docs", "accept any scope", "none"
	Description  string // full adopter-voiced description
	Availability string // when the key has effect: "always", "domain sidecars only", …
}

// VarEntry describes one config var. Description text is carried verbatim
// from the catalog descriptor — the catalog stays the sole var authority;
// configspec attaches only the availability clause.
type VarEntry struct {
	Key          string
	Description  string
	Availability string
}

// DataKey describes one adopter-settable sidecar data: key of one artifact.
type DataKey struct {
	Kind        string // "skills", "agents", "docs"
	Artifact    string // artifact name; "agents-doc" uses kind "docs"; "_base" covers local artifacts
	Key         string
	Description string
}

// Keys returns every described config.yaml and sidecar key. Sidecar fields
// carry the "sidecar." path prefix.
func Keys() []Entry { return keys }

// DataKeys returns the per-artifact sidecar data-key descriptions.
func DataKeys() []DataKey { return dataKeys }

// VarEntries derives the var descriptions from the catalog's config-var
// descriptors (empty or "var" Target — the init-routing descriptors are not
// vars: keys), description text verbatim, availability clause attached here.
// invariant: configspec-var-derivation
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
	"gateCmd":           "Consumed while an enabled artifact's template references it, and by the `{{=awf:gateCmd}}` placeholder in convention parts (including the rendered pre-push hook payload's part channel).",
	"gateCmdFull":       "Consumed while an enabled artifact's template references it.",
	"checkCmd":          "Consumed while an enabled artifact's template references it, and by the `{{=awf:checkCmd}}` placeholder in convention parts.",
	"commitGateCmd":     "Consumed by the rendered commit-msg hook payload while the hooks singleton is enabled.",
	"testCmd":           "Consumed while an enabled artifact's template references it.",
	"activeMdRegenCmd":  "Consumed while an enabled artifact's template references it (the decision-index regeneration steps in the chain skills).",
	"invariantTestPath": "Consumed while an enabled artifact's template references it (the invariant-backing guidance in the decision docs and skills).",
}

// keys is the hand-authored description table for config.yaml and sidecar
// keys; the reflection parity test keeps it bidirectionally matched to the
// config structs.
// invariant: configspec-key-parity
var keys = []Entry{
	{
		Path: "prefix", Type: "string", Default: "none — required, set at init",
		Description:  "The name prefix for rendered skills: a skill renders to `<prefix>-<name>` (directory and frontmatter name), and rendered prose references skills by that prefixed name. Must be non-empty, without path separators.",
		Availability: "Always.",
	},
	{
		Path: "docsDir", Type: "string", Default: "docs",
		Description:  "Root directory for rendered documentation: managed docs render to `<docsDir>/<name>.md`, decisions to `<docsDir>/decisions/`, plans to `<docsDir>/plans/`, domain docs to `<docsDir>/domains/`. Relative, without `..`.",
		Availability: "Always.",
	},
	{
		Path: "vars", Type: "key → value map", Default: "seeded with every catalog-referenced var as an empty string at init",
		Description:  "Freeform values templates interpolate. A key with a value renders it; a present-but-empty key is an open to-do (rendered artifacts referencing it degrade to generic prose and a non-failing note nudges you); a deleted key is the deliberate, git-auditable decline of that var — the generic prose renders silently. A non-empty key no rendered artifact references is failing drift.",
		Availability: "Each key is consumed only while an enabled artifact's template (or a `gateCmd`/`checkCmd` part placeholder) references it.",
	},
	{
		Path: "skills", Type: "string list", Default: "the workflow-core set at init",
		Description:  "Enabled skills. Catalog names render from the embedded templates; a name with a `local: true` sidecar is a hand-maintained project-local skill. The enabled set must be requirement-closed: `awf add skill` enables a skill's full requirement closure, and `awf remove` refuses while enabled artifacts still require the target.",
		Availability: "Always.",
	},
	{
		Path: "agents", Type: "string list", Default: "every catalog agent at init",
		Description:  "Enabled review agents. A reviewing skill's dispatched agent must stay enabled while the skill is — removal refuses upfront; `awf add skill` auto-enables the pair.",
		Availability: "Always.",
	},
	{
		Path: "docs", Type: "string list", Default: "empty at init (the always-on docs are not listed here)",
		Description:  "Enabled toggleable docs (architecture, testing, development, …). The always-on docs — the agent guide, workflow, this reference — render regardless and are not listed here.",
		Availability: "Always.",
	},
	{
		Path: "domains", Type: "string list", Default: "none",
		Description:  "Freeform domain keys. Each renders a generated `<docsDir>/domains/<name>.md` doc (its per-domain decision index plus your `current-state` convention part) and can declare a file territory via the domain sidecar's `paths:`.",
		Availability: "Always.",
	},
	{
		Path: "targets", Type: "string list", Default: "claude",
		Description:  "Enabled adapter runtimes. Skills and agents render once per target into that runtime's layout; docs are runtime-neutral and render once.",
		Availability: "Always.",
	},
	{
		Path: "invariants.disabled", Type: "bool", Default: "false",
		Description:  "Explicit opt-out of invariant-backing enforcement. With enforcement neither configured (no `sources`) nor disabled, gated commands refuse once decision docs carry taggable invariants — set `sources` or set this true.",
		Availability: "Always.",
	},
	{
		Path: "invariants.sources", Type: "list of {globs, marker} mappings", Default: "none — enforcement unconfigured",
		Description:  "Where invariant-backing comments live. Each entry pairs path globs with the literal comment marker that prefixes a backing `invariant: <slug>` tag in those files. Non-empty enables enforcement: every tagged invariant in an implemented decision doc must have a matching backing comment.",
		Availability: "Always.",
	},
	{
		Path: "invariants.sources[].globs", Type: "string list", Default: "none",
		Description:  "Anchored path globs matched against slash-separated repo-relative paths: `*.go` is top-level only, any-depth is `**/*.go`. At least one glob per source.",
		Availability: "Within each `invariants.sources` entry.",
	},
	{
		Path: "invariants.sources[].marker", Type: "string", Default: "none",
		Description:  "The literal comment marker (`//`, `#`, `--`, …) that prefixes a backing `invariant: <slug>` tag in the entry's files. Must be non-empty.",
		Availability: "Within each `invariants.sources` entry.",
	},
	{
		Path: "audit.baseBranch", Type: "string", Default: "main",
		Description:  "The branch `awf audit` compares against when computing the branch's commit range; `--base <ref>` overrides per run.",
		Availability: "Read by `awf audit`.",
	},
	{
		Path: "audit.allowedTypes", Type: "string list", Default: "the Conventional Commits type set (build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test)",
		Description:  "Commit types `awf commit-gate` and `awf audit` accept. Absent key = the default set; an explicit empty list = accept any type. (Absent and empty differ.)",
		Availability: "Read by `awf commit-gate` and `awf audit`.",
	},
	{
		Path: "audit.allowedScopes", Type: "list of scope entries (bare string, or {name, meaning})", Default: "accept any scope",
		Description:  "The project's Conventional Commits scope taxonomy — the single home for commit scopes; rendered prose quotes it from here. Absent = accept any scope; entries are enforced by `awf commit-gate`/`awf audit` and editing them reflags referencing rendered artifacts.",
		Availability: "Read by `awf commit-gate`, `awf audit`, and every rendered artifact quoting the scope list.",
	},
	{
		Path: "audit.allowedScopes[].name", Type: "string", Default: "none",
		Description:  "The scope token as it appears in a commit subject (`feat(<name>): …`). A bare-string list entry is shorthand for a name-only entry.",
		Availability: "Within each `audit.allowedScopes` entry.",
	},
	{
		Path: "audit.allowedScopes[].meaning", Type: "string", Default: "empty",
		Description:  "Optional human meaning for the scope, shown wherever the taxonomy is rendered for people choosing a scope.",
		Availability: "Within each `audit.allowedScopes` entry.",
	},
	{
		Path: "audit.subjectMaxLength", Type: "int", Default: "72",
		Description:  "Maximum commit-subject length `awf commit-gate` and `awf audit` accept.",
		Availability: "Read by `awf commit-gate` and `awf audit`.",
	},
	{
		Path: "audit.dependencyManifests", Type: "string list (anchored path globs)", Default: "a broad manifest set (**/go.mod, **/package.json, **/Cargo.toml, …)",
		Description:  "Globs identifying dependency manifests; `awf audit` flags a manifest change without a lockfile-style co-change. Absent = the default set; explicit empty = the rule is off.",
		Availability: "Read by `awf audit`.",
	},
	{
		Path: "audit.diffThreshold", Type: "int", Default: "400",
		Description:  "Changed-line count above which `awf audit` advises that a commit likely bundles more than one concern.",
		Availability: "Read by `awf audit`.",
	},
	{
		Path: "audit.domainDocStaleness", Type: "bool", Default: "true",
		Description:  "Advisory rule: warn when a domain's decisions changed in range without its generated domain doc being re-rendered.",
		Availability: "Read by `awf audit`; inert without configured domains.",
	},
	{
		Path: "audit.domainCodeStaleness", Type: "bool", Default: "true",
		Description:  "Advisory rule: warn when a domain's declared `paths:` territory changed in range without a co-change to that domain's `current-state` convention part. Inert for a domain without `paths:`.",
		Availability: "Read by `awf audit`; inert without configured domains.",
	},
	{
		Path: "audit.undocumentedDomain", Type: "bool", Default: "true",
		Description:  "Advisory rule: warn when decision docs tag a domain key that is not configured under `domains:`.",
		Availability: "Read by `awf audit`.",
	},
	{
		Path: "audit.uncommittedChanges", Type: "bool", Default: "true",
		Description:  "Advisory rule: warn when the working tree carries uncommitted changes at audit time.",
		Availability: "Read by `awf audit`.",
	},
	{
		Path: "bootstrap.enabled", Type: "bool", Default: "false (key absent) — awf init scaffolds it true",
		Description:  "Renders the self-pinning `.awf/bootstrap.sh` installer (pinned to the rendering awf version, checksum-verified) and the `.awf/upgrade.sh` porcelain. Absent and false both mean: do not render.",
		Availability: "Always.",
	},
	{
		Path: "hooks.enabled", Type: "bool", Default: "false (key absent) — awf init scaffolds it true",
		Description:  "Renders the three inert git-hook payload scripts under `.awf/hooks/` (pre-commit, commit-msg, pre-push). awf never activates hooks or touches git config — wiring the payloads into your hook setup is yours.",
		Availability: "Always.",
	},
	{
		Path: "sidecar.data", Type: "key → value map", Default: "empty — catalog defaults apply",
		Description:  "Per-artifact structured render data, overriding the artifact's catalog default per top-level key; a present-but-null key declines the default explicitly. See the per-artifact data-key list below for what each key does.",
		Availability: "Keys must be referenced by the artifact's template — an unreferenced key is failing drift; rejected entirely on domain sidecars (paths-only) and on the config-reference sidecar (its tables are generated).",
	},
	{
		Path: "sidecar.sections", Type: "section-name → override map", Default: "empty",
		Description:  "Per-section overrides for the artifact's declared sections. Body replacement is by convention part (a file at the section's parts path); this map holds the structured overrides — currently `drop`.",
		Availability: "Section names must be catalog-declared for the artifact; unknown names refuse at open. Rejected on domain sidecars.",
	},
	{
		Path: "sidecar.sections.<name>.drop", Type: "bool", Default: "false",
		Description:  "Omits the named section from the rendered artifact entirely. A drop beats a convention part; a data key referenced only inside a dropped section counts as unused.",
		Availability: "Within a declared section's override entry.",
	},
	{
		Path: "sidecar.local", Type: "bool", Default: "false",
		Description:  "Marks the artifact project-local: awf renders nothing for it and treats your hand-maintained file at the conventional output path as authoritative (frontmatter still validated for skills/agents). A local artifact's convention parts and data keys are unconsumed by construction.",
		Availability: "Skills, agents, docs, and the always-on singletons; rejected on domain sidecars.",
	},
	{
		Path: "sidecar.paths", Type: "string list (anchored path globs)", Default: "none",
		Description:  "A domain's file territory, matched against slash-separated repo-relative paths. Powers the domain-code-staleness audit advisory: territory changes expect a co-change to the domain's `current-state` part.",
		Availability: "Domain sidecars only — rejected at open on any other kind.",
	},
}

// dataKeys is the hand-authored per-artifact data-key description table; the
// parity test derives the expected set from the catalog and the embedded
// templates (include-expanded), so an undescribed key cannot ship.
// invariant: configspec-data-parity
var dataKeys = []DataKey{
	{Kind: "skills", Artifact: "brainstorming", Key: "errorBoundaries", Description: "The error-handling boundaries the design-sections step walks (list); unset, the section keeps its generic boundary prose."},
	{Kind: "skills", Artifact: "brainstorming", Key: "loadBearingExamples", Description: "Project-specific examples of load-bearing decisions for the definitions section (list); unset, the generic examples render."},
	{Kind: "skills", Artifact: "tdd", Key: "testSurfaces", Description: "The project's test surfaces (list of {name, kind, location}) the skill routes new tests to; the default names generic unit/integration/e2e surfaces."},
	{Kind: "skills", Artifact: "adr-lifecycle", Key: "adrStates", Description: "The decision-record lifecycle states (list of {name, meaning, mutability}) the skill's state table renders; the default is the four-state lifecycle."},
	{Kind: "skills", Artifact: "proposing-adr", Key: "adrSections", Description: "The required decision-record section names, in order (list); the default is Context through Alternatives Considered."},
	{Kind: "skills", Artifact: "proposing-adr", Key: "adrTriggers", Description: "The project's load-bearing triggers that warrant a decision record (list); the default names the generic boundary/dependency/format/workflow triggers."},
	{Kind: "skills", Artifact: "executing-plans", Key: "e2eSuitePaths", Description: "Where the project's end-to-end suites live (prose or list) for the gate-tier guidance; unset, the generic tier prose renders."},
	{Kind: "skills", Artifact: "_base", Key: "slug", Description: "The local skill's name identifier interpolated into its frontmatter; synthesized from the artifact name at declaration — override only to diverge the rendered name token."},
	{Kind: "skills", Artifact: "_base", Key: "description", Description: "The local skill's frontmatter description — the when-to-use line agent runtimes surface. `awf new skill` seeds it; keep it current."},
	{Kind: "agents", Artifact: "_base", Key: "slug", Description: "The local agent's name identifier interpolated into its frontmatter; synthesized from the artifact name at declaration — override only to diverge the rendered name token."},
	{Kind: "agents", Artifact: "_base", Key: "description", Description: "The local agent's frontmatter description — the dispatch-time summary agent runtimes surface. `awf new agent` seeds it; keep it current."},
	{Kind: "docs", Artifact: "_base", Key: "title", Description: "The local doc's display title — its H1 and document-map label. `awf new doc` seeds it from the name; override to set a custom title."},
	{Kind: "docs", Artifact: "_base", Key: "description", Description: "The local doc's one-line summary — the document-map description and the lede under its H1. `awf new doc` seeds it; keep it current."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "focusItems", Description: "The reviewer's project-focus lens items (list of {name, description}); the default focuses decision clarity and consequences honesty."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "docCurrencyItems", Description: "The doc-currency checks the reviewer applies (list of {check}); the default checks same-commit doc updates and index regeneration."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "reviewSubject", Description: "The one-word subject label the review spine addresses (default: the decision record)."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "readStep", Description: "The reviewer's opening read instruction — what to read in full before applying lenses."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "digestLabel", Description: "The label heading the reviewer's returned digest."},
	{Kind: "agents", Artifact: "adr-reviewer", Key: "digestSummary", Description: "The digest's summary skeleton — the bullet template the reviewer fills per review."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "focusItems", Description: "The reviewer's project-focus lens items (list of {name, description}); the default focuses step exactness and dependency order."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "docCurrencyItems", Description: "The doc-currency checks the reviewer applies (list of {check}); the default checks that the plan schedules every doc update its changes invalidate."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "reviewSubject", Description: "The one-word subject label the review spine addresses (default: the plan)."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "readStep", Description: "The reviewer's opening read instruction — what to read in full before applying lenses."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "digestLabel", Description: "The label heading the reviewer's returned digest."},
	{Kind: "agents", Artifact: "plan-reviewer", Key: "digestSummary", Description: "The digest's summary skeleton — the bullet template the reviewer fills per review."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "correctnessTraps", Description: "The correctness traps the reviewer checks first (list of {description}); the default names error paths and boundary conditions."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "focusItems", Description: "The reviewer's project-focus lens items (list of {name, description}); the default focuses plan adherence and test coverage."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "docCurrencyItems", Description: "The doc-currency checks the reviewer applies (list of {check}); the default checks same-commit updates of every doc stating the old behaviour."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "reviewSubject", Description: "The one-word subject label the review spine addresses (default: the diff)."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "readStep", Description: "The reviewer's opening read instruction — what to read in full before applying lenses."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "digestLabel", Description: "The label heading the reviewer's returned digest."},
	{Kind: "agents", Artifact: "code-reviewer", Key: "digestSummary", Description: "The digest's summary skeleton — the bullet template the reviewer fills per review."},
	{Kind: "docs", Artifact: "glossary", Key: "terms", Description: "The glossary's terms as a `term: meaning` map; the table renders always sorted (case-insensitive, pipes escaped), and empty terms or meanings, interior newlines, or case-insensitive duplicates fail the render naming the key. Unset, the doc renders a pointer telling you where to add terms."},
	{Kind: "docs", Artifact: "agents-doc", Key: "commands", Description: "Extra command entries for the agent guide's Commands section (list of {cmd, desc}-shaped mappings rendered as lines); unset, only the built-in command list renders."},
	{Kind: "docs", Artifact: "agents-doc", Key: "docMap", Description: "Extra document-map entries for the agent guide (list rendered after the managed docs); unset, only the managed docs render."},
	{Kind: "docs", Artifact: "agents-doc", Key: "invariants", Description: "The project's hard-rules list for the agent guide's Invariants section (list of {ref, text} mappings); unset, the section renders its generic invariants prose."},
}
