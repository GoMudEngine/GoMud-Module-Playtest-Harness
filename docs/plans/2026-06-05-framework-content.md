# Framework Content Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax. This plan is content/spec-heavy rather than code-heavy; "verification" is schema validity + completeness + a live driver run, not unit tests.

**Goal:** Produce the engine-agnostic framework content: a personality schema + the three standard personalities, a goals schema + example, a targets schema, a report-format spec, an "engine profile" template (the slot for engine-specific facts), and a reference Claude Code driver that consumes the `mudagent` JSON protocol end-to-end.

**Architecture:** All content lives in the harness repo under `framework/`. The design principle from generalizing DOGMud's assets: **the *structure* is universal; the *engine-specific* facts (command names, world map, mechanics) are isolated into a single `engine-profile.yaml` an operator fills in.** Personalities and the driver reference the profile rather than hard-coding DOGMud lore.

**Source material:** DOGMud's `tools/testing/roles/*.md`, `goals/*.yaml`, `targets.yaml`, the report structure in `.claude/commands/test-mud.md`, and `mud_bridge.py`/`ai_player.py`. Generalize, don't copy — strip DOGMud spell names (`chrysalis-glow`), zones (Thornwall), NPCs (Kerra), and the `set charset` SOP into profile slots.

**Depends on:** the `mudagent` adapter's JSON protocol (events: `output`/`gmcp`/`beacon`/`status`/`error`; commands: plain line / `{"control":"quit"}`).

**Where:** `~/workspace/gomud-playtest-harness/framework/`.

---

### Task 1: Personality schema + the three personalities

**Files:** `framework/personality-schema.md`, `framework/personalities/bug-finder.md`, `framework/personalities/feature-tester.md`, `framework/personalities/feel-tester.md`

- [ ] **Step 1: Write the schema spec**

`framework/personality-schema.md` — define the required sections every personality file has, engine-agnostic:

```markdown
# Personality Schema

A personality is a Markdown file an agent loads as its role prompt. Required sections:

## Role
One paragraph: who the tester is and their core objective.

## Playstyle
How to approach the session (exploration breadth, edge-case appetite, pacing).
NO engine-specific content — refer to the engine profile for command names.

## What to Report
The finding taxonomy (use verbatim across all personalities):
- **BUG** — clearly broken (errors shown to player, crashes, missing text, dead exits).
- **CONCERN** — works but seems wrong (balance, unclear messaging).
- **OBSERVATION** — notable behavior worth recording.
- **PASS** — a feature confirmed working.
- **FAIL** — a stated goal not met.
- **BLOCKED** — could not proceed (stuck, dead, missing prerequisite).

## Survival
Generic guidance: don't die needlessly, recover between fights. Engine-specific
healing/resource commands come from the engine profile, not here.

## Targeting
Universal rule: address NPCs/items by the exact keywords shown in room text.

## Engine Profile
A line instructing the agent to load `engine-profile.yaml` for this server's
command names, world map, and mechanics.
```

- [ ] **Step 2: Write the three personalities** following the schema, with zero DOGMud specifics. Each references the engine profile for commands.

`framework/personalities/bug-finder.md` (excerpt of the contract — write the full file):

```markdown
# Bug-Finder

## Role
You are an exploratory QA tester hunting for defects. Your objective is breadth
and edge cases, not progression.

## Playstyle
- Visit every room, try every exit, interact with every NPC and item.
- Deliberately try edge cases: commands with no/invalid arguments, using items
  in the wrong context, targeting things that aren't present, malformed input.
- After each action, check the response against what a player would expect.

## What to Report
[the verbatim taxonomy from the schema]

## Survival
Avoid dying mid-investigation; recover between fights using the recovery
commands listed in the engine profile.

## Targeting
Use the exact keywords from room descriptions.

## Engine Profile
Load `engine-profile.yaml` for this server's commands, world, and mechanics.
```

Write `feature-tester.md` (methodical, goal-driven validation of specific features) and `feel-tester.md` (natural play, reports on game feel/UX) analogously.

- [ ] **Step 3: Verify** no DOGMud-specific tokens remain:

```bash
cd ~/workspace/gomud-playtest-harness
grep -riE "chrysalis|thornwall|dogmud|kerra|sanctum|dustwalk|set charset" framework/personalities/ || echo "clean"
```
Expected: `clean`.

- [ ] **Step 4: Commit**

