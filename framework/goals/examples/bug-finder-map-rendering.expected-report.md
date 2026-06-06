# Playtest Report: UI & rendering integrity sweep

> **Real captured finding.** This report is condensed from driving the harness
> (bug-finder personality) against **stock GoMud** during the 2026-06-06 sanity
> check (`docs/e2e/2026-06-06-three-profiles-sanity.md`). The bug below was
> root-caused and **fixed upstream in GoMudEngine/GoMud PR #602.**

**Date:** 2026-06-06
**Target:** GoMud (stock) — Frostfang start zone
**Personality:** bug-finder
**Account:** aitester
**Goals file:** bug-finder-map-rendering.yaml
**Duration:** ~6 minutes, 11 commands

## Summary
Swept the player-facing text surfaces in and around Frostfang's Town Square for
rendering integrity. Room/exit rendering, status panels, and edge-input handling
were clean. One real bug surfaced on a map surface: a map sign rendered a Go
format-string artifact instead of a zone-completion percentage.

## Goal Results
- [x] room-render — PASS: Town Square and three adjacent rooms render clean prose; exits and "also here" lines well-formed.
- [ ] maps-and-signs — FAIL: the Town Square map sign renders `Map of Frostfang (%!d(<nil>)%)` (see BUG). The `map` command itself renders correctly.
- [x] status-panels — PASS: status / inventory / skills panels show real values; no blank fields.
- [x] examine-targets — PASS (with CONCERN): examinable objects resolve; one visible NPC isn't examinable by its displayed name (see CONCERN).
- [x] edge-input — PASS: empty targets, nonsense commands, and missing-argument commands all return clean messages; no panic.

## Findings

### BUG: Map sign renders a Go format-string error
- **Where:** Frostfang, Town Square — looking at the map sign.
- **What I did:** `look sign`
- **What happened:** the map title rendered `Map of Frostfang (%!d(<nil>)%)`.
- **What should happen:** the title should show a zone-completion percentage,
  e.g. `Map of Frostfang (0%)`.
- **Why it matters:** `%!d(<nil>)` is what Go's `fmt` prints when a `%d` verb
  receives `nil` — a real formatting bug, not cosmetic noise. It affects only
  the script-rendered map *sign* path; the `map` command renders correctly,
  which is why a human using `map` would never see it.
- **Repro (raw telnet client, no special tooling):** connect, go to Town Square,
  `look sign`.

### CONCERN: Visible NPC not examinable by displayed name
The room lists `Also here: guard`, but `look guard` returns "Look at what???".
A player can see the guard but can't examine it by the name shown.

## Stats
- Commands sent: 11
- Errors seen: 0 (no panics)
- Bugs / Concerns / Observations: 1 / 1 / 0

---

**Why this example matters.** An AI bug-finder reads *every line as text*, not as
a game — so it caught a format-string artifact on a surface (a map sign) that
players using the normal `map` command never see. That is exactly the class of
defect this personality exists to find, and this one shipped as a real upstream
fix (PR #602).
