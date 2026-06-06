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

> **Prerequisite:** a GoMud build that includes the AI port. Once the upstream
> PR is merged, any recent `master` has it. Until then, use a branch that
> includes it. The AI port is **off by default** — you opt in below.

From inside your GoMud checkout:

```bash
# 1. Install the module from the registry (downloads source, verifies sha256)
go run . module install playtest

# 2. Rebuild — modules compile into the server binary
go generate && go build -o go-mud-server   # or: make build

# 3. Enable the AI port and configure the module in config.yaml:
#      Network:
#        AIPort: 55555
#      Modules:
#        playtest:
#          AccountName: aitester
#          AccountPassword: "<choose-a-strong-password>"
#          SafeMode: true
#          SandboxZoneTag: ""        # optional: confine the tester to a tagged zone

# 4. Start the server. On boot the module ensures the test account exists,
#    flagged IsAI, with safe mode applied.
./go-mud-server
```

Then, on the machine driving the test (can be the same box):

```bash
# 5. Grab the adapter + framework content from this repo's releases, then:
mudagent --target localhost:55555 --manifest run.yaml
#    ...and have your agent read its stdout / write its stdin.
```

A `run.yaml` names the connection, the test-account credentials, the active
personality, and the goals file. See
[`docs/usage/playtest-module.md`](docs/usage/playtest-module.md) for the full
reference and a worked example.

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

TBD before first release.
