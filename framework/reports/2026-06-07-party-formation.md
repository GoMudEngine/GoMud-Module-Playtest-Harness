# Multi-Agent Playtest Report: party-formation

**Date:** 2026-06-07
**Scenario:** party-formation (mode: party)
**Agents:** leader (feature-tester, char "Leadara"), member (feel-tester, char "Membrina")
**Server:** local (AI port 55555, playtest module v0.1.2)

## Summary
Two testers connected on the AI port, each created a character via the new-player
flow, and advanced past the pre-tutorial ghost into Town Square (Frostfang). The
leader created a party and invited the member, who accepted. Both clients then
showed the same two-person party, and a party-chat line from the leader reached
the member. Both connections received their own per-round beacon stream throughout.

## Group Goal Results
- [x] form — PASS: after `party create` / `party invite Membrina` / `party accept`,
  the `party` roster listed **both** Leadara and Membrina on **both** clients.
- [x] party-chat — PASS: leader's `party say` was delivered to the member as
  `(party) Leadara says, "Hello team, leader here"`.

## Per-Agent Outcomes
- leader (feature-tester): created party and invited cleanly; party + chat worked;
  71 per-round beacons received; no errors.
- member (feel-tester): accepted the invite cleanly; received party chat; 64
  per-round beacons received; no errors.

## Findings
### PASS: Party formation is consistent two-sided
Invite → accept produced matching two-person party membership on both clients.

### PASS: Party chat delivered
`party say` from the leader reached the member's event stream.

### OBSERVATION: Ghost onboarding precedes party play
Fresh AI-port characters start as nameless ghosts in the Void and must `start`
(race → name → confirm → skip tutorial) before party commands are meaningful.
Both agents handled it; a future auto-advance helper would streamline first runs.

## Stats
- Agents: 2
- Group goals: 2/2 PASS
- Beacons received: leader 71, member 64 (per-connection)
- Bugs / Concerns / Observations: 0 / 0 / 1
