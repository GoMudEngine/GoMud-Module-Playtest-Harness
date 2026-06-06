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
- **CONCERN — test character is statless.** `status` shows the account as a
  `neutral scrub`, race `ghostly spirit`, with **all attributes 0** (Strength 0,
  Speed 0, …). See "Provisioning findings" — this is a headless-creation artifact.

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
| BUG | `look sign` map title: `%!d(<nil>)%` format-string leak. |
| CONCERN | `look <visible NPC name>` (`look guard`) → "Look at what???". |
| CONCERN | Descriptive room nouns (elms, etc.) not examinable. |
| CONCERN | `help commands` returns "No help found" despite the "type help for commands" hint. |
| OBS | Stray `inbox` / `mudletmap` commands appear post-login (rejected as "not recognized") — origin unclear (client-detection/onboarding?). Worth tracing. |

## Provisioning findings (our `playtest` module — see docs/followups.md)

The headless `NewUserRecord`-based provisioning produces an **incomplete
character**: no name (`nameless-1505890`), all-zero attributes, and an odd
default race (`ghostly spirit`). Interactive creation would set a name, roll
stats, and pick a race. The account is loginable and playable-enough for harness
testing, but a fuller test character (name + baseline stats) would make agent
runs more representative. Tracked as a follow-up.

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
