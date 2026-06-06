# End-to-End Smoke: Phase-2 `Playtest.Round` beacons

**Date:** 2026-06-05
**Result:** ✅ PASS — per-round beacons flow from the module through the adapter
as `beacon` events.

Follows the [Phase-1 smoke](2026-06-05-mudagent-smoke.md). This run verifies the
Phase-2 structured-verification path: the `playtest` module emits a
`Playtest.Round` GMCP beacon each round (via the `gmcp` module's `SendGMCPEvent`),
and the adapter surfaces it as a first-class `beacon` event.

Raw stream: [`2026-06-05-beacons-smoke.jsonl`](2026-06-05-beacons-smoke.jsonl).

## Setup

- Server: GoMud `feature/ai-port` + the `playtest` module (`Beacons: true`,
  default) + the bundled `gmcp` module, AI port `55555`.
- Account: the provisioned `IsAI` `aitester` (now spawning in room 1, Town
  Square, after the start-room fix).

## How it was driven

The adapter was left **idle** (no commands) for ~14 seconds, then quit — beacons
are a per-round heartbeat, so they arrive even with no agent activity:

```sh
( sleep 14; printf '{"control":"quit"}\n'; sleep 1 ) \
  | mudagent --target localhost:55555 --user aitester --password testpass123
```

## Result

Three `Playtest.Round` beacons over ~14s idle (≈ one per round), each surfaced as
a `beacon` event with a sequential round number and a structured state snapshot:

```json
{"type":"beacon","event":"Round","data":{"round":1314154,"hp":6,"hp_max":6,"sp":5,"sp_max":5,"room_id":1}}
{"type":"beacon","event":"Round","data":{"round":1314155,"hp":6,"hp_max":6,"sp":5,"sp_max":5,"room_id":1}}
{"type":"beacon","event":"Round","data":{"round":1314156,"hp":6,"hp_max":6,"sp":5,"sp_max":5,"room_id":1}}
```

(Round numbers are large because GoMud's round counter is global and persistent;
only their monotonic increase matters for pacing.)

## What this proves

- The module's `NewRound` hook fires and reaches the `gmcp` exported
  `SendGMCPEvent` for the connected `IsAI` user.
- The adapter classifies `Playtest.*` packages as `beacon` events (suffix as the
  event name) and leaves real GMCP state packages as `gmcp`.
- The agent now has a **reliable per-round tick** (replacing the Phase-1
  quiescence heuristic) plus an atomic `{round, hp, hp_max, sp, sp_max, room_id}`
  snapshot to score goals against.
