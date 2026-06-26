package project

import "fmt"

// Target places adapter (tool-specific) artifacts for one runtime. Neutral
// artifacts (AGENTS.md, docs, domains, hooks) are not target-scoped (ADR-0016).
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

// claudeTarget is the sole built-in target. Adding a second runtime is a new
// Target value plus its placement, not a render-loop change.
var claudeTarget = Target{
	Name:       "claude",
	SkillDir:   ".claude/skills",
	AgentDir:   ".claude/agents",
	BridgeFile: "CLAUDE.md",
}
