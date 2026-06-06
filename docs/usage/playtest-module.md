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

1. **A GoMud build with the AI port.** The port was contributed to GoMud core
   (see [`docs/pr/2026-06-05-engine-ai-port-pr.md`](../pr/2026-06-05-engine-ai-port-pr.md))
   and is **merged into `master`** — any recent `master` has it; on an older
   build use a branch that includes it. You'll also need **Go installed** and a
   working `go build` (this is how every GoMud module is compiled in).
2. **The AI port enabled.** It ships **disabled** (`Network.AI.Port: 0`). To
   turn it on, open **`_datafiles/config.yaml`** in the root of your GoMud
   checkout (the folder with `main.go`), find the `Network:` section, and under
   `AI:` set `Port: 55555` (or any non-zero port). Full key reference in §4;
   the exact YAML and an update-safe alternative are in step 3 of the README
   Quick Start.
3. **The `gmcp` module** if you want structured state / Phase-2 beacons. It's
   bundled with GoMud by default (you don't install it separately). The adapter
   degrades to plain text without it, but goal verification is weaker.

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

There are **two separate config surfaces**, edited in **two different files**.
Getting this right is the single most common stumbling block, so read this
before you touch anything:

| What | Lives in | How to set it |
|------|----------|---------------|
| **Engine / network** (`Network.AI.*`) | `_datafiles/config.yaml` (or `_datafiles/config-overrides.yaml`) in your checkout root | Edit the file directly. Read at **boot** → restart to apply. |
| **Module** (`Modules.playtest.*`) | `modules/playtest/files/data-overlays/config.yaml` (the module's bundled defaults, present after install) | Edit that overlay file. Restart to apply. |

> ⚠️ **The module-config gotcha.** It is tempting to add a `Modules:` →
> `playtest:` block to `config.yaml` or `config-overrides.yaml`. **Don't** — it
> won't take effect. During boot testing we confirmed that the module's own
> overlay default **overrides** a hand-edited base `config.yaml` value, and a
> nested `Modules.*` block in `config-overrides.yaml` does **not** merge into the
> module config map (it leaves the value empty). So your setting is silently
> ignored. Until the admin web config UI path is finalized (see
> [`docs/followups.md`](../followups.md)), the **confirmed working way** to set
> module config is editing `modules/playtest/files/data-overlays/config.yaml`.
> (The module needs no account/password, so this rarely matters in practice —
> the defaults are usable as-is.)

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
| `SafeMode` | `true` | Apply structural safety to AI-port testers (see §6). |
| `SandboxZoneTag` | *(empty)* | If set, confine AI-port testers to rooms carrying this tag. Empty = no confinement. |
| `DeathProtection` | `true` | Protect AI-port characters from permadeath (high extra-lives). |
| `Beacons` | `true` | Emit a `Playtest.Round` GMCP beacon each round to AI-port sessions (requires the bundled `gmcp` module; see §4a). |

There is **no `AccountName`/`AccountPassword`** — the module does not provision
or own an account. The agent logs in (or creates a character via the normal
new-player flow) like any player; the module's behaviors key off the **AI-port
connection**.

### 4a. Beacons (structured verification, Phase 2)

When `Beacons` is on, the module hooks each game round (`events.NewRound`) and
sends a `Playtest.Round` GMCP package to every live session **on the AI port**:

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

The module creates **nothing** at boot — no account, no character. If `Enabled`,
it simply **registers behaviors** that key off the AI-port connection:

1. The per-round `Playtest.Round` beacon (if `Beacons`).
2. The structural safe-mode snap-back (if `SafeMode` + a `SandboxZoneTag`) and
   death protection on AI-port spawn (if `DeathProtection`) — see §6.
3. The `ai-flag` / `ai-list` admin commands.

The test character is created/logged-in by the **agent** at run time (next), not
provisioned by the module.

### During a test run (the adapter + your agent)

```
your agent ──spawn──▶ mudagent --target host:55555 --manifest run.yaml
   ▲  │  JSON events (stdout)        │ telnet + GMCP, on the AI port
   │  ▼                              ▼
 decide command ──stdin──▶  [GoMud + AI port + playtest + gmcp]
```

1. Your agent spawns `mudagent` with a run manifest.
2. The adapter opens a telnet connection **to the AI port** and performs IAC/GMCP
   negotiation (server `WILL GMCP` → client `DO GMCP` → `Core.Hello` /
   `Core.Supports.Set`). With `--user`/`--password` it auto-logs-in; otherwise
   the agent drives login — and **creates a character via the normal new-player
   flow if none exists** (`new` → username → password → …). New GoMud characters
   start as a pre-tutorial ghost; the agent advances past it like a real player.
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

> **See it concretely.** [`framework/goals/examples/`](../../framework/goals/examples/)
> has a worked example per personality — the scenario, the goals YAML you'd
> write, and the report you should expect back. The bug-finder one is a real
> find that became upstream fix
> [GoMud PR #602](https://github.com/GoMudEngine/GoMud/pull/602).

---

## 6. Safe mode — what it really means

The original design imagined "intercept harmful actions and refuse them."
Validation against GoMud showed that's not possible the way you'd hope:
**combat applies damage synchronously inside the combat step**, so a module
event listener can't cancel an attack after it's begun. Safe mode is therefore
**structural** — it prevents the tester from being *in a position* to harm live
players, rather than vetoing individual actions:

- **Sandbox confinement (optional).** If `SandboxZoneTag` is set, an AI-port
  tester is confined to rooms carrying that tag (room tags are GoMud's documented
  extensibility hook). A move that would leave the sandbox is **snapped back**.
  This **fails closed**: if the destination isn't provably inside the sandbox,
  the tester is returned, not allowed to roam.
- **Death protection.** With `DeathProtection`, an AI-port character is shielded
  from permadeath (high extra-lives) on spawn, working within GoMud's global
  permadeath setting (which a module can't toggle per-account).
- **No-combat restriction is deferred** (see Limitations / `docs/followups.md`).
  Applying a buff by id isn't yet resolved, so combat safety today is delivered
  by confinement + death protection, not a no-combat flag.

If you run **without** a sandbox zone, the tester roams your live world as a
death-protected character. That's fine for many servers, but if you have PvP or
destructive interactions you care about, **set a `SandboxZoneTag`** and tag a
contained area.

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
4. **Least privilege for the tester character.** Keep `SafeMode` on, set a
   `SandboxZoneTag`, give the character a strong password when you create it, and
   don't grant it admin.

### Operational cautions

- **Don't expose the AI port to the public internet** without deliberate
  thought. It's a real login surface (anyone reaching it can create/log into a
  character). Bind it where only your agent runner can reach it, or firewall it.
- **Choose a strong password** when the agent creates the tester character — it's
  a normal account on a real login surface.
- **The cap and rate limit are guardrails, not security.** `AI.MaxConnections`
  and `AI.CommandsPerRound` bound resource use; they aren't authentication.

---

## 9. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| **"Invalid login" / can't log in** | The character doesn't exist yet (nothing is provisioned). | Run `mudagent` **without** `--user`/`--password` and have the agent create one via the new-player flow (`new` → username → password → …); pass those credentials on later runs to auto-login. |
| **"…not flagged as AI but connected on the AI port"** | **Benign.** The module keys off the AI-port connection, not the `IsAI` flag, so it doesn't pre-flag accounts. | Nothing to fix — beacons/safe-mode work regardless. Use `ai-flag <name>` if you want the character excluded from a leaderboard. |
| **Adapter can't connect** | AI port not enabled (`Network.AI.Port: 0`), wrong port, or firewalled. | Set a non-zero `Network.AI.Port` in `_datafiles/config.yaml`, restart, confirm it's listening (`netstat -an \| grep 55555`), and make sure `--target` matches. |
| **A config change had no effect** | Engine config (`Network.*`) is read at **boot**; module config lives in the overlay; module **code** changes need a recompile. | Restart after config/overlay edits; run `go generate ./... && go build` after changing module code. |
| **"AI connection pool is full"** | `Network.AI.MaxConnections` reached. | Close stale sessions or raise the cap. |
| **Commands silently dropped** | Hitting `Network.AI.CommandsPerRound`. | Pace your agent to the per-round `beacon` tick. |
| **No `beacon` events** | `gmcp` module absent, `Beacons: false`, or no client logged in on the AI port yet. | Ensure `gmcp` is present (bundled by default) and `Beacons: true`; beacons fire per round once a client is logged in on the AI port. |
| **No GMCP state at all** | `gmcp` module not present, or the client didn't complete the GMCP handshake. | Confirm the `gmcp` module is compiled in. |
| **Tester wandered into live areas** | No `SandboxZoneTag` set, or the target zone has no rooms carrying that tag. | Set `SandboxZoneTag` and tag a contained area. |

---

## 10. Uninstall / cleanup

```bash
go run . module remove playtest
go generate && go build -o go-mud-server
```

Any character the agent created persists as normal account data unless you
delete it through normal admin tooling. If you flagged it `IsAI` (via `ai-flag`),
clear it with `ai-flag <name> off` while the module is still installed.
