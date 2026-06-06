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
- [ ] **Step 3:** Reconcile `docs/usage/playtest-module.md` (beacons section) and
  the design doc's Phase-2 notes with what shipped.

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
