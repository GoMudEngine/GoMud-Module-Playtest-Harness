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

// GoMud actually sends lowercase prompts; the driver must match them.
func TestLoginMatchesLowercasePrompts(t *testing.T) {
	l := NewLogin("aitester", "pw")
	out, _ := l.OnText(`username (or "new"): `)
	assert.Equal(t, "aitester", out)
	out, _ = l.OnText("password: ")
	assert.Equal(t, "pw", out)
}

// If the account is already connected (stale link-dead session), the server asks
// to kick it; the driver answers yes so the agent can reconnect.
func TestLoginKicksStaleSession(t *testing.T) {
	l := NewLogin("aitester", "pw")
	l.OnText(`username (or "new"): `)
	l.OnText("password: ")
	out, _ := l.OnText("User is already connected. Kick them? [y/n]:")
	assert.Equal(t, "y", out)
}
