Ground guide-first, in order: the agent guide, then the document-map docs relevant to the touched area, then its domain docs under `{{ .layout.domainsDir }}`. Current-state documentation is what binds. Consult the recent history of the touched paths (`git log --oneline -20 <path>`) only when current state leaves what you are seeing unexplained.

For managed context calls, start bare: directories provide tier-0 orientation, while exact, staged, and range-selected files also carry tier-1 direct relationships. Request only the named facets the active lens requires, and never prescribe `--full`.
