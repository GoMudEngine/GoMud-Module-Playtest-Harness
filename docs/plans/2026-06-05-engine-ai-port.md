# Engine AI-Port Primitives Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dedicated, connection-capped, rate-limited AI-only telnet port plus an `IsAI` user flag to vanilla GoMud, as a clean upstream PR.

**Architecture:** This is **Track 1** of the playtest-harness project — the irreducible engine primitives a registry module cannot provide. We add three network config fields, a `ConnType` on the connection, ANSI-stripping + per-round rate-limiting for AI connections, an `ActiveAIConnectionCount`, an `IsAI` field on `UserRecord`, and the `main.go` wiring that opens the AI listener and threads the connection type through. All policy (provisioning, safe-mode, flagging) is deliberately **out of scope** — it lives in the `playtest` module (a separate plan).

**Tech Stack:** Go 1.x, GoMud engine (`GoMudEngine/GoMud`), `gopkg.in/yaml.v2`, standard `net`/`regexp`/`sync/atomic`.

**Where this runs:** All file paths below are inside the vanilla GoMud checkout at `~/GoMud` (`C:\Users\Calabe Davis\GoMud`), `origin = GoMudEngine/GoMud`. Work happens on a branch cut from `origin/master`.

**Test commands:** Use `go test ./internal/<pkg>/ -run <Name> -v` for targeted tests, `make validate` for fmt+vet, and `make test` for the full pass. (On Windows, the `go test` invocations work directly in PowerShell or via the Bash tool.)

---

### Task 0: Branch setup

**Files:** none (git only)

- [ ] **Step 1: Fetch upstream master and cut a clean branch**

```bash
cd ~/GoMud
git fetch origin
git checkout -b feature/ai-port origin/master
```

- [ ] **Step 2: Add the fork as a push remote (DOGMud is pruuk's fork of GoMud)**

```bash
git remote add fork https://github.com/pruuk/DOGMud 2>/dev/null || true
git remote -v
```
Expected: `fork` points at `https://github.com/pruuk/DOGMud`, `origin` at `GoMudEngine/GoMud`.

- [ ] **Step 3: Confirm a clean starting build**

Run: `make build`
Expected: builds with no errors (baseline before any changes).

---

### Task 1: AI network config fields + validation defaults

**Files:**
- Modify: `internal/configs/config.network.go` (struct lines 3-17, `Validate()` lines 19-61)
- Test: `internal/configs/config.network_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/configs/config.network_test.go`:

```go
package configs

import "testing"

func TestNetworkValidateAIDefaults(t *testing.T) {
	n := &Network{
		AIPort:             -1,
		MaxAIConnections:   0,
		AICommandsPerRound: 0,
	}
	n.Validate()

	if int(n.AIPort) != 0 {
		t.Errorf("AIPort: negative should clamp to 0 (disabled), got %d", int(n.AIPort))
	}
	if int(n.MaxAIConnections) != 20 {
		t.Errorf("MaxAIConnections: <1 should default to 20, got %d", int(n.MaxAIConnections))
	}
	if int(n.AICommandsPerRound) != 2 {
		t.Errorf("AICommandsPerRound: <1 should default to 2, got %d", int(n.AICommandsPerRound))
	}
}

func TestNetworkValidateAIPreservesValidValues(t *testing.T) {
	n := &Network{
		AIPort:             55555,
		MaxAIConnections:   10,
		AICommandsPerRound: 5,
	}
	n.Validate()

	if int(n.AIPort) != 55555 || int(n.MaxAIConnections) != 10 || int(n.AICommandsPerRound) != 5 {
		t.Errorf("Validate must not overwrite valid values: got AIPort=%d Max=%d Cmds=%d",
			int(n.AIPort), int(n.MaxAIConnections), int(n.AICommandsPerRound))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/configs/ -run TestNetworkValidateAI -v`
Expected: FAIL — `Network` has no field `AIPort` (compile error).

- [ ] **Step 3: Add the three fields to the `Network` struct**

In `internal/configs/config.network.go`, add these lines inside the `Network struct` (place after the `MaxSSHConnections` line):

```go
	AIPort             ConfigInt `yaml:"AIPort"`             // Dedicated telnet port for AI clients (0 = disabled)
	MaxAIConnections   ConfigInt `yaml:"MaxAIConnections"`   // Maximum number of concurrent AI connections
	AICommandsPerRound ConfigInt `yaml:"AICommandsPerRound"` // Max commands an AI connection may submit per round
```

- [ ] **Step 4: Add the defaults to `Validate()`**

In `internal/configs/config.network.go`, inside `func (n *Network) Validate()`, add (place after the `MaxSSHConnections` default block):

```go
	if n.AIPort < 0 {
		n.AIPort = 0
	}

	if n.MaxAIConnections < 1 {
		n.MaxAIConnections = 20 // default
	}

	if n.AICommandsPerRound < 1 {
		n.AICommandsPerRound = 2 // default
	}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/configs/ -run TestNetworkValidateAI -v`
Expected: PASS (both tests).

- [ ] **Step 6: Commit**

```bash
git add internal/configs/config.network.go internal/configs/config.network_test.go
git commit -m "feat(net): add AIPort/MaxAIConnections/AICommandsPerRound config

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `IsAI` flag on `UserRecord`

**Files:**
- Modify: `internal/users/userrecord.go` (struct lines 32-58)
- Test: `internal/users/userrecord_isai_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/users/userrecord_isai_test.go`:

```go
package users

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestUserRecordIsAISerializes(t *testing.T) {
	u := UserRecord{Username: "tester", IsAI: true}

	out, err := yaml.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "isai: true") {
		t.Errorf("expected 'isai: true' in YAML, got:\n%s", out)
	}

	var back UserRecord
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.IsAI {
		t.Errorf("IsAI should round-trip true, got false")
	}
}

