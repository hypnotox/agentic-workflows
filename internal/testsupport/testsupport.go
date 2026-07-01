// Package testsupport provides shared test-fixture helpers used across awf's
// test suites: project-config scaffolding, ADR frontmatter fixtures,
// file-writing primitives, and the seam-swap idiom. It is a leaf package —
// only the Go standard library may be imported here (see the gitfixture
// subpackage for the go-git-dependent helpers) — so it is safe to import from
// any package's tests without risking an import cycle (ADR-0044). deps_test.go
// enforces the zero-internal-deps constraint mechanically.
package testsupport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WriteFile creates path's parent directories and writes content to it,
// failing the test on either error. The primitive every other file-writing
// helper in this package is built from.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // coverage-ignore: MkdirAll under a fresh t.TempDir() fails only on a permission fault a test cannot trigger
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { // coverage-ignore: WriteFile into a dir just created above fails only on a permission fault a test cannot trigger
		t.Fatal(err)
	}
}

// WriteAwfConfig writes <root>/.awf/config.yaml with the given content — the
// project-fixture seed step every scaffold-style helper across awf's test
// suites starts from.
func WriteAwfConfig(t *testing.T, root, yamlContent string) {
	t.Helper()
	WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), yamlContent)
}

// SwapVar overrides *seam with val for the duration of the test, restoring
// the original value via t.Cleanup. Covers the swapGetwd/swapHasGoMod/
// forceNonInteractive-shaped idiom: a *T package-private seam variable
// reassigned for one test, in the same package as the test. Does not cover
// internal/adr's swapNow, an external-test-package accessor of a different
// shape (ADR-0044 Context).
func SwapVar[T any](t *testing.T, seam *T, val T) {
	t.Helper()
	orig := *seam
	*seam = val
	t.Cleanup(func() { *seam = orig })
}

// WriteGoModule writes a minimal go.mod (pinned to this repo's Go toolchain
// version) and an f.go containing srcBody under dir.
func WriteGoModule(t *testing.T, dir, modPath, srcBody string) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, "go.mod"), "module "+modPath+"\n\ngo 1.26\n")
	WriteFile(t, filepath.Join(dir, "f.go"), srcBody)
}

// WriteProfile writes dir/cover.out with a "mode: set\n" prefix followed by
// body, returning the file's path.
func WriteProfile(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "cover.out")
	WriteFile(t, p, "mode: set\n"+body)
	return p
}

// ADROption configures an ADR fixture built by ADR.
type ADROption func(*adrOpts)

type adrOpts struct {
	title             string
	date              string
	tags              []string
	domains           []string
	retiresInvariants []string
	body              string
}

// WithTitle sets the ADR's number+title heading text — the part after
// "# ADR-", e.g. "0001: My Title". Defaults to "0001: T".
func WithTitle(title string) ADROption { return func(o *adrOpts) { o.title = title } }

// WithDate sets the frontmatter date field. Omitted from the frontmatter
// entirely when never called — some fixtures deliberately carry no date.
func WithDate(date string) ADROption { return func(o *adrOpts) { o.date = date } }

// WithTags sets the frontmatter tags array.
func WithTags(tags ...string) ADROption { return func(o *adrOpts) { o.tags = tags } }

// WithDomains sets the frontmatter domains array.
func WithDomains(domains ...string) ADROption { return func(o *adrOpts) { o.domains = domains } }

// WithRetiresInvariants sets the frontmatter retires_invariants array.
func WithRetiresInvariants(slugs ...string) ADROption {
	return func(o *adrOpts) { o.retiresInvariants = slugs }
}

// WithBody appends raw markdown (e.g. "## Context\nx\n") after the title
// heading.
func WithBody(body string) ADROption { return func(o *adrOpts) { o.body = body } }

// ADR builds a ---delimited ADR frontmatter fixture as a raw string: a status
// field plus any of date/tags/domains/retires_invariants supplied via opts, a
// "# ADR-<title>" heading, and an optional trailing body. It intentionally
// does not import internal/adr and marshal its real frontmatter struct —
// doing so would break this package's zero-internal-deps invariant (see
// ADR-0044's Consequences).
func ADR(status string, opts ...ADROption) string {
	o := adrOpts{title: "0001: T"}
	for _, opt := range opts {
		opt(&o)
	}
	var b strings.Builder
	b.WriteString("---\nstatus: " + status + "\n")
	if o.date != "" {
		b.WriteString("date: " + o.date + "\n")
	}
	if o.tags != nil {
		b.WriteString("tags: [" + strings.Join(o.tags, ", ") + "]\n")
	}
	if o.domains != nil {
		b.WriteString("domains: [" + strings.Join(o.domains, ", ") + "]\n")
	}
	if o.retiresInvariants != nil {
		b.WriteString("retires_invariants: [" + strings.Join(o.retiresInvariants, ", ") + "]\n")
	}
	b.WriteString("---\n# ADR-" + o.title + "\n")
	b.WriteString(o.body)
	return b.String()
}
