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
| **`playtest` module** | Server-side policy keyed to the AI-port connection: per-round **beacons**, a structural **safe mode** + death protection, and `ai-flag`/`ai-list` admin commands. No account provisioning. | The **GoMud module registry** — `go run . module install playtest`. |
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
2. `mudagent` connects and logs in — into an existing character, or creating one
   via the normal new-player flow — and streams structured JSON events — clean game text (`output`), GMCP state
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

**4. (Optional) Tune the module.** There's **no account to set up** — your agent
logs into or creates a character itself (step 6). Sensible defaults ship in
**`modules/playtest/files/data-overlays/config.yaml`**; edit there only to change
them:

```yaml
# modules/playtest/files/data-overlays/config.yaml
Enabled: true
SafeMode: true
SandboxZoneTag: ""         # optional: confine the tester to a tagged zone
DeathProtection: true      # shield the tester from permadeath
Beacons: true              # per-round Playtest.Round GMCP beacons (needs the gmcp module)
```

> ⚠️ Set module config **in that overlay file** — not by adding a `Modules:` →
> `playtest:` block to `config.yaml`/`config-overrides.yaml`, where the overlay
> default wins and your value is silently ignored. See
> [Gotchas](#gotchas--troubleshooting). (The admin web config UI is the intended
> long-term path.)

**5. Start the server.**

```bash
./go-mud-server
```

The module registers its behaviors (per-round beacons, safe mode, death
protection) for any client **on the AI port** — there is no account to provision
and nothing to flag ahead of time.

