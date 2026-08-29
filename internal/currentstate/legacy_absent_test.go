package currentstate_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// legacyAuthorityIdents are the identifiers of the deleted ADR-derived authority
// engines: the anchor-supersession/coverage model, the ADR-projected indexes,
// and the invariant-declaration scanner. ADR-0133/0135 make canonical topic
// claims the sole active authority, so none of these may reappear in shipped Go
// or in an embedded runtime template. Each is a CamelCase Go identifier, so a
// whole-word match never trips ordinary prose.
var legacyAuthorityIdents = []string{
	"SupersessionRef", "AnnotatedAnchors", "Chains", "Retirers",
	"StateCovered", "PartiallySuperseded", "DeclaringADRs",
	"RenderActiveMD", "RenderDomainIndex",
}

// legacyContextFields are the old ContextResult expansion fields: the
// ADR-derived governing/related/background context that ADR-0134's topic-centric
// context replaced. They are scoped to the context producer rather than banned
// tree-wide, because Related collides with the live adr.ADR.Related frontmatter
// field and Background/Plans are ordinary words elsewhere; that file is the one
// place the legacy result lived, so their absence there proves the expansion is
// gone without a false positive. ADR-0195 carved the producer out of
// internal/project into internal/contextq; the suffix follows it.
var legacyContextFields = []string{"Governing", "Related", "Background", "Pitfalls", "Plans"}

// migrationApprovalPath is retired cutover authority. No production file may
// name it now that permanent locks and the generic journal are the only live
// upgrade authority.
const migrationApprovalPath = "current-state-migration.yaml"

// bridgeImportPath is the deleted cross-schema bridge package; no production file
// may import it (its inventory, readiness, snapshot, and approval parsers went
// with it, ADR-0136).
const bridgeImportPath = `"github.com/hypnotox/agentic-workflows/internal/bridge"`

// contextGoSuffix identifies the rewritten context producer among the walked
// files without depending on the test's working directory.
const contextGoSuffix = "internal/contextq/context.go"

// bannedWholeWords returns which banned identifiers occur in body as whole words.
// The pure matcher is unit-tested directly so the tree scan cannot pass vacuously.
func bannedWholeWords(body string, banned []string) []string {
	var hit []string
	for _, w := range banned {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`).MatchString(body) {
			hit = append(hit, w)
		}
	}
	return hit
}

// productionGoSources walks the shipped Go tree (internal/ and cmd/, no tests)
// and hands each file's slash path and contents to fn, returning the count. It
// deliberately never descends docs/decisions, docs/plans, or the changelog: a
// historical ADR that discusses the retired supersession model in its prose stays
// legal, because it is history, not shipped authority.
func productionGoSources(t *testing.T, fn func(path, body string)) int {
	t.Helper()
	seen := 0
	repoRoot := filepath.Join("..", "..")
	testsupport.WalkRepoSources(t, repoRoot, func(path string, body []byte) {
		if !strings.HasPrefix(path, "internal/") && !strings.HasPrefix(path, "cmd/") {
			return
		}
		seen++
		fn(path, string(body))
	})
	return seen
}

// TestLegacyAuthorityAbsent is the deterministic denylist that keeps the deleted
// ADR-derived authority from creeping back after the current-state cutover
// (ADR-0133/0134/0135). It scans shipped Go and runtime templates for the retired
// identifiers, confines the legacy context fields to the file they were removed
// from, and forbids both the deleted bridge import and migration approval path.
// The companion behavioral assertion that the
// retired decision output is no longer planned lives in internal/project, where
// the output plan is reachable without an import cycle.
