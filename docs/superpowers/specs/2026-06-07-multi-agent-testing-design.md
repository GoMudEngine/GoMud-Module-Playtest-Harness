# Multi-Agent / Party Testing — Design

**Date:** 2026-06-07
**Status:** Approved design (pre-implementation)
**Scope:** A general N-agent testing framework for the GoMud playtest harness, with
a 2-agent party run as the v1 validated deliverable. Other interaction modes
(adversarial, parallel, scripted scenarios) are scenario files the same framework
runs, added incrementally after v1.

---

## 1. Goal

Today the harness drives **one** agent: one Claude session → one `mudagent` → one
character → one report, paced on the per-round `Playtest.Round` beacon. This design
adds **multiple independent tester agents** in a single coordinated run so the
harness can exercise multiplayer features and player-to-player interactions:
parties, PvP, contested resources, trade, racing, and scripted social scenarios.

The four interaction modes in scope:

- **party / allied** — testers coordinate toward a shared goal (invite/accept,
  party chat, group combat, shared XP/loot, follow, the GMCP `Party` namespace).
- **adversarial / non-allied** — independent players who interact but aren't
  allied (PvP swings, contested pickups, trade/haggling, competition/racing).
- **independent parallel** — N testers each pursuing their own goals with no
  required interaction (coverage / concurrency / soak).
- **emergent / scripted scenarios** — multi-step choreographed sequences across
  agents (A sets something up, B reacts).

## 2. Key decisions (settled during brainstorming)

| Decision | Choice | Why |
|----------|--------|-----|
| Agent execution model | **Independent agents + conductor** | Adversarial/emergent testing is only meaningful with genuinely independent minds; one brain playing all sides can't surprise itself or find real multiplayer bugs. |
| Coordination substrate | **In-game first + minimal blackboard** | Interactions happen through GoMud, so they actually exercise the features under test; a small shared file covers only what the game can't convey. |
| Run specification | **One scenario file** | A single YAML composes mode + roster + group goals + optional choreography — one thing to read and report against. |
| Death / lethal-PvP | **Client-side only; scenario declares, harness verifies** | Death protection is already a global config toggle and only matters under permadeath; v1 needs no module change. Per-agent / runtime control is a follow-up. |
| v1 deliverable | **General framework, validated by a 2-agent party run** | Build the general machinery once; prove it with the smallest real interaction; add other modes as scenario files later. |

**Everything in v1 is client-side → push-only, no module release.** (See the
README "Server side vs. client side" note: only `module/playtest/*` changes need a
release.)

## 3. Architecture

```
conductor (/playtest-scenario)
  │  parse scenario, check preconditions, seed blackboard,
  │  spawn agents, run readiness barrier, aggregate reports
  ├─ agent A ─▶ mudagent ─▶ char A ┐
  ├─ agent B ─▶ mudagent ─▶ char B ┤─ interact IN-GAME (party invite, PvP, trade…)
  └─ agent N ─▶ mudagent ─▶ char N ┘
            └── shared blackboard.json (ready[], phase, signals, findings[]) ──┘
            └── shared clock: the per-round Playtest.Round beacon number ───────┘
```

Two coordination channels:

- **In-game (primary):** A types `party invite B`; B sees the invite in its own
  event stream and accepts. A attacks B; B sees the damage. The interaction is the
  feature under test.
- **Out-of-band blackboard (minimal):** a readiness barrier, the run plan, named
  sync signals (for choreography), and a findings drop (for the combined report).
  The beacon round number is the shared clock so agents can agree on timing
  ("do X on round N") without tight coupling.

## 4. Components

Each is a focused unit with one responsibility and a well-defined interface.

### 4.1 Scenario file — `framework/scenarios/<name>.yaml`
Declares the whole run. Game-agnostic (engine specifics still come from
`engine-profile.yaml`). Ships with starting templates — see §11.

```yaml
name: party-smoke
mode: party                       # party | adversarial | parallel | scenario
summary: Two testers form a party and verify shared party state.

requires:                         # server preconditions — VERIFIED, never set
  permadeath: false
  death_protection: true

roster:
  - id: leader                    # stable id used in goals, choreography, reports
    role: feature-tester          # an existing personality (framework/personalities/)
    target: local                 # a targets.yaml entry; creds optional → creates a char
  - id: member
    role: feel-tester
    target: local
    # goals:                      # OPTIONAL per-agent goals, reusing the do/verify shape
    #   - id: solo-look  do: ...  verify: ...

group_goals:                      # interaction-level objectives, agent-judged
  - id: form
    do: leader creates a party and invites member; member accepts
    verify: both agents see each other in GMCP party state / a PartyUpdated event

choreography:                     # OPTIONAL ordered steps (mainly for `scenario` mode)
  - who: leader   do: party create, then party invite member
  - after: leader.invited         # a named blackboard signal (see 4.3)
    who: member   do: party accept
```

