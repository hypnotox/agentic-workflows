package project

import "fmt"

// Target places adapter (tool-specific) artifacts for one runtime. Neutral
// artifacts (AGENTS.md, docs, domains) are not target-scoped (ADR-0016).
type Target struct {
	Name       string
	SkillDir   string // dir holding rendered skills, e.g. ".claude/skills"
	AgentDir   string // dir holding rendered agents, e.g. ".claude/agents"
	BridgeFile string // adapter bridge file at repo root, "" if none
}

// SkillPath is the output path for a rendered skill under this target.
func (t Target) SkillPath(prefix, name string) string {
	return fmt.Sprintf("%s/%s-%s/SKILL.md", t.SkillDir, prefix, name)
}

// AgentPath is the output path for a rendered agent under this target.
func (t Target) AgentPath(name string) string {
	return fmt.Sprintf("%s/%s.md", t.AgentDir, name)
}

// claudeTarget and cursorTarget are the built-in adapters. Adding a runtime is a
// new Target value plus a registry entry, not a render-loop change (ADR-0037).
var claudeTarget = Target{
	Name:       "claude",
	SkillDir:   ".claude/skills",
	AgentDir:   ".claude/agents",
	BridgeFile: "CLAUDE.md",
}

// cursorTarget renders to Cursor's SKILL.md/subagent layout. Cursor reads
// AGENTS.md natively, so it emits no bridge file (ADR-0037).
var cursorTarget = Target{
	Name:       "cursor",
	SkillDir:   ".cursor/skills",
	AgentDir:   ".cursor/agents",
	BridgeFile: "",
}

// targetRegistry maps an adapter name to its Target. It is the sole enumeration
// of known adapters; resolveTargets rejects any name absent from it.
var targetRegistry = map[string]Target{
	"claude": claudeTarget,
	"cursor": cursorTarget,
}

// resolveTargets maps configured adapter names to their Target values in config
// order, rejecting any unknown name (inv: targets-default-claude).
func resolveTargets(names []string) ([]Target, error) {
	out := make([]Target, 0, len(names))
	for _, n := range names {
		t, ok := targetRegistry[n]
		if !ok {
			return nil, fmt.Errorf("unknown target %q (known: claude, cursor)", n)
		}
		out = append(out, t)
	}
	return out, nil
}