```bash
git add framework/personality-schema.md framework/personalities/
git commit -m "feat(framework): personality schema + 3 generalized personalities

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Goals schema + example

**Files:** `framework/goals/SCHEMA.md`, `framework/goals/example-smoke.yaml`

- [ ] **Step 1: Write the goals schema**

`framework/goals/SCHEMA.md`:

```markdown
# Goals File Schema

A goals file is YAML describing a session's objectives. Game-agnostic.

```yaml
name: <string>            # human-readable session name (required)
summary: <string>         # one-line description of what this run validates
goals:                    # ordered list of objectives (required)
  - id: <short-id>        # stable id for the report to reference
    do: <string>          # what the agent should attempt, in prose
    verify: <string>      # how to judge success (text to see, GMCP state, behavior)
notes:                    # optional: setup/admin/context the agent should know
  - <string>
pass_criteria:            # optional: overall success checklist
  - <string>
```

Verification is agent-judged from observed `output`/`gmcp` events — there is no
formal assertion engine. Phase 2 adds `Playtest.*` GMCP beacons the agent can
key `verify` on for structured scoring.
```

- [ ] **Step 2: Write a generic example**

`framework/goals/example-smoke.yaml`:

```yaml
name: Basic connectivity smoke
summary: Confirm login, movement, look, and inventory work on a fresh server.
goals:
  - id: login
    do: Connect and reach the starting room.
    verify: A Room.Info GMCP event arrives and room text is shown.
  - id: look
    do: Run the look command.
    verify: Output describes the current room, its exits, and any contents.
  - id: move
    do: Take any available exit, then return.
    verify: A Room.Info GMCP event shows a different room, then the original.
  - id: inventory
    do: Check inventory.
    verify: Output (or a Char.Inventory GMCP event) lists carried items or "empty".
notes:
  - Uses only universal commands; no engine-specific content required.
pass_criteria:
  - No errors or stack traces appear in any response.
  - Each goal above is observed to succeed.
```

- [ ] **Step 3: Validate YAML + commit**

```bash
cd ~/workspace/gomud-playtest-harness
python -c "import yaml,sys; yaml.safe_load(open('framework/goals/example-smoke.yaml'))" && echo "valid yaml"
git add framework/goals/
git commit -m "feat(framework): goals schema + generic smoke example

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Targets schema + engine profile template

**Files:** `framework/targets.example.yaml`, `framework/engine-profile.example.yaml`

- [ ] **Step 1: Targets example** (host/port/account per named target; AI port is the default, conventionally 55555 when enabled):

`framework/targets.example.yaml`:

```yaml
local:
  host: localhost
  port: 55555          # the AI port (Network.AIPort); enable it in config.yaml
  user: aitester
  password: CHANGE_ME
```

- [ ] **Step 2: Engine profile template** — the single place engine-specific facts live, so personalities/driver stay generic:

`framework/engine-profile.example.yaml`:

```yaml
# Fill this in for YOUR GoMud server. The agent reads it for command names,
# world orientation, and mechanics so the personalities stay engine-agnostic.
engine: GoMud
commands:
  look: look
  movement: [north, south, east, west, up, down]
  inventory: inventory
  status: status
  attack: attack <target>
  get: get <item>
  recover: []          # e.g. healing spells/commands, if any
setup_commands: []     # commands to run right after login (e.g. terminal/charset)
world:
  starting_area: <describe where new characters begin>
  notes: <anything the agent should know to orient itself>
mechanics:
  health: <how health/resources work and how to recover>
  death: <what happens on death>
```

- [ ] **Step 3: Commit** both files.

---

### Task 4: Report-format spec

**Files:** `framework/report-format.md`

