# Playtest Report: New-player onboarding feel pass

> **Real captured findings.** Condensed from driving the harness (feel-tester
> personality) against **stock GoMud** during the 2026-06-06 sanity check
> (`docs/e2e/2026-06-06-three-profiles-sanity.md`).

**Date:** 2026-06-06
**Target:** GoMud (stock) — Frostfang start zone
**Personality:** feel-tester
**Account:** aitester
**Goals file:** feel-tester-new-player-onboarding.yaml
**Duration:** ~7 minutes, 9 commands

## Summary
Played the opening as a brand-new player with no outside knowledge. The world
itself reads beautifully — evocative rooms, ambient life — but two onboarding
hints dead-end, and the world doesn't always reward the curiosity it invites. A
newcomer can get oriented, but hits avoidable friction in the first minutes.

## Goal Results
- [x] first-impression — DONE: the starting room is vivid and the prompt is clear; the banner points to `help`.
- [x] follow-the-hints — DONE (friction): the "type help for commands" hint dead-ends (see CONCERN).
- [x] explore-naturally — DONE (friction): several described nouns aren't examinable (see CONCERN).
- [x] grade-the-feel — DONE: grades below.

## Findings

### OBSERVATION (positive): The world feels alive
Room descriptions are atmospheric and specific; ambient lines ("A cold wind
blows through the city") and a wandering guard make the space feel inhabited.
A strong first impression.

### CONCERN: The first onboarding hint dead-ends
The login banner says "Type help for commands," but `help commands` returns
"No help found for 'commands'." The single most natural new-player action fails.
Either make `help commands` resolve, or point the banner at a command that works.

### CONCERN: Described nouns aren't examinable
The room mentions "tall elms" and a castle, but `look elms` → "Look at what???".
A curious newcomer reaches for exactly these. A common MUD limitation, but it
teaches players to stop looking — a real feel cost in the first five minutes.

## Feel grades
- **Clear:** mostly — you know where you are; the next *action* is fuzzier.
- **Immersive:** strong — the prose and ambience carry it.
- **Consistent:** weak spot — a hint points at a command that doesn't exist.
- **Forgiving:** mixed — invalid commands fail cleanly, but dead-end hints aren't forgiving of newcomers.
- **Could a new player reach "I'm playing" unaided?** Yes — but with avoidable stumbles in the first few minutes.

## Stats
- Commands sent: 9
- Errors seen: 0 (no panics; "no help" / "look at what" are graceful misses)
- Bugs / Concerns / Observations: 0 / 2 / 1

---

**Why this example matters.** Generalized from a DOGMud "feel pass" goal that
graded an experience on named axes rather than pass/fail. A feel-tester surfaces
the friction functional tests miss — a hint that lies, curiosity that goes
unrewarded — the quiet things that cost you new players.
