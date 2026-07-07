package project

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/audit"
)

// awfPlaceholderRE matches a well-formed sandbox placeholder {{=awf:identifier}}
// (ADR-0057). The awf: namespace avoids the Mustache {{=…=}} set-delimiter
// prefix overlap and is not valid Go/Jinja/Mustache.
var awfPlaceholderRE = regexp.MustCompile(`\{\{=awf:([A-Za-z][A-Za-z0-9]*)\}\}`)

// awfResidualRE catches malformed near-misses the strict form skips (empty
// identifier, interior whitespace, hyphen). Any survivor after substitution is a
// hard error rather than published noise.
var awfResidualRE = regexp.MustCompile(`\{\{=\s*awf`)

// awfEscapeRE matches a backslash-escaped opener `\{{=…awf` — the escape target
// mirrors the residual guard's `\s*awf` scope so it neutralises both passes
// (ADR-0058). The capture group is vestigial (the func replace uses the whole match).
var awfEscapeRE = regexp.MustCompile(`\\\{\{=\s*awf`)

// awfEscSentinel stands in for a `{{=` opener that was backslash-escaped, hiding
// it from the substitution and residual passes; restored to a bare `{{=` after.
// NUL cannot occur in markdown, so it never collides with real content and is
// fully restored before the body leaves substitutePlaceholders.
const awfEscSentinel = "\x00awf-esc\x00"

// placeholderRegistry builds the available {{=awf:key}} values from the resolved
// config/render context. A key is present only when its value is non-empty
// (ADR-0057): an empty value makes the key "not available", so a part using it
// hard-errors instead of rendering nothing.
func (p *Project) placeholderRegistry() (map[string]string, error) {
	reg := map[string]string{}
	put := func(k, v string) {
		if v != "" {
			reg[k] = v
		}
	}
	put("commitScopeList", p.commitScopesDisplay())
	put("commitScopeTable", p.commitScopeTable())
	put("commitScopeSentence", p.commitScopeSentence())
	put("invariantMarkerSentence", p.invariantMarkerSentence())
	put("invariantMarkerTable", p.invariantMarkerTable())
	put("prefix", p.Cfg.Prefix)
	if v, ok := p.Cfg.Vars["gateCmd"].(string); ok {
		put("gateCmd", v)
	}
	if v, ok := p.Cfg.Vars["checkCmd"].(string); ok {
		put("checkCmd", v)
	}
	for k, v := range reg {
		if awfResidualRE.MatchString(v) {
			// A registry value must never itself carry the token, else the
			// residual guard would fire on awf-produced text (ADR-0058).
			// invariant: placeholder-value-token-free
			return nil, fmt.Errorf("registry value for key %q contains a {{=awf token", k)
		}
	}
	return reg, nil
}

// commitScopeTable renders the allowed scopes as a markdown name|meaning table,
// or "" when scopes are accept-any (ADR-0056 meanings, ADR-0057 consumer).
func (p *Project) commitScopeTable() string {
	scopes := audit.Resolve(p.Cfg.Audit).AllowedScopes
	if len(scopes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("| scope | use it for |\n|---|---|\n")
	for _, s := range scopes {
		b.WriteString("| `" + s.Name + "` | " + s.Meaning + " |\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// commitScopeSentence renders a self-contained sentence naming the allowed
// scopes, or "" when scopes are accept-any.
func (p *Project) commitScopeSentence() string {
	list := p.commitScopesDisplay()
	if list == "" {
		return ""
	}
	return "The allowed commit scopes are " + list + "."
}

// invariantMarkerSentence renders a self-contained sentence stating the invariant
// backing-comment markers by file type, or "" when no sources are configured
// (ADR-0064, the commitScopeSentence analog).
func (p *Project) invariantMarkerSentence() string {
	m := p.invariantMarkersDisplay()
	if m == "" {
		return ""
	}
	return "Its marker follows the file's type: " + m + "; the marker comment must open its line (indentation aside)."
}

// invariantMarkerTable renders the glob→marker mapping as a markdown table, or ""
// when no sources are configured (ADR-0064, the commitScopeTable analog).
func (p *Project) invariantMarkerTable() string {
	if p.Cfg.Invariants == nil || len(p.Cfg.Invariants.Sources) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("| files | marker |\n|---|---|\n")
	for _, s := range p.Cfg.Invariants.Sources {
		globs := make([]string, len(s.Globs))
		for j, g := range s.Globs {
			globs[j] = "`" + g + "`"
		}
		b.WriteString("| " + strings.Join(globs, ", ") + " | `" + s.Marker + "` |\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// substitutePlaceholders replaces every {{=awf:key}} in a raw convention-part
// body with its registry value. An unknown or empty-valued key, or any residual
// {{=awf token surviving substitution, is a hard error (ADR-0057).
// invariant: part-placeholder-sandboxed
func (p *Project) substitutePlaceholders(partName, body string, reg map[string]string) (string, error) {
	if !strings.Contains(body, "{{=") {
		return body, nil
	}
	// Protect \{{=…awf escapes: consume the backslash, stand the {{= behind a
	// sentinel so neither pass sees it; the \s*awf tail stays in-body. Restored
	// to a bare {{= after both passes. invariant: escaped-placeholder-literal
	body = awfEscapeRE.ReplaceAllStringFunc(body, func(m string) string {
		rest := m[1:]                    // drop leading backslash: {{= + ws + awf
		return awfEscSentinel + rest[3:] // hide {{=, keep ws + awf
	})
	var subErr error
	out := awfPlaceholderRE.ReplaceAllStringFunc(body, func(m string) string {
		key := awfPlaceholderRE.FindStringSubmatch(m)[1]
		v, ok := reg[key]
		if !ok {
			if subErr == nil {
				subErr = fmt.Errorf("%s: unknown or empty placeholder {{=awf:%s}}; available: %s", partName, key, availableKeys(reg))
			}
			return m
		}
		return v
	})
	if subErr != nil {
		return "", subErr
	}
	if awfResidualRE.MatchString(out) {
		return "", fmt.Errorf("%s: malformed awf placeholder (residual {{=awf); available: %s", partName, availableKeys(reg))
	}
	return strings.ReplaceAll(out, awfEscSentinel, "{{="), nil
}

// availableKeys returns the sorted registry keys for an error message.
func availableKeys(reg map[string]string) string {
	keys := make([]string, 0, len(reg))
	for k := range reg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
