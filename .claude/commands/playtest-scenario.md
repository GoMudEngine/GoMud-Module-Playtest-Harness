---
description: Run a multi-agent (party / adversarial / parallel / scenario) playtest
argument-hint: <scenario-name>
---

# /playtest-scenario `<scenario-name>`

The reference conductor for **multi-agent** runs. It reads a scenario file, spawns
one independent agent per roster entry, coordinates them via the game + a small
shared blackboard, and writes a combined report. Auto-discovered from the repo —
no install. (Single-agent runs still use `/playtest`.)

> ⚠️ **Cost:** each roster agent is an independent LLM loop. **N agents cost
> roughly N× a single `/playtest` run** in tokens and local processing. Start with
> 2 agents, watch your usage rate, and keep rosters small. The server also caps AI
> clients at `Network.AI.MaxConnections` (default 20).

## 1. Load and check the scenario
- The scenario file is `framework/scenarios/<scenario-name>.yaml` (or an
  `examples/<...>.yaml`). Get its machine-readable plan:
  ```sh
  go run ./cmd/ptorch scenario plan framework/scenarios/<scenario-name>.yaml
  ```
  This emits JSON: `name`, `mode`, `max_connections`, `roster` (id/role/target),
  `group_goals`, `requires`, and `warnings`. If the command exits non-zero, the
  file is invalid — show the error and stop.
- **Surface every `warnings` entry to the user** (over-limit roster, COST). If the
  roster exceeds `max_connections`, stop and tell the user to raise
  `Network.AI.MaxConnections` (or lower the roster) before continuing.
- **Surface `requires` as preconditions to confirm** — the conductor does NOT
  change server config. If `requires.permadeath`/`death_protection` matter for the
  run (e.g., a lethal scenario), tell the user to set them on the server first.
  Where detectable in-game (e.g., the status panel showing Lives implies permadeath
  is on), note any mismatch.

## 2. Seed the blackboard
```sh
RUN="<scenario-name>-<date>"      # date passed in by you; do not invent timestamps in code
BB=".playtest/$RUN/blackboard.json"
go run ./cmd/ptorch bb init "$BB" --run "$RUN" --ids "<comma-separated roster ids>"
```

## 3. Spawn one agent per roster entry (background, independent)
For each roster entry, dispatch a **background subagent** whose instructions are
`framework/agent-runner.md`, parameterized with: that entry's `id`, `role`,
`target`, the relevant `group_goals` + per-agent `goals` + any `choreography`
lines naming it, the blackboard path `$BB`, and a private bridge dir
`.playtest/$RUN/<id>/`. Each agent connects, creates/logs in its character, and
marks itself ready.

(Other agent runtimes can spawn OS processes instead — the scenario file +
blackboard CLI are the engine-agnostic contract; subagents are just the reference.)

## 4. Readiness barrier
Wait for all agents to be present, then start the run:
```sh
until go run ./cmd/ptorch bb allready "$BB"; do sleep 1; done   # exit 0 = all ready
go run ./cmd/ptorch bb phase "$BB" --set running
```

## 5. Let agents run; wait for completion
Agents now play their assignments, interacting in-game and via signals. Wait for
all background subagents to finish (each writes its per-agent report and appends
its findings to the blackboard), then:
```sh
go run ./cmd/ptorch bb phase "$BB" --set done
```

## 6. Aggregate the combined report
Read the final blackboard and each per-agent report:
```sh
go run ./cmd/ptorch bb dump "$BB"
```
Write the combined report per `framework/multi-agent-report-format.md` to
`framework/reports/<date>-<scenario-name>.md`: scenario summary, group-goal
results with cross-agent evidence, per-agent outcomes, and the merged/deduped
findings (already deduped per agent+title on the blackboard).

## 7. Clean up
Quit any still-running `mudagent`s (each agent does this on finish; clean up
strays as in `.claude/commands/playtest.md` step 7). Report the combined-report
path to the user.
