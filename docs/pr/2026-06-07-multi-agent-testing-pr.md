# PR: Multi-agent / party + PvP testing framework

**Branch:** `feat/multi-agent-testing` → `main`
**Open at:** https://github.com/GoMudEngine/GoMud-Module-Playtest-Harness/pull/new/feat/multi-agent-testing
**Type:** client-side only — **no `module/playtest/*` change, no module release, no registry bump.** Delivery = merge → testers `git pull`.

Copy everything between the rules below into the GitHub PR description.

---

## Summary

Adds a general **N-agent ("party") testing framework** to the harness: a conductor
runs multiple independent tester agents from one **scenario file**, coordinating
**in-game** plus a small shared **blackboard**, and writes a combined report.
Entirely client-side — nothing in `module/playtest/*` changes, so no release/registry
bump is needed (testers get it via `git pull`).

**Core framework**
- `internal/scenario` — scenario file parse/validate + cost/limit warnings.
- `internal/blackboard` — race-safe shared state (lock-file + atomic writes):
  readiness barrier, run phases, named signals, deduped findings.
- `cmd/ptorch` — thin CLI the conductor/agents call for all scenario/blackboard ops.
- `framework/scenarios/` — schema, generic template, and worked examples (one per
  mode) + a party expected-report.
- `framework/agent-runner.md`, `framework/multi-agent-report-format.md`,
  `.claude/commands/playtest-scenario.md` (the `/playtest-scenario` conductor).
- README section documenting the `Network.AI.MaxConnections` limit (default 20) and
  the N× token/processing **cost warning**.

**Also in this PR (bundled additions)**
- **PvP scenario + setup guide** — `examples/adversarial-pvp.yaml` plus a "Running a
  PvP scenario" guide in `SCHEMA.md`. PvP is **server-config only** (no module
  change): set `GamePlay.PVP.Enabled: enabled` and lower `GamePlay.PVP.MinimumLevel`;
  lethal/permadeath additionally needs `Death.PermaDeath: true` + module
  `DeathProtection: false`.
- **Optional ghost auto-advance** — per-roster `onboarding: auto` (default) | `full`
  (drive the real new-player flow, e.g. so a feel-tester can grade onboarding).
- **`death_protection` → `perma_death_protection`** scenario-field rename for
  precision (it maps to the module's `DeathProtection`, which is perma-death-only).
- **JSON tags** so `ptorch scenario plan` emits clean snake_case output.

## Design / plan

- Spec: `docs/superpowers/specs/2026-06-07-multi-agent-testing-design.md`
- Plan: `docs/superpowers/plans/2026-06-07-multi-agent-testing.md`
  (+ `…-additions.md`)

## Test plan

- [x] `go build ./...`, `go vet ./...`, `go test ./...` — all green (scenario +
  blackboard incl. concurrency, ptorch).
- [x] All shipped scenarios pass `ptorch scenario validate`.
- [x] **Live 2-agent party E2E** — two-sided party + chat, per-connection beacons to
  both agents (`docs/e2e/2026-06-07-multiagent-party.md`).
- [x] **Live 2-agent PvP E2E** — combat → damage → downed → bleed-out → clean
  respawn (`docs/e2e/2026-06-07-multiagent-pvp.md`). Confirmed defeat→respawn needs
  no module change (bleed-out death at HP ≤ -10 auto-suicides → respawn, ignoring
  `ExtraLives` under non-permadeath).

## Not in scope (deferred, see `docs/followups.md`)

- Lethal/permadeath PvP auto-test (documented setup; not auto-run).
- Per-agent perma-death protection (the only future item that would touch
  `module/playtest/*` → a real module release).
- >2-agent soak tuning; tight turn-by-turn combat choreography; the literal
  `/playtest-scenario` subagent path on a clean clone (E2Es drove the conductor
  flow manually).

🤖 Generated with [Claude Code](https://claude.com/claude-code)
