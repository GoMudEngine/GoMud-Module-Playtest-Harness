package playtest

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

func TestShouldSnapBack(t *testing.T) {
	// AI-port tester leaving the sandbox -> snap back.
	assert.True(t, shouldSnapBack(true, "sandbox", []string{"town"}))
	// AI-port tester staying in the sandbox -> no snap back.
	assert.False(t, shouldSnapBack(true, "sandbox", []string{"sandbox", "quiet"}))
	// No sandbox tag configured -> never snap back.
	assert.False(t, shouldSnapBack(true, "", []string{"town"}))
	// Not an AI-port session -> never snap back.
	assert.False(t, shouldSnapBack(false, "sandbox", []string{"town"}))
}

func TestContainsTag(t *testing.T) {
	assert.True(t, containsTag([]string{"a", "sandbox"}, "sandbox"))
	assert.False(t, containsTag([]string{"a", "b"}, "sandbox"))
	assert.False(t, containsTag(nil, "sandbox"))
}

func TestApplyDeathProtection(t *testing.T) {
	m := &PlaytestModule{cfg: Config{DeathProtection: true}}
	u := &users.UserRecord{Character: &characters.Character{ExtraLives: 5}}
	m.applyDeathProtection(u)
	assert.Equal(t, 999, u.Character.ExtraLives, "should raise low ExtraLives")

	// Already high -> unchanged.
	u2 := &users.UserRecord{Character: &characters.Character{ExtraLives: 1000}}
	m.applyDeathProtection(u2)
	assert.Equal(t, 1000, u2.Character.ExtraLives, "should not lower a higher count")

	// Disabled -> no change.
	mOff := &PlaytestModule{cfg: Config{DeathProtection: false}}
	u3 := &users.UserRecord{Character: &characters.Character{ExtraLives: 5}}
	mOff.applyDeathProtection(u3)
	assert.Equal(t, 5, u3.Character.ExtraLives, "disabled -> unchanged")
}