- [ ] **Step 1: Write the spec** (generalized from DOGMud's report structure):

`framework/report-format.md`:

```markdown
# Report Format

The agent writes one Markdown report per session.

```markdown
# Playtest Report: <session name or "Exploratory">

**Date:** <YYYY-MM-DD>
**Target:** <target name>
**Personality:** <bug-finder | feature-tester | feel-tester>
**Account:** <username>
**Goals file:** <filename or "none">
**Duration:** <~minutes>, <N> commands

## Summary
<2-4 sentence narrative of the session arc.>

## Goal Results
- [x] <goal id>: <do> — PASS: <evidence>
- [ ] <goal id>: <do> — FAIL: <observed vs expected>
- [ ] <goal id>: <do> — BLOCKED: <why>
(Omit this section for exploratory runs with no goals file.)

## Findings
### BUG: <title>
<what you did, what happened, what should have happened>
### CONCERN: <title>
### OBSERVATION: <title>
### PASS: <title>

## Stats
- Commands sent: <N>
- Errors seen: <N>
- Bugs / Concerns / Observations: <N> / <N> / <N>
```
```

- [ ] **Step 2: Commit.**

---

### Task 5: Reference Claude Code driver

A slash command demonstrating one agent consuming `mudagent` end-to-end. It proves the contract; other runtimes can consume the same JSON protocol differently.

**Files:** `framework/drivers/playtest.md`

- [ ] **Step 1: Write the driver** as a Claude Code slash command. Because a slash command can't hold a live bidirectional pipe across tool calls, the reference driver runs `mudagent` as a **background process bridged to append-only files** (the proven DOGMud pattern, but consuming clean JSON instead of raw telnet):

`framework/drivers/playtest.md` (contract — write the full command):

```markdown
---
description: Run an AI playtest session via the mudagent adapter
---

# /playtest <target> <personality> [goals-file]

1. Read `framework/targets.example.yaml` (or the operator's `targets.yaml`) for
   `<target>` → host/port/user/password. Read
   `framework/personalities/<personality>.md` and (if given) the goals file and
   `framework/engine-profile.yaml`.
2. Start the adapter in the background, bridging stdio to files:
   - stdout JSON events → append to `.playtest/events.jsonl`
   - stdin commands ← read from `.playtest/commands.txt`
   (Use a tiny shell/Python bridge, or run `mudagent` with a runtime that pipes
   these files. Command shape:
   `mudagent --target <host:port> --user <user> --password <pass>`.)
3. Wait until `events.jsonl` contains `{"type":"status","state":"logged_in"}`.
   Run any `setup_commands` from the engine profile.
4. Main loop until an exit condition:
   - Read new lines from `events.jsonl` (output text + GMCP state).
   - Decide the next command from the personality + goals + engine profile +
     current state.
   - Append the command to `.playtest/commands.txt`, then wait for response
     quiescence (no new events for ~1-2s — there is no round signal on the wire)
     and read the new events.
   - Track findings and goal progress as you go.
5. Exit when: all goals met, ~30 min elapsed, stuck for 10+ commands, or a fatal
   `error`/`disconnected` status.
6. Write a report per `framework/report-format.md` to
   `framework/reports/<date>-<target>-<personality>.md`.
7. Append `{"control":"quit"}` to `.playtest/commands.txt`; stop the background
   adapter.
```

- [ ] **Step 2: Add `.playtest/` to `.gitignore`** (runtime scratch) and create `framework/reports/.gitkeep`.

- [ ] **Step 3: Commit.**

---

### Task 6: End-to-end dry run + doc reconciliation

- [ ] **Step 1:** With `~/GoMud` running (engine `feature/ai-port`, `AIPort: 55555`, `playtest` module provisioning `aitester`) and `mudagent` built, run the reference driver with `feature-tester` and `framework/goals/example-smoke.yaml`. Confirm it logs in, runs the smoke goals, and produces a report file.
- [ ] **Step 2:** Note any protocol/flow mismatches and fix the relevant content/driver. Confirm the engine-profile slot is sufficient to keep the personalities engine-agnostic (no DOGMud or GoMud-internal leakage).
- [ ] **Step 3:** Reconcile `README.md` / `docs/usage/playtest-module.md` with the ACTUAL personality names, goals schema, targets keys, and driver flow. Commit.

---

## Self-Review

**Spec coverage (vs. design "Track 2C"):** personality schema + 3 personalities ✓ (Task 1); goals schema ✓ (Task 2); report-format spec ✓ (Task 4); reference Claude Code driver consuming the adapter ✓ (Task 5). Plus the **engine-profile** mechanism (Task 3) — the key generalization that keeps personalities/driver free of any specific game's commands and lore (the lesson from DOGMud's 348-line engine-specific system prompt).

**De-DOGMud guarantee:** Task 1 Step 3 greps for DOGMud tokens; engine-specific facts are confined to `engine-profile.yaml`.

**Known approximation:** the driver's background-bridge mechanism (Task 5) is runtime-specific; the file-bridge is the concrete reference (DOGMud-proven), and the JSON protocol it consumes is the stable contract other runtimes reuse. The "response quiescence" pacing is the documented substitute for the absent on-the-wire round signal.
