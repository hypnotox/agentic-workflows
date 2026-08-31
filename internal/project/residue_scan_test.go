package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
)

// residueADRRe matches a concrete awf ADR citation - `ADR-` followed by four
// digits. The `ADR-NNNN` authoring placeholder never matches.
var residueADRRe = regexp.MustCompile(`ADR-[0-9]{4}`)

// identityExempt lists template files whose repo-identity literal is a
// required reference to awf or to the pi-tools prerequisite, not residue.
// Entries fail when stale.
var identityExempt = map[string]bool{
	"bootstrap/awf-bootstrap.sh.tmpl":   true,
	"bootstrap/awf-upgrade.sh.tmpl":     true,
	"agents-doc/AGENTS.md.tmpl":         true,
	"docs/pi-runtime-reference.md.tmpl": true,
	"pi/awf-subagents/index.ts.tmpl":    true,
}

// identityLiterals are the banned repo-identity tokens.
var identityLiterals = []string{"hypnotox", "agentic-workflows"}

// repoLocalInstructionLiterals name awf-repository-only tooling that shipped
// templates must never instruct adopters to run.
var repoLocalInstructionLiterals = []string{"./x ", "cmd/repoaudit"}

func repositoryLocalInstruction(src string) string {
	for _, lit := range repoLocalInstructionLiterals {
		if strings.Contains(src, lit) {
			return lit
		}
	}
	return ""
}

// TestLiveTemplateAndCurrentStateRetiredConfigGuidanceAbsent protects both raw
// live guidance sources and the default rendered adopter documentation while
// naming the few unrelated uses of local terminology that remain truthful.
// invariant: rendering/templates:retired-config-guidance-absent (TestLiveTemplateAndCurrentStateRetiredConfigGuidanceAbsent)
func TestLiveTemplateAndCurrentStateRetiredConfigGuidanceAbsent(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	allowProjectLocal := map[string]int{
		".awf/topics/parts/config/migrations-and-locks/current-state.md": 0, // supported-floor migration policy
		".awf/topics/parts/rendering/pi-workflows/current-state.md":      2, // Pi preference-file locality
		"templates/pi/awf-subagents/index.ts.tmpl":                       1, // Pi preference-file locality
	}
	bannedGuidance := []string{
		"local: true",
		"generated local docs",
		"Session-local skills, agents, and docs",
		"configured local skill",
	}
	inspected := 0
	checkSource := func(path string, body []byte) {
		t.Helper()
		inspected++
		src := string(body)
		for _, banned := range bannedGuidance {
			if strings.Contains(src, banned) {
				t.Errorf("%s presents retired artifact configuration or capability %q", path, banned)
			}
		}
		if got, allowed := strings.Count(src, "project-local"), allowProjectLocal[path]; got != allowed {
			t.Errorf("%s has %d project-local reference(s); want exactly %d unrelated allowed reference(s)", path, got, allowed)
		}
	}
	for _, dir := range []string{"templates", ".awf/topics/parts", ".awf/domains/parts"} {
		root := filepath.Join(repoRoot, filepath.FromSlash(dir))
		testsupport.WalkRepoFiles(t, root, func(string) bool { return true }, func(rel string, body []byte) {
			checkSource(dir+"/"+rel, body)
		})
	}
	if inspected < 40 {
		t.Fatalf("inspected only %d live template/current-state source files", inspected)
	}

	root, _ := syncedProject(t, crefYAML, nil)
	for _, path := range []string{"docs/working-with-awf.md", "docs/config-reference.md"} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range bannedGuidance {
			if strings.Contains(string(body), banned) {
				t.Errorf("rendered %s presents retired artifact configuration or capability %q", path, banned)
			}
		}
	}
	configReference, err := os.ReadFile(filepath.Join(root, "docs", "config-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"skills", "agents", "docs", "targets", "docsDir", "sidecar.local"} {
		if strings.Contains(string(configReference), "\n| `"+key+"` |") {
			t.Errorf("rendered config reference presents retired key %s", key)
		}
	}
}

