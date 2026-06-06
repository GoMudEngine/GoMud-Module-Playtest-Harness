# Three-Profile Sanity Check

**Date:** 2026-06-06
**Target:** local `~/GoMud` (`feature/ai-port` + `playtest` module + `gmcp`), AI port `55555`
**Account:** `aitester` (provisioned, `IsAI`)
**Result:** ✅ PASS — all three personalities drove live sessions; the harness
(login/reconnect, commands, movement, GMCP, beacons, rate limit, start-room fix)
works end to end. Several real findings surfaced (below).

Raw captures: `/tmp/{bug,feat,feel}.jsonl` (not checked in — ephemeral). This
doc is the record.

> One adapter bug was found **and fixed** mid-check (reconnect/kick handling —
> see "Adapter findings"). The runs below are post-fix.

---

## bug-finder (edge cases / breadth)

Commands: `look sign`, `look guard`, `get`, `look zzznothing`, `flarbexplode`, `north`.

- **BUG — map renders a Go format-string error.** `look sign` shows
  `Map of Frostfang (%!d(<nil>)%)`. `%!d(<nil>)` is what Go's `fmt` prints when a
  `%d` verb is given `nil` — a real formatting bug in the map title.
- **CONCERN — visible NPC not examinable by name.** The room lists
  `Also here: guard`, but `look guard` → `Look at what???`. A player can see the
  guard but can't look at it by its displayed name.
- **PASS — graceful edge-case handling.** `get` → `Get what?`; `look zzznothing`
  → `Look at what???`; `flarbexplode` → `flarbexplode not recognized. Type help
  for commands.`; `north` → clean movement to a richly-described room.

## feature-tester (validate features)

Commands: `help`, `status`, `skills`, `say hello there`, `who`.

- **PASS** — `help` (help system), `skills` (`No Skills! Visit a guild…`),
  `say` (`You say, "hello there"`), `status` (full Info/Attributes/Wealth panel)
  all work.
- **CONCERN — `who` returned nothing.** A bare `who` produced only a prompt (the
  online table is shown at login, but the command itself output nothing).
- **NOT A BUG (corrected) — test character is a pre-tutorial ghost.** `status`
  shows `neutral scrub`, race `ghostly spirit`, all attributes 0. Per the
  maintainer, **new GoMud players start as a ghost** in the base zone until they
  take the tutorial or choose to play (stats/name come later). So this is the
  normal starting state, not a provisioning defect. The open question is whether
  the agent should be primed about it / advanced through the tutorial — see
  "Provisioning findings".

## feel-tester (natural new-player play)

Commands: `look`, `look elms`, `south`, `look fountain`, `help commands`.

- **PASS / OBSERVATION (positive)** — room descriptions are evocative and
  well-written; the world feels alive (the `guard` wanders between rooms;
  ambient `A cold wind blows through the city`).
- **CONCERN — descriptive nouns aren't examinable.** The room mentions "Tall
  elms" and a castle, but `look elms` → `Look at what???`. A curious new player
  will reach for these. (Common MUD limitation, but a feel friction.)
- **CONCERN — onboarding hint dead-ends.** The login banner says "Type help for
  commands", but `help commands` → `No help found for "commands"`. The natural
  new-player query fails.

---

## What the sanity check VALIDATED (harness works)

- **Login + reconnect:** clean login on the AI port; the "already connected —
  Kick them? [y/n]" reconnect prompt is now answered automatically
  (`Reconnecting…`) — see Adapter findings.
- **Commands & movement:** look/get/say/help/status/skills/move all round-trip;
  invalid commands/targets fail gracefully.
- **GMCP + beacons:** GMCP state flows; a single feature-tester session received
  **7 `Playtest.Round` beacons** — the per-round heartbeat works live.
- **Output:** ANSI-stripped clean text (plus `raw`).
- **Start-room fix:** the account spawns in real rooms (Town Square / Cobblestone
  Way, Frostfang), not "The Void".

## Game findings (for the GoMud maintainer / content)

| Sev | Finding |
|-----|---------|
| BUG | `look sign` map title shows `%!d(<nil>)%`. **Root cause confirmed:** the `GetMap` scripting function (`internal/scripting/room_func.go:667`) builds the template data **without** a `ZoneCompletePct` key, but the shared `maps/map.template` formats the title with `printf "%s (%d%%)" .Title .ZoneCompletePct` → `%d` of `nil`. The `map` *command* (`internal/usercommands/skill.map.go:220`) supplies it, so only the room-script map **sign** path is affected (which is why it doesn't show via the `map` command / web client). One-line fix. **FIXED** on branch `fix/map-zonecompletepct` (GetMap defaults `ZoneCompletePct` to 0; verified live → `Map of Frostfang (0%)`); PR drafted at `docs/pr/2026-06-06-map-zonecompletepct-pr.md`. Not encoding-related. |
| CONCERN | `look <visible NPC name>` (`look guard`) → "Look at what???". |
| CONCERN | Descriptive room nouns (elms, etc.) not examinable. |
| CONCERN | `help commands` returns "No help found" despite the "type help for commands" hint. |
| OBS | Stray `inbox` / `mudletmap` commands appear post-login (rejected as "not recognized") — origin unclear (client-detection/onboarding?). Worth tracing. |

## Provisioning / new-player state (see docs/followups.md)

The provisioned account logs in as a **pre-tutorial ghost** (race `ghostly
spirit`, 0 stats, `nameless-<id>`). Per the maintainer this is the **normal**
GoMud new-player state — players begin as a ghost in the base zone and become a
full character via the tutorial or by choosing to play. So it is *not* a
provisioning bug.

Two harness questions follow (design, not defects): (1) should agents be
**primed** that they start as a ghost and how to proceed (factual, in the
engine-profile — enough to navigate without over-coaching the feel-tester's
onboarding evaluation)? (2) should provisioning optionally **advance** the test
account through tutorial/creation so agents test as a "real" character? Tracked
as follow-ups.

## Adapter findings

- **FIXED — reconnect/kick handling.** The login driver did not handle
  `User is already connected. Kick them? [y/n]:`, so a session that collided with
  a stale link-dead login failed ("Too many mistakes"). The driver now answers
  `y` and reconnects (verified live: `Reconnecting…`). Regression test added.
- **Follow-up — pre-login command race.** The adapter forwards stdin commands
  immediately, even before `logged_in`; a command sent during login is consumed
  as the username. The reference driver already waits for `logged_in` (contract
  respected), so this is agent-side; hardening the adapter to gate stdin until
  `logged_in` would be belt-and-suspenders. Tracked as a follow-up.
