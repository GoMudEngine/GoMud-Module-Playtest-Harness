# PR: Add an opt-in, capped, rate-limited AI-client telnet port

> This file is the canonical PR description for the Track-1 engine change.
> Paste it into the GitHub PR body when opening
> `pruuk/DOGMud:feature/ai-port → GoMudEngine/GoMud:master`.

## TL;DR

Adds an **optional, disabled-by-default** dedicated telnet port for AI/test
clients, with its own connection cap, per-round command rate limit, and
clean (ANSI-stripped) output, plus a persisted `IsAI` flag on user records.
**With default config this PR changes nothing** — the AI port is off
(`AIPort: 0`) until an operator enables it.

## Why

I'm building a **generalized AI playtest harness** for GoMud: a way for any
operator to point an AI agent at their server, have it play through a set of
goals using standard "tester personalities," and emit a report. This is the
reusable, de-DOGMud'd generalization of infrastructure my DOGMud fork already
runs in production.

The harness ships as community artifacts (a registry module + an external
adapter binary + framework content) — but those can't provide the one thing
that must live in the engine: a **dedicated, bounded port for AI traffic**.
A second listener, a connection-type tag, a per-type connection cap, output
shaping, and an input-loop rate gate all sit below the plugin/module API, so
they can only be contributed to core. This PR is exactly that core slice and
nothing more; all *policy* (account provisioning, sandbox/safe-mode, flagging
commands) stays in a separate, opt-in community module.

(Heads-up was given to the maintainer on Discord that this PR — the AI port
"and other stuff" — was coming; this description is written so reviewers who
weren't part of that conversation have full context.)

## What this PR adds

All additions are gated behind `AIPort > 0`. Nothing activates unless an
operator opts in.

1. **A dedicated AI telnet listener** — a second `net.Listener` on a
   configurable `AIPort`, separate from the human telnet port(s).
2. **A connection type** — `ConnType` (`ConnHuman` / `ConnAI`) recorded on
   `ConnectionDetails` at accept time, readable (atomically) for the
   connection's lifetime.
3. **An independent connection cap** — `MaxAIConnections` (default `20`),
   counted separately from human connections via a new
   `ActiveAIConnectionCount()`. The cap'd-out client gets a clear message and
   is disconnected.
4. **Clean output for AI clients** — AI connections strip ANSI SGR escape
   codes from output so agents read plain text. Telnet IAC sequences (incl.
   GMCP) are preserved untouched.
5. **Per-round command rate limiting** — `AICommandsPerRound` (default `2`)
   bounds how many commands an AI connection may submit per game round;
   excess commands are dropped with a notice.
6. **A persisted `IsAI` user flag** — `bool` on `UserRecord`
   (`yaml:"isai,omitempty"`), the reusable primitive that policy code (e.g.
   leaderboard exclusion) can filter on.

## What this PR deliberately does NOT add

To keep the diff thin and the responsibility boundary clean, this PR has **no
policy**:

- No auto-provisioning of accounts.
- No safe-mode / sandbox confinement / combat restrictions.
- No `IsAI` flagging commands.
- No leaderboard changes.

Those live in the separate, opt-in `playtest` community module (published to
the module registry), built on top of the primitives above.

## Backwards compatibility / default behavior

- `AIPort` ships as `0` (**disabled**). A stock server opens the same ports it
  does today; this PR is a no-op until configured.
- `TelnetListenOnPort` gains a `connType` parameter; all existing call sites
  are updated to pass `ConnHuman`, preserving current behavior exactly.
- `connections.Add(...)` gains a **variadic** `connType` argument, so any
  out-of-tree callers continue to compile unchanged.

## New configuration

Under `Network:` in `config.yaml`:

| Key | Default | Meaning |
|-----|---------|---------|
| `AIPort` | `0` | Telnet port for AI clients. `0` disables. Set e.g. `55555` to enable. |
| `MaxAIConnections` | `20` | Max concurrent AI connections (independent of human cap). |
| `AICommandsPerRound` | `2` | Max commands an AI connection may submit per round. |

Plus `UserRecord.IsAI` (`isai` in user YAML), defaulting to `false`/omitted.

## Behavior details (when enabled)

- **Cap reached:** new AI connection receives
  `!!! AI connection pool is full. Try again later. !!!` and is closed.
- **Rate limit hit:** the command is dropped and the client is told
  `Command dropped — AI rate limit (N/round). Wait for the next round.`
- **Greeting:** AI-port connections get a one-line notice that the port is for
  AI clients.
- **Mismatch warnings (post-login):** an `IsAI` account on a human port, or a
  non-`IsAI` account on the AI port, gets a non-blocking warning. (Cosmetic;
  no enforcement.)

## Testing

- Unit tests added for: network config defaulting/validation, `ConnType`
  get/set, `StripAnsi`, the per-round rate limiter, and
  `ActiveAIConnectionCount`.
- `make validate` (gofmt + vet) and `make test` pass.
- Manual: with `AIPort` disabled, listeners are unchanged; with `AIPort: 55555`,
  a raw telnet client to the port sees the greeting, clean output, the cap, and
  the rate-limit notice.

## Follow-ups (not in this PR)

- The `playtest` community module (provisioning + safe-mode + flagging), via
  the module registry.
- The `mudagent` external adapter and the framework content (personalities,
  goals schema, report spec) — in the standalone
  [`gomud-playtest-harness`](https://github.com/pruuk/gomud-playtest-harness)
  repo.
