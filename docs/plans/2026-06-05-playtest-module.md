# playtest Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Build the `playtest` GoMud community module — boot-time provisioning of an `IsAI`-flagged test account, structural safe-mode (sandbox confinement + death protection), and `ai-flag`/`ai-list` admin commands. Policy only; the AI-port primitives live in the engine (Track 1).

**Architecture:** A standard GoMud module registered via `init()` → `plugins.New("playtest", ...)`. **It imports GoMud-internal packages, so it only compiles inside a GoMud checkout.** Source-of-truth lives in the harness repo at `module/playtest/`; development and testing **sync that source into `~/GoMud/modules/playtest/`** and run there. It requires the `IsAI` field, so build against the engine `feature/ai-port` branch (or master once the engine PR merges).

**Tech stack:** Go, GoMud plugin API (`internal/plugins`, `internal/users`, `internal/rooms`, `internal/events`, `internal/characters`), testify for unit tests.

**Verified GoMud APIs (from investigation):**
- Boot hook: `plug.Callbacks.SetOnLoad(func(){...})` runs after data/users load (`internal/plugins/plugins.go:555`, invoked at `main.go:301`).
- Users: `users.LoadUser(name)` `(*UserRecord, error)`; `users.CreateUser(*UserRecord) error`; `users.SaveUser(UserRecord) error`; `UserRecord.IsAI bool`; `users.Exists(name)`.
- Commands: `plug.AddUserCommand(cmd, handler, allowWhenDowned, isAdminOnly)`; handler `func(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error)`; `user.SendText(...)`.
- Movement: `events.RoomChange{UserId, FromRoomId, ToRoomId, Unseen}` fires **after** the move (cannot cancel) — confinement is a **snap-back**. `rooms.LoadRoom(id) *Room`; `rooms.Room.Tags []string`; `rooms.MoveToRoom(userId, roomId, ...)`.
- Death protection: `Character.ExtraLives int` (direct). (A `NoCombat` buff exists but needs a shipped buff id — **out of Phase-1 scope**, see Task 6.)
- Config: `plug.Config.Get("Key")` returns `any`; defaults from `files/data-overlays/config.yaml` under `Modules.playtest.*`.

**Dev loop / sync (run from `~/GoMud`):**
```bash
# PowerShell:
Copy-Item -Recurse -Force ~/workspace/gomud-playtest-harness/module/playtest ~/GoMud/modules/
# then:
cd ~/GoMud && go generate ./... && go test ./modules/playtest/ && go build ./...
```
Commits land in the **harness repo** (`module/playtest/`). The `~/GoMud/modules/playtest/` copy is a build sandbox (do not commit it to GoMud).

**Test commands:** `go test ./modules/playtest/` and `go build ./...`, run inside `~/GoMud` after syncing.

---

### Task 0: Module scaffold that registers and compiles

**Files (in harness repo `module/playtest/`):** `playtest.go`, `files/data-overlays/config.yaml`

- [ ] **Step 1: Create the module config defaults**

`module/playtest/files/data-overlays/config.yaml`:

```yaml
Enabled: true
AccountName: aitester
AccountPassword: ""        # operator MUST set; empty disables provisioning
SafeMode: true
SandboxZoneTag: ""         # empty = no confinement
DeathProtection: true
```

- [ ] **Step 2: Create the module entrypoint**

`module/playtest/playtest.go`:

```go
package playtest

import (
	"embed"

	"github.com/GoMudEngine/GoMud/internal/plugins"
)

//go:embed files/*
var files embed.FS

// PlaytestModule wires AI-playtest policy on top of the engine's AI-port
// primitives: account provisioning, structural safe-mode, and admin commands.
type PlaytestModule struct {
	plug *plugins.Plugin
	cfg  Config
}

var module PlaytestModule

func init() {
	module = PlaytestModule{
		plug: plugins.New(`playtest`, `0.1`),
	}
	if err := module.plug.AttachFileSystem(files); err != nil {
		panic(err)
	}

	module.plug.Callbacks.SetOnLoad(module.onLoad)
}

// onLoad runs after the world and users are loaded.
func (m *PlaytestModule) onLoad() {
	m.cfg = loadConfig(m.plug)
	// provisioning + listeners + commands are wired in later tasks
}
```

