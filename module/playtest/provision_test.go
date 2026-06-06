package playtest

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

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

func TestEnsureStartRoom(t *testing.T) {
	m := &PlaytestModule{}

	// Void character (RoomId -1) -> reset to the start-room alias so login resolves it.
	u := &users.UserRecord{Character: &characters.Character{RoomId: -1}}
	m.ensureStartRoom(u)
	assert.Equal(t, rooms.StartRoomIdAlias, u.Character.RoomId)

	// A real (non-negative) room is left untouched.
	u2 := &users.UserRecord{Character: &characters.Character{RoomId: 5}}
	m.ensureStartRoom(u2)
	assert.Equal(t, 5, u2.Character.RoomId)
}
