package playtest

import (
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// shouldSnapBack decides whether a moved user must be returned to the sandbox.
// Pure for testability. Snap back when: the account is AI-flagged, a sandbox
// tag is configured, and the destination room does not carry that tag (fail
// closed: not provably inside the sandbox -> refuse).
func shouldSnapBack(isAI bool, sandboxTag string, destTags []string) bool {
	if !isAI || sandboxTag == "" {
		return false
	}
	for _, t := range destTags {
		if t == sandboxTag {
			return false
		}
	}
	return true
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
	if dest == nil {
		return events.Continue
	}
	if shouldSnapBack(u.IsAI, m.cfg.SandboxZoneTag, dest.Tags) {
		// RoomChange fires post-move, so return them to where they came from.
		// The resulting RoomChange has a sandbox-tagged destination, so it does
		// not re-trigger a snap-back (no recursion).
		rooms.MoveToRoom(evt.UserId, evt.FromRoomId)
		u.SendText(`The sandbox boundary holds you here.`)
	}
	return events.Continue
}
