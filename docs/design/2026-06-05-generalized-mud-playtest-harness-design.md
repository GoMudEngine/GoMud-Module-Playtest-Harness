# Generalized MUD Playtest Harness — Design

**Date:** 2026-06-05
**Status:** Approved design, revised post-validation. Implementation plans in
`docs/plans/`.
**Author:** Calabe Davis (with Claude)

## Revision note (2026-06-05, post live-code validation)

The original design was validated against the live `~/GoMud` checkout
(upstream `GoMudEngine/GoMud` master, commit `bab2131f`) and the DOGMud
fork (`~/workspace/DOGMud`). The validation confirmed the module/plugin/
GMCP infrastructure but corrected several assumptions and changed one
structural decision. Material changes from the first draft:

1. **The harness is now a two-track project, not a single module.** A
   DOGMud-style dedicated **AI-only telnet port** (with a connection cap)
   is wanted. That feature is *core-engine surgery* — a separate listener,
   a connection-type field, config fields, an input-loop rate gate, and a
   user-record flag — none of which a registry module can do (modules only
   register via `init()` and the plugin API). So the port primitives are
   contributed **upstream to GoMud core as a PR (Track 1)**, and the
   policy/tooling ships as **registry artifacts (Track 2)** on top.
2. **Port correction.** Vanilla GoMud listens on `33333`/`44444`, not
   `55555`. `55555` is DOGMud's *AI port*. The new AI port ships **disabled
   by default** (`AI.Port: 0`) so the upstream PR is a no-op for stock
   servers; operators (and the harness setup docs) enable it by setting a
   port — `55555` is the recommended value, matching DOGMud. The adapter
   targets whatever port the operator chose.
3. **No leaderboard system exists upstream.** "Auto-exclude test accounts
   from leaderboards" has nothing to exclude from. Reframed: Track 1 adds
   the `IsAI` user flag as a reusable primitive; any present/future
   leaderboard (DOGMud already has one as a module) filters on it.
4. **Safe-mode mechanism changed.** Combat applies damage synchronously
   inside the `DoCombat` listener — there is no cancelable pre-attack
   event. "Cannot harm live players" is therefore implemented
   *structurally*: sandbox-zone confinement via room tags and/or a
   `NoCombat`-style restriction on the test account, not by intercepting
   and refusing actions.
5. **Permadeath is global + per-character, not per-account-toggleable.**
   "Death-protected test account" is implemented via a high `ExtraLives`
   count and/or a revive-on-death buff, working within the global
   permadeath setting.

Resolved open questions (see end of doc for the originals):

- **Repo:** standalone repo `GoMudEngine/GoMud-Module-Playtest-Harness` (personal
  account; offer transfer to `GoMudEngine` later). DOGMud is `pruuk`'s
  existing fork of GoMud, so the engine PR rides on a clean branch pushed
  to that fork.
- **Core/module split:** *minimal core, fat module* — Track 1 adds only
  the irreducible primitives; all policy (provisioning, safe-mode,
  account flagging commands, beacons) lives in Track 2.
- **Beacon packaging:** bundled inside the `playtest` module (one registry
  entry), calling gmcp's exported `SendGMCPEvent`.
- **Personalities:** ship the standard three; add more on demand.

## Summary

Generalize DOGMud's internal "smoketester" framework into an
engine-agnostic, agent-agnostic AI playtest harness that **any** GoMud
server operator can use and **any** AI agent framework can drive. The
point: a generalized tester harness so a developer who starts from GoMud
can have their agent of choice connect, run our standard personalities
against a set of goals they define, and produce a readable report.

Delivered as two tracks:

- **Track 1 — Engine PR to GoMud core.** Contributes the AI-only port
  primitives upstream so *every* GoMud server gets a dedicated,
  rate-limited, connection-capped AI port natively.