`mode` sets expectations/defaults: `party` expects party formation; `parallel`
expects no interaction; `adversarial` allows conflicting per-agent goals;
`scenario` drives the `choreography` block in order.

### 4.2 Conductor — `.claude/commands/playtest-scenario.md`
The new reference driver (auto-discovered, like `/playtest`). Steps:

1. Parse `framework/scenarios/<name>.yaml`.
2. **Check limits & preconditions** (see §6): roster size ≤ `Network.AI.MaxConnections`;
   `requires` vs live server state — **warn** on mismatch, never mutate config.
3. Seed `.playtest/<run>/blackboard.json` with `phase: lobby` and the plan.
4. Spawn one **agent runner** per roster entry (concurrent), each handed its role,
   assignment, `mudagent` bridge paths, and the blackboard path.
5. **Readiness barrier:** wait until every agent has set `ready[id]=true`, then set
   `phase: running`.
6. Wait for all agents to finish; set `phase: done`.
7. **Aggregate** per-agent findings into the combined scenario report.

### 4.3 Blackboard — `.playtest/<run>/blackboard.json`
The only out-of-band channel. Deliberately minimal.

```json
{
  "run": "party-smoke-2026-06-07",
  "phase": "lobby",
  "ready":   { "leader": true, "member": true },
  "signals": { "leader.invited": 1314530 },
  "findings": [
    { "agent": "leader", "type": "BUG", "title": "...", "round": 1314540 }
  ]
}
```

- **`phase`** — `lobby` → `running` → `done`. Drives the readiness barrier.
- **`ready`** — each agent sets its id `true` once in the world; conductor flips
  `phase` to `running` when all are present (prevents A acting before B exists).
- **`signals`** — named events mapped to the beacon round they fired; lets
  choreography reference `after: <signal>` loosely instead of hard-coupling agents.
- **`findings`** — agents append findings here for the combined report.

Writes are append/merge-oriented; readers tolerate partial state. Concurrency is
low (a handful of agents, sub-second cadence) so a simple read-modify-write with a
last-writer-wins merge on the `findings` array and per-key updates elsewhere is
sufficient. (Implementation note: agents write their *own* keys — `ready[selfId]`,
`signals[self-prefixed]`, and append-only `findings` — so cross-agent write
conflicts are structurally avoided.)

### 4.4 Agent runner
Per character: the **existing** single-agent play loop (connect → login or create a
character → play → write report), now additionally:

- reads its **role** (personality) and **assignment** (group goals + its roster
  line + any per-agent goals) from the scenario;
- after entering the world, sets `ready[id]=true` and waits for `phase==running`;
- interacts **in-game**, emits named **signals** to the blackboard for choreography,
  and paces on the **shared beacon round clock**;
- on finish, writes its per-agent report and appends its **findings** to the
  blackboard.

`mudagent` and the personalities are reused **unchanged**.

### 4.5 Reports
- **Per-agent report** — exactly today's `report-format.md`, one per roster id, so
  existing single-run conventions/tooling still apply.
- **Combined scenario report** — `framework/reports/<date>-<scenario>.md`:
  scenario summary; **group-goal results** (PASS/FAIL with cross-agent evidence,
  e.g. "leader's GMCP party shows member; member's shows leader"); a merged/deduped
  findings list tagged by agent; and a per-agent outcome line.

## 5. Run lifecycle (data flow)

1. Conductor parses scenario, checks limits + `requires` (warn on mismatch), seeds
   `blackboard.json` (`phase:lobby`), creates per-agent bridge dirs.
2. Spawns N agent runners concurrently. Each connects, logs in or **creates a
   character**, sets `ready[id]=true`, polls for `phase==running`.
3. Barrier: conductor sees all `ready` → sets `phase:running`.
4. Agents pursue role + group goals, interacting in-game, emitting signals, pacing
   on the beacon round clock.
5. Each agent writes its per-agent report and appends findings; conductor sets
   `phase:done` when all complete.
6. Conductor writes the combined scenario report.

## 6. Limits, cost & safety (MUST be documented prominently)

These two points must be explicit in the README and the conductor command doc, and
the conductor must enforce/surface them:

### 6.1 Connection limit
- GoMud caps concurrent AI clients at **`Network.AI.MaxConnections`** — **default
  20** (any value `< 1` is coerced to 20). This is a **preconfigured limit users
  can raise or lower** in `_datafiles/config.yaml` (or `config-overrides.yaml`),
  alongside `Network.AI.Port` and `Network.AI.CommandsPerRound`.
