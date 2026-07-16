package project

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

// AgentDialect names the target-native encoding for rendered agents.
type AgentDialect string

const (
	MarkdownAgentDialect AgentDialect = "markdown"
	TOMLAgentDialect     AgentDialect = "toml"
)

// ReviewDispatchStyle names how a target directs a workflow to a reviewer.
type ReviewDispatchStyle string

const (
	NativeReviewDispatch  ReviewDispatchStyle = "native"
	GenericReviewDispatch ReviewDispatchStyle = "generic"
)

// Target places adapter (tool-specific) artifacts for one runtime. Neutral
// artifacts (AGENTS.md, docs, domains) are not target-scoped (ADR-0016).
type Target struct {
	Name           string
	SkillDir       string // dir holding rendered skills, e.g. ".claude/skills"
	AgentDir       string // dir holding rendered agents, e.g. ".claude/agents"
	AgentSuffix    string // agent filename suffix, including its extension
	AgentDialect   AgentDialect
	BridgeFile     string // adapter bridge file at repo root, "" if none
	BridgeTemplate string
	ReviewStyle    ReviewDispatchStyle
}

// SkillPath is the output path for a rendered skill under this target.
func (t Target) SkillPath(prefix, name string) string {
	return fmt.Sprintf("%s/%s-%s/SKILL.md", t.SkillDir, prefix, name)
}

// AgentPath is the output path for a rendered agent under this target.
func (t Target) AgentPath(name string) string {
	suffix := t.AgentSuffix
	if suffix == "" {
		suffix = ".md"
	}
	return fmt.Sprintf("%s/%s%s", t.AgentDir, name, suffix)
}

func (t Target) agentCommentStyle() render.CommentStyle {
	if t.AgentDialect == TOMLAgentDialect {
		return render.TOMLComment
	}
	return render.HTMLComment
}

// claudeTarget and cursorTarget are the built-in adapters. Adding a runtime is a
// new Target value plus a registry entry, not a render-loop change (ADR-0037).
var claudeTarget = Target{
	Name:           "claude",
	SkillDir:       ".claude/skills",
	AgentDir:       ".claude/agents",
	AgentSuffix:    ".md",
	AgentDialect:   MarkdownAgentDialect,
	BridgeFile:     "CLAUDE.md",
	BridgeTemplate: bridgeTID,
	ReviewStyle:    NativeReviewDispatch,
}

// cursorTarget renders to Cursor's SKILL.md/subagent layout. Cursor reads
// AGENTS.md natively, so it emits no bridge file (ADR-0037).
var cursorTarget = Target{
	Name:         "cursor",
	SkillDir:     ".cursor/skills",
	AgentDir:     ".cursor/agents",
	AgentSuffix:  ".md",
	AgentDialect: MarkdownAgentDialect,
	BridgeFile:   "",
	ReviewStyle:  NativeReviewDispatch,
}

var codexTarget = Target{
	Name:         "codex",
	SkillDir:     ".agents/skills",
	AgentDir:     ".codex/agents",
	AgentSuffix:  ".toml",
	AgentDialect: TOMLAgentDialect,
	ReviewStyle:  NativeReviewDispatch,
}

var piTarget = Target{
	Name:         "pi",
	SkillDir:     ".pi/skills",
	AgentDir:     ".pi/skills",
	AgentSuffix:  ".md",
	AgentDialect: MarkdownAgentDialect,
	ReviewStyle:  GenericReviewDispatch,
}

var geminiTarget = Target{
	Name:           "gemini",
	SkillDir:       ".gemini/skills",
	AgentDir:       ".gemini/agents",
	AgentSuffix:    ".md",
	AgentDialect:   MarkdownAgentDialect,
	BridgeFile:     "GEMINI.md",
	BridgeTemplate: "gemini/GEMINI.md.tmpl",
	ReviewStyle:    NativeReviewDispatch,
}

var copilotTarget = Target{
	Name:         "copilot",
	SkillDir:     ".github/skills",
	AgentDir:     ".github/agents",
	AgentSuffix:  ".agent.md",
	AgentDialect: MarkdownAgentDialect,
	ReviewStyle:  NativeReviewDispatch,
}

// targetRegistry maps an adapter name to its Target. It is the sole enumeration
// of known adapters; resolveTargets rejects any name absent from it.
var targetRegistry = map[string]Target{
	"claude":  claudeTarget,
	"codex":   codexTarget,
	"copilot": copilotTarget,
	"cursor":  cursorTarget,
	"gemini":  geminiTarget,
	"pi":      piTarget,
}

// KnownTargets returns the known adapter names in sorted order. The bespoke
// `awf {enable,disable,list} target` path validates against this set (inv: target-cli).
// invariant: target-cli
func KnownTargets() []string {
	return slices.Sorted(maps.Keys(targetRegistry))
}

// resolveTargets maps configured adapter names to their Target values in config
// order, rejecting any unknown name (inv: targets-default-claude).
func resolveTargets(names []string) ([]Target, error) {
	out := make([]Target, 0, len(names))
	for _, n := range names {
		t, ok := targetRegistry[n]
		if !ok {
			return nil, fmt.Errorf("unknown target %q (known: %s)", n, strings.Join(KnownTargets(), ", "))
		}
		out = append(out, t)
	}
	return out, nil
}