- [ ] **Step 3: Create a minimal config loader (filled in Task 1)**

`module/playtest/config.go`:

```go
package playtest

import "github.com/GoMudEngine/GoMud/internal/plugins"

// Config is the resolved module configuration.
type Config struct {
	Enabled         bool
	AccountName     string
	AccountPassword string
	SafeMode        bool
	SandboxZoneTag  string
	DeathProtection bool
}

func loadConfig(p *plugins.Plugin) Config { return Config{} } // implemented in Task 1
```

- [ ] **Step 4: Sync, generate, build**

```bash
Copy-Item -Recurse -Force ~/workspace/gomud-playtest-harness/module/playtest ~/GoMud/modules/
cd ~/GoMud && go generate ./... && go build ./...
```
Expected: `modules/all-modules.go` now imports `.../modules/playtest`; build is clean.

- [ ] **Step 5: Commit (harness repo)**

```bash
cd ~/workspace/gomud-playtest-harness
git add module/playtest/
git commit -m "feat(playtest): module scaffold (registers, compiles into GoMud)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 1: Config loader

**Files:** `module/playtest/config.go`, `module/playtest/config_test.go`

- [ ] **Step 1: Write the failing test**

`module/playtest/config_test.go`:

```go
package playtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	// fakeGetter simulates plug.Config.Get returning nil for unset keys.
	c := buildConfig(func(string) any { return nil })
	assert.False(t, c.Enabled) // nil -> zero value; real defaults come from overlay yaml
	assert.Equal(t, "", c.AccountName)
}

func TestConfigReadsValues(t *testing.T) {
	vals := map[string]any{
		"Enabled":         true,
		"AccountName":     "aitester",
		"AccountPassword": "secret",
		"SafeMode":        true,
		"SandboxZoneTag":  "playtest-sandbox",
		"DeathProtection": true,
	}
	c := buildConfig(func(k string) any { return vals[k] })
	assert.True(t, c.Enabled)
	assert.Equal(t, "aitester", c.AccountName)
	assert.Equal(t, "secret", c.AccountPassword)
	assert.Equal(t, "playtest-sandbox", c.SandboxZoneTag)
	assert.True(t, c.DeathProtection)
}
```

- [ ] **Step 2: Run (sync first), verify fail**

```bash
Copy-Item -Recurse -Force ~/workspace/gomud-playtest-harness/module/playtest ~/GoMud/modules/
cd ~/GoMud && go test ./modules/playtest/ -run TestConfig -v
```
Expected: FAIL (`buildConfig` undefined).

- [ ] **Step 3: Implement**

Replace `module/playtest/config.go`:

```go
package playtest

import "github.com/GoMudEngine/GoMud/internal/plugins"

// Config is the resolved module configuration.
type Config struct {
	Enabled         bool
	AccountName     string
	AccountPassword string
	SafeMode        bool
	SandboxZoneTag  string
	DeathProtection bool
}

// getter abstracts plug.Config.Get for testability.
type getter func(string) any

func asString(v any) string { s, _ := v.(string); return s }
func asBool(v any) bool     { b, _ := v.(bool); return b }

// buildConfig resolves config from a getter. Defaults for unset keys come from
// the module's data-overlays/config.yaml, so a nil getter yields zero values.
func buildConfig(get getter) Config {
	return Config{
		Enabled:         asBool(get("Enabled")),
		AccountName:     asString(get("AccountName")),
		AccountPassword: asString(get("AccountPassword")),
		SafeMode:        asBool(get("SafeMode")),
		SandboxZoneTag:  asString(get("SandboxZoneTag")),
		DeathProtection: asBool(get("DeathProtection")),
	}
}

// loadConfig reads the module's live config via the plugin API.
func loadConfig(p *plugins.Plugin) Config {
	return buildConfig(func(k string) any { return p.Config.Get(k) })
}
```

- [ ] **Step 4: Sync, run, verify pass; commit**

```bash
Copy-Item -Recurse -Force ~/workspace/gomud-playtest-harness/module/playtest ~/GoMud/modules/
cd ~/GoMud && go test ./modules/playtest/ -run TestConfig -v
```
Expected: PASS. Then commit in the harness repo:
```bash
cd ~/workspace/gomud-playtest-harness
git add module/playtest/config.go module/playtest/config_test.go
git commit -m "feat(playtest): config loader

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Boot-time account provisioning

