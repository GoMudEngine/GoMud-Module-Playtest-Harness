# Agent Runner (per-agent role in a scenario)

You are ONE tester in a multi-agent scenario. The conductor gave you: your
**agent id**, your **role** (a personality), your **target**, your **assignment**
(group goals + any per-agent goals + your choreography lines), the **blackboard
path** (`bb`), and your private `mudagent` bridge files. Follow your personality
(`framework/personalities/<role>.md`) and the engine profile
(`framework/engine-profile.yaml`) throughout.

## 1. Connect and enter the world
Start your `mudagent` exactly as the single-agent driver does
(`.claude/commands/playtest.md` step 2), using your target's host/port and creds.
With blank creds, create a character via the new-player flow. Poll your events
file until `{"type":"status","state":"logged_in"}`.

## 2. Join the lobby barrier
Mark yourself ready, then wait for the conductor to start the run:
```sh
go run ./cmd/ptorch bb ready  "$BB" --id "$AGENT_ID"
# wait until the run is RUNNING (poll; the conductor flips it once ALL are ready)
until [ "$(go run ./cmd/ptorch bb phase "$BB")" = "running" ]; do sleep 1; done
```

## 3. Play your assignment
Pursue your role + group goals + per-agent goals, interacting **in the game**
(party invites, attacks, trades happen through `mudagent`, not the blackboard).
Pace on the per-round `Playtest.Round` beacon, as in the single-agent loop.

- **Emit signals** when your choreography says you've reached a cue (use the
  current beacon round):
  ```sh
  go run ./cmd/ptorch bb signal "$BB" --name "$AGENT_ID.ready" --round "$ROUND"
  ```
- **Wait on another agent's cue** (a `choreography.after`):
  ```sh
  until go run ./cmd/ptorch bb dump "$BB" | grep -q '"other.ready"'; do sleep 1; done
  ```

## 4. Record findings
Whenever you find something, drop it on the blackboard so it reaches the combined
report:
```sh
go run ./cmd/ptorch bb finding "$BB" --agent "$AGENT_ID" --type BUG --title "short title" --round "$ROUND"
```

## 5. Finish
When your goals are met (or an exit condition from the single-agent loop is hit),
write your per-agent report per `framework/report-format.md`, then quit your
`mudagent`. The conductor aggregates once all agents finish.
