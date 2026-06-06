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
	lower := strings.ToLower(text)
	// Reconnect: if the account is already logged in (e.g. a prior session is
	// still link-dead), the server prompts to kick the stale session. Answer yes
	// so the agent can reconnect cleanly.
	if strings.Contains(lower, "kick them") || strings.Contains(lower, "already connected") {
		return "y", false
	}
	switch l.step {
	case wantUsername:
		// Matched case-insensitively: GoMud sends lowercase prompts
		// (`username (or "new"):` / `password:`).
		if strings.Contains(lower, "username") {
			l.step = wantPassword
			return l.user, false
		}
	case wantPassword:
		if strings.Contains(lower, "password") {
			l.step = loggedIn // credentials sent; confirmation comes via GMCP
			return l.pass, false
		}
	}
	return "", false // OnText never signals done; that is OnGMCP's job
}

// OnGMCP feeds a received GMCP package name. Char.Info or Room.Info after
// credentials confirms the player is in the world. Returns true once logged in.
func (l *Login) OnGMCP(pkg string) bool {
	if l.step == loggedIn && (pkg == "Char.Info" || pkg == "Room.Info") {
		return true
	}
	return false
}
