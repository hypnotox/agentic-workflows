package currentstate

import (
	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Universe is a passive projection of loaded ADR records, topic values, and
// source bytes retained for residual readers outside this package. It performs
// no current-state validation or transaction pairing.
type Universe struct {
	ADRs    []adr.ADR
	Topics  []topic.Topic
	Sources map[string][]byte
}

// Universe returns the passive projection of a Loaded snapshot.
func (l Loaded) Universe() Universe {
	return Universe{ADRs: l.ADRs, Topics: l.Topics.All(), Sources: l.Sources}
}
