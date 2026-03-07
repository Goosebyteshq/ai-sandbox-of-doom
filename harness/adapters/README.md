# Harness Adapters

Provider-specific adapters (`codex`, `gemini`, `cloud`) plug into the shared engine.

Current adapters:

- `provider.go`: shared provider adapter interface + registry
  - `codex`: primary implementation (supervisor-capable)
  - `gemini`: stub
  - `cloud` (claude alias): stub
- `mock`: deterministic scripted adapter used for harness testing without live model calls.
  - Includes fixture-run integration tests and `events.jsonl` replay tests.