- **Track 2 — Registry artifacts.** The `mudagent` adapter, the `playtest`
  module, and the framework content/spec — published via the
  [GoMud-Modules registry](https://github.com/GoMudEngine/GoMud-Modules)
  and a release of the standalone repo. This exercises both the *creation*
  and *consumption* sides of the module system the upstream owner asked
  contributors to test.

Developed against the existing vanilla GoMud checkout at `~/GoMud` as the
live test/build target.

## Motivation

DOGMud already has a working AI smoketester (`tools/mud_bridge.py`,
`tools/testing/roles/*.md`, `tools/testing/goals/*.yaml`,
`.claude/commands/test-mud.md`) and a dedicated AI port. It works, but it
is:

- **Claude-Code-specific** — driven by a file-poll loop and a fixed
  `sleep` cadence baked into a slash command.
- **DOGMud-specific** — port wiring lives in DOGMud's forked core;
  personalities and goals reference DOGMud content, commands, and zones.
- **Brittle** — goal verification scrapes cleaned ANSI text rather than
  reading structured state.

The upstream GoMud owner asked the community to test creating modules and
using the module registry. This project serves that request, contributes
a genuinely useful primitive (the AI port) to the engine, and produces a
reusable tool for every GoMud server.

### Upstream context (as of 2026-06-05)

GoMud `master` (commit `bab2131f`) restructured the module ecosystem:

- Official modules were **removed from the engine repo** and now live in
  the registry. A fresh checkout bundles only `cleanup`, `follow`, `gmcp`,
  `webhelp`. *(Validated.)*
- A **module manager is built into the server binary** —
  `go run . module <list|info|install|remove|update|package>`. It fetches
  the registry live, verifies sha256, extracts into `modules/<name>/`, and
  records installs in `modules/modules.lock.yaml`. *(Validated:
  `main.go:84`, `modmanager.go:26`, `install.go`, `lockfile.go:16`.)*
- `module package <name>` packages a local module into a `.tar.gz` and
  prints its SHA256 — the publish step is one command. *(Validated:
  `package.go`.)*
- Modules are compiled into the binary; install/remove requires a rebuild.
  `go generate` regenerates `modules/all-modules.go` from installed
  modules. *(Validated: `cmd/generate/module-imports.go:60`.)*
- The registry entry schema is `name/description/version/author/url/
  sha256` only — **no dependency field**. *(Validated: `registry.go:20`.)*

## Goals

- Any AI agent framework can connect to a GoMud server's AI port, run a
  set of goals using a standard set of personalities, and produce a
  structured report.
- Engine-agnostic: works against vanilla GoMud, not just DOGMud.
- A dedicated, capped AI port so test traffic is isolated and bounded.
- Robust goal verification via structured GMCP state, not text scraping.
- Distributed through the official module registry + upstream PR.

## Non-Goals

- Replacing DOGMud's existing internal testing assets (they remain; this
  is a parallel, generalized tool).
- Building an AI agent / LLM runner. The harness exposes a contract;
  bring-your-own agent.
- An in-game admin command to *launch* runs (cut — external agents drive
  runs; revisit only if demand appears).

## Architecture

### Track 1 — Engine AI-port primitives (GoMud core PR)

A clean PR against `GoMudEngine/GoMud` master adding the irreducible
primitives that a module cannot provide. Modeled on DOGMud's "AI client
infrastructure," generalized and de-DOGMud'd. Scope:

1. **A dedicated AI telnet listener.** A second `net.Listener` on a
   configurable `AI.Port` (ships `0` = disabled; operators set e.g. `55555`
   to enable), separate from the human telnet ports.
2. **A connection type.** A `ConnType` (Human/AI) recorded on the
   connection at accept time and readable for its lifetime.
3. **A connection cap.** `AI.MaxConnections` (default `20`) enforced
   independently of human connections; the 21st AI connection is refused
   with a clear message.
4. **Clean output for AI clients.** AI connections strip ANSI so agents
   read plain text without parsing escape codes (telnet IAC preserved).
5. **Per-round rate limiting.** `AI.CommandsPerRound` (default `2`) gates
   how many commands an AI connection may submit per round, with a clear
   "rate limited" notice.
6. **The `IsAI` user flag.** A persisted `bool` on `UserRecord` marking an
   account as an AI/test account — the reusable primitive policy code
   (leaderboards, etc.) filters on.

Track 1 is *primitives only*. It does **not** auto-provision accounts,
implement safe-mode, or add flagging commands — those are Track 2 policy.

### Track 2A — `playtest` server module (the registry citizen)

A GoMud community module (Go) that compiles into the server, registering
via `init()` against `github.com/GoMudEngine/GoMud/internal/plugins`.
*(Validated registration pattern: `plugins.New(name, version)`, e.g.
`modules/gmcp/gmcp.go:53`.)* Published as a `.tar.gz` + sha256 (produced
by `module package`) referenced from `module-registry.yaml`. Capabilities:

1. **Test-account auto-provisioning.** At boot, idempotently ensure a
   configurable AI-test account exists and is `IsAI`-flagged. *(Feasible:
   `users.CreateUser` works from Go at boot — `users.go:404`.)* This
   removes the brittle login/create dance from the adapter.
