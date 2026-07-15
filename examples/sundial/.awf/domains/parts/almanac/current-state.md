The almanac domain is one package, `internal/almanac`, implementing the cosine
day-length model ADR-0001 adopted: clamped latitude, polar-safe collapse, solar
noon shifted four minutes per degree of longitude. Accuracy is minutes, not
seconds: a deliberate ceiling. Anything touching the declination term or the
clamp must keep `almanac-clamped-latitude` backed.
