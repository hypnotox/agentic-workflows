The cli domain is `cmd/sundial`: decimal-degrees argument parsing (ADR-0002) and
the week-table print. It owns every exit code (2 for usage) and no model logic;
ADR-0003's proposed cache would slot between it and the almanac.
