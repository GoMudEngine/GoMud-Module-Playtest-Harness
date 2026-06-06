# End-to-End Smoke: mudagent ↔ live GoMud + playtest module

**Date:** 2026-06-05
**Result:** ✅ PASS — full stack verified (engine AI port + playtest module
provisioning + mudagent adapter contract).

This is the recorded end-to-end smoke proving the whole harness talks to itself:
a real GoMud server (with the engine AI-port branch and the `playtest` module)
auto-provisions a flagged test account, and the `mudagent` adapter connects on
the AI port, logs in, and streams structured JSON events that an agent consumes.

Raw captured event stream: [`2026-06-05-mudagent-smoke.jsonl`](2026-06-05-mudagent-smoke.jsonl).

## Setup

- **Server:** GoMud on branch `feature/ai-port` (the AI-port engine PR) with the
  `playtest` module compiled in.
- **Config:** `Network.AI.Port: 55555`; `Modules.playtest` provisioning enabled
  (account `aitester`). On boot the module provisioned the account with
  `isai: true` and `extralives: 999` (idempotent across reboots).
- **Adapter:** `mudagent` built from this repo (`go build ./cmd/mudagent`).

## How it was driven

The adapter handles connect + GMCP negotiation + login itself; the "agent" only
supplies game commands on stdin and reads JSON events on stdout.

```sh
# server already running on :55555 with the provisioned account
( sleep 6; printf 'look\n'; sleep 4; printf 'inventory\n'; sleep 4; \
  printf '{"control":"quit"}\n'; sleep 1 ) \
  | mudagent --target localhost:55555 --user aitester --password testpass123 \
  > smoke.jsonl
```

**Input (stdin):** `look` → `inventory` → `{"control":"quit"}`
(the username/password are NOT sent on stdin — the adapter drives login from the
server's text prompts using the `--user`/`--password` it was given).

## Result

30 JSON events emitted. Status transitions in order:

```json
{"type":"status","state":"connected"}
{"type":"status","state":"logged_in"}
{"type":"status","state":"disconnected"}
```

Structured GMCP packages received (clean — no garbage from non-GMCP
sub-negotiations like MSP music):

| Package | Count |
|---------|-------|
| `Char` | 1 |
| `Game` | 1 |
| `Gametime` | 3 |
| `Room.Info` | 1 |

Example `Room.Info` event (structured JSON the agent can score goals against,
rather than scraping text):

```json
{"type":"gmcp","package":"Room.Info","data":{"num":-1,"name":"The Void",
"area":"Nowhere","environment":"default","coords":"Nowhere, 7, 0, -99",
"exits":{"drift":-1}, ...}}
```

Plus 23 `output` events carrying the cleaned game text (ANSI-stripped) with the
original in a `raw` field.

## What this proves

- The engine AI port accepts the connection and the adapter completes GMCP
  negotiation (`IAC WILL GMCP` → `DO GMCP` → `Core.Hello`/`Core.Supports.Set`).
- The `playtest` module's boot provisioning produced a working, loginable
  account.
- The adapter's **text-prompt login** works against GoMud's real (lowercase)
  prompts, and login completion is signalled via `Room.Info`/`Char.Info` GMCP.
- The line-in / JSON-line-out contract works end to end: an agent can drive a
  session purely through `mudagent`'s stdio.

## Notes / follow-ups

- **Two bugs were found and fixed by this very smoke** (see git history): the
  login driver now matches prompts case-insensitively (GoMud sends lowercase
  `username:` / `password:`), and the telnet parser now ignores non-GMCP
  sub-negotiations (an MSP music command had been mis-emitted as a garbage GMCP
  event). Both have regression tests.
- The provisioned account spawned in **"The Void" / "Nowhere"** — the headless
  character needs a proper starting room assigned during provisioning. Tracked
  in `docs/followups.md`. It does not affect the contract demonstrated here.
- `AI.CommandsPerRound` (default 2) bounds command throughput; the agent paces by
  response quiescence (there is no per-round signal on the wire).
