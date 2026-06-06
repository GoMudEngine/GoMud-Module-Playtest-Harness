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

// shouldSnapBack decides whether a moved user must be returned to the sandbox.
// Pure for testability. Snap back when: the account is AI-flagged, a sandbox
// tag is configured, and the destination room does not carry that tag (fail
// closed: not provably inside the sandbox -> refuse).
func shouldSnapBack(isAI bool, sandboxTag string, destTags []string) bool {
	if !isAI || sandboxTag == "" {
		return false
	}
	return !containsTag(destTags, sandboxTag)
}

func (m *PlaytestModule) registerSafeMode() {
	if !m.cfg.SafeMode || m.cfg.SandboxZoneTag == "" {
		return
	}
	m.plug.ReserveTags(m.cfg.SandboxZoneTag)
	events.RegisterListener(events.RoomChange{}, m.onRoomChange)
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
	if dest == nil || !shouldSnapBack(u.IsAI, m.cfg.SandboxZoneTag, dest.Tags) {
		return events.Continue
	}
	// Only snap back to an origin that is itself inside the sandbox. Otherwise
	// (the operator-error case of an AI account placed outside the sandbox) we
	// would bounce it between two non-sandbox rooms every tick — an event storm.
	// In normal play the origin is always sandbox-tagged, so this never trips.
	from := rooms.LoadRoom(evt.FromRoomId)
	if from == nil || !containsTag(from.Tags, m.cfg.SandboxZoneTag) {
		mudlog.Warn("playtest", "msg", "AI account outside sandbox with no sandboxed origin to return to", "userId", evt.UserId)
		return events.Continue
	}
	// The resulting RoomChange has a sandbox-tagged destination, so it does not
	// re-trigger a snap-back (no recursion).
	rooms.MoveToRoom(evt.UserId, evt.FromRoomId)
	u.SendText(`The sandbox boundary holds you here.`)
	return events.Continue
}
