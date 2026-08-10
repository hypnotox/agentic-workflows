package migrate

// globalTopicPathOwnershipGeneration activates combined applies: global and
// paths topic metadata. Metadata is already valid input, so upgrade only
// advances the schema stamp and deliberately rewrites no topic files.
const globalTopicPathOwnershipGeneration = 41

func applyGlobalTopicPathOwnership(_ string, _ *Changes) error { return nil }
