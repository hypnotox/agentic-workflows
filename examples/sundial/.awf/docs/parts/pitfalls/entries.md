## Entries

- **`time.Now()` in tests.** The sun table depends on the date; a test that formats
  "today" goes red twice a year at the solstices. Fix the date with `time.Date`.
- **Longitude sign confusion.** East is positive; a flipped sign shifts solar noon
  by minutes per degree and looks like a model bug (it isn't — check the input
  first).
