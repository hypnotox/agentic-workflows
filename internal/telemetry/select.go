package telemetry

import (
	"errors"
	"fmt"
	"time"
)

func ParseSelectorTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
func ValidateSelector(s Selector) error {
	if s.EffortID != nil && !Identifier(*s.EffortID) {
		return fmt.Errorf("invalid effort id %q", *s.EffortID)
	}
	if s.SessionID != nil && !Identifier(*s.SessionID) {
		return fmt.Errorf("invalid session id %q", *s.SessionID)
	}
	if s.Since != nil && s.Until != nil && !s.Since.Before(*s.Until) {
		return errors.New("selector since must be before until")
	}
	return nil
}
func selectObservation(o Observation, s Selector) bool {
	return (s.Since == nil || !o.Timestamp.Before(*s.Since)) && (s.Until == nil || o.Timestamp.Before(*s.Until))
}
