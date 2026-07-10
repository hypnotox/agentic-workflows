// Command sundial prints this week's approximate sunrise and sunset times
// for a location given as decimal degrees.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"example.com/sundial/internal/almanac"
	"example.com/sundial/internal/schedule"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: sundial <latitude> <longitude>")
		os.Exit(2)
	}
	lat, latErr := strconv.ParseFloat(os.Args[1], 64)
	lon, lonErr := strconv.ParseFloat(os.Args[2], 64)
	if latErr != nil || lonErr != nil {
		fmt.Fprintln(os.Stderr, "sundial: latitude and longitude must be decimal degrees")
		os.Exit(2)
	}
	fmt.Print(schedule.Week(almanac.Location{Latitude: lat, Longitude: lon}, time.Now()))
}
