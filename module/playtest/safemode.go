package playtest

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// containsTag reports whether tags includes tag.
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// shouldSnapBack decides whether a moved tester must be returned to the sandbox.
// Pure for testability. Snap back when: the mover is on the AI port, a sandbox
// tag is configured, and the destination room does not carry that tag (fail
// closed: not provably inside the sandbox -> refuse).
func shouldSnapBack(isAITester bool, sandboxTag string, destTags []string) bool {
	if !isAITester || sandboxTag == "" {
		return false
	}
	return !containsTag(destTags, sandboxTag)
}

func (m *PlaytestModule) registerSafeMode() {
	// Sandbox confinement needs a tag to confine to.
	if m.cfg.SafeMode && m.cfg.SandboxZoneTag != "" {
		m.plug.ReserveTags(m.cfg.SandboxZoneTag)
		events.RegisterListener(events.RoomChange{}, m.onRoomChange)
	}
	// Death protection is applied to a tester when it spawns into the world.
	if m.cfg.DeathProtection {
		events.RegisterListener(events.PlayerSpawn{}, m.onPlayerSpawn)
	}
}

func (m *PlaytestModule) onRoomChange(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.RoomChange)
	if !ok || evt.UserId == 0 {
		return events.Continue // not a user move
	}
	u := users.GetByUserId(evt.UserId)
	if u == nil {
		return events.Continue
	}
	dest := rooms.LoadRoom(evt.ToRoomId)
	if dest == nil || !shouldSnapBack(isAIConnection(u.ConnectionId()), m.cfg.SandboxZoneTag, dest.Tags) {
		return events.Continue
	}
	// Only snap back to an origin that is itself inside the sandbox. Otherwise
	// (the operator-error case of a tester placed outside the sandbox) we would
	// bounce it between two non-sandbox rooms every tick — an event storm. In
	// normal play the origin is always sandbox-tagged, so this never trips.
	from := rooms.LoadRoom(evt.FromRoomId)
	if from == nil || !containsTag(from.Tags, m.cfg.SandboxZoneTag) {
		mudlog.Warn("playtest", "msg", "AI tester outside sandbox with no sandboxed origin to return to", "userId", evt.UserId)
		return events.Continue
	}
	// The resulting RoomChange has a sandbox-tagged destination, so it does not
	// re-trigger a snap-back (no recursion).
	rooms.MoveToRoom(evt.UserId, evt.FromRoomId)
	u.SendText(`The sandbox boundary holds you here.`)
	return events.Continue
}

// onPlayerSpawn applies death protection to a tester as it enters the world,
// keyed off the AI-port connection rather than an account flag.
func (m *PlaytestModule) onPlayerSpawn(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.PlayerSpawn)
	if !ok || !isAIConnection(evt.ConnectionId) {
		return events.Continue
	}
	if u := users.GetByUserId(evt.UserId); u != nil {
		m.applyDeathProtection(u)
	}
	return events.Continue
}

// applyDeathProtection shields a tester from permadeath by granting a large
// ExtraLives count (works within GoMud's global permadeath setting, which a
// module can't toggle per-account).
func (m *PlaytestModule) applyDeathProtection(u *users.UserRecord) {
	if !m.cfg.DeathProtection || u.Character == nil {
		return
	}
	if u.Character.ExtraLives < 999 {
		u.Character.ExtraLives = 999
	}
}
