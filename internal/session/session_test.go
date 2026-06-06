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

// Scripts the GoMud side: WILL GMCP, login prompts, then a Room.Info GMCP, over
// an in-memory net.Pipe; asserts the emitted JSON event stream.
//
// net.Pipe is fully synchronous (no internal buffer), so the server-side
// script runs in its own goroutine with a concurrent drain reader.  That
// drain reader consumes all client→server bytes (GMCP negotiation, credentials)
// so neither side ever blocks waiting for the other to read before it can write.
func TestSessionLogsInAndEmitsEvents(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()

	var out bytes.Buffer
	cfg := Config{User: "bob", Pass: "secret"}
	done := make(chan error, 1)
	go func() { done <- Run(cli, strings.NewReader(""), &out, cfg) }()

	// Server-side script.
	go func() {
		defer srv.Close()

		// Concurrent drain: consumes all client→server writes (GMCP negotiation,
		// "bob\r\n", "secret\r\n") so server Write calls never deadlock on Pipe.
		go func() {
			drain := make([]byte, 512)
			for {
				_, err := srv.Read(drain)
				if err != nil {
					return
				}
			}
		}()

		srv.Write([]byte{telnet.IAC, telnet.WILL, telnet.GMCP})
		time.Sleep(50 * time.Millisecond) // let client finish GMCP negotiation writes
		srv.Write([]byte("Username (or \"new\"): "))
		time.Sleep(50 * time.Millisecond) // let client send username
		srv.Write([]byte("Password: "))
		time.Sleep(50 * time.Millisecond) // let client send password
		srv.Write(telnet.FrameGMCP("Room.Info", `{"name":"Town Square"}`))
		srv.Write([]byte("You are in the town square.\r\n"))
		time.Sleep(250 * time.Millisecond) // let session process final data before close
	}()

	require.NoError(t, <-done)

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
			if txt, ok := e["text"].(string); ok && strings.Contains(txt, "town square") {
				sawOutput = true
			}
		}
	}
	assert.True(t, sawLoggedIn, "should emit logged_in status")
	assert.True(t, sawGMCP, "should emit Room.Info gmcp event")
	assert.True(t, sawOutput, "should emit room text output")
}
