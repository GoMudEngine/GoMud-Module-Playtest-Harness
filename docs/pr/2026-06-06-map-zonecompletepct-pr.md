# PR: fix `%!d(<nil>)` in script-rendered map titles

> Canonical PR description for the map-bug fix (branch `fix/map-zonecompletepct`,
> off `GoMudEngine/GoMud:master`). Separate from the AI-port PR.

# Description

Looking at a map **sign** (or any map produced by the `GetMap` scripting
function) renders a broken title — e.g. `Map of Frostfang (%!d(<nil>)%)` — instead
of a percentage.

The shared `maps/map` template formats the title with a zone-completion percent:
`printf "%s (%d%%)" .Title .ZoneCompletePct`. The `map` *command* supplies
`ZoneCompletePct`, but `GetMap` (`internal/scripting/room_func.go`, used by map
signs and other scripted maps) did **not** — so the template formatted `nil` with
`%d` and leaked `%!d(<nil>)` into the title.

Reproducible with a raw telnet client (no special client) via `look <map sign>`
in Frostfang's Town Square — so it is not a client/encoding artifact.

`GetMap` has no viewing user to measure exploration against (unlike the `map`
command), so it now defaults `ZoneCompletePct` to `0`, matching the `map`
command's own fallback (`internal/usercommands/skill.map.go`). Script-rendered
map titles now read e.g. `Map of Frostfang (0%)`.

## Changes

- `internal/scripting/room_func.go` — `GetMap` now includes `ZoneCompletePct: 0`
  in the template data passed to `maps/map`, preventing the `%!d(<nil>)` leak.

---

## Notes

`0` is the minimal, safe default that matches existing behavior; maintainers may
prefer signs to show the viewing player's actual exploration percent (would
require threading a user into `GetMap`) or to omit the percent for static signs —
happy to adjust. Found by an AI playtest harness driving a `bug-finder`
personality (verified server-side with a raw socket client).
