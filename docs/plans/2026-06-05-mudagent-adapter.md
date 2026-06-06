# mudagent Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Build `mudagent` — a single static Go binary that connects to a GoMud server, performs telnet/GMCP negotiation and text-prompt login, and exposes a line-in (stdin) / JSON-line-out (stdout) protocol so any AI agent can drive a playtest session without touching sockets.

**Architecture:** A standalone Go module in the `gomud-playtest-harness` repo (`module github.com/pruuk/gomud-playtest-harness`). The binary lives at `cmd/mudagent`; reusable logic lives in `internal/` packages: `protocol` (JSON event/command types), `telnet` (IAC/GMCP stream parsing + ANSI strip), and `session` (connect → negotiate → login → stream). No dependency on the GoMud codebase — it's a pure network client. Tested with unit tests + an integration test against an in-process scripted telnet server.

**Tech stack:** Go 1.x stdlib only (`net`, `bufio`, `encoding/json`, `regexp`), plus `gopkg.in/yaml.v3` for the run manifest and `github.com/stretchr/testify` for tests (matches GoMud's test convention).

**Protocol facts (verified against GoMud):**
- Server sends `IAC WILL GMCP` (`FF FB C9`); client must reply `IAC DO GMCP` (`FF FD C9`). Then client sends `IAC SB GMCP Core.Hello {json} IAC SE` and `IAC SB GMCP Core.Supports.Set [json-array] IAC SE`. Server pushes updates as `IAC SB GMCP <Package> <json> IAC SE` (`SB`=250, `SE`=240, IAC=255).
- Login is **text prompts** (GMCP `Char.Login` is not wired server-side): server sends a username prompt (contains `Username`), client sends the username; then a password prompt (contains `Password`), client sends the password. Success is signalled by GMCP `Char.Info`/`Room.Info` beginning to flow (no explicit "you are logged in" text).
- There is **no per-round signal** on the wire. The adapter delimits a command's response by quiescence (no new bytes for a short window), not by round ticks.
- **ANSI:** stock/upstream GoMud does NOT strip ANSI anywhere. Server-side ANSI stripping on the AI port is added by *our engine PR* (Track 1), mirroring DOGMud — so any real AI-port server the adapter talks to will have it, but it is NOT a stock-GoMud feature. The adapter therefore does NOT rely on it: it strips ANSI itself on every text token, which is correct whether or not the server already stripped (double-strip is a no-op).

**Where this runs:** All paths are in `~/workspace/gomud-playtest-harness` (`C:\Users\Calabe Davis\workspace\gomud-playtest-harness`). Test against the local `~/GoMud` server (built from `feature/ai-port`, `AIPort: 55555`).

**Test commands:** `go test ./...` from the repo root; `go build ./cmd/mudagent`.

---

### Task 0: Module + package skeleton

**Files:** Create `go.mod`, `cmd/mudagent/main.go` (stub), `.gitignore` already exists.

- [ ] **Step 1: Initialize the module**

```bash
cd ~/workspace/gomud-playtest-harness
go mod init github.com/pruuk/gomud-playtest-harness
go get github.com/stretchr/testify@v1.11.1
go get gopkg.in/yaml.v3
```

- [ ] **Step 2: Add a buildable stub**

Create `cmd/mudagent/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("mudagent: not yet implemented")
}
```

- [ ] **Step 3: Verify**

Run: `go build ./cmd/mudagent && go vet ./...`
Expected: builds clean.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum cmd/mudagent/main.go
git commit -m "feat(mudagent): module skeleton

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 1: JSON protocol types

**Files:** Create `internal/protocol/protocol.go`, `internal/protocol/protocol_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/protocol/protocol_test.go`:

```go
package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventMarshalsOnePerLine(t *testing.T) {
	line, err := Marshal(Event{Type: "status", State: "connected"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"status","state":"connected"}`, line)
}

