package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCommandStagesUseIndependentDeadlines(t *testing.T) {
	first, cancelFirst := newGitCommandContext()
	second, cancelSecond := newGitCommandContext()
	defer cancelSecond()
	firstDeadline, firstOK := first.Deadline()
	secondDeadline, secondOK := second.Deadline()
	if !firstOK || !secondOK {
		t.Fatalf("stage deadlines present = %v, %v", firstOK, secondOK)
	}
	for name, deadline := range map[string]time.Time{"first": firstDeadline, "second": secondDeadline} {
		remaining := time.Until(deadline)
		if remaining < gitCommandTimeout-time.Second || remaining > gitCommandTimeout {
			t.Fatalf("%s stage deadline remaining = %v, want approximately %v", name, remaining, gitCommandTimeout)
		}
	}
	cancelFirst()
	if !errors.Is(first.Err(), context.Canceled) || second.Err() != nil {
		t.Fatalf("stage cancellation leaked: first=%v second=%v", first.Err(), second.Err())
	}

}
