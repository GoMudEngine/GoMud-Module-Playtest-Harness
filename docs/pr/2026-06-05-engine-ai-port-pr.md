# Description

Adds an **optional, disabled-by-default** dedicated telnet port for AI/test
clients, with its own connection cap, per-round command rate limit, and clean
(ANSI-stripped) output, plus a persisted `IsAI` flag on user records. **With
default config this PR changes nothing** — the AI port is off (`AI.Port: 0`)
until an operator enables it.

This is the engine slice of a generalized AI playtest harness for GoMud (a way
for operators to point an AI agent at their server, have it play through a set
of goals using standard tester "personalities," and emit a report). The
harness itself ships as community artifacts — a registry module and an external
adapter — but a dedicated, bounded port for AI traffic can only live in the
engine: a second listener, a connection-type tag, a per-type cap, output
shaping, and an input-loop rate gate all sit below the plugin/module API. This
PR is exactly that core slice and nothing more; all *policy* (account
provisioning, sandbox/safe-mode, flagging commands) stays in a separate, opt-in
community module. (A heads-up was given to the maintainer on Discord that this
change was coming; this description is written so reviewers who weren't part of
that conversation have full context.)

## Changes

- Added `Network.AI.Port` (`0` = disabled), `Network.AI.MaxConnections` (default
  `20`), and `Network.AI.CommandsPerRound` (default `2`) config fields, with
  `Validate()` defaults — mirrors the existing `SSHPort` convention.
- Added `_datafiles/config.yaml` entries for the three keys, shipped DISABLED
  (`AI.Port: 0`).
- Added `UserRecord.IsAI bool` (`yaml:"isai,omitempty"`) — a persisted flag
  marking an account as an AI/test account.
- Added `connections.ConnType` (`ConnHuman`/`ConnAI`) on `ConnectionDetails`,
  with atomic `ConnType()` / `SetConnType()`.
- Added `connections.StripAnsi` + a `stripAnsi` flag, wired into `Write` so AI
  connections receive plain text (telnet IAC preserved).
- Added `ConnectionDetails.AICommandAllowed(round, max)` — a per-round command
  rate limiter for AI connections.
- Changed `connections.Add(conn, ws)` to `Add(conn, ws, connType ...ConnType)`
  (variadic, backwards-compatible) and added `ActiveAIConnectionCount()`.
- Changed `TelnetListenOnPort(...)` to take a `connType` parameter; existing
  human/local call sites pass `ConnHuman` (behavior unchanged).
- Added an AI listener in `main()` that opens only when `AI.Port > 0`, with an
  independent cap, ANSI stripping, a pre-`SendInput` rate gate, an AI-port
  greeting, and post-login port-mismatch warnings.
- Added unit tests (testify) for config defaulting, `ConnType`, `StripAnsi`,
  the rate limiter, and `ActiveAIConnectionCount`.

---

## What this PR deliberately does NOT add

To keep the diff thin and the responsibility boundary clean, this PR contains
**no policy**: no account auto-provisioning, no safe-mode / sandbox
confinement, no `IsAI` flagging commands, no leaderboard changes. Those live in
the separate, opt-in `playtest` community module built on these primitives.

## Backwards compatibility

- `AI.Port` ships `0` (disabled): a stock server opens the same ports as before;
  this PR is a no-op until configured.
- `connections.Add(...)`'s new `connType` is **variadic**, so existing 2-arg
  callers (including out-of-tree ones) compile unchanged and default to
  `ConnHuman`.
- `TelnetListenOnPort`'s new parameter is internal; all in-tree call sites are
  updated to `ConnHuman`.

## New configuration

| Key | Default | Meaning |
|-----|---------|---------|
| `Network.AI.Port` | `0` | Telnet port for AI clients. `0` disables. Set e.g. `55555` to enable. |
| `Network.AI.MaxConnections` | `20` | Max concurrent AI connections (independent of human cap). |
| `Network.AI.CommandsPerRound` | `2` | Max commands an AI connection may submit per round. |

Plus `UserRecord.IsAI` (`isai` in user YAML), defaulting to `false`/omitted.

## Behavior when enabled

- **Cap reached:** new AI connection receives
  `!!! AI connection pool is full. Try again later. !!!` and is closed.
- **Rate limit hit:** the command is dropped and the client is told
  `Command dropped — AI rate limit (N/round). Wait for the next round.`
- **Greeting / mismatch warnings:** AI-port connections get a one-line port
  notice; a mismatched account (AI on a human port, or vice versa) gets a
  non-blocking post-login warning.

## Test plan

- [x] `go test ./internal/configs/ ./internal/connections/ ./internal/users/` — green.
- [x] `go build ./...` — clean.
- [x] Boot with `AI.Port: 0` — listeners unchanged, no AI port (no-op default).
- [x] Boot with `AI.Port: 55555` — server reaches "Server Ready" and binds
  `0.0.0.0:55555` alongside `33333`/`44444`; greeting, clean output, cap, and
  rate-limit verified over a raw telnet connection.
