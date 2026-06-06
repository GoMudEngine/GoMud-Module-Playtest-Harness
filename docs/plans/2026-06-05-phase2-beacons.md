# Phase 2 — Structured Goal Verification (GMCP Beacons) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add `Playtest.*` GMCP beacons so an AI agent can pace on a reliable
per-round signal and score goals against structured state instead of scraping
text — the structured-verification half of the harness.

**Why:** Phase 1 proved the contract but exposed a gap: there is **no per-round
signal on the wire**, so the adapter paces by response *quiescence* (fragile),
and goals are verified by reading text. Phase 2 closes both: the `playtest`
module emits a `Playtest.Round` GMCP beacon each round (a tick + a compact state
snapshot), the adapter surfaces `Playtest.*` packages as `beacon` events, and
goals can `verify` against that structured state.

**Architecture — three tracks:**
- **A. Module beacons** (`module/playtest/`): hook `events.NewRound`, and for
  each connected `IsAI` user, send a `Playtest.Round` GMCP package via the
  `gmcp` module's exported `SendGMCPEvent`. Off by default behind a config flag.
- **B. Adapter plumbing** (`internal/session`): recognize `Playtest.*` GMCP and
  emit them as `beacon` events (`{"type":"beacon","event":...,"data":...}`)
  instead of generic `gmcp`.
- **C. Framework**: let goals `verify` against beacon state; update the driver to
  pace on `Playtest.Round` (replacing the quiescence heuristic) and score goals
  from beacons.

**Depends on:** the `gmcp` module (for `SendGMCPEvent`) — a runtime prerequisite
documented in the module's `info`/README (the registry has no dependency field).
Requires the Phase-1 engine `IsAI` flag and AI port.

**Confirmed APIs:**
- Cross-module send: `GetExportedFunction("SendGMCPEvent")` →
  `func(userId int, moduleName string, payload any)` (reference:
  `internal/usercommands/go.go:142`). Confirm the accessor name available to a
  module (the plugins registry exposes `GetExportedFunction`).
- Trigger: `events.NewRound{RoundNumber uint64, TimeNow time.Time}` via
  `events.RegisterListener`.
- Testers: `users.GetAllActiveUsers() []*UserRecord`, filter `u.IsAI`.
- Vitals/room for the payload: read the same `Character` fields the `gmcp`
  module's `Char.Vitals` uses (`modules/gmcp/gmcp.Char.go`) — mirror them.

---

## Beacon timing — the core design decision

The *timing* of a beacon (how often it fires, and relative to what) is the most
important decision in this plan: it determines whether the agent gets a
trustworthy heartbeat or has to keep guessing.

### What the timing must do

A beacon serves two jobs:

1. **Pacing** — signal "the world has advanced; it is meaningful to look and act
   now." Phase 1 had no such signal, so the adapter waited for **response
   quiescence** (output quiet for ~1–2s). That is fragile: background
   combat/regen ticks keep dribbling output, so "quiet" is ambiguous — the agent
   acts mid-resolution or waits needlessly.
2. **Scoring** — give an **atomic** structured snapshot to verify goals against
   (`hp went up`, `room_id == X`) instead of scraping text.

The decisive external constraint: the engine's AI rate limit is **per-round**
(`AICommandsPerRound`, default 2). The agent's command budget refreshes once per
round, so its natural decision cadence is already per-round.

### Decision: per-round, hooked on `events.NewRound`

Reasons it won:

- **Matches the command budget.** One tick per round = "budget reset, here is the
  world, decide your move(s)." The pacing signal and the rate limit share one
  clock — no other cadence aligns this cleanly.
- **Cleanly module-hookable.** GoMud already drives combat, auto-heal, and
  respawns off `NewRound` listeners; a module listens via
  `events.RegisterListener(events.NewRound{})` with zero engine changes.
- **Coarse enough not to spam.** One small GMCP frame per round per `IsAI` user,
  and AI connections are capped (`MaxAIConnections`), so traffic is bounded.
- **Steady heartbeat even on idle rounds.** The agent can always "wait for the
  next `Playtest.Round`" and know it will arrive — so it never stalls during
  quiet exploration, which is exactly when a confused agent would otherwise hang.

### Alternatives considered

