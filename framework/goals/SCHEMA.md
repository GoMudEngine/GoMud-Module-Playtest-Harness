# Goals File Schema

A **goals file** is YAML describing a playtest session's objectives. It is
game-agnostic: you describe *what* to attempt and *how* to judge success in
prose; the agent figures out the engine-specific commands from the engine
profile.

## Fields

- `name` (string, required) — human-readable session name.
- `summary` (string) — one line describing what this run validates.
- `goals` (list, required) — ordered objectives. Each goal has:
  - `id` (short string) — stable identifier the report references.
  - `do` (string) — what the agent should attempt, in prose.
  - `verify` (string) — how to judge success: text to look for, GMCP state to
    check, or behavior to observe.
- `notes` (list of strings, optional) — setup, admin steps, or context the agent
  should know.
- `pass_criteria` (list of strings, optional) — an overall success checklist.

## Example shape

```yaml
name: <session name>
summary: <one line>
goals:
  - id: <short-id>
    do: <what to attempt>
    verify: <how to judge success>
notes:
  - <context>
pass_criteria:
  - <overall success condition>
```

## Verification model

Verification is **agent-judged** from observed `output` and `gmcp` events — there
is no formal assertion engine. Write `verify` so the agent can tell from what it
sees whether the goal succeeded.

Phase 2 of the harness adds `Playtest.*` GMCP beacons the agent can key `verify`
on for structured, less brittle scoring; until then, prefer `verify` conditions
that reference GMCP state (e.g. a `Room.Info` change) over exact text matches
where possible.
