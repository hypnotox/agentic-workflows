package filepublication

import "os"

// MoveNoReplaceAt atomically moves one entry to an absent sibling destination
// relative to anchor. Names must contain only the final path component.
func MoveNoReplaceAt(anchor *os.File, fromName, toName string) error {
	return publishNoReplaceAt(anchor, fromName, toName)
}
