package effort

import "time"

func memorySkeleton(slug string, createdAt time.Time) []byte {
	metadata, err := encodeMemory(MemoryMetadata{
		Effort: slug, Phase: "Not started.", Next: "Record the next concrete action.", Updated: formatMemoryTime(createdAt),
	}, memoryBody())
	if err != nil {
		panic(err)
	}
	return metadata
}

func memoryBody() []byte {
	return []byte("## Brief\n\nDescribe the intended outcome and link the effort's durable artifacts (ADR, plan, worktree, branch) as they come to exist; a resuming session reads this first.\n\n" +
		"## Decision log\n\nAppend one entry per settled decision: \"- D<n> <date> <phase> (user|autonomous) <decision>. Why: <one line>.\" A user entry whose decision changes scope, design, authority, or previously-approved output adds an indented \"Record:\" block quoting the user's load-bearing wording verbatim; other user entries need none. Reviewers check artifacts against user entries. Full rules: the workflow doc's working-memory section.\n\n" +
		"## Observations\n\nAppend friction, surprises, near-misses, and recurrences when they happen: \"- <date> <phase> <observation>.\" The retrospective reads this log as its primary input.\n\n" +
		"## Handoff log\n\nNo handoffs recorded. Append one line per session boundary; a resuming session reads this audit after the Brief.\n")
}
