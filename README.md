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

### Server side vs. client side (and when a release is cut)

It helps to split the harness down the middle:

- **Server side — the `playtest` module** runs *inside* your GoMud server. A
  server gets it (and updates to it) through the **module registry**:
  `go run . module install playtest`. The registry pins a specific release by
  sha256, so installed servers only pick up module changes when a **new release
  is cut and the registry entry is bumped**.
- **Client side — the harness** (`mudagent`, `framework/`, the `/playtest`
  driver) runs on **your** machine and points at that server. You get it (and
  updates to it) by **cloning / `git pull`-ing this repo** — no release involved.

In practice the **same person usually does both**: install the module on the
server, clone the repo to drive it. So as a contributor the rule is simply:
**changing `module/playtest/*` (server code) → cut a release + bump the registry;
changing anything else (all client-side) → just push, testers `git pull`.**

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

The steps above set up the **server** side. **This repo *is* the agent side** —
clone it, run Claude Code from the repo root, and the `/playtest` command is
already there. Everything ships with **working defaults**, so the only thing you
might change is your server's address.

1. **Clone this repo and `cd` in.** You need **Go** installed (`go run` builds the
   adapter on the fly — no separate build) and a running GoMud with the AI port
   enabled (the server steps above).
2. **Point it at your server** *(only if it isn't `localhost:55555`)*: in
   `framework/targets.yaml`, set the `local` target's `host`/`port`. Leave
   `user`/`password` blank to have the agent **create a character** on the first
   run. `framework/engine-profile.yaml` already carries stock-GoMud defaults — no
   edit needed.
3. **Run Claude Code in the repo** and call the command:
   ```
   /playtest local bug-finder examples/bug-finder-map-rendering.yaml
   ```
   Three inputs: **where** (`local` target), **role** (`bug-finder` personality),
   **what to test** (a goals file under `framework/goals/`). The agent connects,
   logs in or creates a character, plays the personality against those goals,
   paces on the per-round beacons, and writes a report to `framework/reports/`.

**No copying templates, no building binaries, no installing the command.** Swap in
`feature-tester`/`feel-tester` for a different lens; the
[worked examples](framework/goals/examples/) give a ready goals file per
personality. Omit the goals file for a free-form exploratory run.

> **What you edit vs. what just works:** out of the box you edit *at most*
> `framework/targets.yaml` (host/port). `engine-profile.yaml`, the personalities,
> the driver, and the goals examples all work as-is for stock GoMud.

> **Not using Claude Code?** Any runtime works — the adapter speaks a simple
> line-in / JSON-out protocol. Use
> [`.claude/commands/playtest.md`](.claude/commands/playtest.md) as the reference
> loop (spawn → read events → decide → write command → pace on beacons) and the
> [personalities](framework/personalities/) as role prompts.

---

## Run a MULTI-agent playtest (party / adversarial / parallel / scenario)

For testing multiplayer features — parties, PvP, contested resources, trade,
scripted social scenarios — the `/playtest-scenario` conductor runs several
independent tester agents from one **scenario file** and writes a combined report.

```
/playtest-scenario party-formation
```

- **Define the run in one file:** `framework/scenarios/<name>.yaml` — mode, a
  roster of agents (each with a role/personality and target), group goals, and an
  optional scripted choreography. Start from `framework/scenarios/template.yaml`
  or copy a worked example under `framework/scenarios/examples/` (one per mode;
  `party-formation` is validated end-to-end). Schema:
  [`framework/scenarios/SCHEMA.md`](framework/scenarios/SCHEMA.md).
- **How it coordinates:** agents interact *through the game* (real `party invite`,
  attacks, trades) plus a tiny shared blackboard for a readiness barrier, scripted
  timing, and findings collection. Reports follow
  [`framework/multi-agent-report-format.md`](framework/multi-agent-report-format.md).

> ⚠️ **Connection limit & cost — read this.**
> - GoMud caps concurrent AI clients at **`Network.AI.MaxConnections` (default
>   20)**. It's a preconfigured limit you can raise or lower in
>   `_datafiles/config.yaml` (or `config-overrides.yaml`). Set
>   `requires.max_connections` in your scenario to match so the conductor warns
>   early if your roster is too big.
> - **Running many orchestrated agents is expensive.** Each agent is an
>   independent LLM loop, so **N agents cost roughly N× the tokens and local
>   processing/time of a single `/playtest` run.** Use with caution: start with 2
>   agents, prefer the smallest roster that exercises the feature, and **watch your
>   usage rate.** Large rosters and long runs multiply quickly.

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
- **Engine profile**: [`engine-profile.yaml`](framework/engine-profile.yaml)
  — your server's command names, world, and mechanics so the personalities stay
  generic. Ships with stock-GoMud defaults; edit only if your server differs.
- **Report**: a structured markdown document per the
  [report-format spec](framework/report-format.md).
- **Reference driver**: [`.claude/commands/playtest.md`](.claude/commands/playtest.md)
  — the `/playtest` Claude Code slash command (auto-discovered from the repo),
  demonstrating one agent consuming the adapter end-to-end. Proves the contract;
  not the only supported consumer.

---

## Repository layout

```
.claude/commands/  — the /playtest slash command (auto-discovered by Claude Code)
cmd/mudagent/      — the adapter binary entrypoint
internal/          — adapter packages (protocol, telnet, session)
module/playtest/   — the playtest module source (compiles inside a GoMud checkout)
framework/         — personalities, goals/report schemas, engine-profile.yaml, targets.yaml
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