**6. Drive a test run** from any machine that can reach the AI port (the same box
is fine). Download the `mudagent` binary for your OS + the framework content from
this repo's
[Releases](https://github.com/GoMudEngine/GoMud-Module-Playtest-Harness/releases),
then point it at the AI port:

```bash
# First run / no character yet — the agent creates one via the normal new-player
# flow (creating + onboarding is part of what a tester exercises). Omit creds:
mudagent --target localhost:55555

# Repeat runs with an existing character — pass its credentials to auto-login:
mudagent --target localhost:55555 --user aitester --password "your-password"
```

`mudagent` prints one JSON event per line (output / gmcp / status / beacon); your
agent reads those and writes the next command to its stdin — including, on a
first run, the new-player creation responses (`new` → username → password → …).

That's the whole loop. For the personality + goals + report conventions your
agent should follow, and worked end-to-end examples, see the
[full usage guide](docs/usage/playtest-module.md) and
[`framework/`](framework/).

---

## Run a playtest with your AI agent

The steps above set up the **server** side. The **agent** side runs on your
machine (wherever your AI tool runs) and drives the MUD through `mudagent`. Here's
the first run with **Claude Code** — any agent that can spawn a process and
read/write its stdio works the same way:

1. **Get the agent-side bits:** the `mudagent` binary for your OS (from
   [Releases](https://github.com/GoMudEngine/GoMud-Module-Playtest-Harness/releases))
   on your `PATH`, and this repo's [`framework/`](framework/) folder.
2. **Tell the agent about your world** — copy the templates and fill them in:
   ```bash
   cp framework/engine-profile.example.yaml framework/engine-profile.yaml
   cp framework/targets.example.yaml        framework/targets.yaml
   ```
   - `engine-profile.yaml` — command names, world orientation, mechanics. This is
     the one place engine-specific facts live, so the generic personalities stay
     portable. (For stock GoMud the example defaults are close.)
   - `targets.yaml` — a named target (e.g. `local`) with the AI-port `host`/`port`
     and the credentials your tester character will use. **On the first run the
     agent creates that character** via the normal new-player flow if it doesn't
     exist yet, so any username/password you choose is fine.
3. **Install the driver** as a Claude Code slash command:
   ```bash
   cp framework/drivers/playtest.md .claude/commands/playtest.md
   ```
4. **Run it** — the driver takes three inputs, `/playtest <target> <personality> <goals>`:
   ```
   /playtest local bug-finder examples/bug-finder-map-rendering.yaml
   ```
   That's **where** (the `local` target), **the role** (the `bug-finder`
   personality), and **what to test** (a goals file, path relative to
   `framework/goals/`). Claude spawns `mudagent`, connects to the AI port, logs in
   (or creates a character), plays the personality against those goals, paces on
   the per-round beacons, and writes a report to `framework/reports/`.

The [worked examples](framework/goals/examples/) give a ready goals file for each
personality (scenario → goals → expected report) — start by copying one. Swap in
`feature-tester` or `feel-tester` for a different lens; omit the goals file for a
free-form exploratory run (the agent plays to the personality with no set
objectives).

> **Not using Claude Code?** Any runtime works — `mudagent` speaks a simple
> line-in / JSON-out protocol. Use
> [`framework/drivers/playtest.md`](framework/drivers/playtest.md) as the reference
> for the loop (spawn → read events → decide → write command → pace on beacons)
> and the [personalities](framework/personalities/) as role prompts.

---

## Gotchas & troubleshooting

Setup snags, most-common first. The
[full table is in the usage guide](docs/usage/playtest-module.md#9-troubleshooting).

| Symptom | Cause & fix |
|---------|-------------|
| **"Invalid login" / agent can't log in** | The character doesn't exist yet. Run `mudagent` **without** `--user`/`--password` and have the agent create one via the new-player flow (`new` → username → password → …); on later runs pass those credentials to auto-login. |
| **`mudagent` can't connect** | AI port off or wrong number. Check `Network.AI.Port` in `_datafiles/config.yaml` is non-zero and matches `--target`, then confirm it's listening: `netstat -an \| grep 55555`. Restart the server after a config change. |
| **"…account is not flagged as AI but connected on the AI port"** | **Benign.** The module identifies testers by the AI-port connection, not the `IsAI` flag, so it doesn't pre-flag accounts — beacons/safe-mode still work. If you want the tester excluded from a leaderboard, flag it once with the `ai-flag <name>` admin command. |
| **A config change did nothing** | Engine config (`Network.*`) is read at **boot** — restart. Module config lives in the overlay file (not a `Modules:` block); adding/changing *module code* needs `go generate ./... && go build` first (modules are compiled in). |
| **No `beacon` events arrive** | The `gmcp` module must be present (bundled by default) and `Beacons` must be `true` in the module overlay. Beacons fire per round once a client is logged in on the AI port. |
| **Tester wandered into live areas** | No `SandboxZoneTag` set, or the target zone has no rooms carrying that tag. Set the tag and tag a contained area. |

---

## What it actually does to your server

Be clear-eyed about this before installing — details and rationale are in the
[usage & trust doc](docs/usage/playtest-module.md):

- **It does NOT create or own an account.** Your agent logs into a character, or
  creates one via the normal new-player flow — exactly like a real player. The
  module's behaviors key off the **AI-port connection**, so nothing is
  provisioned or flagged at boot.
- **Safe mode is structural, not magic.** A tester (any client on the AI port)
  is kept from harming live players by *confinement* (optional sandbox-zone tag,
  fail-closed snap-back) plus death protection, because GoMud applies combat
  damage synchronously and a module can't "cancel" an attack mid-flight.
- **It applies death protection** to AI-port characters (high extra-lives) and
  emits a per-round beacon to them — all scoped to the live connection.
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
- **Goals**: game-agnostic YAML *you* write. **Start with the
  [worked examples](framework/goals/examples/)** — one end-to-end example per
  personality, each with the **scenario**, the **goals file**, and the
  **expected report** (the bug-finder one is a real find that became upstream fix
  [GoMud PR #602](https://github.com/GoMudEngine/GoMud/pull/602)). For the format
  itself see the [schema](framework/goals/SCHEMA.md); for the bare shape there are
  minimal templates ([smoke](framework/goals/example-smoke.yaml),
  [beacon-scored](framework/goals/example-beacon.yaml)). `verify` conditions can
  score against `gmcp` state or per-round `beacon` state
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
