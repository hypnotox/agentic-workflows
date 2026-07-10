package schedule

import (
	"strings"
	"testing"
	"time"

	"example.com/sundial/internal/almanac"
)

func TestWeekHasSevenRows(t *testing.T) {
	out := Week(almanac.Location{Latitude: 52.5, Longitude: 13.4},
		time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC))
	if got := strings.Count(out, "\n"); got != 7 {
		t.Errorf("Week rendered %d rows, want 7", got)
	}
}
