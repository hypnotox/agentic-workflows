## Data flow

`main` → `schedule.Week(location, today)` → seven `almanac.Sun` calls → formatted
table on stdout. Errors exist only at the argument boundary; the model itself is
total: polar day and night collapse to full- or zero-length days (ADR-0001).
