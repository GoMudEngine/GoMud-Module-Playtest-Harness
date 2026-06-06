package session

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud-Module-Playtest-Harness/internal/protocol"
	"github.com/GoMudEngine/GoMud-Module-Playtest-Harness/internal/telnet"
)

// Config holds the connection's runtime parameters.
type Config struct {
	User string
	Pass string
}

// Run drives one session to completion: it reads server bytes from conn,
// negotiates GMCP, logs in, emits JSON events to out, and forwards agent
// commands read from in to the MUD. Returns when conn closes or in ends.
func Run(conn io.ReadWriteCloser, in io.Reader, out io.Writer, cfg Config) error {
	var outMu, connMu sync.Mutex

	emit := func(e protocol.Event) {
		line, err := protocol.Marshal(e)
		if err != nil {
			return
		}
		outMu.Lock()
		io.WriteString(out, line+"\n")
		outMu.Unlock()
	}
	send := func(b []byte) {
		connMu.Lock()
		conn.Write(b)
		connMu.Unlock()
	}

	emit(protocol.Event{Type: "status", State: "connected"})

	parser := telnet.NewParser()
	// Auto-login is a convenience for existing accounts. With no credentials the
	// agent drives login itself (and creates a character via the normal new-player
	// flow if none exists) — login is part of what a tester exercises.
	var login *Login
	if cfg.User != "" {
		login = NewLogin(cfg.User, cfg.Pass)
	}
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
					if login != nil {
						if s, _ := login.OnText(clean); s != "" {
							send([]byte(s + "\r\n"))
						}
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
					ev := gmcpEvent(tok.GMCPPackage, tok.GMCPData)
					emit(ev)
					// Login completion is signalled by real GMCP state packages —
					// Char.Info / Room.Info arrive once the player is in the world,
					// whether they logged into an existing character or just created
					// one. Works with or without adapter auto-login.
					if ev.Type == "gmcp" && !loggedIn &&
						(tok.GMCPPackage == "Char.Info" || tok.GMCPPackage == "Room.Info") {
						loggedIn = true
						emit(protocol.Event{Type: "status", State: "logged_in"})
					}
				}
			}
		}
		if err != nil {
			emit(protocol.Event{Type: "status", State: "disconnected"})
			return nil
		}
	}
}

// rawJSON returns a json.RawMessage that is ALWAYS valid JSON, so an emitted
// event line is never malformed. Empty -> null; valid JSON -> verbatim; anything
// else (a server sending a non-JSON GMCP payload) -> encoded as a JSON string.
func rawJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	if json.Valid(b) {
		return json.RawMessage(b)
	}
	s, _ := json.Marshal(string(b))
	return json.RawMessage(s)
}

// gmcpEvent classifies a received GMCP package into an event. Playtest.* packages
// become "beacon" events (the suffix after "Playtest." is the event name); all
// others are generic "gmcp" events.
func gmcpEvent(pkg string, data []byte) protocol.Event {
	if suffix, ok := strings.CutPrefix(pkg, "Playtest."); ok {
		return protocol.Event{Type: "beacon", Event: suffix, Data: rawJSON(data)}
	}
	return protocol.Event{Type: "gmcp", Package: pkg, Data: rawJSON(data)}
}
