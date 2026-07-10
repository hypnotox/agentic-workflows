// Package schedule renders almanac days as a plain-text weekly table.
package schedule

import (
	"fmt"
	"strings"
	"time"

	"example.com/sundial/internal/almanac"
)

// Week renders seven days of sun events starting at from, one row per day.
func Week(loc almanac.Location, from time.Time) string {
	var b strings.Builder
	for i := 0; i < 7; i++ {
		day := almanac.Sun(loc, from.AddDate(0, 0, i))
		fmt.Fprintf(&b, "%s  rise %s  set %s\n",
			day.Date.Format("Mon 2006-01-02"),
			day.Sunrise.Format("15:04"),
			day.Sunset.Format("15:04"))
	}
	return b.String()
}