func TestOutputEventOmitsEmptyFields(t *testing.T) {
	line, err := Marshal(Event{Type: "output", Text: "hello"})
	require.NoError(t, err)
	assert.NotContains(t, line, `"package"`)
	assert.NotContains(t, line, `"state"`)
	assert.Contains(t, line, `"text":"hello"`)
}

func TestParseCommandPlainLine(t *testing.T) {
	cmd := ParseCommand("look around")
	assert.Equal(t, CommandKindText, cmd.Kind)
	assert.Equal(t, "look around", cmd.Text)
}

func TestParseCommandControlQuit(t *testing.T) {
	cmd := ParseCommand(`{"control":"quit"}`)
	assert.Equal(t, CommandKindControl, cmd.Kind)
	assert.Equal(t, "quit", cmd.Control)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/protocol/ -v`
Expected: FAIL (package/types undefined).

- [ ] **Step 3: Implement**

Create `internal/protocol/protocol.go`:

```go
// Package protocol defines the line-oriented JSON contract between mudagent
// (stdout events / stdin commands) and an external AI agent.
package protocol

import (
	"encoding/json"
	"strings"
)

// Event is one JSON object emitted per line on stdout.
type Event struct {
	Type    string          `json:"type"`              // output | gmcp | beacon | status | error
	Text    string          `json:"text,omitempty"`    // output: cleaned text
	Raw     string          `json:"raw,omitempty"`     // output: original (with ANSI), optional
	Package string          `json:"package,omitempty"` // gmcp: package name, e.g. "Char.Vitals"
	Event   string          `json:"event,omitempty"`   // beacon: event name
	Data    json.RawMessage `json:"data,omitempty"`    // gmcp/beacon: structured payload
	State   string          `json:"state,omitempty"`   // status: connected | logged_in | disconnected
	Message string          `json:"message,omitempty"` // error: message
}

// Marshal renders an Event as a single JSON line (no trailing newline).
func Marshal(e Event) (string, error) {
	b, err := json.Marshal(e)
	return string(b), err
}

// CommandKind distinguishes a game command from an adapter control verb.
type CommandKind int

const (
	CommandKindText CommandKind = iota
	CommandKindControl
)

// Command is one parsed stdin line.
type Command struct {
	Kind    CommandKind
	Text    string // for CommandKindText
	Control string // for CommandKindControl, e.g. "quit"
}

type controlLine struct {
	Control string `json:"control"`
}

// ParseCommand interprets a stdin line: a JSON object with a "control" key is a
// control verb; anything else is sent verbatim to the MUD.
func ParseCommand(line string) Command {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "{") {
		var cl controlLine
		if err := json.Unmarshal([]byte(trimmed), &cl); err == nil && cl.Control != "" {
			return Command{Kind: CommandKindControl, Control: cl.Control}
		}
	}
	return Command{Kind: CommandKindText, Text: line}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/protocol/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/protocol/
git commit -m "feat(mudagent): JSON event/command protocol types

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: ANSI strip utility

**Files:** Create `internal/telnet/ansi.go`, `internal/telnet/ansi_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/telnet/ansi_test.go`:

```go
package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripAnsi(t *testing.T) {
	assert.Equal(t, "red text", string(StripAnsi([]byte("\x1b[31mred\x1b[0m text"))))
	assert.Equal(t, "plain", string(StripAnsi([]byte("plain"))))
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/telnet/ -run TestStripAnsi -v`
Expected: FAIL (StripAnsi undefined).

- [ ] **Step 3: Implement**

Create `internal/telnet/ansi.go`:

```go
package telnet

import "regexp"

// ansiRegexp matches CSI escape sequences (color/style and common cursor ops).
var ansiRegexp = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

// StripAnsi removes ANSI escape sequences, returning clean text. The adapter
// always strips, so it is correct whether or not the server also stripped
// (stock GoMud does not strip; our AI-port engine PR / DOGMud do — double-strip
// is a no-op).
func StripAnsi(p []byte) []byte {
	return ansiRegexp.ReplaceAll(p, nil)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/telnet/ -run TestStripAnsi -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telnet/ansi.go internal/telnet/ansi_test.go
git commit -m "feat(mudagent): ANSI strip fallback

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Telnet/GMCP stream parser

A stateful parser that consumes raw server bytes and yields three token kinds: plain text, an IAC negotiation command (e.g. WILL GMCP), and a complete GMCP sub-negotiation payload (`<package> <json>`).

**Files:** Create `internal/telnet/parser.go`, `internal/telnet/parser_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/telnet/parser_test.go`:

```go
package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParserSplitsTextAndIAC(t *testing.T) {
	p := NewParser()
	// "hi" + IAC WILL GMCP + "yo"
	toks := p.Feed([]byte{'h', 'i', IAC, WILL, GMCP, 'y', 'o'})
	assert.Equal(t, []Token{
		{Kind: TokenText, Text: []byte("hi")},
		{Kind: TokenIAC, Command: WILL, Option: GMCP},
		{Kind: TokenText, Text: []byte("yo")},
	}, toks)
}

func TestParserExtractsGMCP(t *testing.T) {
	p := NewParser()
	payload := []byte(`Char.Vitals {"hp":10}`)
	in := append([]byte{IAC, SB, GMCP}, payload...)
	in = append(in, IAC, SE)
	toks := p.Feed(in)
	assert.Len(t, toks, 1)
	assert.Equal(t, TokenGMCP, toks[0].Kind)
	assert.Equal(t, "Char.Vitals", toks[0].GMCPPackage)
	assert.JSONEq(t, `{"hp":10}`, string(toks[0].GMCPData))
}

func TestParserHandlesSplitFeeds(t *testing.T) {
	p := NewParser()
	toks := p.Feed([]byte{IAC, SB, GMCP, 'R', 'o', 'o', 'm'})
	assert.Empty(t, toks) // incomplete SB, buffered
	toks = p.Feed(append([]byte(" {}"), IAC, SE))
	assert.Len(t, toks, 1)
	assert.Equal(t, "Room", toks[0].GMCPPackage)
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/telnet/ -run TestParser -v`
Expected: FAIL (Parser undefined).

- [ ] **Step 3: Implement**

Create `internal/telnet/parser.go`:

```go
package telnet

import (
	"bytes"
	"strings"
)

// Telnet protocol bytes.
const (
	IAC  = 255
	DONT = 254
	DO   = 253
	WONT = 252
	WILL = 251
	SB   = 250
	SE   = 240
	GMCP = 201
)

// TokenKind classifies a parser output token.
type TokenKind int

const (
	TokenText TokenKind = iota
	TokenIAC            // a 3-byte IAC command (e.g. IAC WILL GMCP)
	TokenGMCP          // a complete GMCP sub-negotiation payload
)

// Token is one unit emitted by the parser.
type Token struct {
	Kind        TokenKind
	Text        []byte // TokenText
	Command     byte   // TokenIAC: WILL/WONT/DO/DONT
	Option      byte   // TokenIAC: the option byte (e.g. GMCP)
	GMCPPackage string // TokenGMCP: package name
	GMCPData    []byte // TokenGMCP: JSON payload (may be empty)
}

type parserState int

const (
	stData parserState = iota
	stIAC              // saw IAC
	stCmd              // saw IAC + (WILL/WONT/DO/DONT), expecting option
	stSB               // inside SB ... awaiting IAC SE
	stSBIAC            // inside SB and saw IAC, expecting SE
)

// Parser is a stateful telnet stream decoder. Feed bytes; get tokens. Safe to
// call across arbitrary chunk boundaries (state + partial buffers persist).
type Parser struct {
	state   parserState
	cmd     byte
	sbBuf   []byte
	textBuf []byte
}

// NewParser returns a ready parser.
func NewParser() *Parser { return &Parser{state: stData} }

func (p *Parser) flushText(out []Token) []Token {
	if len(p.textBuf) > 0 {
		t := make([]byte, len(p.textBuf))
		copy(t, p.textBuf)
		out = append(out, Token{Kind: TokenText, Text: t})
		p.textBuf = p.textBuf[:0]
	}
	return out
}

// Feed consumes bytes and returns any complete tokens produced.
func (p *Parser) Feed(b []byte) []Token {
	var out []Token
	for _, c := range b {
		switch p.state {
		case stData:
			if c == IAC {
				out = p.flushText(out)
				p.state = stIAC
			} else {
				p.textBuf = append(p.textBuf, c)
			}
		case stIAC:
			switch c {
			case WILL, WONT, DO, DONT:
				p.cmd = c
				p.state = stCmd
			case SB:
				p.sbBuf = p.sbBuf[:0]
				p.state = stSB
			case IAC:
				p.textBuf = append(p.textBuf, IAC) // escaped 0xFF
				p.state = stData
			default:
				p.state = stData // ignore other 2-byte commands
			}
		case stCmd:
			out = append(out, Token{Kind: TokenIAC, Command: p.cmd, Option: c})
			p.state = stData
		case stSB:
			if c == IAC {
				p.state = stSBIAC
			} else {
				p.sbBuf = append(p.sbBuf, c)
			}
		case stSBIAC:
			if c == SE {
				out = append(out, parseGMCP(p.sbBuf))
				p.sbBuf = p.sbBuf[:0]
				p.state = stData
			} else {
				p.sbBuf = append(p.sbBuf, c) // IAC IAC inside SB → literal
				p.state = stSB
			}
		}
	}
	out = p.flushText(out)
	return out
}

// parseGMCP splits a sub-negotiation buffer that begins with the GMCP option
// byte into a package name and JSON payload.
func parseGMCP(buf []byte) Token {
	body := buf
	if len(body) > 0 && body[0] == GMCP {
		body = body[1:]
	}
	s := string(body)
	pkg, data, _ := strings.Cut(s, " ")
	return Token{
		Kind:        TokenGMCP,
		GMCPPackage: strings.TrimSpace(pkg),
		GMCPData:    bytes.TrimSpace([]byte(data)),
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/telnet/ -run TestParser -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add internal/telnet/parser.go internal/telnet/parser_test.go
git commit -m "feat(mudagent): stateful telnet/GMCP stream parser

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: GMCP handshake framing

Helpers to build the client's outbound negotiation: reply `IAC DO GMCP`, and frame `IAC SB GMCP <pkg> <json> IAC SE`.

**Files:** Create `internal/telnet/gmcp.go`, `internal/telnet/gmcp_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/telnet/gmcp_test.go`:

```go
package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoGMCP(t *testing.T) {
	assert.Equal(t, []byte{IAC, DO, GMCP}, DoGMCP())
}

func TestFrameGMCP(t *testing.T) {
	got := FrameGMCP("Core.Hello", `{"client":"mudagent","version":"1"}`)
	want := append([]byte{IAC, SB, GMCP}, []byte(`Core.Hello {"client":"mudagent","version":"1"}`)...)
	want = append(want, IAC, SE)
	assert.Equal(t, want, got)
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/telnet/ -run "TestDoGMCP|TestFrameGMCP" -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Create `internal/telnet/gmcp.go`:

```go
package telnet

// DoGMCP is the client's acceptance of the server's WILL GMCP.
func DoGMCP() []byte { return []byte{IAC, DO, GMCP} }

// FrameGMCP builds a GMCP sub-negotiation: IAC SB GMCP "<pkg> <json>" IAC SE.
func FrameGMCP(pkg, json string) []byte {
	out := []byte{IAC, SB, GMCP}
	out = append(out, []byte(pkg)...)
	out = append(out, ' ')
	out = append(out, []byte(json)...)
	out = append(out, IAC, SE)
	return out
}

// SupportedPackages is the default Core.Supports.Set list the adapter enables.
var SupportedPackages = []string{
	"Char 1", "Char.Info 1", "Char.Vitals 1", "Char.Inventory 1",
	"Char.Stats 1", "Char.Affects 1", "Room 1", "Room.Info 1",
}
```

- [ ] **Step 4: Run to verify pass / commit**

Run: `go test ./internal/telnet/ -v` (whole package green)
```bash
git add internal/telnet/gmcp.go internal/telnet/gmcp_test.go
git commit -m "feat(mudagent): GMCP handshake framing helpers

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Login driver

A small state machine that, given a stream of cleaned text, decides when to send the username and password.

**Files:** Create `internal/session/login.go`, `internal/session/login_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/session/login_test.go`:

```go
package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginSendsUsernameThenPassword(t *testing.T) {
	l := NewLogin("bob", "secret")

	// Username prompt seen -> send username.
	out, done := l.OnText("Welcome!\r\nUsername (or \"new\"): ")
	assert.Equal(t, "bob", out)
	assert.False(t, done)

	// Password prompt seen -> send password.
	out, done = l.OnText("Password: ")
	assert.Equal(t, "secret", out)
	assert.False(t, done)

	// Nothing more to send for unrelated text.
	out, done = l.OnText("some room text")
	assert.Equal(t, "", out)
	assert.False(t, done)
}

func TestLoginMarksDoneOnCharInfo(t *testing.T) {
	l := NewLogin("bob", "secret")
	l.OnText("Username: ")
	l.OnText("Password: ")
	assert.True(t, l.OnGMCP("Char.Info"))
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/session/ -run TestLogin -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Create `internal/session/login.go`:

```go
// Package session drives a full mudagent connection: negotiate, log in, stream.
package session

import "strings"

type loginStep int

const (
	wantUsername loginStep = iota
	wantPassword
	loggedIn
)

// Login is a text-prompt login state machine. The server uses text prompts
// (GMCP Char.Login is not wired server-side), and signals success by sending
// Char.Info / Room.Info GMCP rather than a confirmation line.
type Login struct {
	user, pass string
	step       loginStep
}

// NewLogin returns a login driver for the given credentials.
func NewLogin(user, pass string) *Login {
	return &Login{user: user, pass: pass, step: wantUsername}
}

// OnText feeds a chunk of cleaned server text. It returns the line to send (or
// "" if none) and whether login is fully complete.
func (l *Login) OnText(text string) (send string, done bool) {
	switch l.step {
	case wantUsername:
		if strings.Contains(text, "Username") {
			l.step = wantPassword
			return l.user, false
		}
	case wantPassword:
		if strings.Contains(text, "Password") {
			l.step = loggedIn // credentials sent; confirmation comes via GMCP
			return l.pass, false
		}
	}
	return "", l.step == loggedIn
}

// OnGMCP feeds a received GMCP package name. Char.Info or Room.Info after
// credentials confirms the player is in the world. Returns true once logged in.
func (l *Login) OnGMCP(pkg string) bool {
	if l.step == loggedIn && (pkg == "Char.Info" || pkg == "Room.Info") {
		return true
	}
	return false
}
```

- [ ] **Step 4: Run to verify pass / commit**

Run: `go test ./internal/session/ -run TestLogin -v`
```bash
git add internal/session/login.go internal/session/login_test.go
git commit -m "feat(mudagent): text-prompt login driver

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Session loop (connect → negotiate → login → stream)

Wires the parser, GMCP handshake, login driver, ANSI fallback, and the stdin/stdout protocol into a running session over an `io.ReadWriteCloser` (a `net.Conn` in production, a pipe in tests).

**Files:** Create `internal/session/session.go`, `internal/session/session_test.go`

- [ ] **Step 1: Write the failing test (integration against a scripted server)**

Create `internal/session/session_test.go`:

```go
package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pruuk/gomud-playtest-harness/internal/telnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedServer plays the GoMud side: WILL GMCP, login prompts, then a
// Room.Info GMCP, over an in-memory net.Pipe.
func TestSessionLogsInAndEmitsEvents(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()

	var out bytes.Buffer
	cfg := Config{User: "bob", Pass: "secret"}
	done := make(chan error, 1)
	go func() { done <- Run(cli, strings.NewReader(""), &out, cfg) }()

	// Server side script.
	srv.Write([]byte{telnet.IAC, telnet.WILL, telnet.GMCP})
	buf := make([]byte, 64)
	srv.SetReadDeadline(time.Now().Add(time.Second))
	srv.Read(buf) // expect IAC DO GMCP + Core.Hello/Supports
	srv.Write([]byte("Username (or \"new\"): "))
	srv.Read(buf) // bob\r\n
	srv.Write([]byte("Password: "))
	srv.Read(buf) // secret\r\n
	srv.Write(telnet.FrameGMCP("Room.Info", `{"name":"Town Square"}`))
	srv.Write([]byte("You are in the town square.\r\n"))
	time.Sleep(100 * time.Millisecond)
	srv.Close()
	require.NoError(t, <-done)

	// Assert the emitted JSON event stream contains the expected events.
	var sawLoggedIn, sawGMCP, sawOutput bool
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	for sc.Scan() {
		var e map[string]any
		require.NoError(t, json.Unmarshal(sc.Bytes(), &e))
		switch e["type"] {
		case "status":
			if e["state"] == "logged_in" {
				sawLoggedIn = true
			}
		case "gmcp":
			if e["package"] == "Room.Info" {
				sawGMCP = true
			}
		case "output":
			if strings.Contains(e["text"].(string), "town square") {
				sawOutput = true
			}
		}
	}
	assert.True(t, sawLoggedIn, "should emit logged_in status")
	assert.True(t, sawGMCP, "should emit Room.Info gmcp event")
	assert.True(t, sawOutput, "should emit room text output")
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/session/ -run TestSession -v`
Expected: FAIL (Run/Config undefined).

- [ ] **Step 3: Implement**

Create `internal/session/session.go`:

```go
package session

import (
	"bufio"
	"encoding/json"
	"io"
	"sync"

	"github.com/pruuk/gomud-playtest-harness/internal/protocol"
	"github.com/pruuk/gomud-playtest-harness/internal/telnet"
)

// Config holds the connection's runtime parameters.
type Config struct {
	User string
	Pass string
}

// Run drives one session to completion: it reads server bytes from conn,
// negotiates GMCP, logs in, emits JSON events to out, and forwards agent
// commands read from in to the MUD. It returns when conn closes or in ends.
func Run(conn io.ReadWriteCloser, in io.Reader, out io.Writer, cfg Config) error {
	var writeMu sync.Mutex
	emit := func(e protocol.Event) {
		line, err := protocol.Marshal(e)
		if err != nil {
			return
		}
		writeMu.Lock()
		io.WriteString(out, line+"\n")
		writeMu.Unlock()
	}
	send := func(b []byte) {
		writeMu.Lock() // serialize socket writes with nothing else; conn is independent of out
		writeMu.Unlock()
		conn.Write(b)
	}

	emit(protocol.Event{Type: "status", State: "connected"})

	parser := telnet.NewParser()
	login := NewLogin(cfg.User, cfg.Pass)
	loggedIn := false

	// Goroutine: forward agent stdin commands to the MUD.
	go func() {
		sc := bufio.NewScanner(in)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			cmd := protocol.ParseCommand(sc.Text())
			switch cmd.Kind {
			case protocol.CommandKindControl:
				if cmd.Control == "quit" {
					conn.Close()
					return
				}
			case protocol.CommandKindText:
				send([]byte(cmd.Text + "\r\n"))
			}
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			for _, tok := range parser.Feed(buf[:n]) {
				switch tok.Kind {
				case telnet.TokenText:
					clean := string(telnet.StripAnsi(tok.Text))
					if clean != "" {
						emit(protocol.Event{Type: "output", Text: clean, Raw: string(tok.Text)})
					}
					if s, _ := login.OnText(clean); s != "" {
						send([]byte(s + "\r\n"))
					}
				case telnet.TokenIAC:
					// Accept GMCP; refuse other options to avoid negotiation hangs.
					if tok.Option == telnet.GMCP && tok.Command == telnet.WILL {
						send(telnet.DoGMCP())
						hello, _ := json.Marshal(map[string]string{"client": "mudagent", "version": "1"})
						send(telnet.FrameGMCP("Core.Hello", string(hello)))
						sup, _ := json.Marshal(telnet.SupportedPackages)
						send(telnet.FrameGMCP("Core.Supports.Set", string(sup)))
					} else if tok.Command == telnet.WILL {
						send([]byte{telnet.IAC, telnet.DONT, tok.Option})
					} else if tok.Command == telnet.DO {
						send([]byte{telnet.IAC, telnet.WONT, tok.Option})
					}
				case telnet.TokenGMCP:
					emit(protocol.Event{Type: "gmcp", Package: tok.GMCPPackage, Data: rawJSON(tok.GMCPData)})
					if !loggedIn && login.OnGMCP(tok.GMCPPackage) {
						loggedIn = true
						emit(protocol.Event{Type: "status", State: "logged_in"})
					}
				}
			}
		}
		if err != nil {
			emit(protocol.Event{Type: "status", State: "disconnected"})
			if err == io.EOF {
				return nil
			}
			return nil // closed by quit or peer; treated as clean end
		}
	}
}

// rawJSON returns valid json.RawMessage, defaulting to null for empty payloads.
func rawJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/session/ -v`
Expected: PASS (login + session tests). If the scripted-server timing is flaky, increase the `time.Sleep` to 200ms — note any change.

- [ ] **Step 5: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat(mudagent): session loop with GMCP negotiation + JSON protocol

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Manifest + CLI + main

**Files:** Create `internal/session/manifest.go`, `internal/session/manifest_test.go`, rewrite `cmd/mudagent/main.go`

- [ ] **Step 1: Write the failing manifest test**

Create `internal/session/manifest_test.go`:

```go
package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest([]byte("target: localhost:55555\nuser: aitester\npassword: secret\n"))
	require.NoError(t, err)
	assert.Equal(t, "localhost:55555", m.Target)
	assert.Equal(t, "aitester", m.User)
	assert.Equal(t, "secret", m.Password)
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/session/ -run TestParseManifest -v`
Expected: FAIL.

- [ ] **Step 3: Implement manifest**

Create `internal/session/manifest.go`:

```go
package session

import "gopkg.in/yaml.v3"

// Manifest is the run configuration (connection + account). Personality and
// goals are consumed by the agent/driver, not the adapter, so they are not
// required here; unknown keys are ignored.
type Manifest struct {
	Target   string `yaml:"target"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// ParseManifest decodes a run manifest from YAML bytes.
func ParseManifest(b []byte) (Manifest, error) {
	var m Manifest
	err := yaml.Unmarshal(b, &m)
	return m, err
}
```

- [ ] **Step 4: Implement main with flags**

Rewrite `cmd/mudagent/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/pruuk/gomud-playtest-harness/internal/session"
)

func main() {
	target := flag.String("target", "", "host:port of the MUD AI port (overrides manifest)")
	user := flag.String("user", "", "test account username (overrides manifest)")
	pass := flag.String("password", "", "test account password (overrides manifest)")
	manifestPath := flag.String("manifest", "", "path to a run manifest YAML")
	flag.Parse()

	var m session.Manifest
	if *manifestPath != "" {
		b, err := os.ReadFile(*manifestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mudagent: read manifest: %v\n", err)
			os.Exit(2)
		}
		if m, err = session.ParseManifest(b); err != nil {
			fmt.Fprintf(os.Stderr, "mudagent: parse manifest: %v\n", err)
			os.Exit(2)
		}
	}
	if *target != "" {
		m.Target = *target
	}
	if *user != "" {
		m.User = *user
	}
	if *pass != "" {
		m.Password = *pass
	}
	if m.Target == "" || m.User == "" {
		fmt.Fprintln(os.Stderr, "mudagent: --target and --user (or a manifest) are required")
		os.Exit(2)
	}

	conn, err := net.DialTimeout("tcp", m.Target, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mudagent: dial %s: %v\n", m.Target, err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := session.Run(conn, os.Stdin, os.Stdout, session.Config{User: m.User, Pass: m.Password}); err != nil {
		fmt.Fprintf(os.Stderr, "mudagent: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Verify build + tests + cross-compile**

```bash
go test ./... 
go build ./cmd/mudagent
GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/mudagent   # cross-compile sanity (PowerShell: $env:GOOS='linux'; $env:GOARCH='amd64'; go build ...)
```
Expected: all green, both builds succeed.

- [ ] **Step 6: Commit**

```bash
git add internal/session/manifest.go internal/session/manifest_test.go cmd/mudagent/main.go
git commit -m "feat(mudagent): run manifest, CLI flags, and main entrypoint

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Live end-to-end smoke against ~/GoMud

> Manual verification against the real server. Requires the engine `feature/ai-port` branch built with `AIPort: 55555` and a test account that exists (create one manually for now, or wait for the `playtest` module). No unit test.

- [ ] **Step 1: Build and run the GoMud server** with `AIPort: 55555` (see the engine plan). Ensure a known account exists (e.g. create `aitester` via the normal client once).

- [ ] **Step 2: Run the adapter and drive it by hand**

```bash
echo '{"control":"quit"}' | ./mudagent --target localhost:55555 --user aitester --password <pass>
```
Better: run interactively, type `look`, observe JSON `output` + `gmcp` events on stdout, then type `{"control":"quit"}`.
Expected: `{"type":"status","state":"connected"}`, GMCP negotiation, `{"type":"status","state":"logged_in"}`, `output`/`gmcp` events for `look`, clean exit.

- [ ] **Step 3: Note any protocol mismatches** (prompt text differences, GMCP package names) and adjust `login.go` / `SupportedPackages` accordingly, with a follow-up commit.

---

## Self-Review

**Spec coverage (vs. design "Track 2B — mudagent"):** telnet+IAC+GMCP negotiation ✓ (Tasks 3,4,6); login via provisioned account ✓ (Task 5); line-in/JSON-line-out protocol ✓ (Tasks 1,6); events output/gmcp/status/error ✓ (Task 6); control verbs incl. quit ✓ (Tasks 1,6); manifest + target flag ✓ (Task 7); single static binary + cross-compile ✓ (Task 7). **Beacon events** are Phase-2 (the `beacon` Event type exists but is unused until the module emits `Playtest.*`). **Round-tick boundaries** are intentionally NOT emitted — documented as unavailable on the wire; the agent paces via response quiescence.

**Known approximations to revisit at Task 8:** exact login prompt text (`Username`/`Password` substring match), the `Core.Supports.Set` package list, and ANSI regex breadth. All are isolated and adjustable.

**Type consistency:** `protocol.Event`/`ParseCommand`/`Command`, `telnet.Parser`/`Token`/`Feed`/`DoGMCP`/`FrameGMCP`/`StripAnsi`, `session.Run`/`Config`/`NewLogin`/`Manifest`/`ParseManifest` — used identically across tasks.
