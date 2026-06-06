# playtest module

Server-side policy for the GoMud AI playtest harness, built on the engine's
AI-port primitives (`IsAI`, the AI listener, conn cap, rate limit). It compiles
into a GoMud server and, at boot, provisions a flagged AI test account and
applies a structural safe mode.

## Requirements (read before installing)

The module registry has no dependency field, so these prerequisites are listed
here — installing without them will fail to build or silently lose features:

- **A GoMud with the AI-port primitives** (the `IsAI` user field + the AI
  listener). These landed via the engine AI-port PR; on a GoMud release/commit
  that predates it, the module **will not compile** (`IsAI undefined`).
- **The bundled `gmcp` module** — required for the `Playtest.Round` beacons. If
  it's absent, beacons disable gracefully (startup warning), but you lose the
  per-round pacing/state tick the harness relies on.
- **`AccountPassword` must be set** via the admin config UI — empty disables
  provisioning (see Configuration).

## What it does

- **Boot-time provisioning** (`provision.go`): idempotently ensures the
  configured account exists, flagged `IsAI`, mirroring GoMud's real
  account-creation flow. Verified: creates `aitester` with `isai: true` and
  `extralives: 999`; a second boot does not re-create it.
- **Structural safe mode** (`safemode.go`): when a `SandboxZoneTag` is set, an
  `IsAI` account that leaves a room carrying that tag is **snapped back** (a
  `RoomChange` listener — the event fires post-move, so this is a fail-closed
  snap-back, not a pre-move veto). This keeps the tester away from live players.
- **Death protection** (`provision.go`): grants the test account a high
  `ExtraLives` count so it survives within GoMud's global permadeath setting.
- **Admin commands** (`commands.go`): `ai-flag <username> [on|off]` and
  `ai-list` (admin-only).

## Configuration

Keys live under `Modules.playtest.*`:

| Key | Default | Meaning |
|-----|---------|---------|
| `Enabled` | `true` | Master switch. |
| `AccountName` | `aitester` | Test account username. |
| `AccountPassword` | `""` | **Must be set** — empty disables provisioning. |
| `SafeMode` | `true` | Enable confinement (needs `SandboxZoneTag`). |
| `SandboxZoneTag` | `""` | Room tag to confine the tester to. Empty = no confinement. |
| `DeathProtection` | `true` | Grant high `ExtraLives`. |
| `Beacons` | `true` | Emit `Playtest.Round` GMCP beacon each round to IsAI users (requires `gmcp` module). |

> **How to set these (read this — it's the #1 gotcha):** the **confirmed
> working** way today is to edit this module's own defaults file,
> **`files/data-overlays/config.yaml`**, then restart the server. Do **not** add
> a `Modules:` → `playtest:` block to the server's `config.yaml` /
> `config-overrides.yaml`: the module's overlay default overrides a hand-edited
> base `config.yaml`, and a nested `Modules.*` block in `config-overrides.yaml`
> does not merge (it leaves the value empty, so provisioning is silently
> skipped). The admin web config UI / config API is the intended long-term path
> (being finalized). Full operator walkthrough + troubleshooting:
> <https://github.com/GoMudEngine/GoMud-Module-Playtest-Harness/blob/main/docs/usage/playtest-module.md>

## Beacons (Phase 2)

`Beacons: true` (default) emits a `Playtest.Round` GMCP package to every
connected IsAI user at the end of each game round. The payload is:

```json
{"round": 42, "hp": 30, "hp_max": 50, "sp": 10, "sp_max": 20, "room_id": 1001}
```

This gives the agent a reliable per-round pacing tick plus a compact state
snapshot it can use to score goal progress between commands.

Requires the bundled `gmcp` module. If the `gmcp` module is absent (its
`SendGMCPEvent` export is not in the plugin registry), the beacon is disabled
gracefully with a startup warning — no crash or error propagation.

## Limitations / follow-ups

- **`ai-flag` on an online account** updates the on-disk record, so the change
  applies on the player's next save/reconnect rather than instantly to the live
  session. Flag accounts while offline for immediate effect.
- **NoCombat buff is deferred.** A `no-combat` restriction would require shipping
  a buff definition and applying it by id, which isn't yet resolved. Phase-1
  "cannot harm live players" is delivered by **sandbox confinement** (keep the
  tester away from live players) plus death protection. If you run without a
  `SandboxZoneTag`, the tester roams the live world as a flagged,
  death-protected account.

## Development

This module only compiles inside a GoMud checkout. Source of truth is the
`gomud-playtest-harness` repo (`module/playtest/`); develop by placing it under
a GoMud checkout's `modules/playtest/`, then `go generate && go build`. Requires
a GoMud with the `IsAI` field (the AI-port engine PR).
