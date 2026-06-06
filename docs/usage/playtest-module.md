# Playtest Harness — Usage, Behavior & Trust

Everything an operator or integrator needs to understand **what the harness
does, what happens when you run it, and what you are (and aren't) trusting**.

> **Status: pre-release / design-stage.** The configuration keys, command
> names, and adapter flags below are the planned v1 interface. They are
> documented up front deliberately — this doc doubles as the spec the
> implementation targets — and will be kept in sync as the code lands.

---

## 1. The pieces and how they fit

| Piece | Runs where | Trust surface |
|-------|-----------|---------------|
| **AI port** (engine PR) | inside your GoMud server | core engine code, reviewed upstream |
| **`playtest` module** | compiled into your GoMud server | **full server privileges** — auditable Go source |
| **`mudagent` adapter** | a separate process on your machine | a network client; no server privileges |
| **personalities / goals / report spec** | data your agent reads | inert files |

The only piece that runs *inside* your server with full privileges is the
`playtest` module. The adapter is "just" a telnet client. The personalities
and goals are data. Keep that mental model — it's the whole basis of the
trust discussion in §8.

---

## 2. Prerequisites

1. **A GoMud build with the AI port.** The port is contributed to GoMud core
   (see [`docs/pr/2026-06-05-engine-ai-port-pr.md`](../pr/2026-06-05-engine-ai-port-pr.md)).
   Once merged, any recent `master` has it; until then you need a branch that
   includes it.
2. **The AI port enabled.** It ships **disabled** (`AI.Port: 0`). You opt in by
   setting a port (conventionally `55555`).
3. **The `gmcp` module** if you want structured state / Phase-2 beacons. It's
   bundled with GoMud by default. The adapter degrades to plain text without
   it, but goal verification is weaker.

---

## 3. Installing the `playtest` module

### Option A — via the registry (convenient)

```bash
go run . module install playtest
go generate && go build -o go-mud-server   # rebuild; modules compile in
```

What `module install` actually does (verified against GoMud's module manager):

1. Fetches the central `module-registry.yaml` from `GoMudEngine/GoMud-Modules`.
2. Finds the `playtest` entry and downloads the `.tar.gz` at its `url` (a
   tagged release on this repo).
3. **Verifies the archive's sha256** against the value in the registry entry.
4. Extracts it to `modules/playtest/` and records the install in
   `modules/modules.lock.yaml`.

The archive contains **Go source** (`playtest.go` + `files/...`), not a
binary. You then **compile it yourself**. See §8 for why that matters.

### Option B — from source (maximum control)

Skip the registry entirely: copy `modules/playtest/` from this repo into your
GoMud checkout's `modules/` directory, then `go generate && go build`. You're
building the exact source you can read here.

### Removing it

```bash
go run . module remove playtest
go generate && go build -o go-mud-server
```

---

## 4. Configuration reference

Module config lives under `Modules.playtest.*` in `config.yaml` (read by the
module via the standard plugin config API). Network config lives under
`Network.*`.

### Network (engine)

| Key | Default | Meaning |
|-----|---------|---------|
| `Network.AI.Port` | `0` | AI telnet port. `0` = disabled. Set `55555` to enable. |
| `Network.AI.MaxConnections` | `20` | Max concurrent AI connections. |
| `Network.AI.CommandsPerRound` | `2` | Max commands per AI connection per round. |

### Module (`Modules.playtest.*`)

| Key | Default | Meaning |
|-----|---------|---------|
| `Enabled` | `true` | Master switch for the module. |
| `AccountName` | `aitester` | Username of the auto-provisioned test account. |
| `AccountPassword` | *(empty)* | Password for the test account. **If empty, provisioning is skipped with a warning** — no insecure default credential is ever shipped. |
| `SafeMode` | `true` | Apply structural safety to the test account (see §6). |
| `SandboxZoneTag` | *(empty)* | If set, confine the test account to rooms carrying this tag. Empty = no confinement. |
| `DeathProtection` | `true` | Protect the test account from permadeath (high extra-lives / revive-on-death). |
| `Beacons` | `true` | Emit a `Playtest.Round` GMCP beacon each round to connected `IsAI` users (requires the bundled `gmcp` module; see §4a). |

### 4a. Beacons (structured verification, Phase 2)

When `Beacons` is on, the module hooks each game round (`events.NewRound`) and
sends a `Playtest.Round` GMCP package to every connected `IsAI` user:

```json
{"round": <n>, "hp": <int>, "hp_max": <int>, "sp": <int>, "sp_max": <int>, "room_id": <int>}
```

The adapter surfaces this as a `beacon` event
(`{"type":"beacon","event":"Round","data":{...}}`). It gives the agent a
**reliable per-round pacing tick** (replacing the brittle quiescence heuristic)
plus an atomic state snapshot to score goals against.

**Dependency:** beacons require the bundled `gmcp` module (the module calls its
exported `SendGMCPEvent`). If `gmcp` is absent, beacons disable themselves with a
startup warning — no crash — and the adapter falls back to quiescence pacing.

---

## 5. What happens at boot and during a run

### At server boot (the module)

1. If `Enabled` and `AccountPassword` is set, the module **idempotently
   ensures** the test account exists (`users.CreateUser` if missing) and is
   flagged `IsAI`. Running it every boot is safe.
2. If `SafeMode`, it applies the structural safety measures in §6 to that
   account.
3. It registers the `ai-flag` / `ai-list` admin commands.

### During a test run (the adapter + your agent)

```
your agent ──spawn──▶ mudagent --target host:55555 --manifest run.yaml
   ▲  │  JSON events (stdout)        │ telnet + GMCP, on the AI port
   │  ▼                              ▼
 decide command ──stdin──▶  [GoMud + AI port + playtest + gmcp]
```

1. Your agent spawns `mudagent` with a run manifest.
2. The adapter opens a telnet connection **to the AI port**, performs IAC/GMCP
   negotiation (server `WILL GMCP` → client `DO GMCP` → `Core.Hello` /
   `Core.Supports.Set`), and logs in using the test-account credentials.
3. The adapter streams **one JSON object per line** to stdout:
   ```json
   {"type":"status","state":"connected"}
   {"type":"status","state":"logged_in"}
   {"type":"output","text":"<clean text>","raw":"<ansi>"}
   {"type":"gmcp","package":"Char.Vitals","data":{ }}
   {"type":"gmcp","package":"Room.Info","data":{ }}
   {"type":"beacon","event":"command_ack","data":{ }}   // Phase 2
   {"type":"error","message":"..."}
   ```
   Because the connection is on the AI port, the game text is already
   ANSI-stripped by the engine; `raw` preserves the original if your agent
   wants it.
4. Your agent decides the next command and writes it to the adapter's stdin —
   a plain line is sent to the MUD verbatim; `{"control":"quit"}` (and future
   control verbs) drive the adapter itself.
5. The adapter sends the command, waits one round, and streams the response.
   **Remember the rate limit:** the engine allows `AI.CommandsPerRound` (default
   2) commands per round; excess are dropped with a notice. Your agent should
   pace itself to the round tick.
6. Your agent verifies goals against GMCP/beacon state plus text and, on
   completion, writes a **report** per the report-format spec.

---

## 6. Safe mode — what it really means

The original design imagined "intercept harmful actions and refuse them."
Validation against GoMud showed that's not possible the way you'd hope:
**combat applies damage synchronously inside the combat step**, so a module
event listener can't cancel an attack after it's begun. Safe mode is therefore
**structural** — it prevents the tester from being *in a position* to harm live
players, rather than vetoing individual actions:

- **Sandbox confinement (optional).** If `SandboxZoneTag` is set, the test
  account is confined to rooms carrying that tag (room tags are GoMud's
  documented extensibility hook). Movement that would leave the sandbox is
  refused. This **fails closed**: if confinement can't be guaranteed, the move
  is refused, not allowed.
- **No-combat restriction.** The test account is prevented from initiating or
  participating in combat against live players (via a no-combat restriction on
  the account). Prevention is up front, not a post-hoc cancel.
- **Death protection.** With `DeathProtection`, the account is shielded from
  permadeath (high extra-lives and/or a revive-on-death effect), working within
  GoMud's global permadeath setting (which a module can't toggle per-account).

If you run **without** a sandbox zone, the tester roams your live world as a
flagged, no-combat, death-protected account. That's fine for many servers, but
if you have PvP or destructive interactions you care about, **set a
`SandboxZoneTag`** and tag a contained area.

> **Leaderboards:** vanilla GoMud has no player leaderboard, so there's nothing
> to exclude the tester from out of the box. The engine PR adds the `IsAI`
> primitive precisely so that any present or future leaderboard (DOGMud already
> has one as a module) can filter test accounts with a one-line check.

---

## 7. Admin commands

| Command | What it does |
|---------|--------------|
| `ai-flag <username> [on\|off]` | Set or clear the `IsAI` flag on an account. |
| `ai-list` | List accounts flagged `IsAI` and any currently-connected AI-port sessions. |

These are module-provided (registered via the plugin command API), keeping the
engine PR free of policy.

---

## 8. Security & trust model

This is the section to read before installing **any** GoMud module, including
this one.

### What you are actually installing

`module install` downloads a `.tar.gz` of **Go source** and you **compile it
into your own server binary**. You are not running an opaque executable handed
to you — you're building readable source you can audit in this repo. This is
the same trust posture as adding any open-source Go dependency.

### What the sha256 does and doesn't guarantee

- **Does:** guarantee *integrity* — the archive you download matches the exact
  bytes recorded in the registry entry. The registry entry is added by a **PR
  the GoMud maintainer reviews and merges**, so the author can't silently swap
  the tarball later without another reviewed registry PR.
- **Does NOT:** guarantee *safety*. A matching hash means "these are the
  blessed bytes," not "these bytes are harmless." Integrity ≠ intent.

### Modules are fully privileged

A GoMud module is compiled-in Go with **no sandbox**. It can do anything the
server process can do — read/write data files, open connections, modify game
state. This is true of every module in the registry, not just this one. There
is no technical containment to fall back on.

### So what *are* the protections?

1. **Auditable source.** Read `modules/playtest/` in this repo before you
   compile it. It's small and focused.
2. **Reviewed registry entry.** The hash is gated by a maintainer-reviewed PR.
3. **Build from source if you prefer.** Skip the registry; vendor the source
   directly (§3, Option B).
4. **Least privilege for the tester account.** Keep `SafeMode` on, set a
   `SandboxZoneTag`, give the account a strong password, and don't grant it
   admin.

### Operational cautions

- **Don't expose the AI port to the public internet** without deliberate
  thought. It's a real login surface. Bind it where only your agent runner can
  reach it, or firewall it.
- **Set a strong `AccountPassword`.** The module ships no default credential
  and skips provisioning if the password is blank — by design.
- **The cap and rate limit are guardrails, not security.** `AI.MaxConnections`
  and `AI.CommandsPerRound` bound resource use; they aren't authentication.

---

## 9. Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| Adapter can't connect | AI port not enabled (`AI.Port: 0`) or firewalled. |
| "AI connection pool is full" | `AI.MaxConnections` reached; close stale sessions or raise it. |
| Commands silently dropped | Hitting `AI.CommandsPerRound`; pace to the round tick. |
| Test account doesn't exist on boot | `AccountPassword` is empty (provisioning skipped) or `Enabled: false`. Check server logs for the warning. |
| Tester wandered into live areas | No `SandboxZoneTag` set, or the target zone isn't tagged. |
| No GMCP state in events | `gmcp` module not present, or client didn't complete the GMCP handshake. |

---

## 10. Uninstall / cleanup

```bash
go run . module remove playtest
go generate && go build -o go-mud-server
```

The test account persists as data unless you remove it; clear its `IsAI` flag
with `ai-flag <name> off` (while the module is still installed) or delete the
account through normal admin tooling.
