## Surfaces

- **CLI boundary:** wrong usage or non-numeric coordinates: exit 2 with a usage
  line on stderr.
- **Model output:** implausible times: check the latitude clamp and the
  declination term in `internal/almanac` before suspecting formatting.
