# Harness Engine

Core event processing, checkpoint scheduling, and gate logic lives here.

Current implementation:

- `bus.go`: typed JSONL event bus writer for `.doombox/events.jsonl`
- `checkpoint_trigger.go`: action-cluster checkpoint trigger (`checkpoint_every_actions`)
- `checkpoint_store.go`: checkpoint snapshot writer (`.doombox/checkpoints/*.json`)
- `immediate_trigger.go`: immediate checkpoint trigger policy (test fail, risky touch, large diff, pre-commit/push)
- `permission_denials.go`: append-only permission denial logger (`.doombox/permission-denials.jsonl`)
- `tool_classification.go`: tool invocation risk classifier (`safe`, `justify`, `block`)
- `precommit_gate.go`: harness-managed pre-commit gate evaluator
- `test_discipline.go`: fast/integration test batch execution helpers
- `prepush_gate.go`: harness-managed pre-push gate evaluator
- `rubric.go`: trajectory rubric scoring over events/checkpoints
- Policy rules come from `.doombox/policy.json`
- Policy supports sensitive path prefixes
- Policy supports risky path prefixes
- Policy supports blocked command prefixes
- Policy supports review-required command prefixes
- `bus_test.go`: unit tests for schema-safe emission and typed helpers
- `checkpoint_trigger_test.go`: unit tests for periodic action-based checkpoints
- `checkpoint_store_test.go`: unit tests for checkpoint persistence and required fields
- `immediate_trigger_test.go`: unit tests for immediate risk/event triggers
- `permission_denials_test.go`: unit tests for denial logging and validation
- `tool_classification_test.go`: unit tests for command/path risk classification
- `precommit_gate_test.go`: unit tests for gate decisions and file classification
- `test_discipline_test.go`: unit tests for fast/integration test execution
- `prepush_gate_test.go`: unit tests for pre-push integration gate
- `rubric_test.go`: unit tests for trajectory scoring and loaders

Supported typed event helpers:

- session lifecycle (`session_start`, `session_end`)
- tool/edit/test signals (`tool_invocation`, `edit_cluster`, `test_result`)
- supervisor/gate signals (`checkpoint_due`, `gate_decision`, `permission_denied`)
