# Follow-ups

Non-blocking items deferred from reviews, to revisit later.

## mudagent adapter

- ~~**Gate stdin commands until `logged_in`.**~~ SUPERSEDED by v0.1.1: the agent
  now *intentionally* drives login + character creation over stdin from connect,
  so stdin must stay open from the start. The driver/personalities own the login
  flow (they respond to prompts in order). Gating would break agent-driven
  character creation, so it's deliberately not done.
- **Reap the stdin goroutine on server-initiated disconnect.** When the server
  closes the connection, `session.Run`'s main loop returns, but the stdin reader
  goroutine stays blocked on `Scan()` until its reader hits EOF. Harmless for the
  single-session CLI (the process exits when `Run` returns), but real debt if
  `Run` is ever embedded in a long-lived service that calls it repeatedly. Fix:
  signal the goroutine to stop (context/done channel) when the read loop exits.
- **Login prompt matching is case-sensitive** (`"Username"` / `"Password"`).
  Correct for GoMud; a server with differently-cased prompts would stall login.
  If multi-engine support is ever wanted, make the prompt markers configurable.

## playtest module

- ~~**Provisioned account spawns in "The Void".**~~ FIXED 2026-06-05:
  provisioning (and `flagExisting`, which repairs pre-existing accounts) now sets
  a void character's room to `rooms.StartRoomIdAlias`, so login resolves it to
  the configured `StartRoom`. Verified live: the account now spawns in room 1
  ("Town Square") instead of `Nowhere`.
- **New-player ghost state — addressed via guidance in v0.1.1.** New GoMud
  characters begin as a pre-tutorial ghost (0 stats, nameless). v0.1.1 handles it
  with *guidance* rather than provisioning: the engine-profile's `onboarding`
  field states the facts, the driver tells the agent to advance past the ghost
  (tutorial / choose to play), and the feel-tester grades the experience. See
  "What's next" for the optional auto-advance helper.
- **NoCombat buff** is deferred (see the module plan, Task 6). Confinement +
  death-protection are the Phase-1 safety mechanism. Revisit once the way a
  module ships/references a buff definition is understood.
- **Finalize the operator path for setting module config** (e.g.
  `Modules.playtest.SandboxZoneTag`). Verified during boot testing: a module
  overlay default overrides a hand-edited base `config.yaml`, and a nested
  `Modules.*` block in `config-overrides.yaml` does NOT merge into the module
  config map (it leaves the value empty). The admin web config UI / config API
  (flat dot-key SetVal) is the presumed correct path — verify it end-to-end and
  document the steps. Until then, the only confirmed way is editing the module's
  `data-overlays/config.yaml` (not operator-friendly). Lower priority now that
  v0.1.1 needs no account/password — the defaults are usable as-is.

---

## What's next (v0.2 ideas)

Bigger enhancements, none blocking — pick up when there's appetite.

