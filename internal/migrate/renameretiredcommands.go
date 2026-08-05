package migrate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"gopkg.in/yaml.v3"
)

// retiredSubcommands maps each subcommand ADR-0159 retired to its replacement.
// The keys are the only place in the production tree that spells the retired
// names, which is what the rename is migrating adopters off of.
var retiredSubcommands = map[string]string{
	"sync":        "render",
	"invariants":  "check invariants",
	"prose-gate":  "check prose",
	"memory-gate": "check memory",
	"commit-gate": "check commit",
}

// retiredCommandRe splits a var value into its first token, the whitespace after
// it, and the second token, anchored at the start so only a value whose *leading*
// token is an awf invocation can match. Everything past the second token is left
// out of the match and spliced back verbatim, so trailing arguments survive.
var retiredCommandRe = regexp.MustCompile(`^(\S+)(\s+)(sync|invariants|prose-gate|memory-gate|commit-gate)(\s|$)`)

// awfInvocation reports whether tok invokes awf itself: bare `awf`, `./awf`, or
// any path ending in `/awf` (the latter two share the suffix test). A value whose
// first token is another runner (`./x check`, `make gate`) is deliberately left
// alone: awf does not own that command's vocabulary.
func awfInvocation(tok string) bool {
	return tok == "awf" || strings.HasSuffix(tok, "/awf")
}

// rewriteRetiredCommand rewrites the retired subcommand in an `<awf-invocation>
// <subcommand>` value, preserving the invocation token, the separating
// whitespace, and every trailing argument. It reports whether it changed
// anything, so a caller only rewrites the keys that actually move.
func rewriteRetiredCommand(value string) (string, bool) {
	m := retiredCommandRe.FindStringSubmatchIndex(value)
	if m == nil {
		return value, false
	}
	if !awfInvocation(value[m[2]:m[3]]) {
		return value, false
	}
	return value[:m[6]] + retiredSubcommands[value[m[6]:m[7]]] + value[m[7]:], true
}

// applyRenameRetiredCommands ports schema 18 -> 19: ADR-0159 renamed `awf sync`
// to `awf render` and regrouped `invariants`, `prose-gate`, `memory-gate`, and
// `commit-gate` under `awf check` with no aliases, so an adopter var still
// holding `./awf prose-gate` would fail inside a hook rather than at upgrade
// time. It rewrites only a value whose leading token is an awf invocation,
// through config.SetMappingString so config.yaml serialization stays owned by
// internal/config (ADR-0026). A config absent on disk is a no-op, and a value
// naming another runner or spelling no retired subcommand is untouched, so the
// migration is idempotent and safe to replay.
func applyRenameRetiredCommands(root string, out *Changes) error {
	return editConfig(root, out, func(src []byte, planned *Changes) ([]byte, error) {
		out := src
		var doc struct {
			Vars map[string]any `yaml:"vars"`
		}
		// A config too malformed to decode falls straight through: the strict parse
		// reports it, and a migration must not turn it into a second error.
		if yaml.Unmarshal(src, &doc) == nil {
			keys := make([]string, 0, len(doc.Vars))
			for k := range doc.Vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				s, isString := doc.Vars[k].(string)
				if !isString {
					continue
				}
				next, changed := rewriteRetiredCommand(s)
				if !changed {
					continue
				}
				edited, err := config.SetMappingString(out, "vars", k, next)
				if err != nil { // coverage-ignore: SetMappingString is total apart from its own parse, and src parsed as YAML a few lines above
					return nil, err
				}
				out = edited
				planned.Add(fmt.Sprintf("rename-retired-commands: vars.%s: rewrote %q to %q", k, s, next))
			}
		}
		return out, nil
	})
}
