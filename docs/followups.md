# Follow-ups

Non-blocking items deferred from reviews, to revisit later.

## mudagent adapter

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

- **Provisioned account spawns in "The Void".** The headless
  `NewUserRecord`-based creation does not assign a playable starting room, so the
  test account lands in `Nowhere`. Assign a proper start room/zone during
  provisioning (e.g. the configured start room) so an agent has somewhere to
  play. Found in the 2026-06-05 E2E smoke (`docs/e2e/`).
- **NoCombat buff** is deferred (see the module plan, Task 6). Confinement +
  death-protection are the Phase-1 safety mechanism. Revisit once the way a
  module ships/references a buff definition is understood.
- **Finalize the operator path for setting module config** (e.g.
  `Modules.playtest.AccountPassword`). Verified during boot testing: a module
  overlay default overrides a hand-edited base `config.yaml`, and a hand-edited
  nested `Modules.*` block in `config-overrides.yaml` does NOT merge into the
  module config map (it poisons the "already set" check, leaving the value
  empty). The admin web config UI / config API (flat dot-key SetVal) is the
  presumed correct path — verify it sets `AccountPassword` end-to-end, and
  document the exact steps in the usage doc. Until then, the only confirmed way
  to set it is the module's `data-overlays/config.yaml` default (not
  operator-friendly).