> The exact account-creation sequence must mirror GoMud's real new-character flow. Before writing, READ `internal/inputhandlers/login.go` (the `FinalizeLoginOrCreate` / create path) and `internal/users/users.go` (`CreateUser`, `NewUserRecord`, `Exists`, `LoadUser`, `SaveUser`) and `internal/characters/character.go` (`New`) in `~/GoMud`, and replicate the minimal valid sequence (username, hashed password, a starting `Character` with a valid starting room/race). The skeleton below names the contract; fill the creation body from the real flow.

**Files:** `module/playtest/provision.go`

- [ ] **Step 1: Read the real creation flow** (no code yet) — note the exact calls used to create a brand-new account headlessly: how the password is hashed/set, how `characters.New()` is initialized, and how the starting room/zone is assigned.

- [ ] **Step 2: Implement provisioning**

`module/playtest/provision.go` (adjust the creation body to match the real flow read in Step 1):

```go
package playtest

import (
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ensureTestAccount idempotently makes sure the configured AI-test account
// exists and is IsAI-flagged. Safe to call every boot.
func (m *PlaytestModule) ensureTestAccount() {
	if !m.cfg.Enabled {
		return
	}
	if m.cfg.AccountName == "" || m.cfg.AccountPassword == "" {
		mudlog.Warn("playtest", "msg", "provisioning skipped: AccountName/AccountPassword not set")
		return
	}

	if users.Exists(m.cfg.AccountName) {
		m.flagExisting()
		return
	}

	// Create the account, mirroring the real new-character flow (see Step 1).
	// Pseudocode contract — replace with the verified sequence:
	//   u := users.NewUserRecord(0, 0)
	//   u.Username = m.cfg.AccountName
	//   u.SetPassword(m.cfg.AccountPassword)   // use the same hashing the create flow uses
	//   u.Character = characters.New()
	//   u.Character.Name = m.cfg.AccountName
	//   <assign starting room/zone/race exactly as the create flow does>
	//   u.IsAI = true
	//   if err := users.CreateUser(u); err != nil { mudlog.Error(...) ; return }
	//   m.applyDeathProtection(u)  // Task 5
	//   users.SaveUser(*u)

	mudlog.Info("playtest", "msg", "provisioned AI test account", "name", m.cfg.AccountName)
}

// flagExisting ensures an already-existing account carries the IsAI flag.
func (m *PlaytestModule) flagExisting() {
	u, err := users.LoadUser(m.cfg.AccountName)
	if err != nil {
		mudlog.Error("playtest", "msg", "load existing test account", "error", err)
		return
	}
	if !u.IsAI {
		u.IsAI = true
		if err := users.SaveUser(*u); err != nil {
			mudlog.Error("playtest", "msg", "save IsAI flag", "error", err)
		}
	}
}
```

- [ ] **Step 3: Wire into onLoad**

In `playtest.go`, extend `onLoad`:

```go
func (m *PlaytestModule) onLoad() {
	m.cfg = loadConfig(m.plug)
	m.ensureTestAccount()
}
```

- [ ] **Step 4: Sync, build, boot-verify**

```bash
Copy-Item -Recurse -Force ~/workspace/gomud-playtest-harness/module/playtest ~/GoMud/modules/
cd ~/GoMud && go build ./...
```
Then set `Modules.playtest.AccountPassword` in `~/GoMud/_datafiles/config-overrides.yaml` (or config.yaml), boot the server, and confirm: the server log shows "provisioned AI test account", and the user file (e.g. `_datafiles/.../users/aitester.yaml`) exists with `isai: true`. Reboot once more and confirm it does NOT re-create (idempotent) — log should be silent or show the flag already set.

- [ ] **Step 5: Commit (harness repo)**

```bash
cd ~/workspace/gomud-playtest-harness
git add module/playtest/provision.go module/playtest/playtest.go
git commit -m "feat(playtest): idempotent boot-time test-account provisioning

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: ai-flag / ai-list admin commands

**Files:** `module/playtest/commands.go`

- [ ] **Step 1: Implement the commands**

`module/playtest/commands.go`:

```go
package playtest

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func (m *PlaytestModule) registerCommands() {
	m.plug.AddUserCommand(`ai-flag`, m.cmdAIFlag, false, true) // admin only
	m.plug.AddUserCommand(`ai-list`, m.cmdAIList, false, true)
}