2. **Safe / sandbox mode (structural).** For flagged test accounts:
   - confine to a tagged sandbox zone using room tags
     (`rooms.Room.Tags` — the documented extensibility hook), refusing to
     leave when confinement is enabled (**fail closed**);
   - apply a `NoCombat`-style restriction so the account cannot fight live
     players (combat damage is synchronous and cannot be canceled by an
     event listener, so prevention must be structural);
   - death-protect via high `ExtraLives` / revive-on-death buff.
3. **Account-flagging commands.** `AddUserCommand`-registered admin
   commands to set/list `IsAI` accounts (the policy layer over the core
   flag; DOGMud shipped these in core, we keep them in the module per the
   minimal-core decision).
4. **GMCP test-beacon enrichment (Phase 2).** A sub-package emitting
   structured `Playtest.*` GMCP messages so the adapter scores goals from
   structured data instead of ANSI scraping. **Depends on the existing
   `gmcp` module**, calling its exported
   `SendGMCPEvent(userId int, moduleName string, payload any)`. *(Validated
   export: `modules/gmcp/gmcp.go:59`; real cross-module usage at
   `usercommands/go.go:142`.)* It does not reimplement GMCP.

Per current module conventions, the published archive extracts to:

```
playtest.go               # registers via init() against internal/plugins
files/
  datafiles/
    templates/help/...    # help files
    html/...              # optional admin/web assets
  data-overlays/
    config.yaml           # module config defaults (test account name,
                          # sandbox zone tag, safe-mode toggles)
    keywords.yaml         # optional help-topic registration
```

*(Validated layout against bundled modules and `modules/README.md:145`;
embed via `//go:embed files/*` + `plug.AttachFileSystem(files)`.)*

Because the registry schema carries no dependency field, the **`gmcp`
module prerequisite** for the beacon capability is documented in the
module's `info` description and README rather than auto-resolved.

### Track 2B — `mudagent` reference adapter (the "any agent" contract)

A **Go single static binary** — matches the engine, cross-compiles to one
dependency-free executable per OS. Responsibilities:

- Connect to the AI port (telnet), handle all IAC / GMCP negotiation and
  login (using the provisioned test account), so the agent never touches
  sockets or the login flow. *(GMCP handshake validated: server sends
  `IAC WILL GMCP`; client replies `IAC DO GMCP` then `Core.Hello` /
  `Core.Supports.Set` — `modules/gmcp/gmcp.go:202,355`.)*
- Expose a **line-in / JSON-line-out stdio protocol** (below). Any agent
  framework spawns it as a subprocess.
- Stream structured events: cleaned game text, GMCP state snapshots
  (`Char.Vitals`, `Room.Info`, `Char.Inventory`, etc. — validated package
  names), `Playtest.*` beacon events, round-tick boundaries, and
  connection status.

The adapter needs **no engine changes** — it just opens a socket, exactly
as DOGMud's `mud_bridge.py` does against `:55555`.

### Track 2C — Engine-agnostic framework content + spec

- **Personality schema** plus the standard three personalities
  (`bug-finder`, `feature-tester`, `feel-tester`), rewritten with no
  DOGMud specifics.
- **Goals schema** — game-agnostic YAML describing session objectives the
  operator defines.
- **Report-format spec** — the structured markdown report shape.
- **Reference Claude Code driver** (a slash command) demonstrating one
  agent consuming the adapter end-to-end. Proves the contract; not the
  only supported consumer.

## Data Flow

```
agent runner ──spawn──▶ mudagent --target host:55555 --manifest run.yaml
   ▲  │                      │
   │  │ JSON events (stdout) │ telnet + GMCP (AI port)
   │  ▼                      ▼
 decide cmd ──stdin──▶  [GoMud + AI port (Track 1) + playtest + gmcp]
```

1. The agent runner spawns `mudagent`, pointing it at the target AI port
   and a run manifest (active personality + goals).
2. The adapter connects, logs in to the provisioned test account, and
   streams JSON events to stdout.
3. The agent reads events, decides the next command, writes it to the
   adapter's stdin.
4. The adapter sends the command, waits one round, and streams the
   response.
5. The agent verifies goals against beacon / GMCP events plus text, and on
   completion writes a report per the spec.

## JSON Protocol

**Events** — one JSON object per line on stdout:

```json
{"type":"output","text":"<cleaned>","raw":"<ansi>"}
{"type":"gmcp","package":"Char.Vitals","data":{ }}
{"type":"beacon","event":"command_ack","data":{ }}
{"type":"status","state":"connected"}
{"type":"error","message":"..."}
```

`status.state` is one of `connected | logged_in | disconnected`.

**Commands** — one per line on stdin:

