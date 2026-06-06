# GitHub Issue draft — AI-client telnet port (engine primitives)

> Post this as a new issue on `GoMudEngine/GoMud` before opening the PR.
> Suggested labels: `enhancement`. Suggested title below.

---

**Title:** Add an opt-in, capped AI-client telnet port (engine primitives for an AI playtest harness)

## Summary

I'd like to add a small set of **opt-in, disabled-by-default** engine primitives
that enable a dedicated telnet port for AI/test clients: a separate listener, an
independent connection cap, per-round command rate limiting, ANSI-stripped
output, and a persisted `IsAI` flag on user records. With default config this is
a **no-op** — nothing changes for a stock server until an operator sets a port.

This is the engine slice of a generalized **AI playtest harness** I'm building
for GoMud (point an AI agent at your server, have it play through operator-defined
goals using standard tester "personalities," and emit a report). I gave a heads-up
on Discord that this was coming; opening this issue to get agreement on the
engine-side change as an actionable item before I send the PR.

## Why this needs to live in the engine (not a module)

A dedicated, bounded port for AI traffic can't be delivered as a registry module:
the second `net.Listener`, a connection-type tag on `ConnectionDetails`, a
per-type cap, output shaping in `Write`, and an input-loop rate gate all sit
below the plugin/module API. Everything *above* that line (account provisioning,
sandbox/safe-mode, flagging commands) will ship as a separate, opt-in community
module — none of it is proposed here.

## Proposed primitives (the whole scope)

1. `Network.AIPort` (0 = disabled), `Network.MaxAIConnections` (default 20),
   `Network.AICommandsPerRound` (default 2), following the existing `SSHPort`
   "0 disables" convention. Shipped disabled in `config.yaml`.
2. A `ConnType` (`ConnHuman`/`ConnAI`) on `ConnectionDetails`, set at accept time.
3. A dedicated AI listener that opens only when `AIPort > 0`, with an independent
   connection cap.
4. ANSI stripping on output for AI connections (telnet IAC preserved).
5. A per-round command rate limiter for AI connections.
6. A persisted `UserRecord.IsAI bool` flag — the reusable primitive that policy
   code (e.g. leaderboard exclusion) can filter on.

## Explicitly out of scope (lives in a separate module)

- Account auto-provisioning, safe-mode / sandbox confinement, `IsAI` flagging
  commands, leaderboard changes.

## Compatibility

- Disabled by default → no behavior change for existing servers.
- `connections.Add(...)` gains a **variadic** `connType`, so existing callers
  compile unchanged.
- Thin diff, primitives only, unit-tested (testify).

## Question for maintainers

Is this acceptable as an actionable item, and is the scope/shape right before I
open the PR? Happy to adjust naming, defaults, or split it further. The PR is
ready to go pending agreement here.