- ~~**Run out of the box (clone → set configs → go).**~~ DONE 2026-06-07
  (commit `8b47705`), addressing Volte6's v0.1.2 review:
  - **Real committed working config files**, no `.example` to copy:
    `framework/engine-profile.yaml` (stock-GoMud defaults, placeholders filled)
    and `framework/targets.yaml` (localhost:55555, blank creds = agent creates a
    character). Un-gitignored; `.example` files removed.
  - **`/playtest` auto-discovered** — driver moved to `.claude/commands/playtest.md`,
    so running Claude Code from the repo root exposes it with no install.
  - **No build step** — driver runs `go run ./cmd/mudagent`; `--user`/`--password`
    passed only when set.
  - README agent quickstart rewritten to "clone → (edit `targets.yaml` host/port
    only if not localhost:55555) → run Claude Code → `/playtest`", with a "what
    you edit vs what just works" note.
  **E2E VALIDATED 2026-06-07:** ran the full client path against a local server
  (AI port 55555, v0.1.2 module) by driving the adapter exactly as the
  `.claude/commands/playtest.md` driver does — committed `targets.yaml` default
  (`localhost:55555`, **blank creds**) → `go run ./cmd/mudagent` → connected on
  the AI port → agent **created a character** via the new-player flow
  (`new` → user → pass×2 → blank email → `n` → `y`) → `status:logged_in` → six
  per-round `Playtest.Round` **beacons** flowed with the
  `{round, hp, hp_max, sp, sp_max, room_id}` snapshot. The only piece not
  exercised headlessly is Claude Code's literal `/playtest` slash-command
  auto-discovery + the LLM agent loop (the end user's path; I simulated the
  driver's steps by hand). So the mechanics are fully proven; a human running
  `/playtest` from a clean clone is the last confirmation — good thing to hand to
  Volte.
- ~~**Admin web pages — `/admin/playtest-config` + `/admin/playtest-about`.**~~
  DONE in v0.1.2 (Volte6's suggestion): the Config page edits the module's keys
  via the admin config API, the About page documents the module. (Eyeball the
  rendered pages once with an admin login — they couldn't be authed-rendered
  headlessly, but match gmcp's proven pattern.)
- ~~**Agent-side quickstart / make `/playtest` turnkey.**~~ PARTLY DONE: the
  README now has an agent-side "run your first playtest" quickstart and documents
  installing the `/playtest` slash command (goals file is a first-class input).
  The deeper out-of-box ergonomics (real default files vs examples, run-from-root)
  are folded into the next-session priority above.
- **No-combat restriction (buff).** Today combat safety is confinement + death
  protection. A proper `no-combat` buff applied to AI-port characters would stop
  them initiating combat at all — needs the "how a module ships + references a
  buff definition" question resolved first.
- **Auto-advance past the ghost.** Optional helper so a fresh tester reaches a
  representative (statted/named) character quickly, instead of the agent driving
  the tutorial on every first run.
- **Run manifests.** Flesh out `run.yaml` (target + creds + personality + goals
  in one file) and ship a worked example; `mudagent --manifest` is stubbed but
  under-documented.
- **Group / multi-tester runs (party mechanics).** DESIGNED + IN PROGRESS — spec
  `docs/superpowers/specs/2026-06-07-multi-agent-testing-design.md`, plan
  `docs/superpowers/plans/2026-06-07-multi-agent-testing.md`. v1 ships the general
  N-agent framework (scenario file, conductor, blackboard, combined report,
  starting templates) validated by a 2-agent party run. Deferred to follow-ups:
  lethal-PvP / per-agent death-protection (the only part that would touch
  `module/playtest/*` → a release); >2-agent soak tuning; tight turn-by-turn combat
  choreography.
- **Leaderboard exclusion, reliably.** v0.1.1 dropped on-spawn `IsAI` flagging
  (the `SaveUser`-on-spawn was non-deterministic). If excluding testers from a
  leaderboard matters, find a reliable way to flag AI-port characters (or just
  use the `ai-flag` admin command).
- **Engine niceties (small upstream PRs).** (a) An "create account offline /
  without registering an online session" variant of `users.CreateUser` — would
  have avoided the v0.1.0 phantom entirely. (b) `UserIndex.AddUser` opens the
  index file `O_RDWR` with no create, failing silently if it doesn't exist yet.
  (c) Soften the "not flagged as AI" warning, now that AI-port testers may
  legitimately be unflagged. (d) ~~Log the telnet/AI listeners at boot.~~ DONE —
  branch `feat/log-telnet-listeners` (pushed to pruuk/DOGMud; PR to open). Logs
  each successful telnet/AI listener like SSH does.
- **Adapter cleanups.** `Login.OnGMCP` is now unused (login completion is
  detected from `Char.Info`/`Room.Info` in `session.go`) — remove or repurpose.
  Reap the stdin reader goroutine on server-initiated disconnect. Make the
  login-prompt markers configurable for non-GoMud engines.
- **More examples / a second engine profile.** A non-stock-GoMud engine-profile
  example would prove the engine-agnostic claim; more goal examples per
  personality.