func TestUserRecordIsAIOmittedWhenFalse(t *testing.T) {
	u := UserRecord{Username: "human"}
	out, err := yaml.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "isai:") {
		t.Errorf("isai should be omitted when false, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/users/ -run TestUserRecordIsAI -v`
Expected: FAIL — `UserRecord` has no field `IsAI` (compile error).

- [ ] **Step 3: Add the field**

In `internal/users/userrecord.go`, add inside the `UserRecord struct` YAML-tagged block (place after the `ScreenReader` line):

```go
	IsAI           bool                  `yaml:"isai,omitempty"`         // Flagged as an AI/test account
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/users/ -run TestUserRecordIsAI -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/users/userrecord.go internal/users/userrecord_isai_test.go
git commit -m "feat(users): add persisted IsAI flag to UserRecord

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: `ConnType` on `ConnectionDetails`

**Files:**
- Modify: `internal/connections/connectiondetails.go` (struct lines 115-131; add type + methods)
- Test: `internal/connections/conntype_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/connections/conntype_test.go`:

```go
package connections

import "testing"

func TestConnTypeDefaultsToHuman(t *testing.T) {
	cd := NewConnectionDetails(1, nil, nil, nil)
	if cd.ConnType() != ConnHuman {
		t.Errorf("default ConnType should be ConnHuman, got %d", cd.ConnType())
	}
}

func TestConnTypeSetGet(t *testing.T) {
	cd := NewConnectionDetails(2, nil, nil, nil)
	cd.SetConnType(ConnAI)
	if cd.ConnType() != ConnAI {
		t.Errorf("ConnType should be ConnAI after SetConnType, got %d", cd.ConnType())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/connections/ -run TestConnType -v`
Expected: FAIL — `ConnHuman`/`ConnType` undefined (compile error).

- [ ] **Step 3: Add the `ConnType` type and constants**

In `internal/connections/connectiondetails.go`, add near the top (after the `ConnState` const block, before the `ConnectionDetails` struct):

```go
// ConnType distinguishes human telnet/web connections from AI client connections.
type ConnType uint32

const (
	ConnHuman ConnType = 0
	ConnAI    ConnType = 1
)
```

- [ ] **Step 4: Add the field to the struct**

In the `ConnectionDetails struct` (lines 115-131), add:

```go
	connType ConnType
```

- [ ] **Step 5: Add the atomic getter/setter**

Add these methods anywhere in `connectiondetails.go` (e.g. after the struct's existing accessor methods):

```go
// ConnType returns the connection type (human or AI). Safe for concurrent reads.
func (cd *ConnectionDetails) ConnType() ConnType {
	return ConnType(atomic.LoadUint32((*uint32)(&cd.connType)))
}

// SetConnType sets the connection type. Set once at accept time.
func (cd *ConnectionDetails) SetConnType(t ConnType) {
	atomic.StoreUint32((*uint32)(&cd.connType), uint32(t))
}
```

(`sync/atomic` is already imported — see lines 1-15.)

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/connections/ -run TestConnType -v`
Expected: PASS (both tests).

- [ ] **Step 7: Commit**

```bash
git add internal/connections/connectiondetails.go internal/connections/conntype_test.go
git commit -m "feat(connections): add ConnType (human/AI) to ConnectionDetails

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: ANSI-strip helper + Write wiring

**Files:**
- Modify: `internal/connections/connectiondetails.go` (imports lines 1-15; struct; `Write` method lines 242-279; add helper + setter)
- Test: `internal/connections/stripansi_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/connections/stripansi_test.go`:

```go
package connections

import (
	"bytes"
	"testing"
)

func TestStripAnsiRemovesColorCodes(t *testing.T) {
	in := []byte("\x1b[31mred\x1b[0m text")
	got := StripAnsi(in)
	if !bytes.Equal(got, []byte("red text")) {
		t.Errorf("expected %q, got %q", "red text", got)
	}
}

func TestStripAnsiLeavesPlainText(t *testing.T) {
	in := []byte("no codes here")
	if got := StripAnsi(in); !bytes.Equal(got, in) {
		t.Errorf("plain text should be unchanged, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/connections/ -run TestStripAnsi -v`
Expected: FAIL — `StripAnsi` undefined (compile error).

- [ ] **Step 3: Add the `regexp` import**

In `internal/connections/connectiondetails.go`, add `"regexp"` to the import block (lines 1-15), keeping imports sorted:

```go
	"regexp"
```

- [ ] **Step 4: Add the helper + compiled regexp**

Add near the top of `connectiondetails.go` (after the `ConnType` constants from Task 3):

```go
// ansiStripRegexp matches SGR (color/style) escape sequences.
var ansiStripRegexp = regexp.MustCompile("\x1b\\[[0-9;]*m")

// StripAnsi removes ANSI SGR escape sequences from p, returning clean text.
func StripAnsi(p []byte) []byte {
	return ansiStripRegexp.ReplaceAll(p, nil)
}
```

- [ ] **Step 5: Run helper test to verify it passes**

Run: `go test ./internal/connections/ -run TestStripAnsi -v`
Expected: PASS (both tests).

- [ ] **Step 6: Add the `stripAnsi` field + setter**

In the `ConnectionDetails struct`, add:

```go
	stripAnsi bool
```

Add the setter method:

```go
// SetStripAnsi enables ANSI escape stripping on output (for AI clients).
func (cd *ConnectionDetails) SetStripAnsi(on bool) {
	cd.stripAnsi = on
}
```

- [ ] **Step 7: Wire stripping into `Write`**

In `func (cd *ConnectionDetails) Write(p []byte)` (lines 242-279), immediately after the existing block that converts `\n` to `\r\n` and the empty-length check (i.e. right before the `if cd.sshChannel != nil` branch), insert:

```go
	if cd.stripAnsi && (len(p) == 0 || p[0] != byte(term.TELNET_IAC)) {
		p = StripAnsi(p)
		if len(p) == 0 {
			return 0, nil
		}
	}
```

- [ ] **Step 8: Build to verify wiring compiles**

Run: `go build ./internal/connections/`
Expected: builds cleanly.

- [ ] **Step 9: Commit**

```bash
git add internal/connections/connectiondetails.go internal/connections/stripansi_test.go
git commit -m "feat(connections): strip ANSI on output for AI connections

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Per-round AI rate-limit method

**Files:**
- Modify: `internal/connections/connectiondetails.go` (struct; add method)
- Test: `internal/connections/ratelimit_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/connections/ratelimit_test.go`:

```go
package connections

import "testing"

func TestAICommandAllowedWithinLimit(t *testing.T) {
	cd := NewConnectionDetails(10, nil, nil, nil)
	if !cd.AICommandAllowed(1, 2) {
		t.Error("1st command in round 1 should be allowed")
	}
	if !cd.AICommandAllowed(1, 2) {
		t.Error("2nd command in round 1 should be allowed")
	}
	if cd.AICommandAllowed(1, 2) {
		t.Error("3rd command in round 1 should be denied (max 2)")
	}
}

func TestAICommandAllowedResetsNextRound(t *testing.T) {
	cd := NewConnectionDetails(11, nil, nil, nil)
	cd.AICommandAllowed(1, 1) // uses the round-1 budget
	if cd.AICommandAllowed(1, 1) {
		t.Error("2nd command in round 1 should be denied (max 1)")
	}
	if !cd.AICommandAllowed(2, 1) {
		t.Error("1st command in round 2 should be allowed after reset")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/connections/ -run TestAICommandAllowed -v`
Expected: FAIL — `AICommandAllowed` undefined (compile error).

- [ ] **Step 3: Add the counter fields**

In the `ConnectionDetails struct`, add:

```go
	aiCommandRound int64
	aiCommandCount int
```

- [ ] **Step 4: Add the method**

Add to `connectiondetails.go`:

```go
// AICommandAllowed enforces a per-round command budget for AI connections.
// It is called only from the connection's own input goroutine, so it needs no lock.
func (cd *ConnectionDetails) AICommandAllowed(currentRound int64, maxPerRound int) bool {
	if currentRound != cd.aiCommandRound {
		cd.aiCommandRound = currentRound
		cd.aiCommandCount = 0
	}
	cd.aiCommandCount++
	return cd.aiCommandCount <= maxPerRound
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/connections/ -run TestAICommandAllowed -v`
Expected: PASS (both tests).

- [ ] **Step 6: Commit**

```bash
git add internal/connections/connectiondetails.go internal/connections/ratelimit_test.go
git commit -m "feat(connections): add per-round AI command rate limiter

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: `connections.Add` connType param + `ActiveAIConnectionCount`

**Files:**
- Modify: `internal/connections/connections.go` (`Add` lines 47-64; add count fn near `ActiveConnectionCount` lines 290-296)
- Test: `internal/connections/aicount_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/connections/aicount_test.go`:

```go
package connections

import "testing"

func TestActiveAIConnectionCount(t *testing.T) {
	base := ActiveAIConnectionCount()

	human := Add(nil, nil)             // defaults to ConnHuman
	ai1 := Add(nil, nil, ConnAI)
	ai2 := Add(nil, nil, ConnAI)

	if got := ActiveAIConnectionCount(); got != base+2 {
		t.Errorf("expected %d AI connections, got %d", base+2, got)
	}

	// cleanup
	Remove(human.ConnectionId())
	Remove(ai1.ConnectionId())
	Remove(ai2.ConnectionId())

	if got := ActiveAIConnectionCount(); got != base {
		t.Errorf("expected count to return to %d after removal, got %d", base, got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/connections/ -run TestActiveAIConnectionCount -v`
Expected: FAIL — `Add` takes only 2 args / `ActiveAIConnectionCount` undefined (compile error).

- [ ] **Step 3: Add the variadic connType to `Add`**

Replace `func Add(conn net.Conn, wsConn *websocket.Conn) *ConnectionDetails {` and its body (lines 47-64) with:

```go
func Add(conn net.Conn, wsConn *websocket.Conn, connType ...ConnType) *ConnectionDetails {

	lock.Lock()
	defer lock.Unlock()

	connectCounter++

	connDetails := NewConnectionDetails(
		connectCounter,
		conn,
		wsConn,
		nil,
	)

	if len(connType) > 0 {
		connDetails.SetConnType(connType[0])
	}

	netConnections[connDetails.ConnectionId()] = connDetails

	return connDetails
}
```

- [ ] **Step 4: Add `ActiveAIConnectionCount`**

In `internal/connections/connections.go`, add after `ActiveConnectionCount` (lines 290-296):

```go
// ActiveAIConnectionCount returns the number of currently-registered AI connections.
func ActiveAIConnectionCount() int {
	lock.RLock()
	defer lock.RUnlock()

	count := 0
	for _, cd := range netConnections {
		if cd.ConnType() == ConnAI {
			count++
		}
	}
	return count
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/connections/ -run TestActiveAIConnectionCount -v`
Expected: PASS.

- [ ] **Step 6: Run the full connections + configs + users test packages**

Run: `go test ./internal/connections/ ./internal/configs/ ./internal/users/ -v`
Expected: all PASS. (The existing `Add(conn, nil)` call site in `main.go` still compiles because `connType` is variadic — it will be updated explicitly in Task 7.)

- [ ] **Step 7: Commit**

```bash
git add internal/connections/connections.go internal/connections/aicount_test.go
git commit -m "feat(connections): thread ConnType through Add; add ActiveAIConnectionCount

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Wire the AI listener, cap, rate gate, and messages into `main.go`

**Files:**
- Modify: `main.go` (`TelnetListenOnPort` def lines 1185-1229; call sites lines 326-337; input dispatch ~line 951; welcome ~line 699; post-login ~line 886)

> No unit test — `main.go` is wired into the process. Verification is build + boot + a manual AI-port connection. The logic it calls (`AICommandAllowed`, `ActiveAIConnectionCount`, `StripAnsi`, config defaults) is unit-tested in Tasks 1–6.

- [ ] **Step 1: Add the `connType` parameter to `TelnetListenOnPort`**

Change the signature (line 1185) to:

```go
func TelnetListenOnPort(hostname string, portNum int, wg *sync.WaitGroup, maxConnections int, connType connections.ConnType) net.Listener {
```

- [ ] **Step 2: Replace the cap check + Add in the accept loop**

In the accept loop (lines ~1208-1224), replace the `if maxConnections > 0 { ... }` block AND the `connections.Add(conn, nil)` handoff with:

```go
			if maxConnections > 0 {
				activeCount := connections.ActiveConnectionCount()
				if connType == connections.ConnAI {
					activeCount = connections.ActiveAIConnectionCount()
				}
				if activeCount >= maxConnections {
					if connType == connections.ConnAI {
						conn.Write([]byte("\n\n\n!!! AI connection pool is full. Try again later. !!!\n\n\n"))
					} else {
						conn.Write([]byte(fmt.Sprintf("\n\n\n!!! Server is full (%d connections). Try again later. !!!\n\n\n", activeCount)))
					}
					conn.Close()
					continue
				}
			}

			wg.Add(1)
			connDetails := connections.Add(conn, nil, connType)
			if connType == connections.ConnAI {
				connDetails.SetStripAnsi(true)
			}
			// hand off the connection to a handler goroutine so that we can continue handling new connections
			go handleTelnetConnection(connDetails, wg)
```

- [ ] **Step 3: Update the existing human call sites**

In `main()` (lines 326-337), update the two existing calls to pass `connections.ConnHuman`:

```go
	allServerListeners := make([]net.Listener, 0, len(c.Network.TelnetPort))
	for _, port := range c.Network.TelnetPort {
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			if s := TelnetListenOnPort(``, p, &wg, int(c.Network.MaxTelnetConnections), connections.ConnHuman); s != nil {
				allServerListeners = append(allServerListeners, s)
			}
		}
	}

	if c.Network.LocalPort > 0 {
		TelnetListenOnPort(`127.0.0.1`, int(c.Network.LocalPort), &wg, 0, connections.ConnHuman)
	}
```

- [ ] **Step 4: Add the AI listener**

Immediately after the `LocalPort` block from Step 3, add:

```go
	if c.Network.AIPort > 0 {
		if s := TelnetListenOnPort(``, int(c.Network.AIPort), &wg, int(c.Network.MaxAIConnections), connections.ConnAI); s != nil {
			allServerListeners = append(allServerListeners, s)
		}
	}
```

- [ ] **Step 5: Add the rate-limit gate before `worldManager.SendInput(wi)`**

At the input dispatch (line ~951), immediately BEFORE `worldManager.SendInput(wi)`, insert:

```go
		if connDetails.ConnType() == connections.ConnAI {
			netCfg := configs.GetNetworkConfig()
			if !connDetails.AICommandAllowed(int64(util.GetRoundCount()), int(netCfg.AICommandsPerRound)) {
				connections.SendTo(
					[]byte(fmt.Sprintf("Command dropped — AI rate limit (%d/round). Wait for the next round.\r\n", int(netCfg.AICommandsPerRound))),
					connDetails.ConnectionId(),
				)
				clientInput.Reset()
				userObject.SetUnsentText(``, ``)
				time.Sleep(time.Duration(10) * time.Millisecond)
				continue
			}
		}

		worldManager.SendInput(wi)
```

(`configs`, `util`, `fmt`, and `time` are already imported in `main.go`.)

- [ ] **Step 6: Add the AI-port greeting**

After the welcome splash is sent (line ~699, the `connect-splash` `SendTo`), insert:

```go
	if connDetails.ConnType() == connections.ConnAI {
		connections.SendTo([]byte("\r\nThis port is for AI clients. Human players, please use the standard telnet port.\r\n\r\n"), connDetails.ConnectionId())
	}
```

- [ ] **Step 7: Add the post-login mismatch warnings**

After `worldManager.SendEnterWorld(userObject.UserId, userObject.Character.RoomId)` (line ~886), before `clientInput.Reset()` / `continue`, insert:

```go
		if connDetails.ConnType() == connections.ConnAI && !userObject.IsAI {
			connections.SendTo([]byte("\r\nWarning: this account is not flagged as AI but connected on the AI port.\r\n"), connDetails.ConnectionId())
		} else if connDetails.ConnType() == connections.ConnHuman && userObject.IsAI {
			connections.SendTo([]byte("\r\nWarning: this AI account is connected on a human port. Please use the AI port.\r\n"), connDetails.ConnectionId())
		}
```

- [ ] **Step 8: Build**

Run: `make build`
Expected: builds cleanly with no errors.

- [ ] **Step 9: Commit**

```bash
git add main.go
git commit -m "feat(net): open AI listener, enforce cap + per-round rate limit

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Config defaults + full verification

**Files:**
- Modify: `_datafiles/config.yaml` (Network section, near `TelnetPort` / `MaxTelnetConnections`, ~line 527)

- [ ] **Step 1: Add the AI-port defaults to the shipped config — DISABLED by default**

The AI port is **opt-in**: it ships disabled (`AIPort: 0`) so a stock GoMud server's behavior is unchanged by this PR. Operators turn it on by setting a port. In `_datafiles/config.yaml`, inside the `Network:` block (next to `TelnetPort` and `MaxTelnetConnections`, matching their indentation), add:

```yaml
    AIPort: 0 # Dedicated telnet port for AI playtest clients. 0 = disabled. Set e.g. 55555 to enable.
    MaxAIConnections: 20
    AICommandsPerRound: 2
```

- [ ] **Step 2: Validate formatting and vet**

Run: `make validate`
Expected: passes (gofmt clean, vet clean).

- [ ] **Step 3: Run the full test suite**

Run: `make test`
Expected: all tests pass (including the new ones from Tasks 1–6).

- [ ] **Step 4: Boot with the AI port DISABLED and confirm no behavior change**

Run: `make run` (or `go run . | head -40` in the Bash tool)
Expected: server starts cleanly; listeners on `33333`/`44444` as before; **no** `55555` listener (AIPort is 0); no errors past data-file loading. Stop the server. This proves the PR is a no-op for default configs.

- [ ] **Step 5: Boot with the AI port ENABLED and smoke-check it**

Temporarily set `AIPort: 55555` in `_datafiles/config.yaml`, then `make run`. With the server up, open a raw telnet connection to `localhost:55555`. Expected:
- logs show a listener on `55555` alongside `33333`/`44444`,
- the "This port is for AI clients" greeting appears,
- output contains no raw ANSI color escape codes (clean text),
- submitting more than 2 commands in a single round yields the "AI rate limit" notice.

Stop the server and **revert `AIPort` back to `0`** before committing (the shipped default stays disabled).

- [ ] **Step 6: Push the branch**

```bash
git push -u fork feature/ai-port
```
Expected: branch pushed to `pruuk/DOGMud`. Open PR `pruuk/DOGMud:feature/ai-port → GoMudEngine/GoMud:master` (do this in the GitHub UI when ready).

---

## Self-Review

**Spec coverage (vs. Track 1 in the design doc):**
- AI listener on configurable `AIPort` (default 55555) → Tasks 1, 7, 8 ✅
- `ConnType` (Human/AI) recorded at accept, readable for lifetime → Tasks 3, 6, 7 ✅
- `MaxAIConnections` cap (default 20), 21st refused → Tasks 1, 6, 7 ✅
- ANSI strip for AI clients (IAC preserved) → Task 4 ✅
- `AICommandsPerRound` rate limit (default 2) → Tasks 1, 5, 7 ✅
- `IsAI` persisted user flag → Task 2 ✅
- Out of scope (provisioning, safe-mode, flagging commands) → correctly absent ✅

**Type consistency:** `ConnType`/`ConnHuman`/`ConnAI`, `ConnType()`/`SetConnType()`, `StripAnsi()`/`SetStripAnsi()`, `AICommandAllowed(int64,int)`, `ActiveAIConnectionCount()`, `AIPort`/`MaxAIConnections`/`AICommandsPerRound`, `IsAI` — names used identically across all tasks. ✅

**Placeholder scan:** no TBD/TODO/"handle appropriately"; every code step has real code and exact paths. ✅
