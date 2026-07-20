// Package almanac approximates sun events from a location and a date. The
// cosine day-length model (ADR-0001) trades accuracy for zero dependencies:
// good enough to plan a walk, wrong for navigation.
package almanac

import (
	"math"
	"time"
)

// A Location is a point on Earth in decimal degrees.
type Location struct {
	Latitude  float64
	Longitude float64
}

// Day describes the sun events of one calendar day at a location.
type Day struct {
	Date    time.Time
	Sunrise time.Time
	Sunset  time.Time
}

// Sun returns the approximate sunrise and sunset for the location on the
// given date. Polar day and night collapse to a full- or zero-length day
// rather than an error (ADR-0001).
func Sun(loc Location, date time.Time) Day {
	daylight := dayLength(clampLatitude(loc.Latitude), date.YearDay())
	noon := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, date.Location()).
		Add(-time.Duration(loc.Longitude * 4 * float64(time.Minute)))
	half := daylight / 2
	return Day{Date: date, Sunrise: noon.Add(-half), Sunset: noon.Add(half)}
}

// clampLatitude bounds latitude to [-90, 90] so the model never leaves the
// domain of math.Acos; out-of-range input degrades to the pole (ADR-0001).
func clampLatitude(lat float64) float64 {
	return math.Max(-90, math.Min(90, lat))
}

// dayLength approximates daylight duration via the cosine model: the
// day/night terminator angle follows the seasonal solar declination.
func dayLength(lat float64, yearDay int) time.Duration {
	decl := -23.44 * math.Cos(2*math.Pi*float64(yearDay+10)/365)
	x := -math.Tan(lat*math.Pi/180) * math.Tan(decl*math.Pi/180)
	switch {
	case x <= -1:
		return 24 * time.Hour
	case x >= 1:
		return 0
	}
	return time.Duration(24 * math.Acos(x) / math.Pi * float64(time.Hour))
}
