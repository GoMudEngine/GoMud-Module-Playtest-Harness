# Multi-Agent Testing — Additions Plan (bundled into the same branch/PR)

> Extends `2026-06-07-multi-agent-testing.md`. All client-side → still push-only,
> no `module/playtest/*` change, no registry bump. Branch: `feat/multi-agent-testing`.

**Decisions (confirmed):**
- PvP is exercised via **server config the scenario declares** (no module change).
  Ship a PvP example + a documented setup guide.
- Ghost auto-advance is the **default**; a per-roster `onboarding: full` opts a
  tester into the **real new-player flow**.
- The validated PvP example is **combat + defeat → respawn** (non-permadeath).
- Precision: the module's perma-death guard is **perma-death protection**; the
  scenario field is `perma_death_protection` (maps to the module's `DeathProtection`
  config key, which only matters under `Death.PermaDeath`).
- Add JSON tags so `ptorch scenario plan` emits snake_case nested fields.

## Task A — scenario schema changes + tests (`internal/scenario`)

`Goal` — add json tags: `json:"id"`, `json:"do"`, `json:"verify"`.

`Requires` — rename + extend (all with yaml+json tags):
- `Permadeath *bool` → `yaml:"permadeath" json:"permadeath"`
- `PermaDeathProtection *bool` → `yaml:"perma_death_protection" json:"perma_death_protection"` (renamed from `DeathProtection`)
- `PVP string` → `yaml:"pvp" json:"pvp,omitempty"` (values: enabled | limited | disabled)
- `MinimumLevel int` → `yaml:"minimum_level" json:"minimum_level,omitempty"`
- `MaxConnections int` → `yaml:"max_connections" json:"max_connections"`

`RosterEntry` — add `Onboarding string yaml:"onboarding" json:"onboarding,omitempty"` (values: auto | full; empty = auto).

`Validate()` additions (first-error-wins, after existing checks):
- each roster entry: if `Onboarding != "" && Onboarding != "auto" && Onboarding != "full"` → `roster %q: invalid onboarding %q (want auto|full)`.
- if `Requires.PVP != "" && not in {enabled,limited,disabled}` → `invalid pvp %q (want enabled|limited|disabled)`.

Tests (`scenario_test.go`):
- parse a scenario with `requires.pvp/minimum_level/perma_death_protection` and a roster entry `onboarding: full`; assert the parsed values.
- `Validate` rejects bad onboarding; rejects bad pvp.
- existing `validScenario()` still valid (it sets no onboarding/pvp → defaults OK).

## Task B — `ptorch scenario plan` per-agent onboarding (`cmd/ptorch`)

The `rosterOut` struct in `runScenario`'s `plan` branch currently emits id/role/target.
Add `Onboarding string json:"onboarding,omitempty"` and populate it from each
`RosterEntry.Onboarding`. (The `requires`/`group_goals` snake_case now comes for
free from the json tags added in Task A.)

Test (`main_test.go`): extend `TestScenarioPlanEmitsJSON` (or add one) to assert the
plan JSON has `requires.perma_death_protection` shape (snake_case) and that a roster
entry's `onboarding` round-trips.

## Task C — docs, template, examples, PvP guide

1. **Rename across existing files:** `death_protection:` → `perma_death_protection:`
   in `framework/scenarios/template.yaml` and all four existing examples
   (`party-formation`, `adversarial-contested-pickup`, `parallel-coverage`,
   `scenario-trap-and-spring`). Update the comment to note it = perma-death protection.
2. **`template.yaml`:** document the new optional fields — `requires.pvp`,
   `requires.minimum_level`, and a per-roster `onboarding: auto|full` (commented).
3. **New example `framework/scenarios/examples/adversarial-pvp.yaml`** (mode:
   adversarial): two bug-finder testers; `requires: { pvp: enabled, minimum_level: 1,
   permadeath: false, perma_death_protection: false, max_connections: 20 }`;
   group_goal "combat" — both reach a PvP context and one attacks the other; verify
   damage is exchanged and the loser is defeated and respawns cleanly (no dupes/stuck
   state). Header: "STARTING TEMPLATE until the live PvP E2E is recorded" (flip to
   VALIDATED after the E2E).
4. **`framework/scenarios/SCHEMA.md`:** document `requires.pvp`,
   `requires.minimum_level`, `requires.perma_death_protection` (note it = the
   module's `DeathProtection`, perma-death only), and the roster `onboarding` field;
   add a **"Running a PvP scenario"** section with the exact server steps:
   - `GamePlay.PVP.Enabled: enabled` (or `limited` + a PvP-flagged room),
   - lower `GamePlay.PVP.MinimumLevel` (fresh testers are level 1),
   - for a *lethal* run: `Death.PermaDeath: true` **and** the module's
     `DeathProtection: false`; for defeat→respawn leave PermaDeath off,
   - where each lives (`_datafiles/config.yaml` / module overlay), restart to apply,
   - point at `examples/adversarial-pvp.yaml` as the starting template.
5. **`framework/agent-runner.md`:** add an onboarding branch — if the agent's
   `onboarding` is `full`, go through the REAL new-player flow and (for a feel-tester)
   grade it; otherwise auto-advance past the ghost (start → race → name → confirm →
   skip tutorial), as the party E2E did.
6. **`.claude/commands/playtest-scenario.md`:** surface the new `requires` fields
   (pvp / minimum_level / perma_death_protection) as preconditions to confirm, and
   pass each agent its `onboarding` value when dispatching.
7. **README:** one line in the multi-agent section pointing at the PvP setup guide
   and the `onboarding: full` option.

Validation gate: `go run ./cmd/ptorch scenario validate` on template + all examples
(incl. the new PvP one) must exit 0.

## Task D — final validation

`go build/vet/test ./...` green; all scenarios validate; commit.

## Task E — live PvP E2E (deferred; needs server + user go-ahead)

Boot a server with `PVP.Enabled: enabled`, `PVP.MinimumLevel: 1`, run the
`adversarial-pvp` scenario (two testers, auto-advance, reach a PvP context, exchange
damage, one defeated → respawns), record under `docs/e2e/`, then flip the example
header to VALIDATED. Same server-hygiene rules as the party E2E (unique binary name,
exact-name shutdown, restore `~/GoMud` config). Do NOT run while the user is
smoke-testing.
