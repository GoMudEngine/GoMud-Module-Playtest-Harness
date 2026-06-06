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

Verification is **agent-judged** from observed `output`, `gmcp`, and `beacon`
events — there is no formal assertion engine. Write `verify` so the agent can
tell from what it sees whether the goal succeeded.

Prefer structured state over text matching where possible — it is far less
brittle:

- **`gmcp` events** carry game state (`Char.Vitals`, `Room.Info`, …).
- **`beacon` events** (`{"type":"beacon","event":"Round","data":{...}}`) are
  emitted by the `playtest` module each round with
  `{round, hp, hp_max, sp, sp_max, room_id}`. A `verify` can reference these
  (e.g. "the beacon `room_id` became X", "`hp` increased between beacons",
  "survived N rounds without `hp` reaching 0"). Beacons also give the agent a
  reliable per-round tick to pace on.
