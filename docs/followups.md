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

- **★ NEXT-SESSION PRIORITY — run out of the box (clone → set configs → go).**
  From Volte6's v0.1.2 review (2026-06-06): *"I'm not sure what files are
  important and what are examples to copy and what are actual options to choose,
  and do I copy folders into a new location or what... it should have real working
  files by default rather than examples to copy... I should be able to just clone
  the repo, edit the config yaml, start up claude and type a command."* The
  harness **works** well (he found the generated report useful immediately) — this
  is purely first-run ergonomics. Goal: **clone → tweak a couple configs → run.**
  Concretely:
  - **Ship real, working default files — not `.example` templates to copy.**
    Commit a working `framework/engine-profile.yaml` (stock-GoMud defaults — it
    already nearly matches the example) and `framework/targets.yaml`
    (localhost:55555 defaults), so nothing needs copying. Keep the `.example`
    files as annotated references, and/or have the driver fall back to the
    `.example` when the real file is absent.
  - **Runnable straight from the repo root** with everything ready; the only
    expected edit is server-side (enable/set the AI port).
  - **Kill the "which files matter?" confusion** — a short, prominent "what you
    edit vs what just works" note; cleaner separation of live config vs examples.
  - **Re-walk the cold-start with fresh eyes** (pruuk has configured his own fork
    so long he's lost the initial-setup feel) — aim for near-zero setup so the
    agent + `/playtest` driver are ready immediately after a clone.
  This supersedes the docs-only "agent-side quickstart" below: the README
  quickstart shipped, but the deeper ergonomics are the real fix.
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
