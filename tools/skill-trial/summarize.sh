#!/usr/bin/env bash
# Summarize one task's stream-json transcript as Markdown.
# Usage: tools/skill-trial/summarize.sh <results/<label>/<task> directory>
set -euo pipefail

dir="$1"
t="$dir/transcript.jsonl"
[ -s "$t" ] || { echo "skill-trial: empty transcript: $t" >&2; exit 1; }

echo "# $(basename "$dir")"
echo
echo "Exit status: $(cat "$dir/exit-status" 2>/dev/null || echo unknown)"
echo
echo "Skills available at start:"
jq -r 'select(.type == "system" and .subtype == "init") |
  "- awf: \([.slash_commands[] | select(startswith("awf-"))] | join(", "))\n- agentic-skills: \([.slash_commands[] | select(startswith("agentic-skills:"))] | length) skills\n- model: \(.model), permission mode: \(.permissionMode)"' "$t"
echo
echo "## Result"
echo
jq -r 'select(.type == "result") |
  "- terminal_reason: \(.terminal_reason // .subtype)\n- turns: \(.num_turns)\n- duration_s: \((.duration_ms // 0) / 1000 | floor)\n- cost_usd: \(.total_cost_usd)\n- permission_denials: \(.permission_denials | length)"' "$t"
jq -r 'select(.type == "result") | .permission_denials[]? | "  - denied \(.tool_name): \(.tool_input | tostring | .[0:160])"' "$t"
echo
echo "## Tool calls"
echo
echo "Indented calls ran inside a subagent."
echo
jq -r 'select(.type == "assistant") | . as $m | .message.content[]? | select(.type == "tool_use") |
  (if $m.parent_tool_use_id then "    " else "" end) + "- " + .name + ": " +
  ((.input.skill // .input.command // .input.file_path // .input.pattern //
    ((.input.subagent_type // "") + " " + (.input.description // "")) // "") | tostring
   | gsub("\n"; " ⏎ ") | .[0:200])' "$t"
echo
echo "## Final message"
echo
jq -r 'select(.type == "result") | .result // ""' "$t"
