package playtest

import (
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// ensureTestAccount idempotently makes sure the configured AI-test account
// exists and is IsAI-flagged. Safe to call every boot. Mirrors GoMud's real
// account-creation sequence (NewUserRecord -> SetUsername -> SetPassword ->
// CreateUser); NewUserRecord builds a playable Character via characters.New().
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
	if err := users.CreateUser(u); err != nil {
		mudlog.Error("playtest", "msg", "create test account", "error", err)
		return
	}
	mudlog.Info("playtest", "msg", "provisioned AI test account", "name", m.cfg.AccountName)
}

// flagExisting ensures an already-existing account carries the IsAI flag.
func (m *PlaytestModule) flagExisting() {
	u, err := users.LoadUser(m.cfg.AccountName)
	if err != nil {
		mudlog.Error("playtest", "msg", "load existing test account", "error", err)
		return
	}
	if !u.IsAI {
		u.IsAI = true
		if err := users.SaveUser(*u); err != nil {
			mudlog.Error("playtest", "msg", "save IsAI flag", "error", err)
		}
	}
}