- The conductor **checks `len(roster)` against this limit at startup** and refuses
  / warns clearly if the scenario asks for more agents than the server allows,
  pointing the user at the config key to raise it.

### 6.2 Cost & resource warning
- **Running a party of many orchestrated agents is expensive** — each agent is an
  independent LLM loop, so **token usage and local processing/time scale with the
  number of agents** (N agents ≈ N× the cost/load of a single `/playtest` run, plus
  conductor overhead).
- The docs must say, plainly: **use with caution, start small (2 agents), and watch
  your usage/rate.** Prefer the smallest roster that exercises the feature; large
  rosters and long runs multiply quickly.
- The conductor should **echo the roster size and a cost caution** before spawning,
  and (where the runtime supports it) run agents in the background so the user can
  monitor.

### 6.3 Server-precondition safety
- The conductor **only reads** server state to verify `requires`; it never mutates
  server config. A lethal-PvP or permadeath scenario on a mismatched server gets a
  clear warning, not a silent wrong run.

## 7. v1 deliverable & validation

**Build (general):**
- scenario schema + parser (with limit/precondition checks),
- the conductor command,
- the blackboard + readiness barrier,
- the agent-runner adaptation,
- the combined scenario report format,
- the **shipped starting templates** (generic + one per mode — see §11).

**Validate (v1):** one **2-agent party** scenario end-to-end against a local server
(AI port + playtest module): leader creates a party, invites member, member
accepts, both verify shared party state via GMCP, combined report generated.
Recorded under `docs/e2e/`.

**Later (same framework, new scenario files):** adversarial (non-lethal: contested
pickup, race, trade), parallel (coverage/soak), scripted scenarios. Validated
incrementally; no framework changes expected.

## 8. Explicitly deferred (YAGNI)

- Lethal-PvP module work; per-agent / runtime death-protection control (the only
  future bits that touch `module/playtest/*` → a release).
- >2-agent stress/soak tuning.
- Live conductor micro-management of agents mid-run (beyond barrier + aggregation).
- Tight turn-by-turn party-combat choreography beyond what in-game interaction +
  the round clock naturally provide.

## 9. Testing strategy

- **Unit (Go, pure, no network):** the **scenario parser** (valid/invalid files,
  mode defaults, optional blocks) and the **blackboard** (readiness-barrier
  transition, signal recording, finding merge/dedup, per-key isolation).
- **E2E (live):** the 2-agent party run, captured (input + event streams +
  blackboard + combined report) under `docs/e2e/`, mirroring the single-agent E2E
  already on record.

## 10. Reference-implementation note (Claude Code)

The conductor spawns each agent as a **background subagent** (one per roster
entry), coordinating via the shared blackboard + in-game. Other agent runtimes
spawn processes instead. The **scenario file + blackboard contract is the
engine-/agent-agnostic part**; the subagent-based conductor is just the reference
consumer — the same stance as today's `/playtest` driver.

## 11. Shipped templates & examples

New users must have ready-made scenarios to start from — not a blank file. v1
ships, under `framework/scenarios/`:

- **`template.yaml`** — a **generic, heavily annotated** scenario with every field
  documented inline (mode options, roster, `requires`, group goals, optional
  per-agent goals, optional choreography). The "start from scratch" reference. This
  is the orchestrator + runner template in one (the runner config lives in each
  `roster[]` entry).
- **`examples/party-formation.yaml`** — the **v1 validated** 2-agent party scenario
  (leader creates → invites → member accepts → verify GMCP party state). Ships with
  its **expected combined report** alongside, mirroring how `goals/examples/`
  pairs a goals file with its expected report.
- **`examples/adversarial-contested-pickup.yaml`** — a 2-agent non-lethal
  adversarial example (two testers race for the same item; verify exactly one gets
  it).
- **`examples/parallel-coverage.yaml`** — an independent-parallel example (N
  testers each run their own short goals concurrently; no interaction expected).
- **`examples/scenario-trap-and-spring.yaml`** — a scripted/choreographed example
  (A stages something on one round, B reacts after the `A.ready` signal),
  demonstrating the `choreography` + `signals` mechanism.

**Validation status must be honest in each file's header comment:** the party
example is validated end-to-end in v1; the other three ship as **adaptable starting
templates** (illustrative, structurally valid, validated as their modes are
exercised post-v1) — clearly labelled so users don't mistake an unvalidated
template for a proven run. This mirrors the single-agent `goals/examples/`
convention.

Each example is a complete, runnable scenario file a user can copy, point at their
server (edit `targets.yaml` only if not `localhost:55555`), and run with
`/playtest-scenario <name>`.
