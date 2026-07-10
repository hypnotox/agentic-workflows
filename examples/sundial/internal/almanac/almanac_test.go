package almanac

import (
	"testing"
	"time"
)

func TestClampLatitude(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{in: 91, want: 90},
		{in: -120, want: -90},
		{in: 52.5, want: 52.5},
	} {
		if got := clampLatitude(tc.in); got != tc.want {
			t.Errorf("clampLatitude(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSunPolarNightCollapses(t *testing.T) {
	winter := time.Date(2026, time.December, 21, 0, 0, 0, 0, time.UTC)
	day := Sun(Location{Latitude: 89}, winter)
	if !day.Sunrise.Equal(day.Sunset) {
		t.Errorf("polar night must collapse to a zero-length day, got %v-%v", day.Sunrise, day.Sunset)
	}
}
