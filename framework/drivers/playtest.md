---
description: Run an AI playtest session by driving the mudagent adapter
argument-hint: <target> <personality> [goals-file]
---

# /playtest `<target> <personality> [goals-file]`

A reference Claude Code driver. It proves the harness contract: spawn `mudagent`,
drive it through its line-in / JSON-line-out protocol, and write a report. Other
agent runtimes can consume the same protocol differently — this is one concrete
consumer, not the only one.

## 1. Load configuration

- Read `framework/targets.yaml` (copy of `targets.example.yaml`); look up
  `<target>` → `host`, `port`, `user`, `password`.
- Read `framework/personalities/<personality>.md` — this is your role prompt.
- Read `framework/engine-profile.yaml` (copy of the example) — command names,
  world orientation, mechanics. **All engine-specific behavior comes from here.**
- If `[goals-file]` was given, read `framework/goals/<goals-file>`.

## 2. Start the adapter (background, file-bridged)

A slash command can't hold a live pipe across tool calls, so bridge `mudagent`'s
stdio to files with `tail -f`:

```sh
mkdir -p .playtest && : > .playtest/commands.txt && : > .playtest/events.jsonl
tail -n +1 -f .playtest/commands.txt \
  | mudagent --target <host>:<port> --user <user> --password <password> \
  > .playtest/events.jsonl 2>&1 &
```

You issue a command by appending one line to `.playtest/commands.txt`; you read
results from new lines in `.playtest/events.jsonl`. Each event is one JSON
object: `{"type":"output","text":...}`, `{"type":"gmcp","package":...,"data":...}`,
`{"type":"status","state":"connected|logged_in|disconnected"}`,
`{"type":"error","message":...}`. (`mudagent` handles connect + GMCP
negotiation. With `--user`/`--password` it also auto-logs-in to an existing
account; without them — or if the account doesn't exist yet — *you* drive login
and character creation via commands, see step 3.)

## 3. Log in, or create a character

Poll `.playtest/events.jsonl` until `{"type":"status","state":"logged_in"}`
(`Room.Info`/`Char.Info` confirms you're in the world). Getting there depends on
whether your character exists:

- **It exists** (you passed `--user`/`--password`): the adapter logs in
  automatically — just wait for `logged_in`.
- **It doesn't exist yet** (you see the `username (or "new")` prompt repeat, an
  "invalid login", or no auto-login): **create a character via the normal
  new-player flow** — this is part of what a tester exercises, and a feel-tester
  should grade it. Append responses one per line to `.playtest/commands.txt`,
  following the prompts. On stock GoMud the sequence is: `new` → desired
  username → password → password again → email (blank is fine) →
  screen reader? `n` → confirm `y`. You then enter the world.
- **New characters begin as a pre-tutorial "ghost"** (see the engine profile's
  `onboarding`). Take the tutorial or choose to start playing to become a full
  character before attempting goals that need stats or items.

If `disconnected`/`error` arrives first, abort and report. Then run any
`setup_commands` from the engine profile.

## 4. Play (main loop)

Repeat until an exit condition:
1. Read new lines from `.playtest/events.jsonl` — the `output` text, `gmcp`
   state, and `beacon` events are your view of the world.
2. Decide the next command from your **personality** + **goals** + **engine
   profile** + current state.
3. Append the command (one line) to `.playtest/commands.txt`.
4. **Pace on the round beacon:** wait for the next
   `{"type":"beacon","event":"Round"}` event (the `playtest` module emits one per
   round). It is a reliable per-round tick and carries
   `{round, hp, hp_max, sp, sp_max, room_id}` — use it for pacing and goal
   scoring. *Fallback:* if no beacons arrive (the `playtest`/`gmcp` modules are
   absent), fall back to response quiescence (~1–2s with no new events).
5. Pace yourself within a round too: the server caps AI input at
   `AI.CommandsPerRound` (default 2) per round; a dropped command is reported back
   as an `output` notice.
6. Track findings and goal progress as you go (the beacon snapshot is good
   evidence for `verify` conditions).

## 5. Exit conditions

Stop when any holds: all goals met; ~30 minutes elapsed; stuck for 10+ commands
with no progress; or a fatal `error`/`disconnected` status.

## 6. Write the report

Produce a report per `framework/report-format.md` at
`framework/reports/<date>-<target>-<personality>[-<goals>].md`.

## 7. Clean up

```sh
printf '%s\n' '{"control":"quit"}' >> .playtest/commands.txt   # closes mudagent
sleep 1
pkill -f 'tail -n +1 -f .playtest/commands.txt' 2>/dev/null || true
```

Report completion (and the report path) to the user.