| Option | When it fires | Pros | Cons / why not |
|---|---|---|---|
| **Per-command / "ack"** (the design doc's original phrasing) | after each submitted command completes | precise command↔result correlation; true request/response semantics | **No clean module hook** — commands flow through `worldManager.SendInput` and combat resolves *synchronously inside* the round step; there is no "command N for user X finished" event a module can listen to (would require an engine change). Also finer than the per-round budget. **Deferred** — see Open questions. |
| **Per-turn** (`NewTurn`, the sub-unit of a round) | every turn (several per round) | finer resolution | noisy; misaligned with the per-round command budget; no agent benefit (it cannot act more than `AICommandsPerRound` per round). |
| **Event-driven markers only** (level-up, death, room-change, hp-threshold) | only when something "interesting" happens | low noise; semantically rich for goals | **no heartbeat** — silent during quiet play, so no pacing signal and the agent can stall; also requires enumerating "interesting" up front. |
| **On-demand** (agent requests a snapshot) | when the agent asks | zero idle traffic; agent controls cadence | the request itself costs a command (rate-limit budget) and a round-trip, and re-introduces "when do I ask?" — it moves the timing problem rather than solving it. |
| **Periodic wall-clock** (every N ms) | on a timer | trivial to implement | decoupled from game state — a beacon landing mid-round captures a half-resolved world; races the round processing. |

### Sub-decision: where *within* the round

"Hook `NewRound`" does not fully pin the timing. `NewRound`'s own listeners
(combat, auto-heal) mutate state *during* that event, so the snapshot's freshness
depends on listener ordering:

- Fire **early** in `NewRound` → snapshot reflects the *previous* round's
  fully-resolved state (clean boundary, but one round "behind" this round's
  combat).
- Fire **after** the combat/heal listeners → reflects this round's outcome
  (freshest, but depends on being ordered last).

**Decision for v1:** emit a consistent "state as of the round boundary" — order
the beacon listener so it runs *after* the round's combat/heal/world-update
listeners (confirm/match how `gmcp` orders its own emissions). Either choice is
defensible as long as it is consistent and documented; the implementer must pin
this down rather than leave it to listener-registration luck.

### Net

Per-round is the only cadence that is simultaneously (a) hookable without engine
changes, (b) aligned to the rate-limit budget the agent already lives by, and
(c) a guaranteed heartbeat so the agent never stalls. It is the **backbone**;
optional event-driven `Playtest.<Marker>` beacons (Open questions) layer on top
**with no adapter change** — the adapter surfaces *all* `Playtest.*` packages as
beacons — so this choice is the floor, not the ceiling.

---

## Track A — Module: `Playtest.Round` beacon

### Task A1: Beacon config flag

**Files (harness `module/playtest/`, developed in `~/GoMud/modules/playtest/`):**
- Modify `files/data-overlays/config.yaml`: add `Beacons: true`.
- Modify `config.go`: add `Beacons bool` to `Config` + `buildConfig`; extend
  `config_test.go` (`TestConfigReadsValues`) to cover it.

- [ ] Add the field, default, and test; sync + `go test ./modules/playtest/`.

### Task A2: Round-beacon emitter

**Files:** create `module/playtest/beacons.go`

The payload schema (keep it small and stable):

```json
{"round": <uint64>, "hp": <int>, "hp_max": <int>, "room_id": <int>}
```

- [ ] **Step 1:** Add a pure helper for the payload so it is unit-testable, and
  test it:

```go
// beaconPayload is the Playtest.Round GMCP body.
type beaconPayload struct {
	Round  uint64 `json:"round"`
	HP     int    `json:"hp"`
	HPMax  int    `json:"hp_max"`
	RoomID int    `json:"room_id"`
}
```

- [ ] **Step 2:** Implement the emitter. Resolve `SendGMCPEvent` once (cache the
  function value), then on each `NewRound` send a `Playtest.Round` to every
  connected `IsAI` user. Skeleton (fill vitals from the real `Character` fields —
  mirror `modules/gmcp/gmcp.Char.go`'s `Char.Vitals`):

```go
package playtest

import (
	"encoding/json"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

type gmcpSender func(userId int, module string, payload any)

func (m *PlaytestModule) registerBeacons() {
	if !m.cfg.Beacons {
		return
	}
	// Resolve the gmcp module's exported sender. Use the same accessor
	// usercommands/go.go:142 uses; confirm its qualified name for a module.
	f, ok := getExportedFunction("SendGMCPEvent")
	if !ok {
		mudlog.Warn("playtest", "msg", "Beacons enabled but gmcp SendGMCPEvent not found; is the gmcp module installed?")
		return
	}
	send, ok := f.(func(int, string, any))
	if !ok {
		mudlog.Error("playtest", "msg", "SendGMCPEvent has unexpected signature")
		return
	}
	m.sendGMCP = send
	events.RegisterListener(events.NewRound{}, m.onNewRound)
}

func (m *PlaytestModule) onNewRound(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.NewRound)
	if !ok {
		return events.Continue
	}
	for _, u := range users.GetAllActiveUsers() {
		if !u.IsAI || u.Character == nil {
			continue
		}
		p := beaconPayload{
			Round:  evt.RoundNumber,
			HP:     u.Character.Health,    // confirm field names vs gmcp.Char.go
			HPMax:  u.Character.HealthMax, // ^
			RoomID: u.Character.RoomId,
		}
		b, _ := json.Marshal(p)
		m.sendGMCP(u.UserId, "Playtest.Round", json.RawMessage(b))
	}
	return events.Continue
}
```

(Add `sendGMCP gmcpSender` to the `PlaytestModule` struct. `getExportedFunction`
is whatever accessor the module uses — resolve it in Step 2.)

- [ ] **Step 3:** Wire `m.registerBeacons()` into `onLoad` (after the others).
  Sync, `go build ./...`, `go test ./modules/playtest/`.

### Task A3: Document the gmcp dependency

- [ ] Update `module/playtest/README.md`: `Beacons: true` requires the `gmcp`
  module; payload schema; that beacons are emitted per round to `IsAI` users.

---

## Track B — Adapter: surface `Playtest.*` as `beacon` events

**Files:** `internal/session/session.go`, `internal/session/session_test.go`

- [ ] **Step 1:** Add a failing test: feed a `Playtest.Round` GMCP frame through
  a scripted server (or unit-test the classification helper) and assert the
  emitted event is `{"type":"beacon","event":"Round","data":{...}}`, while a
  normal `Char.Vitals` still emits `{"type":"gmcp",...}`.

- [ ] **Step 2:** In the `TokenGMCP` branch of `session.Run`, classify by
  package prefix:

```go
case telnet.TokenGMCP:
	if suffix, ok := strings.CutPrefix(tok.GMCPPackage, "Playtest."); ok {
		emit(protocol.Event{Type: "beacon", Event: suffix, Data: rawJSON(tok.GMCPData)})
	} else {
		emit(protocol.Event{Type: "gmcp", Package: tok.GMCPPackage, Data: rawJSON(tok.GMCPData)})
		if !loggedIn && login.OnGMCP(tok.GMCPPackage) {
			loggedIn = true
			emit(protocol.Event{Type: "status", State: "logged_in"})
		}
	}
```

(Add `"strings"` to imports. `protocol.Event` already has `Event`/`Data`.)

- [ ] **Step 3:** `go test ./...`, `go build ./...`, `go vet ./...`. Commit.

---

## Track C — Framework: verify + pace on beacons

**Files:** `framework/goals/SCHEMA.md`, `framework/drivers/playtest.md`,
`framework/goals/example-smoke.yaml` (or a new beacon example)

- [ ] **Step 1:** Extend the goals SCHEMA: a `verify` may reference beacon state
  (e.g. "the `Playtest.Round` `hp` increased", "`room_id` became X"). Note that
  beacon-based verification is more robust than text matching.

- [ ] **Step 2:** Update the reference driver (`drivers/playtest.md`): replace
  the "wait for response quiescence" pacing with "wait for the next
  `{"type":"beacon","event":"Round"}` event" — a reliable per-round tick. Keep
  quiescence as a fallback for servers without the beacon (gmcp/playtest absent).

- [ ] **Step 3:** Add a goals example that scores against beacon state.

---

## Track D — End-to-end verification

- [ ] **Step 1:** Build `~/GoMud` (engine `feature/ai-port` + synced module with
  `Beacons: true`), with the `gmcp` module present. Boot; drive `mudagent`
  against the provisioned account and confirm the captured stream now contains
  `{"type":"beacon","event":"Round","data":{"round":N,...}}` events, roughly one
  per round.
- [ ] **Step 2:** Capture the run into `docs/e2e/` (a Phase-2 smoke alongside the
  Phase-1 one) and reference it.
- [ ] **Step 3 — docs, ONLY AFTER beacons are implemented and the Track-D smoke
  is green:** Update the repo **`README.md`** to reflect beacons as shipped — the
  "How it works" data-flow (beacon events now flow alongside output/gmcp), and
  the Personalities/goals section (goals can verify against beacon state; the
  reference driver paces on `Playtest.Round` instead of quiescence). Also
  reconcile `docs/usage/playtest-module.md` (beacons section + the `Beacons`
  config key + the gmcp-module dependency) and the design doc's Phase-2 notes
  with what shipped. The README must describe *shipped, tested* behavior — do not
  update it from the plan ahead of implementation.

---

## Self-Review checklist (run after implementing)

- Beacons off by default (`Beacons: true` is opt-in via config); absent `gmcp`
  module logs a warning and degrades gracefully (no beacons, adapter falls back
  to quiescence pacing).
- `Playtest.*` packages never leak as generic `gmcp` events; non-`Playtest`
  packages are unaffected.
- Payload field names match the real `Character` vitals source (verified against
  `gmcp.Char.go`, not guessed).
- The per-round beacon does not spam non-AI users (filtered by `IsAI`).

## Open design questions (decide during implementation)

- **Per-command ack vs per-round tick.** This plan ships the per-round beacon
  (achievable via `NewRound`). True per-command acknowledgment would need a
  command-completion hook — investigate `internal/hooks` / the input pipeline if
  finer granularity is wanted later.
- **Goal-marker beacons.** Beyond `Playtest.Round`, consider event-driven markers
  (`Playtest.LevelUp`, `Playtest.Death`, `Playtest.Entered` for a tagged room) if
  goal scoring needs them. Add as `Playtest.<Marker>` packages; the adapter
  already surfaces all `Playtest.*` as beacons, so no adapter change is needed.