// TestTemplateSourceResidue scans every embedded template source - all
// branches of every conditional, which no render-based sweep can cover - and
// fails on a concrete awf ADR citation or on a repo-identity literal outside
// the explicit exemption list (ADR-0082).
// invariant: rendering/templates:template-source-residue (TestTemplateSourceResidue)
func TestTemplateSourceResidue(t *testing.T) {
	// The marker sits on the assertion rather than on the var it guards, so the
	// proof site contains the check that proves the ADR-0131 invariant.
	// invariant: rendering/sync-and-drift:residue-exemptions-pinned-three (TestTemplateSourceResidue)
	if len(identityExempt) != 5 ||
		!identityExempt["bootstrap/awf-bootstrap.sh.tmpl"] ||
		!identityExempt["bootstrap/awf-upgrade.sh.tmpl"] ||
		!identityExempt["agents-doc/AGENTS.md.tmpl"] ||
		!identityExempt["docs/pi-runtime-reference.md.tmpl"] ||
		!identityExempt["pi/awf-subagents/index.ts.tmpl"] {
		t.Error("identity-exemption list must name exactly the bootstrap, upgrade, agents-doc, Pi reference, and pi-tools prerequisite templates")
	}
	used := map[string]bool{}
	err := fs.WalkDir(templates.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			return err
		}
		src := string(b)
		if m := residueADRRe.FindString(src); m != "" {
			t.Errorf("%s cites %s - decision rationale lives in the decisions directory, never in shipped templates (ADR-0082)", path, m)
		}
		for _, lit := range identityLiterals {
			if !strings.Contains(src, lit) {
				continue
			}
			if identityExempt[path] {
				used[path] = true
			} else {
				t.Errorf("%s carries repo-identity literal %q outside the exemption list (ADR-0082)", path, lit)
			}
		}
		if lit := repositoryLocalInstruction(src); lit != "" {
			t.Errorf("%s instructs adopters to run repository-local tooling %q", path, lit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path := range identityExempt {
		if !used[path] {
			t.Errorf("stale identity exemption %q - the template no longer carries a repo-identity literal; remove the entry via a successor ADR", path)
		}
	}
}

func TestRepositoryLocalInstructionDetection(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{name: "portable", src: "run ./awf audit", want: ""},
		{name: "repository runner", src: "run ./x custom-check", want: "./x "},
		{name: "repository command", src: "invoke cmd/repoaudit", want: "cmd/repoaudit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := repositoryLocalInstruction(tc.src); got != tc.want {
				t.Fatalf("repositoryLocalInstruction(%q) = %q, want %q", tc.src, got, tc.want)
			}
		})
	}
}

// collectStrings walks an any-typed catalog Data value and appends every
// string it holds (map values and list elements, recursively) to out. Any
// other composite type is reported instead of silently dropped - a Data entry
// written as []string or map[string]string would otherwise escape the scan
// with no signal, recreating the unscanned-surface class this guard closes.
func collectStrings(t *testing.T, site string, v any, out *[]string) {
	t.Helper()
	switch x := v.(type) {
	case nil, bool, int, float64:
		// Non-prose scalars: nothing to scan.
	case string:
		*out = append(*out, x)
	case map[string]any:
		for _, mv := range x {
			collectStrings(t, site, mv, out)
		}
	case []any:
		for _, e := range x {
			collectStrings(t, site, e, out)
		}
	default:
		t.Errorf("%s carries a Data value of unexpected type %T - use map[string]any/[]any/scalars so the residue scan sees every string", site, v)
	}
}

// TestCatalogDataResidue extends the ADR-0082 residue rule to the catalog's
// shipped prose: every Data string of every skill, agent, doc, and the domain
// doc, each doc's Title/Desc, and every var descriptor's Description, Default,
// and Options render into adopter artifacts or adopter-facing prompts exactly
// like template source does, so a concrete awf ADR citation or a repo-identity
// literal there is the same leak the templates scan catches (a citation
// resolves to nothing - or to the adopter's own unrelated ADR - in every
// corpus but awf's). Descriptors are scanned here directly rather than
// deferred to configspec-description-residue, which reads VarEntries and so
// skips the routing-target descriptors and never sees Default/Options. No
// exemptions: unlike the bootstrap templates, no catalog string references
// awf-the-product. An ordinary gate test, deliberately slug-free: ADR-0082
// owns the principle and this is its enforcement catching up with the
// catalog's move to Go (ADR-0060 postdates its scan scope).
// invariant: rendering/catalog-and-targets:catalog-defaults-generic-denylist (TestCatalogDataResidue)
func TestCatalogDataResidue(t *testing.T) {
	cat := catalog.Standard
	check := func(site string, strs []string) {
		t.Helper()
		for _, s := range strs {
			if m := residueADRRe.FindString(s); m != "" {
				t.Errorf("%s cites %s - decision rationale lives in the decisions directory, never in shipped catalog prose (ADR-0082)", site, m)
			}
			for _, lit := range identityLiterals {
				if strings.Contains(s, lit) {
					t.Errorf("%s carries repo-identity literal %q (ADR-0082)", site, lit)
				}
			}
			if lit := repositoryLocalInstruction(s); lit != "" {
				t.Errorf("%s instructs adopters to run repository-local tooling %q", site, lit)
			}
		}
	}
	for name, spec := range cat.Skills {
		var strs []string
		collectStrings(t, "skill "+name, spec.Data, &strs)
		check("skill "+name, strs)
	}
	for name, spec := range cat.Agents {
		var strs []string
		collectStrings(t, "agent "+name, spec.Data, &strs)
		check("agent "+name, strs)
	}
	var strs []string
	collectStrings(t, "domainDoc", cat.DomainDoc.Data, &strs)
	check("domainDoc", strs)
	for name, d := range cat.Docs {
		var strs []string
		collectStrings(t, "doc "+name, d.Data, &strs)
		strs = append(strs, d.Title, d.Desc)
		check("doc "+name, strs)
	}
	for _, v := range cat.Vars {
		check("var "+v.Key, append([]string{v.Description, v.Default}, v.Options...))
	}
}
