package effort

func memorySkeleton(slug string) []byte {
	return []byte("Effort: " + slug + "\nPhase: Not started.\nNext: Record the next concrete action.\nUpdated: Not yet updated.\n\n" +
		"## Brief\n\nDescribe the intended outcome and link the effort's durable artifacts (ADR, plan, worktree, branch) as they come to exist; a resuming session reads this first.\n\n" +
		"## Decision log\n\nAppend one entry per settled decision: \"- D<n> <date> <phase> (user|autonomous) <decision>. Why: <one line>.\" A user entry adds an indented \"Record:\" block quoting the user's load-bearing wording verbatim; reviewers check artifacts against user entries. Full rules: the workflow doc's working-memory section.\n\n" +
		"## Observations\n\nAppend friction, surprises, near-misses, and recurrences when they happen: \"- <date> <phase> <observation>.\" The retrospective reads this log as its primary input.\n\n" +
		"## Handoff log\n\nNo handoffs recorded.\n")
}
