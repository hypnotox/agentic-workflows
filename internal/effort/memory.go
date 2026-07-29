package effort

func memorySkeleton(slug string) []byte {
	return []byte("Effort: " + slug + "\nPhase: Not started.\nNext: Record the next concrete action.\nUpdated: Not yet updated.\n\n## Brief\n\nDescribe the intended outcome.\n\n## Decisions\n\nRecord settled implementation decisions.\n\n## Handoff log\n\nNo handoffs recorded.\n")
}
