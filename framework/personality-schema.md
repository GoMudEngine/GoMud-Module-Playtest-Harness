# Personality Schema

A **personality** is a Markdown file an AI agent loads as its role prompt for a
playtest session. Personalities are engine-agnostic: they describe *how* to test,
never a specific game's commands or content. Anything engine-specific (command
names, world map, mechanics) comes from `engine-profile.yaml`, not from here.

Every personality file has these sections:

## Role
One paragraph: who the tester is and their core objective.

## Playstyle
How to approach the session — exploration breadth, edge-case appetite, pacing,
and what to prioritize. No engine-specific content; refer to the engine profile
for command names.

## What to Report
The finding taxonomy, used **verbatim** across all personalities so reports are
consistent and machine-skimmable:

- **BUG** — clearly broken: errors shown to the player, crashes, missing text,
  dead-end exits, commands that do nothing.
- **CONCERN** — works but seems wrong: poor balance, confusing messaging,
  surprising behavior.
- **OBSERVATION** — notable behavior worth recording, neither clearly right nor
  wrong.
- **PASS** — a feature confirmed to work correctly.
- **FAIL** — a stated goal that was not met.
- **BLOCKED** — could not proceed: stuck, dead with no recovery, missing a
  prerequisite.

## Survival
Generic guidance for staying alive long enough to test. Engine-specific recovery
commands (healing, fleeing) come from the engine profile, not from here.

## Targeting
The universal rule: address NPCs, items, and exits by the **exact keywords**
shown in the room/description text.

## Engine Profile
A line instructing the agent to load `engine-profile.yaml` for this server's
command names, world orientation, and mechanics — so the personality stays
engine-agnostic.

---

To add a personality, copy this structure and fill in the Role and Playstyle for
the new testing style. Keep "What to Report", "Survival", and "Targeting"
consistent with the standard three (`bug-finder`, `feature-tester`,
`feel-tester`).
