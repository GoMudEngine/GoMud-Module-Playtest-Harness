# Scenario File Schema

A **scenario file** defines a multi-agent playtest run. It is game-agnostic —
engine-specific commands still come from `engine-profile.yaml`. Run one with the
`/playtest-scenario <name>` conductor (see `.claude/commands/playtest-scenario.md`).

## Fields

- `name` (string, required) — run name; the combined report references it.
- `mode` (string, required) — one of:
  - `party` — testers coordinate toward a shared goal (invite/accept, group play).
  - `adversarial` — independent players interacting non-cooperatively (PvP swings,
    contested resources, trade, racing). Per-agent goals may conflict.
  - `parallel` — N testers each pursue their own goals; no interaction expected.
  - `scenario` — a scripted sequence driven by the `choreography` block.
- `summary` (string) — one line describing what the run validates.
- `requires` (map, optional) — server preconditions the conductor **verifies and
  surfaces** (it never changes server config):
  - `permadeath` (bool) — the server's `Death.PermaDeath` setting the run expects.
  - `perma_death_protection` (bool) — the playtest module's `DeathProtection` setting. **Perma-death only**: it sets a large ExtraLives count, which only matters when the server's `Death.PermaDeath` is on.
  - `pvp` (string) — the server's `GamePlay.PVP.Enabled` the run expects: `enabled` | `limited` | `disabled`.
  - `minimum_level` (int) — the server's `GamePlay.PVP.MinimumLevel`; fresh testers are level 1, so PvP scenarios usually need this lowered.
  - `max_connections` (int) — your server's `Network.AI.MaxConnections` (default 20).
- `roster` (list, required) — the tester agents. Each entry:
  - `id` (string, required) — stable id used in goals/choreography/reports.
  - `role` (string, required) — an existing personality (`framework/personalities/`).
  - `target` (string, required) — a `targets.yaml` entry. Blank creds there means
    the agent creates a character on first run.
  - `goals` (list, optional) — per-agent goals in the standard `id`/`do`/`verify`
    shape (see `framework/goals/SCHEMA.md`).
  - `onboarding` (string, optional) — `auto` (default) advances past the pre-tutorial ghost automatically; `full` drives the **real new-player flow** (so a feel-tester can grade onboarding).
- `group_goals` (list, optional) — interaction-level objectives, agent-judged, in
  the `id`/`do`/`verify` shape. Evidence may span multiple agents.
- `choreography` (list, optional) — ordered steps, mainly for `scenario` mode:
  - `who` (string, required) — a roster id.
  - `do` (string, required) — what that agent does.
  - `after` (string, optional) — a blackboard signal name to wait for first.
  - `round` (int, optional) — an absolute beacon round to act on.

## Verification model

Like single-agent goals, verification is **agent-judged** from observed `output`,
`gmcp`, and `beacon` events across agents — there is no assertion engine. Write
`verify` so an agent can tell from what it sees (and from the other agents' state
on the blackboard) whether the goal succeeded.

## Running a PvP scenario

PvP is gated by **server config**, not by this module — so a PvP run needs a few
flags set on the server before it works. The scenario's `requires:` block declares
them and the conductor warns on mismatch, but a human must set them:

1. **Enable PvP** — in `_datafiles/config.yaml` set `GamePlay.PVP.Enabled: enabled`
   (PvP everywhere) or `limited` (only in PvP-flagged rooms; then make sure the
   testers meet in such a room).
2. **Lower the level gate** — `GamePlay.PVP.MinimumLevel` defaults to 15; fresh
   testers are level 1, so set it to `1` (or level the testers up first).
3. **Choose lethality:**
   - *Defeat → respawn* (default): leave `GamePlay.Death.PermaDeath: false`. The
     loser is defeated and respawns. This is what `examples/adversarial-pvp.yaml`
     tests.
   - *Lethal (permanent) kill*: set `GamePlay.Death.PermaDeath: true` **and** the
     module's perma-death protection off — `DeathProtection: false` in
     `modules/playtest/files/data-overlays/config.yaml` — so the kill resolves.
4. **Restart** the server (engine config is read at boot).
5. Run it: `/playtest-scenario adversarial-pvp` (start from that example; copy and
   adjust the room/approach for your world).

Note: `perma_death_protection` / the module's `DeathProtection` only does anything
under PermaDeath — it guards perma-death, not ordinary defeat/respawn.

## Limits & cost

⚠️ Each roster agent is an independent LLM loop. **N agents cost roughly N× the
tokens and local processing of a single `/playtest` run.** Start with 2, watch
your usage rate, and keep rosters small. The server also caps concurrent AI
clients at `Network.AI.MaxConnections` (default 20) — raise it there if you need
more, and set `requires.max_connections` to match so the conductor can warn early.
The cap is **advisory at the CLI layer** — `ptorch scenario validate` only emits a
warning for an over-limit roster; the conductor (`/playtest-scenario`) is what
stops the run. The check compares against your *declared* `max_connections`, not
the server's actual setting, so keep them in sync.
