package migrate

// Generation 24, explorer-grounding-closure, has no Apply function of its own:
// ADR-0179 paired exploring with the explorer agent and brainstorming with the
// grounding-checker agent, and closing an enabled set over a new structural edge
// is exactly what applyCloseEnabledSet already does. The generation exists so an
// adopter who already enables either dispatching skill gains its paired agent on
// upgrade rather than failing project open on the next gated command.
