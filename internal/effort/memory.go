package effort

func memorySkeleton() []byte {
	return []byte("## Brief\n\nDescribe the intended outcome and link durable artifacts.\n\n" +
		"## Checkpoint\n\nRecord the current phase, completed work, verification, next concrete action, and blockers.\n\n" +
		"## Decision log\n\nRecord settled decisions with user or autonomous provenance and required `Record:` evidence.\n\n" +
		"## Observations\n\nRecord friction, surprises, near-misses, and recurring lessons.\n\n" +
		"## Handoff log\n\nRecord actual session boundaries and the repository state from which to resume.\n")
}
