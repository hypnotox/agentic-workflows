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

// placeholderRegistry builds the available {{=awf:key}} values from the resolved
// config/render context. A key is present only when its value is non-empty
// (ADR-0057): an empty value makes the key "not available", so a part using it
// hard-errors instead of rendering nothing.
func (p *Project) placeholderRegistry() map[string]string {
	reg := map[string]string{}
	put := func(k, v string) {
		if v != "" {
			reg[k] = v
		}
	}
	put("commitScopeList", p.commitScopesDisplay())
	put("commitScopeTable", p.commitScopeTable())
	put("commitScopeSentence", p.commitScopeSentence())
	put("prefix", p.Cfg.Prefix)
	if v, ok := p.Cfg.Vars["gateCmd"].(string); ok {
		put("gateCmd", v)
	}
	if v, ok := p.Cfg.Vars["checkCmd"].(string); ok {
		put("checkCmd", v)
	}
	return reg
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

// substitutePlaceholders replaces every {{=awf:key}} in a raw convention-part
// body with its registry value. An unknown or empty-valued key, or any residual
// {{=awf token surviving substitution, is a hard error (ADR-0057).
// invariant: part-placeholder-sandboxed
func (p *Project) substitutePlaceholders(partName, body string, reg map[string]string) (string, error) {
	if !strings.Contains(body, "{{=") {
		return body, nil
	}
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
	return out, nil
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
