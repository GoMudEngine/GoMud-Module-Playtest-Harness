# playtest module

Server-side policy for the GoMud AI playtest harness, built on the engine's
AI-port primitives (`IsAI`, the AI listener, conn cap, rate limit). It compiles
into a GoMud server and, at boot, provisions a flagged AI test account and
applies a structural safe mode.

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

> **How to set these:** module config is set through the **admin web config UI**
> (or the config API), NOT by hand-editing `config.yaml` / `config-overrides.yaml`.
> The module's `data-overlays/config.yaml` supplies defaults; a module overlay
> default overrides a hand-edited base `config.yaml` value, and a hand-edited
> nested `Modules.*` block in `config-overrides.yaml` does not merge into the
> module config map. Use the admin UI to set `AccountPassword`. (See the repo
> `docs/followups.md` — the exact operator path is being finalized.)

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
