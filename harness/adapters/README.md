# Harness Adapters

Provider-specific adapters (`codex`, `gemini`, `cloud`) plug into the shared engine.

Current adapters:

- `mock`: deterministic scripted adapter used for harness testing without live model calls.
  - Includes fixture-run integration tests and `events.jsonl` replay tests.
