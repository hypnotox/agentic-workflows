## Recipes

- Reproduce a suspicious table with a fixed date: call `schedule.Week` from a test
  with a `time.Date` literal — never `time.Now()` — so the case is replayable.
- Bisect model vs formatting by printing `almanac.Sun` directly for one day.
