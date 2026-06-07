# E2E: 2-agent party run (multi-agent framework)

**Date:** 2026-06-07
**Scenario:** `framework/scenarios/examples/party-formation.yaml` (mode: party)
**Server:** local GoMud, AI port 55555, `playtest` module v0.1.2 (default config:
SafeMode on, DeathProtection on, Beacons on, no sandbox tag).
**Driver:** the conductor flow driven manually through the real `ptorch` CLI +
two `mudagent` file-bridged connections (one per roster agent). This validates the
framework mechanics end-to-end; the genuine independent-LLM path is the actual
`/playtest-scenario` conductor (subagents), which this run stands in for.

## What was exercised

| Step | Tool | Result |
|------|------|--------|
| Parse + plan the scenario | `ptorch scenario plan` | OK — emitted name/mode/max_connections(20)/roster/group_goals/requires/warnings(null) |
| Seed blackboard | `ptorch bb init … --ids leader,member` | board in `lobby`, both `ready=false` |
| Barrier before agents ready | `ptorch bb allready` | exit 3 (not all ready) ✓ |
| Connect leader (AI port), blank creds → create char | `mudagent` | `status:logged_in` |
| Connect member (AI port), blank creds → create char | `mudagent` | `status:logged_in` |
| Mark both ready | `ptorch bb ready --id …` | — |
| Barrier after both ready | `ptorch bb allready` | exit 0 ✓ → `ptorch bb phase --set running` |
| Advance both past the pre-tutorial ghost | in-game `start` → race `human` → name → confirm → skip tutorial | both became full characters in **Town Square [Frostfang] (room 1)** |
| Leader forms party + invites | in-game `party create`, `party invite Membrina` | "You invited Membrina to your party." |
| Emit a blackboard signal | `ptorch bb signal --name leader.invited` | recorded at round 1314487 |
| Member accepts | in-game `party accept` | "You joined the party!" |
| **Group goal `form`** | `party` on both sides | **PASS** — both rosters list **both** Leadara and Membrina on **both** clients |
| **Group goal `party-chat`** | leader `party say …` | **PASS** — member received `(party) Leadara says, "Hello team, leader here"` |
| Per-connection beacons | `Playtest.Round` events | **leader 71, member 64** — both AI connections got their own beacon stream |
| Record findings, finalize | `ptorch bb finding …`, `bb phase --set done`, `bb dump` | 2 PASS findings, phase `done` |

## Final blackboard

```json
{
  "run": "party-formation-2026-06-07",
  "phase": "done",
  "ready": { "leader": true, "member": true },
  "signals": { "leader.invited": 1314487 },
  "findings": [
    { "agent": "leader", "type": "PASS", "title": "Party formation + chat work two-sided", "round": 1314511 },
    { "agent": "member", "type": "PASS", "title": "Joined party via accept; received party chat", "round": 1314511 }
  ]
}
```

## Key confirmations

- **Per-connection beacon targeting works with multiple simultaneous AI clients** —
  each of the two AI-port connections received its own `Playtest.Round` stream
  (the playtest module keys behavior off the AI-port connection, not a shared
  flag). This is the server-side property the multi-agent framework relies on.
- **The blackboard contract works** — readiness barrier (exit-3 → exit-0), phase
  lifecycle (lobby → running → done), named signal, and per-agent findings, all
  through the `ptorch` CLI with atomic cross-process writes.
- **Real party mechanics pass two-sided** — create / invite / accept produce
  consistent membership on both clients, and party chat is delivered.

## Notes / observations

- **Ghost onboarding is required before party play.** Fresh characters spawn as
  nameless pre-tutorial ghosts in the Void; they must `start` (pick race → name →
  confirm → skip tutorial) to become full characters in a shared room before
  `party` commands are meaningful. The engine-profile `onboarding` field and
  `agent-runner.md` already tell agents to advance past the ghost — this run
  confirms that step is load-bearing for party scenarios. (Candidate for the
  deferred "auto-advance past the ghost" helper in `docs/followups.md`.)
- **Party chat is `party say <msg>`** (alias `party chat`); a bare `p <msg>` is not
  party chat. The `party-formation.yaml` goal text already says `party say`.

## Combined report

See `framework/reports/2026-06-07-party-formation.md`.
