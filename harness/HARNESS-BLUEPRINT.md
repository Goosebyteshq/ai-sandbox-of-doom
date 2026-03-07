# Doombox Harness Blueprint (Codex First)

## Context

Doombox currently launches an agent in a high-autonomy container. The next step is to reduce manual supervision cost by adding a first-class harness that manages drift, scope discipline, safety checks, and test discipline using structured data and deterministic gates.

## What we agreed

- Keep agent execution in full-access mode inside the container.
- Move harness implementation into a top-level `harness/` directory so it can be extracted later into its own repository.
- Keep `.doombox` as project-local state:
  - create automatically when missing
  - preserve user-edited files when present
- Make supervision event-driven first:
  - checkpoint every 3 to 5 action clusters (default 4)
  - immediate checkpoints on key risk events
- Require hard commit/push checkpoints:
  - scope checks
  - generated-file checks
  - test discipline checks
  - justification for non-obvious file touches
- Add a dedicated permission-denials log for post-run policy review.
- Run with tmux surfaces for observability:
  - agent pane
  - supervisor pane
  - event/HUD pane

## Product goals

- Minimize user intervention during normal development sessions.
- Catch drift before commit, not after merge.
- Keep an auditable JSON trace for replay and evaluation.
- Support provider adapters with a shared supervisor core.
- Ship Codex first, then Gemini and Cloud.

## Runtime model

- Agent runtime:
  - Codex session in pane 1.
- Supervisor runtime:
  - event consumer, checkpoint engine, gate engine in pane 2.
- Observability runtime:
  - live event stream and top metrics in pane 3.

## Data contracts

- `.doombox/policy.json`: thresholds, risky paths, test commands, gate policy.
- `.doombox/events.jsonl`: append-only raw event stream.
- `.doombox/checkpoints/*.json`: structured checkpoint snapshots.
- `.doombox/todo.json`: open and resolved harness tasks.
- `.doombox/session-log.jsonl`: high-level lifecycle events.
- `.doombox/permission-denials.jsonl`: blocked/escalation-required actions.

## Event-driven trigger policy

- Action-based checkpoint:
  - every `checkpoint_every_actions` action clusters (default `4`).
- Immediate checkpoint:
  - test failure
  - risky file/path touch
  - diff-size threshold exceeded
  - pre-commit
  - pre-push
- Optional fallback:
  - idle timer for stale sessions
  - not primary control path

## Hard gates

- Pre-commit gate:
  - staged file scope validation
  - generated file detection
  - non-obvious file justifications present
  - required tests green after latest meaningful edits
- Pre-push gate:
  - integration tests according to policy
  - unresolved high-risk drift item blocks push

## Test discipline policy

- Run fast tests automatically after meaningful edit clusters.
- Escalate to integration tests on risky touches, large diffs, or pre-push.
- Block commit when required tests are missing or failing.

## Architecture direction

- `harness/engine`: event bus, checkpoint engine, gate engine.
- `harness/adapters`: provider-specific agent adapters.
- `harness/schemas`: JSON schema contracts.
- `harness/scripts`: tmux launch and ops scripts.
- `harness/docs`: extraction notes and migration guidance.

## Research background and references

The design is aligned with trajectory-level supervision, execution-grounded evaluation, and harness-centric architecture trends:

- OpenAI, trace grading guidance: https://platform.openai.com/docs/guides/trace-grading
- OpenAI, evaluation best practices: https://platform.openai.com/docs/guides/evaluation-best-practices
- Anthropic, Claude Code hooks: https://docs.anthropic.com/en/docs/claude-code/hooks
- SWE-bench benchmark (base): https://arxiv.org/abs/2310.06770
- Multi-SWE-bench (multilingual): https://arxiv.org/abs/2504.02605
- SWE-rebench (decontaminated pipeline eval): https://arxiv.org/abs/2505.20411
- SWE-RL (software-evolution RL): https://arxiv.org/abs/2502.18449
- SWE-smith (scaled SWE data): https://arxiv.org/abs/2504.21798
- SWE-Search (search/refinement workflows): https://arxiv.org/abs/2410.20285
- SWE-Gym (agent + verifier environment): https://arxiv.org/abs/2412.21139
- OpenHands SDK (2025): https://arxiv.org/abs/2511.03690

## Future directions

- Rubric-based trajectory scoring with LLM-as-judge over `events.jsonl`.
- Step-level tool-call safety classification before execution.
- Policy canary mode and flip analysis before policy promotion.
- Context eligibility registry and checkpoint-time compaction.
- Optional learned verifier replacing rule-only drift scoring.

## Success criteria

- Reduced out-of-scope edits per task.
- High test-before-commit rate.
- Lower edit-revert loop rate.
- Clear, queryable audit trail of checkpoints and denials.
- Minimal manual oversight in steady-state sessions.
