package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"gopkg.in/yaml.v3"
)

// runNew scaffolds a new templated artifact: an ADR, or a project-local skill/agent
// (ADR-0068). ADR takes a single joined title; skill/agent take a name and a
// separate quoted description.
// touches-invariant: adr-new-version-gated — new-command version gate site; proof in gate_test.go
func runNew(root, kind string, args []string, stdout io.Writer) error {
	switch kind {
	case "adr":
		return newADR(root, args, stdout)
	case "plan":
		return newPlan(root, args, stdout)
	case "skill", "agent", "doc":
		return newLocal(root, kind, args, stdout)
	default:
		return &usageErr{fmt.Sprintf("unknown kind %q (want: adr, plan, skill, agent, doc)", kind)}
	}
}

func newADR(root string, titleWords []string, stdout io.Writer) error {
	if len(titleWords) == 0 {
		return &usageErr{"usage: awf new adr <title>"}
	}
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	path, err := p.NewADR(strings.Join(titleWords, " "))
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, path)
	return nil
}

func newPlan(root string, titleWords []string, stdout io.Writer) error {
	if len(titleWords) == 0 {
		return &usageErr{"usage: awf new plan <title>"}
	}
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	path, err := p.NewPlan(strings.Join(titleWords, " "))
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, path)
	return nil
}

// newLocal scaffolds a project-local artifact (ADR-0068, ADR-0091): a skill,
// agent, or doc. It validates the name (path-aware for doc), writes a declaring
// sidecar carrying the description — plus a derived title for a doc — and a
// starter content part, enables the name in config (seeding a skill/agent's
// referenced vars), and re-renders. The kind parameterizes the two differences:
// the name validator + stub, and the doc-only title / no var-seeding.
func newLocal(root, kind string, args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return &usageErr{fmt.Sprintf("usage: awf new %s <name> \"<description>\"", kind)}
	}
	name := args[0]
	desc := strings.Join(strings.Fields(strings.Join(args[1:], " ")), " ")
	if desc == "" {
		return &usageErr{"description must not be empty"}
	}
	isDoc := kind == "doc"
	if isDoc {
		if err := config.ValidateDocName(name); err != nil {
			return err
		}
	} else if err := config.ValidateArtifactName(kind, name); err != nil {
		return err
	}
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	pl, _ := project.PluralKind(kind) // "skills" / "agents" / "docs"
	if pool, _ := project.CatalogNames(p.Cat, kind); slices.Contains(pool, name) {
		return fmt.Errorf("%s %q already exists (catalog or local) — pick another name", kind, name)
	}
	// The pool guard misses a name that is declared but not enabled (or opted
	// out via local: true); never overwrite files an author may have edited.
	scPath := filepath.Join(config.RootDir(root), pl, name+".yaml")
	partPath := p.Cfg.PartPath(pl, name, "content")
	for _, existing := range []string{scPath, partPath} {
		if _, err := os.Stat(existing); err == nil {
			return fmt.Errorf("%s %q already has authored files (%s) — remove them first or pick another name", kind, name, existing)
		}
	}
	// Declaring sidecar: data.description feeds the base template's frontmatter;
	// a doc also carries a derived title.
	data := map[string]any{"description": desc}
	stub := localPartStub
	if isDoc {
		data["title"] = project.DeriveDocTitle(name)
		stub = localDocPartStub
	}
	scBytes, err := yaml.Marshal(map[string]any{"data": data})
	if err != nil { // coverage-ignore: a string map always marshals
		return err
	}
	if err := os.MkdirAll(filepath.Dir(scPath), 0o755); err != nil { // coverage-ignore: parent is the just-opened .awf tree; fails only on a permission fault a test cannot trigger
		return err
	}
	if err := os.WriteFile(scPath, scBytes, 0o644); err != nil { // coverage-ignore: post-mkdir write; fails only on a permission fault a test cannot trigger
		return err
	}
	if err := os.MkdirAll(filepath.Dir(partPath), 0o755); err != nil { // coverage-ignore: as above
		return err
	}
	if err := os.WriteFile(partPath, []byte(stub), 0o644); err != nil { // coverage-ignore: as above
		return err
	}
	updated, err := config.SetArrayMember(p.Cfg.Source(), pl, name, true)
	if err != nil { // coverage-ignore: config.Load already parsed this config, so SetArrayMember cannot fail here
		return err
	}
	if !isDoc {
		refs, err := project.ScaffoldVarRefs(kind)
		if err != nil { // coverage-ignore: embedded base templates always read and expand
			return err
		}
		if updated, err = seedScaffoldVars(updated, refs); err != nil { // coverage-ignore: config.Load already parsed this config, so re-parsing cannot fail here
			return err
		}
	}
	if err := os.WriteFile(config.ConfigPath(root), updated, 0o644); err != nil { // coverage-ignore: post-validation write; fails only on a permission fault a test cannot trigger
		return err
	}
	return runSync(root, stdout)
}

// seedScaffoldVars seeds each of the scaffolded template's referenced vars as
// an empty key when absent from cfgSrc — the creation-time open to-do
// (ADR-0087 Decision 4). A present key, and a deleted one, are never touched.
// invariant: new-seeds-scaffold-vars
func seedScaffoldVars(cfgSrc []byte, refs []string) ([]byte, error) {
	for _, r := range refs {
		var err error
		if cfgSrc, err = config.SeedVarKey(cfgSrc, r); err != nil {
			return nil, err
		}
	}
	return cfgSrc, nil
}

// localPartStub is the starter body for a new local artifact's content part —
// plain prose only (no live {{=awf:…}} placeholder, which would hard-error if its
// value were unset this render). The leading awf:stub marker line declares the
// part unauthored (ADR-0070): awf check reports it until the author deletes the
// line, and the part still renders verbatim, marker included.
const localPartStub = "<!-- awf:stub -->\n" +
	"Replace this with the artifact's body, then delete the awf:stub marker line above — " +
	"awf check flags this part as unauthored while the marker remains. This file is a " +
	"convention part: edit it to author the content, and see docs/working-with-awf.md for " +
	"the placeholder syntax.\n"

// localDocPartStub is the starter body for a new local doc's content part. The
// doc-standard pointer is prose, not a markdown link — a file-relative link would
// resolve dead from a nested doc's directory (ADR-0020). The leading awf:stub
// marker declares the part unauthored (ADR-0070).
const localDocPartStub = "<!-- awf:stub -->\n" +
	"Replace this with the document body, then delete the awf:stub marker line above — " +
	"awf check flags this part as unauthored while the marker remains. Write it to the " +
	"project's documentation standard (see docs/doc-standard.md). This file is a convention " +
	"part: edit it to author the content.\n"
