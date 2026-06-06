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
