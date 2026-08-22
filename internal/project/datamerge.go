package project

// specializedListDataKeys excludes differently keyed, identity-aware
// transforms from generic same-key list composition.
func specializedListDataKeys(kind, artifact string) []string {
	if kind == "docs" && artifact == "glossary" {
		return []string{"standardTerms"}
	}
	return nil
}