- A plain text line is sent verbatim to the MUD.
- `{"control":"quit"}` (and future control verbs) drive the adapter itself
  rather than the game.

## Error Handling

- **Adapter:** reconnect with backoff; surface disconnects as a `status`
  event; exit non-zero on a fatal/unrecoverable condition.
- **Module:** provisioning is idempotent (safe every boot); safe-mode
  confinement fails closed.
- **Engine:** AI port disabled when `AI.Port = 0`; cap refuses excess AI
  connections with a clear message.

## Testing Strategy

- **Engine (Track 1):** Go unit tests for config defaulting/validation,
  the AI connection counter, the rate-limit gate, and ANSI stripping; the
  standard GoMud boot test (server starts cleanly with the AI port
  enabled). Manual: connect a raw telnet client to the AI port and confirm
  cap + rate-limit + clean output.
- **Module (Track 2A):** Go unit tests for provisioning idempotency and
  safe-mode confinement; boot test with the module registered.
- **Adapter (Track 2B):** Go unit tests for protocol encode/decode; an
  integration test against a mock telnet server.
- **End-to-end:** run `mudagent` against the local `~/GoMud` server (built
  with the Track 1 branch + the module) with each personality and a smoke
  goals file; confirm a report is produced.

## Phasing

1. **Phase 1 — Core harness, text + standard GMCP.**
   Track 1 engine PR (AI port + primitives) + `mudagent` adapter +
   generalized content/spec + the module's account-provisioning and
   safe-mode (no beacon yet). Get an end-to-end run green against
   `~/GoMud`.
2. **Phase 2 — Structured goal verification.**
   GMCP test-beacon enrichment in the module + adapter beacon plumbing +
   goal auto-scoring from structured state.
3. **Phase 3 — Publish.**
   Land the Track 1 PR upstream. Run `go run . module package playtest` to
   produce the `.tar.gz` + sha256, host it on a `gomud-playtest-harness`
   release, and open a registry PR adding the `playtest` entry. Validate
   the full consumer path on a clean checkout: `module install playtest` →
   `make build` → run → drive with `mudagent`. Docs accompany the release.

## Repository & Development Setup

- **Published source of truth:** the standalone repo
  **`GoMudEngine/GoMud-Module-Playtest-Harness`** (engine-agnostic; clean release tags
  and tarball URLs for the registry; its own README / issues / license /
  CI). Holds the adapter, the framework content + spec, the `playtest`
  module source, and these design/plan docs. (Offer transfer to
  `GoMudEngine` later if the owner adopts it.)
- **Engine PR home:** a clean branch (`feature/ai-port`) cut from
  `origin/master` in the `~/GoMud` checkout, pushed to the existing
  `pruuk/DOGMud` fork (which has `upstream = GoMudEngine/GoMud`), and
  PR'd `pruuk/DOGMud:feature/ai-port → GoMudEngine/GoMud:master`. The
  branch is based on pristine master, so the diff is clean and untangled
  from DOGMud's own history. (GitHub allows one fork per account per
  network; DOGMud is that fork.)
- **Compile / run host:** `~/GoMud` (master, `bab2131f`). The `playtest`
  module only compiles when present under a GoMud checkout's `modules/`.
  Dev loop: keep module source in the standalone repo, copy/symlink into
  `~/GoMud/modules/playtest`, then `go generate && go build -o
  go-mud-server` (`go mod tidy` first if it adds deps). The `mudagent`
  binary and framework content build/run entirely from the standalone
  repo, pointed at the running `~/GoMud` server's AI port.
- **Why vanilla GoMud, not DOGMud:** developing against `~/GoMud` proves
  the "works on any GoMud server" claim and keeps the tool free of DOGMud
  coupling. DOGMud is out of this project's loop (except as the fork that
  hosts the engine PR branch).
- **Reference modules** (bundled in `~/GoMud`): `follow` (scripting
  functions), `gmcp` (IAC/connection handling, our beacon dependency),
  `cleanup` (event handling).

## Naming

- Server module: `playtest`
- Reference adapter binary: `mudagent`
- Standalone repo: `gomud-playtest-harness`

(Module/adapter are working names; trivially renameable before publish.)

## Open Questions / Deferred

- Whether to seek `GoMudEngine` adoption (repo transfer) after the tool
  proves out — deferred to post-Phase-3.
- Additional personalities beyond the standard three — deferred; start
  with three, add on demand.
- Final default for `AI.CommandsPerRound` / `AI.MaxConnections` — start with
  DOGMud's values (`2` / `20`); tune if needed.
