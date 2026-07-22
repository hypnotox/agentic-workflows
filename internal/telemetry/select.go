package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ParseSelectorTime accepts the canonical CLI timestamp forms.
func ParseSelectorTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("timestamp must be RFC3339: %w", err)
	}
	return parsed, nil
}

// ValidateSelector validates closed values and the inclusive/exclusive window.
func ValidateSelector(selector Selector) error {
	if selector.EffortID != nil {
		if err := validatePathIdentifier("effortId", *selector.EffortID); err != nil {
			return err
		}
	}
	if selector.SessionID != nil {
		if err := validatePathIdentifier("sessionId", *selector.SessionID); err != nil {
			return err
		}
	}
	if selector.Phase != nil {
		if *selector.Phase == "" || !descriptorContains(descriptor.Vocabularies["phases"], *selector.Phase) {
			return fmt.Errorf("unknown telemetry phase %q", *selector.Phase)
		}
	}
	if selector.Since != nil && selector.Until != nil && !selector.Since.Before(*selector.Until) {
		return errors.New("selector since must be before until")
	}
	return nil
}

func descriptorContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func selectEffortEvents(read EffortRead, selector Selector) ([]EventEnvelope, map[string]bool, error) {
	if err := ValidateSelector(selector); err != nil {
		return nil, nil, err
	}
	if selector.EffortID != nil && read.Metadata.EffortID != *selector.EffortID {
		return []EventEnvelope{}, map[string]bool{}, nil
	}
	phases := projectEventPhases(read.Events)
	selected := make([]EventEnvelope, 0, len(read.Events))
	selectedIDs := make(map[string]bool)
	for _, event := range read.Events {
		if selector.SessionID != nil && event.SessionID != *selector.SessionID {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err != nil { // coverage-ignore: validated protocol events always carry RFC3339Nano timestamps
			continue
		}
		if selector.Since != nil && timestamp.Before(*selector.Since) {
			continue
		}
		if selector.Until != nil && !timestamp.Before(*selector.Until) {
			continue
		}
		if selector.Phase != nil && !phases[event.EventID][*selector.Phase] {
			continue
		}
		selected = append(selected, event)
		selectedIDs[event.EventID] = true
	}
	return selected, selectedIDs, nil
}

func projectEventPhases(events []EventEnvelope) map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(events))
	byID := make(map[string]EventEnvelope, len(events))
	for _, event := range events {
		result[event.EventID] = map[string]bool{}
		byID[event.EventID] = event
		switch event.Kind {
		case "phase_started":
			var payload PhaseStartedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				result[event.EventID][string(payload.Phase)] = true
			}
		case "phase_finished":
			var payload PhaseFinishedPayload
			if json.Unmarshal(event.Payload, &payload) == nil {
				result[event.EventID][string(payload.Phase)] = true
			}
		case "usage_observed":
			var payload UsageObservedPayload
			if json.Unmarshal(event.Payload, &payload) == nil && payload.Phase != "" {
				result[event.EventID][string(payload.Phase)] = true
			}
		}
	}
	order, _ := BuildCausalOrder(events)
	finishedStarts := map[string]bool{}
	for _, finish := range events {
		if finish.Kind != "phase_finished" {
			continue
		}
		var payload PhaseFinishedPayload
		if json.Unmarshal(finish.Payload, &payload) != nil {
			continue
		}
		if _, exists := byID[payload.StartEventID]; !exists {
			continue
		}
		finishedStarts[payload.StartEventID] = true
		for _, event := range events {
			if (event.EventID == payload.StartEventID || order.HappensBefore(payload.StartEventID, event.EventID)) && (event.EventID == finish.EventID || order.HappensBefore(event.EventID, finish.EventID)) {
				result[event.EventID][string(payload.Phase)] = true
			}
		}
	}
	for _, start := range events {
		if start.Kind != "phase_started" || finishedStarts[start.EventID] {
			continue
		}
		var payload PhaseStartedPayload
		if json.Unmarshal(start.Payload, &payload) != nil {
			continue
		}
		for _, event := range events {
			if event.EventID == start.EventID || order.HappensBefore(start.EventID, event.EventID) {
				result[event.EventID][string(payload.Phase)] = true
			}
		}
	}
	return result
}