// ai-flag <username> [on|off]
func (m *PlaytestModule) cmdAIFlag(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		user.SendText(`Usage: ai-flag <username> [on|off]`)
		return true, nil
	}
	name := parts[0]
	on := true
	if len(parts) > 1 && strings.EqualFold(parts[1], "off") {
		on = false
	}
	target, err := users.LoadUser(name)
	if err != nil {
		user.SendText(fmt.Sprintf("No such account: %s", name))
		return true, nil
	}
	target.IsAI = on
	if err := users.SaveUser(*target); err != nil {
		user.SendText("Failed to save: " + err.Error())
		return true, nil
	}
	user.SendText(fmt.Sprintf("%s IsAI = %v", name, on))
	return true, nil
}

// ai-list
func (m *PlaytestModule) cmdAIList(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	// Implementation note: enumerate accounts via the users index/online list
	// (read internal/users for the available enumeration API) and print those
	// with IsAI=true plus any currently-connected AI-port sessions.
	user.SendText(`AI-flagged accounts:`)
	// ... list IsAI users ...
	return true, nil
}
```

> For `ai-list`, read `internal/users` for the account-enumeration API (index walk / online list) and fill the listing loop. Keep it admin-only.

- [ ] **Step 2: Wire into onLoad**

In `playtest.go` `onLoad`, add `m.registerCommands()`.

- [ ] **Step 3: Sync, build, boot-verify**

Build, boot, log in as an admin, run `ai-flag aitester on` then `ai-list` and confirm output. (`ai-list` enumeration may need the index API filled first.)

- [ ] **Step 4: Commit (harness repo)** with the two files.

---

### Task 4: Sandbox confinement (snap-back)

A pure decision helper (unit-tested) plus a `RoomChange` listener that snaps a confined AI account back when it leaves the sandbox-tagged zone.

**Files:** `module/playtest/safemode.go`, `module/playtest/safemode_test.go`

- [ ] **Step 1: Write the failing test**

`module/playtest/safemode_test.go`:

```go
package playtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldSnapBack(t *testing.T) {
	// Confined AI leaving the sandbox -> snap back.
	assert.True(t, shouldSnapBack(true, "sandbox", []string{"town"}))
	// Confined AI staying in the sandbox -> no snap back.
	assert.False(t, shouldSnapBack(true, "sandbox", []string{"sandbox", "quiet"}))
	// No sandbox tag configured -> never snap back.
	assert.False(t, shouldSnapBack(true, "", []string{"town"}))
	// Non-AI account -> never snap back.
	assert.False(t, shouldSnapBack(false, "sandbox", []string{"town"}))
}
```

- [ ] **Step 2: Sync, run, verify fail**

- [ ] **Step 3: Implement**

`module/playtest/safemode.go`:

```go
package playtest

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// shouldSnapBack decides whether a moved user must be returned to the sandbox.
// Pure for testability. Snap back when: the account is AI-flagged, a sandbox
// tag is configured, and the destination room does not carry that tag.
func shouldSnapBack(isAI bool, sandboxTag string, destTags []string) bool {
	if !isAI || sandboxTag == "" {
		return false
	}
	for _, t := range destTags {
		if t == sandboxTag {
			return false
		}
	}
	return true // fail closed: not provably in the sandbox -> refuse
}

func (m *PlaytestModule) registerSafeMode() {
	if !m.cfg.SafeMode || m.cfg.SandboxZoneTag == "" {
		return
	}
	events.RegisterListener(events.RoomChange{}, m.onRoomChange)
}

