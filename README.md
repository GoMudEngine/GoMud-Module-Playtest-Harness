# GoMud Playtest Harness

A generalized, **engine-agnostic, agent-agnostic** AI playtest harness for
[GoMud](https://github.com/GoMudEngine/GoMud) servers. Point the AI agent of
your choice at your MUD, have it play through goals **you** define using a set
of standard tester *personalities*, and get back a structured report you can
read and act on.

> **Status: pre-release / in development.** Interfaces described here are the
> planned v1 and may shift slightly as the code lands. This README is kept in
> sync as each piece is built.

---

## What's in the box

The harness has three parts, shipped through three different channels. It
helps to keep them straight:

| Part | What it is | Where it comes from |
|------|------------|---------------------|
| **AI port** | A dedicated, capped, rate-limited telnet port for AI clients, plus an `IsAI` user flag. | A **pull request to GoMud core** (so *every* GoMud gets it). Disabled by default. |
| **`playtest` module** | Server-side policy: auto-provisions a test account, enforces a structural **safe mode**, adds flagging commands. | The **GoMud module registry** — `go run . module install playtest`. |
| **`mudagent` adapter** | A standalone binary that handles telnet/GMCP/login and exposes a simple **line-in / JSON-out** protocol your agent drives. | A **release binary** from this repo. Runs on your machine, not in the server. |
| **Framework content** | The standard personalities, the goals schema, the report-format spec, and a reference Claude Code driver. | **Files in this repo.** |

In one line: **one PR to the engine, one module in the registry, and
everything your agent actually runs lives here.**

See [`docs/design/`](docs/design/) for the full design and
[`docs/usage/playtest-module.md`](docs/usage/playtest-module.md) for the deep
dive, including the **security & trust model** (read that before you install
anything — including this).

---

## How it works (the short version)

```
your agent ──spawn──▶ mudagent ──telnet+GMCP──▶ GoMud (AI port + playtest module)
     ▲   │  JSON events (stdout)        │
     │   ▼                              │
   decide next command ──stdin──────────┘
```

1. Your agent spawns `mudagent`, pointing it at the server's AI port and a run
   manifest (which personality + which goals).
2. `mudagent` connects, logs into the auto-provisioned test account, and
   streams structured JSON events — clean game text (`output`), GMCP state
   (`gmcp`), connection `status`, and per-round `beacon` events.
3. Your agent reads events, decides the next command, writes it back — pacing on
   the per-round `Playtest.Round` beacon.
4. When the goals are met (or the run ends), your agent writes a report.

You bring the agent. The harness handles the sockets, the login, the
structured state, the per-round pacing, and the conventions.

---

## Quick start (operator)

> **New to GoMud?** This is the condensed path. The **full, hand-holding,
> step-by-step guide** — every config key, where each file lives, the
> security/trust model, and a complete troubleshooting table — is
> **[`docs/usage/playtest-module.md`](docs/usage/playtest-module.md)**. If any
> step below is unclear, that doc walks you through it.

**Before you start you need:**
- A **GoMud checkout you can build** (Go installed; `go build` works).
- A GoMud build that **includes the AI port** — it's merged into GoMud
  `master`, so any recent `master` has it (on an older build, use a branch that
  includes it). The AI port is **off by default**; you turn it on in step 3.

**All paths below are relative to the root of your GoMud checkout** — the folder
that contains `main.go` and the `_datafiles/` directory. Run the `go` commands
from there.

```bash
# 1. Install the module from the registry (downloads the source archive,
#    verifies its sha256, and extracts it to modules/playtest/):
go run . module install playtest

# 2. Rebuild. GoMud modules are compiled INTO the server binary, so a rebuild
#    is REQUIRED after installing or changing module code:
go generate ./... && go build -o go-mud-server        # (or: make build)
```

**3. Enable the AI port.** Open **`_datafiles/config.yaml`**, find the
`Network:` section, and under `AI:` change `Port` from `0` (disabled) to `55555`
(the harness convention):

```yaml
# _datafiles/config.yaml
Network:
  AI:
    Port: 55555          # 0 = disabled; any non-zero port enables the AI port
    MaxConnections: 20   # max concurrent AI clients
    CommandsPerRound: 2  # rate limit per AI client per game round
```

> Don't want to edit the bundled config? Put `Network.AI.Port: 55555` in
> **`_datafiles/config-overrides.yaml`** instead — overrides survive GoMud
> updates. Either works. Engine config is read at **boot**, so restart after
> changing it.

**4. Set the test-account password.** The module refuses to provision its test
account without one (no insecure default is ever shipped). The reliably-working
way today is to edit the module's bundled defaults at
**`modules/playtest/files/data-overlays/config.yaml`** (this file appeared when
you installed the module in step 1):

```yaml
# modules/playtest/files/data-overlays/config.yaml
AccountName: aitester
AccountPassword: "choose-a-strong-password"   # REQUIRED — empty = no provisioning
SafeMode: true
SandboxZoneTag: ""         # optional: confine the tester to a tagged zone
Beacons: true
```

> ⚠️ **Do _not_** try to set these by adding a `Modules:` → `playtest:` block to
> `config.yaml` / `config-overrides.yaml` — the module's own overlay default
> wins and your value is silently ignored (so provisioning is skipped). This is
> the #1 gotcha; see [Gotchas & troubleshooting](#gotchas--troubleshooting). The
> admin web config UI is the intended long-term path (being finalized).

**5. Start the server.** On boot the module creates the test account (if
missing), flags it `IsAI`, and applies safe mode. Confirm it worked by looking
for `provisioned AI test account` in the server log:

```bash
./go-mud-server
```

**6. Drive a test run** from any machine that can reach the AI port (the same
box is fine). Download the `mudagent` binary for your OS + the framework content
from this repo's
[Releases](https://github.com/GoMudEngine/GoMud-Module-Playtest-Harness/releases),
then point it at the AI port with the credentials from step 4:

```bash
mudagent --target localhost:55555 --user aitester --password "choose-a-strong-password"
#   mudagent prints one JSON event per line to stdout (output / gmcp / status /
#   beacon); your agent reads those and writes the next command to its stdin.
```

That's the whole loop. For the personality + goals + report conventions your
agent should follow, and a worked end-to-end example, see the
[full usage guide](docs/usage/playtest-module.md) and
[`framework/`](framework/).

---

## Gotchas & troubleshooting

Setup snags, most-common first. The
[full table is in the usage guide](docs/usage/playtest-module.md#9-troubleshooting).

| Symptom | Cause & fix |
|---------|-------------|
| **Test account never created** — no `provisioned AI test account` in the log | `AccountPassword` is empty. Set it in **`modules/playtest/files/data-overlays/config.yaml`** (Quick Start step 4) — **not** in a `Modules:` block in `config.yaml`, which the overlay overrides. Restart after the change. |
| **`mudagent` can't connect** | AI port off or wrong number. Check `Network.AI.Port` in `_datafiles/config.yaml` is non-zero and matches `--target`, then confirm it's listening: `netstat -an \| grep 55555`. Restart the server after a config change. |
| **"Invalid login"** | The account doesn't exist yet (see the first row) or `--password` doesn't match the password you set in step 4. |
| **A config change did nothing** | Engine config (`Network.*`) is read at **boot** — restart. Adding/changing *module code* needs `go generate ./... && go build` first (modules are compiled in). |
| **No `beacon` events arrive** | The `gmcp` module must be present (bundled by default) and `Modules.playtest.Beacons` must be `true`. Beacons fire per round only once a real AI client is logged in. |
| **Tester wandered into live areas** | No `SandboxZoneTag` set, or the target zone has no rooms carrying that tag. Set the tag and tag a contained area. |

---

## What it actually does to your server

Be clear-eyed about this before installing — details and rationale are in the
[usage & trust doc](docs/usage/playtest-module.md):

- **It creates an account** (the test account) at boot if missing, and flags it
  `IsAI`. Idempotent — safe to boot repeatedly.
- **Safe mode is structural, not magic.** The tester is kept from harming live
  players by *confinement and restriction* (optional sandbox-zone tag + a
  no-combat restriction + death protection), because GoMud applies combat
  damage synchronously and a module can't "cancel" an attack mid-flight. If a
  safety guarantee can't be honored, it **fails closed** (the action is
  refused, not silently allowed).
- **Modules are compiled into your binary and run with full server
  privileges.** That's true of every GoMud module. You install this by
  compiling **auditable Go source** (not an opaque blob) that you can read in
  this repo first.

---

## Personalities, goals, reports

All of this lives under [`framework/`](framework/) and is engine-agnostic — the
one place game-specific facts live is `engine-profile.yaml`.

- **Personalities** (standard three): [`bug-finder`](framework/personalities/bug-finder.md),
  [`feature-tester`](framework/personalities/feature-tester.md),
  [`feel-tester`](framework/personalities/feel-tester.md) — role prompts shaping
  how the agent plays (schema: [`personality-schema.md`](framework/personality-schema.md)).
- **Goals**: game-agnostic YAML *you* write
  ([schema](framework/goals/SCHEMA.md), [example](framework/goals/example-smoke.yaml)).
  `verify` conditions can score against `gmcp` state or per-round `beacon` state
  (`{round, hp, hp_max, sp, sp_max, room_id}`) — far less brittle than text
  scraping.
- **Engine profile**: [`engine-profile.example.yaml`](framework/engine-profile.example.yaml)
  — fill in your server's command names, world, and mechanics so the
  personalities stay generic.
- **Report**: a structured markdown document per the
  [report-format spec](framework/report-format.md).
- **Reference driver**: [`framework/drivers/playtest.md`](framework/drivers/playtest.md)
  — a Claude Code slash command demonstrating one agent consuming `mudagent`
  end-to-end. Proves the contract; not the only supported consumer.

---

## Repository layout

```
cmd/mudagent/      — the adapter binary entrypoint
internal/          — adapter packages (protocol, telnet, session)
module/playtest/   — the playtest module source (compiles inside a GoMud checkout)
framework/         — personalities, goals/report schemas, engine profile, driver
docs/
  design/   — the approved design + revision history
  plans/    — TDD implementation plans (engine PR, module, adapter, content)
  pr/       — ready-to-paste PR descriptions
  issues/   — GitHub issue drafts
  usage/    — operator + integrator documentation (start here to use it)
  e2e/      — recorded end-to-end smoke runs (input + captured event stream)
  followups.md — deferred non-blocking items
```

---

## Contributing & trust

This is community software in active development. The source is public and
auditable; the registry entry is reviewed by the GoMud maintainer; releases
are tagged. None of that makes arbitrary module code inherently "safe" — see
[the trust model](docs/usage/playtest-module.md#security--trust-model) for what
the guarantees actually are and aren't.

## License

[MIT](LICENSE) — use it for whatever you like, no strings beyond keeping the
copyright notice. (GoMud itself is also MIT.)
