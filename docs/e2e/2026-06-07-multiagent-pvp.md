# E2E: 2-agent PvP run (adversarial mode)

**Date:** 2026-06-07
**Scenario:** `framework/scenarios/examples/adversarial-pvp.yaml` (mode: adversarial)
**Server:** local GoMud, AI port 55555, `playtest` module v0.1.2 (default config),
with PvP enabled: `GamePlay.PVP.Enabled: enabled`, `GamePlay.PVP.MinimumLevel: 1`,
`Death.PermaDeath: false`.
**Driver:** conductor flow driven manually via `ptorch` + two `mudagent`
connections (attacker, defender), both auto-advancing past the ghost.

## What was exercised

| Step | Result |
|------|--------|
| `ptorch bb init` + readiness barrier | exit 3 (not ready) → both ready → exit 0 → phase `running` ✓ |
| Connect + create both testers (blank creds) | both `logged_in` ✓ |
| Auto-advance past ghost (`start`→`human`→name→confirm→skip tutorial) | both full characters in **Town Square [Frostfang] (room 1)** ✓ |
| Same room | attacker `look` → "Also here: Defenda and guard" ✓ |
| **PvP engages** | `attack Defenda` → "Attaka prepares to fight you!"; combat each round ✓ |
| **Damage dealt** | defender HP 6 → 3 → 1 → negative; "You are hit by Attaka's attack for 2 damage", "Attaka smashes you with their fists, dealing 3 damage!" ✓ |
| **Defeat** | "you drop to the ground!" at HP -1 (downed) ✓ |
| Bleed-out | "you are bleeding out!" each round, HP ticking down -1 → -10 ✓ |
| **Respawn (clean)** | at HP ≤ -10 the engine auto-`suicide`s → respawn: defender HP back to **6/6**, moved to the death-recovery room (**room 1000000000**) ✓ |
| Per-connection beacons | attacker received 77 `Playtest.Round` beacons; defender's stream tracked HP throughout ✓ |
| Findings + finalize | 2 PASS + 1 OBSERVATION on the blackboard; phase `done` ✓ |

## Group goal result

- [x] **pvp-combat** — PASS. The attacker initiated PvP on the defender in a shared
  PvP-enabled room; the defender took damage (HP dropped via beacons + combat text),
  was defeated (downed), and ultimately **respawned cleanly** (HP restored to full,
  relocated to the death-recovery room) — no stuck state, no duplication, no error.

## Final blackboard findings

```json
"findings": [
  { "agent": "attacker", "type": "PASS", "title": "PvP engaged; dealt damage and downed the defender" },
  { "agent": "defender", "type": "PASS", "title": "Took PvP damage, downed, bled out, respawned cleanly (HP 6/6, death-recovery room)" },
  { "agent": "defender", "type": "OBSERVATION", "title": "PvP defeat resolves via bleed-out to -10 then auto-suicide->respawn (~10 rounds downed, not instant)" }
]
```

## Key confirmations

- **PvP between two AI-port testers works end-to-end** with the server config the
  scenario declares (`PVP.Enabled: enabled`, `PVP.MinimumLevel: 1`).
- **Defeat resolves to a clean respawn** even with the module's perma-death
  protection on — because under non-permadeath the bleed-out path
  (`internal/hooks/NewRound_AutoHeal.go`: downed → -1/round → at ≤ -10 auto-suicide
  → respawn) does **not** consult `ExtraLives`. So a defeat→respawn PvP test needs
  **no** module change and does **not** require turning perma-death protection off.
- The multi-agent framework (barrier, phases, per-connection beacons, findings)
  behaved identically to the party run with adversarial agents.

## Notes / observations

- **PvP defeat is not instant.** After being downed, a loser bleeds out ~1 HP/round
  and only respawns once HP reaches ≤ -10 (~10+ rounds). Agents/scenarios that
  verify "defeat → respawn" should expect a **downed / bleeding-out phase** first,
  not an immediate respawn. (The room's NPCs note "Somebody needs to provide aid!" —
  a downed ally can be revived before the bleed-out completes.)
- **Lethal (permanent) PvP** still needs `Death.PermaDeath: true` **and** the
  module's `DeathProtection: false` (so `ExtraLives` doesn't keep the loser alive
  across the permadeath check) — documented in `framework/scenarios/SCHEMA.md`
  ("Running a PvP scenario"). That path was not exercised here (this run validated
  defeat → respawn).
- Setup recap (what a dev flips): `GamePlay.PVP.Enabled: enabled`,
  `GamePlay.PVP.MinimumLevel: 1`, restart; then `/playtest-scenario adversarial-pvp`.