func (m *PlaytestModule) onRoomChange(e events.Event) events.ListenerReturn {
	rc, ok := e.(events.RoomChange)
	if !ok {
		return events.Continue
	}
	u := users.GetByUserId(rc.UserId)
	if u == nil {
		return events.Continue
	}
	dest := rooms.LoadRoom(rc.ToRoomId)
	if dest == nil {
		return events.Continue
	}
	if shouldSnapBack(u.IsAI, m.cfg.SandboxZoneTag, dest.Tags) {
		// RoomChange fires post-move, so return them to where they came from.
		rooms.MoveToRoom(rc.UserId, rc.FromRoomId, true)
		u.SendText(`The sandbox boundary holds you here.`)
	}
	return events.Continue
}
```

> Confirm exact signatures while implementing: `users.GetByUserId`, `rooms.LoadRoom`, `rooms.MoveToRoom`, and the `events.RoomChange` field names/`ListenerReturn` constants (the investigation reported these; verify in `~/GoMud`).

- [ ] **Step 4: Wire into onLoad** (`m.registerSafeMode()`), sync, build.

- [ ] **Step 5: Boot-verify** (manual): tag a small zone's rooms with the sandbox tag (via the build/room editor), set `SandboxZoneTag`, log in as the AI account, try to walk out, confirm you snap back. Note: requires a tagged room to exist.

- [ ] **Step 6: Commit (harness repo).**

---

### Task 5: Death protection

**Files:** `module/playtest/provision.go` (extend)

- [ ] **Step 1: Implement**

Add to `provision.go`:

```go
// applyDeathProtection shields the test account from permadeath by granting a
// large ExtraLives count (works within GoMud's global permadeath setting).
func (m *PlaytestModule) applyDeathProtection(u *users.UserRecord) {
	if !m.cfg.DeathProtection || u.Character == nil {
		return
	}
	if u.Character.ExtraLives < 999 {
		u.Character.ExtraLives = 999
	}
}
```

Call `m.applyDeathProtection(u)` in both the create path and `flagExisting` (then `SaveUser`).

- [ ] **Step 2: Sync, build, boot-verify** the provisioned account's YAML shows a high `extralives`. Commit.

---

### Task 6: NoCombat buff — scoped OUT of Phase 1 (documented follow-up)

> Applying a `NoCombat` restriction requires a buff **definition** carrying the `no-combat` flag, referenced by buff id. The module would need to ship that buff (a buff data file via overlay) and apply it by id at provisioning. The investigation did not resolve how a module ships a buff definition, so this is deferred. **Phase-1 "cannot harm live players" is delivered by sandbox confinement (Task 4) keeping the account away from live players**, plus death protection (Task 5). Record this gap in the module README and revisit when buff-shipping is understood.

- [ ] **Step 1:** Add a note to `module/playtest/README.md` documenting that NoCombat is a follow-up and confinement is the Phase-1 safety mechanism. Commit.

---

### Task 7: Full boot integration verification

- [ ] **Step 1:** Fresh `~/GoMud` build (engine `feature/ai-port` + synced module), with `Modules.playtest` configured (AccountName/AccountPassword set, SafeMode true, a real `SandboxZoneTag` on a tagged zone). `go test ./modules/playtest/` green; `go build ./...` clean; server boots to "Server Ready".
- [ ] **Step 2:** Confirm end-to-end: account auto-provisioned + `isai: true` + high `extralives`; `ai-list` shows it; confinement snaps the account back at the sandbox boundary.
- [ ] **Step 3:** Update the harness `README.md` / `docs/usage/playtest-module.md` to match the ACTUAL config keys, command names, and behavior as implemented (close any drift from the pre-release docs). Commit.

---

## Self-Review

**Spec coverage (vs. design "Track 2A — playtest module"):** provisioning (idempotent, IsAI) ✓ (Task 2); safe-mode confinement (structural, fail-closed snap-back) ✓ (Task 4); death protection ✓ (Task 5); flagging commands ✓ (Task 3); config under `Modules.playtest.*` ✓ (Task 1); registers via `init()` + embed ✓ (Task 0). **Beacon (Playtest.* GMCP)** is Phase 2 — separate plan. **NoCombat buff** explicitly deferred (Task 6) with confinement as the Phase-1 guarantee.

**Known soft spots flagged for implementation-time verification:** the exact headless account-creation sequence (Task 2 Step 1 reads the real flow), the `ai-list` enumeration API (Task 3), and several `users`/`rooms` signatures (Task 4). Each has an explicit "read the real code and confirm" instruction rather than a guess.

**Type consistency:** `Config`/`buildConfig`/`loadConfig`, `ensureTestAccount`/`flagExisting`/`applyDeathProtection`, `shouldSnapBack`/`registerSafeMode`/`onRoomChange`, `registerCommands`/`cmdAIFlag`/`cmdAIList` — consistent across tasks.
