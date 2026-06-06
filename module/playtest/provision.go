package playtest

import (
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ensureTestAccount idempotently makes sure the configured AI-test account
// exists and is IsAI-flagged with death protection applied. Safe to call every
// boot. Mirrors GoMud's verified account-creation flow.
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

	u := users.NewUserRecord(0, 0)
	if err := u.SetUsername(m.cfg.AccountName); err != nil {
		mudlog.Error("playtest", "msg", "set username", "error", err)
		return
	}
	if err := u.SetPassword(m.cfg.AccountPassword); err != nil {
		mudlog.Error("playtest", "msg", "set password", "error", err)
		return
	}
	u.IsAI = true
	m.applyDeathProtection(u)
	m.ensureStartRoom(u)
	if err := users.CreateUser(u); err != nil {
		mudlog.Error("playtest", "msg", "create test account", "error", err)
		return
	}
	mudlog.Info("playtest", "msg", "provisioned AI test account", "name", m.cfg.AccountName)
}

// flagExisting ensures an already-existing account carries the IsAI flag, death
// protection, and a resolvable start room, persisting the result.
func (m *PlaytestModule) flagExisting() {
	u, err := users.LoadUser(m.cfg.AccountName)
	if err != nil {
		mudlog.Error("playtest", "msg", "load existing test account", "error", err)
		return
	}
	u.IsAI = true
	m.applyDeathProtection(u)
	m.ensureStartRoom(u)
	if err := users.SaveUser(*u); err != nil {
		mudlog.Error("playtest", "msg", "save test account", "error", err)
	}
}

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

// ensureStartRoom rescues a character left in the "Nowhere" void. A freshly
// created character starts at RoomId -1 (characters.StartingRoomId), which is
// never resolved to a real room. Setting it to the start-room alias (0) makes
// login resolve it via MiscData -> config StartRoom -> fallback, so the tester
// lands somewhere playable instead of The Void. Real (non-negative) rooms are
// left untouched.
func (m *PlaytestModule) ensureStartRoom(u *users.UserRecord) {
	if u.Character == nil {
		return
	}
	if u.Character.RoomId < rooms.StartRoomIdAlias {
		u.Character.RoomId = rooms.StartRoomIdAlias
	}
}
