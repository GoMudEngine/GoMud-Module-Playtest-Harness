# Multi-Agent / Party Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a general N-agent testing framework to the playtest harness — a conductor that runs multiple independent tester agents from one scenario file, coordinating in-game plus a small shared blackboard — validated end-to-end by a 2-agent party run.

**Architecture:** Two new pure-Go packages (`internal/scenario` for the scenario file, `internal/blackboard` for race-safe shared state) plus a thin CLI (`cmd/ptorch`) the conductor and agents call for all scenario/blackboard operations. The conductor (`.claude/commands/playtest-scenario.md`) spawns one background subagent per roster entry; agents reuse the existing `mudagent` and personalities unchanged and coordinate via the game world + the blackboard CLI. All client-side → push-only, no module release.

**Tech Stack:** Go 1.25 (`gopkg.in/yaml.v3`, `encoding/json`, `stretchr/testify`), Markdown drivers (Claude Code slash commands), YAML scenario files.

**Spec:** `docs/superpowers/specs/2026-06-07-multi-agent-testing-design.md`

**Conventions to follow (verified in repo):**
- Module path: `github.com/GoMudEngine/GoMud-Module-Playtest-Harness`.
- Tests use `testify` (`assert`/`require`), same package as the code under test (e.g. `package scenario`).
- No `make`; use raw `go`. Run from repo root. Validate with `go build ./...`, `go vet ./...`, `go test ./...`.
- Windows dev host: trust `go build`/`go test` over IDE gofmt diagnostics (CRLF noise); files are LF-clean in git.
- Commit after each task. Branch off `main` first (do not commit straight to `main`).

---

### Task 0: Branch

**Files:** none (git only)

- [ ] **Step 1: Create a feature branch**

```bash
cd ~/workspace/gomud-playtest-harness
git checkout -b feat/multi-agent-testing
git status   # Expected: On branch feat/multi-agent-testing, clean
```

---

### Task 1: Scenario types + `Parse`

**Files:**
- Create: `internal/scenario/scenario.go`
- Test: `internal/scenario/scenario_test.go`

- [ ] **Step 1: Write the failing test**

```go
package scenario

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseValidPartyScenario(t *testing.T) {
	src := []byte(`
name: party-smoke
mode: party
summary: Two testers form a party.
requires:
  permadeath: false
  death_protection: true
roster:
  - id: leader
    role: feature-tester
    target: local
  - id: member
    role: feel-tester
    target: local
group_goals:
  - id: form
    do: leader invites member; member accepts
    verify: both see each other in GMCP party state
choreography:
  - who: leader
    do: party create then invite member
  - who: member
    after: leader.invited
    do: party accept
`)
	s, err := Parse(src)
	require.NoError(t, err)
	assert.Equal(t, "party-smoke", s.Name)
	assert.Equal(t, "party", s.Mode)
	require.Len(t, s.Roster, 2)
	assert.Equal(t, "leader", s.Roster[0].ID)
	assert.Equal(t, "feel-tester", s.Roster[1].Role)
	require.NotNil(t, s.Requires.Permadeath)
	assert.False(t, *s.Requires.Permadeath)
	require.Len(t, s.GroupGoals, 1)
	assert.Equal(t, "form", s.GroupGoals[0].ID)
	require.Len(t, s.Choreography, 2)
	assert.Equal(t, "leader.invited", s.Choreography[1].After)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scenario/ -run TestParseValidPartyScenario -v`
Expected: FAIL — `undefined: Parse` (package doesn't compile yet).

- [ ] **Step 3: Write minimal implementation**

```go
package scenario

import "gopkg.in/yaml.v3"

// Scenario is a multi-agent playtest run definition. Engine-specific behavior
// still comes from engine-profile.yaml; this file is game-agnostic.
type Scenario struct {
	Name         string             `yaml:"name"`
	Mode         string             `yaml:"mode"` // party | adversarial | parallel | scenario
	Summary      string             `yaml:"summary"`
	Requires     Requires           `yaml:"requires"`
	Roster       []RosterEntry      `yaml:"roster"`
	GroupGoals   []Goal             `yaml:"group_goals"`
	Choreography []ChoreographyStep `yaml:"choreography"`
}

// Requires declares server preconditions. The harness VERIFIES/surfaces these;
// it never mutates server config. Pointers distinguish "unset" from "false".
type Requires struct {
	Permadeath      *bool `yaml:"permadeath"`
	DeathProtection *bool `yaml:"death_protection"`
	MaxConnections  int   `yaml:"max_connections"` // 0 → DefaultMaxConnections
}

// RosterEntry is one tester agent in the run.
type RosterEntry struct {
	ID     string `yaml:"id"`     // stable id used in goals/choreography/reports
	Role   string `yaml:"role"`   // an existing personality name
	Target string `yaml:"target"` // a targets.yaml entry
	Goals  []Goal `yaml:"goals"`  // optional per-agent goals
}

// Goal reuses the single-agent do/verify shape.
type Goal struct {
	ID     string `yaml:"id"`
	Do     string `yaml:"do"`
	Verify string `yaml:"verify"`
}

// ChoreographyStep is one ordered step (mainly for `scenario` mode).
type ChoreographyStep struct {
	Who   string `yaml:"who"`   // a roster id
	Do    string `yaml:"do"`    // what that agent does
	After string `yaml:"after"` // optional blackboard signal name to wait for
	Round int    `yaml:"round"` // optional absolute beacon round
}

// Parse decodes a scenario file from YAML bytes.
func Parse(b []byte) (Scenario, error) {
	var s Scenario
	if err := yaml.Unmarshal(b, &s); err != nil {
		return Scenario{}, err
	}
	return s, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scenario/ -run TestParseValidPartyScenario -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scenario/scenario.go internal/scenario/scenario_test.go
git commit -m "feat(scenario): scenario file types + Parse"
```

---

### Task 2: Scenario `Validate`, `MaxConnections`, `Warnings`

**Files:**
- Modify: `internal/scenario/scenario.go`
- Test: `internal/scenario/scenario_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/scenario/scenario_test.go`:

```go
func validScenario() Scenario {
	return Scenario{
		Name: "s", Mode: "party",
		Roster: []RosterEntry{
			{ID: "a", Role: "bug-finder", Target: "local"},
			{ID: "b", Role: "feel-tester", Target: "local"},
		},
	}
}

func TestValidatePasses(t *testing.T) {
	require.NoError(t, validScenario().Validate())
}

func TestValidateRejectsBadMode(t *testing.T) {
	s := validScenario()
	s.Mode = "co-op"
	assert.ErrorContains(t, s.Validate(), "invalid mode")
}

func TestValidateRejectsEmptyRoster(t *testing.T) {
	s := validScenario()
	s.Roster = nil
	assert.ErrorContains(t, s.Validate(), "at least 1 agent")
}

func TestValidateRejectsDuplicateIDs(t *testing.T) {
	s := validScenario()
	s.Roster[1].ID = "a"
	assert.ErrorContains(t, s.Validate(), "duplicate roster id")
}

func TestValidateRejectsMissingRole(t *testing.T) {
	s := validScenario()
	s.Roster[0].Role = ""
	assert.ErrorContains(t, s.Validate(), "missing role")
}

func TestValidateRejectsMissingTarget(t *testing.T) {
	s := validScenario()
	s.Roster[0].Target = ""
	assert.ErrorContains(t, s.Validate(), "missing target")
}

func TestValidateRejectsUnknownChoreographyWho(t *testing.T) {
	s := validScenario()
	s.Choreography = []ChoreographyStep{{Who: "ghost", Do: "wave"}}
	assert.ErrorContains(t, s.Validate(), "not in roster")
}

func TestMaxConnectionsDefaultsTo20(t *testing.T) {
	assert.Equal(t, 20, validScenario().MaxConnections())
	s := validScenario()
	s.Requires.MaxConnections = 5
	assert.Equal(t, 5, s.MaxConnections())
}

func TestWarningsFlagOverLimitAndCost(t *testing.T) {
	s := validScenario()
	s.Requires.MaxConnections = 1 // roster of 2 exceeds it
	w := s.Warnings()
	joined := ""
	for _, x := range w {
		joined += x + "\n"
	}
	assert.Contains(t, joined, "max_connections")

	big := validScenario()
	for i := 0; i < 3; i++ {
		big.Roster = append(big.Roster, RosterEntry{ID: "x" + string(rune('a'+i)), Role: "bug-finder", Target: "local"})
	}
	costJoined := ""
	for _, x := range big.Warnings() {
		costJoined += x + "\n"
	}
	assert.Contains(t, costJoined, "COST")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scenario/ -run 'TestValidate|TestMaxConnections|TestWarnings' -v`
Expected: FAIL — `s.Validate undefined`, etc.

- [ ] **Step 3: Write the implementation**

Append to `internal/scenario/scenario.go`:

```go
import "fmt" // add to the existing import block at the top of the file

// DefaultMaxConnections mirrors GoMud's Network.AI.MaxConnections default.
const DefaultMaxConnections = 20

var validModes = map[string]bool{
	"party": true, "adversarial": true, "parallel": true, "scenario": true,
}

// Validate returns the first structural error, or nil. Cost/limit advisories
// are non-fatal; see Warnings.
func (s Scenario) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("scenario: name is required")
	}
	if !validModes[s.Mode] {
		return fmt.Errorf("scenario %q: invalid mode %q (want party|adversarial|parallel|scenario)", s.Name, s.Mode)
	}
	if len(s.Roster) < 1 {
		return fmt.Errorf("scenario %q: roster must have at least 1 agent", s.Name)
	}
	seen := map[string]bool{}
	for i, r := range s.Roster {
		if r.ID == "" {
			return fmt.Errorf("scenario %q: roster[%d] missing id", s.Name, i)
		}
		if seen[r.ID] {
			return fmt.Errorf("scenario %q: duplicate roster id %q", s.Name, r.ID)
		}
		seen[r.ID] = true
		if r.Role == "" {
			return fmt.Errorf("scenario %q: roster %q missing role", s.Name, r.ID)
		}
		if r.Target == "" {
			return fmt.Errorf("scenario %q: roster %q missing target", s.Name, r.ID)
		}
	}
	for i, c := range s.Choreography {
		if c.Who == "" {
			return fmt.Errorf("scenario %q: choreography[%d] missing who", s.Name, i)
		}
		if !seen[c.Who] {
			return fmt.Errorf("scenario %q: choreography[%d] who %q not in roster", s.Name, i, c.Who)
		}
	}
	return nil
}

// MaxConnections is the scenario's connection cap (default DefaultMaxConnections).
func (s Scenario) MaxConnections() int {
	if s.Requires.MaxConnections > 0 {
		return s.Requires.MaxConnections
	}
	return DefaultMaxConnections
}

// Warnings returns non-fatal advisories: roster over the connection limit, and
// the token/processing cost of running many independent agents.
func (s Scenario) Warnings() []string {
	var w []string
	if n, max := len(s.Roster), s.MaxConnections(); n > max {
		w = append(w, fmt.Sprintf(
			"roster has %d agents but max_connections is %d — raise Network.AI.MaxConnections on the server (default %d) or it will refuse extra connections",
			n, max, DefaultMaxConnections))
	}
	if n := len(s.Roster); n >= 3 {
		w = append(w, fmt.Sprintf(
			"COST: %d independent agents ≈ %dx the tokens/processing of a single run — use with caution and watch your usage rate", n, n))
	}
	return w
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scenario/ -v`
Expected: PASS (all scenario tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scenario/scenario.go internal/scenario/scenario_test.go
git commit -m "feat(scenario): Validate, MaxConnections, and cost/limit Warnings"
```

---

### Task 3: Blackboard types + `Init`/`Load` (atomic, locked)

**Files:**
- Create: `internal/blackboard/blackboard.go`
- Test: `internal/blackboard/blackboard_test.go`

- [ ] **Step 1: Write the failing test**

```go
package blackboard

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitSeedsLobbyAndReadyKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bb.json")
	require.NoError(t, Init(path, "run-1", []string{"leader", "member"}))

	bd, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "run-1", bd.Run)
	assert.Equal(t, PhaseLobby, bd.Phase)
	assert.Equal(t, map[string]bool{"leader": false, "member": false}, bd.Ready)
	assert.NotNil(t, bd.Signals)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/blackboard/ -run TestInitSeeds -v`
Expected: FAIL — `undefined: Init`.

- [ ] **Step 3: Write minimal implementation**

```go
package blackboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Phase values for the run lifecycle.
const (
	PhaseLobby   = "lobby"
	PhaseRunning = "running"
	PhaseDone    = "done"
)

// Finding is one report item dropped by an agent.
type Finding struct {
	Agent string `json:"agent"`
	Type  string `json:"type"` // BUG | CONCERN | OBSERVATION | PASS | FAIL | BLOCKED
	Title string `json:"title"`
	Round int    `json:"round,omitempty"`
}

// Board is the shared out-of-band state for a multi-agent run.
type Board struct {
	Run      string          `json:"run"`
	Phase    string          `json:"phase"`
	Ready    map[string]bool `json:"ready"`
	Signals  map[string]int  `json:"signals"` // name -> beacon round it fired
	Findings []Finding       `json:"findings"`
}

func lockPath(path string) string { return path + ".lock" }

// withLock serializes read-modify-write across processes via an exclusive lock
// file. Each agent runs as its own process, so this prevents lost updates.
func withLock(path string, fn func() error) error {
	deadline := time.Now().Add(10 * time.Second)
	for {
		f, err := os.OpenFile(lockPath(path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			defer os.Remove(lockPath(path))
			return fn()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("blackboard: timed out acquiring lock %s", lockPath(path))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Load reads and decodes the board.
func Load(path string) (Board, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Board{}, err
	}
	var bd Board
	if err := json.Unmarshal(b, &bd); err != nil {
		return Board{}, err
	}
	return bd, nil
}

// save atomically writes the board (temp file + rename).
func save(path string, bd Board) error {
	b, err := json.MarshalIndent(bd, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// update locks, loads, mutates, and saves in one critical section.
func update(path string, fn func(*Board)) error {
	return withLock(path, func() error {
		bd, err := Load(path)
		if err != nil {
			return err
		}
		fn(&bd)
		return save(path, bd)
	})
}

// Init seeds a fresh board in lobby phase with one (unready) entry per agent id.
func Init(path, run string, agentIDs []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	bd := Board{
		Run:     run,
		Phase:   PhaseLobby,
		Ready:   map[string]bool{},
		Signals: map[string]int{},
	}
	for _, id := range agentIDs {
		bd.Ready[id] = false
	}
	return withLock(path, func() error { return save(path, bd) })
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/blackboard/ -run TestInitSeeds -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/blackboard/blackboard.go internal/blackboard/blackboard_test.go
git commit -m "feat(blackboard): Board/Finding types + locked, atomic Init/Load"
```

---

### Task 4: Blackboard `SetReady`/`AllReady`/`SetPhase`/`Phase`

**Files:**
- Modify: `internal/blackboard/blackboard.go`
- Test: `internal/blackboard/blackboard_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/blackboard/blackboard_test.go`:

```go
func TestReadinessBarrier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bb.json")
	require.NoError(t, Init(path, "r", []string{"a", "b"}))

	ok, err := AllReady(path)
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, SetReady(path, "a"))
	ok, _ = AllReady(path)
	assert.False(t, ok, "one of two ready is not all")

	require.NoError(t, SetReady(path, "b"))
	ok, _ = AllReady(path)
	assert.True(t, ok, "all ready now")
}

func TestPhaseSetAndGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bb.json")
	require.NoError(t, Init(path, "r", []string{"a"}))

	p, err := Phase(path)
	require.NoError(t, err)
	assert.Equal(t, PhaseLobby, p)

	require.NoError(t, SetPhase(path, PhaseRunning))
	p, _ = Phase(path)
	assert.Equal(t, PhaseRunning, p)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/blackboard/ -run 'TestReadinessBarrier|TestPhase' -v`
Expected: FAIL — `undefined: AllReady`, etc.

- [ ] **Step 3: Write the implementation**

Append to `internal/blackboard/blackboard.go`:

```go
// SetReady marks one agent present in the world.
func SetReady(path, id string) error {
	return update(path, func(b *Board) {
		if b.Ready == nil {
			b.Ready = map[string]bool{}
		}
		b.Ready[id] = true
	})
}

// AllReady is true only when every tracked agent is ready (and at least one is
// tracked).
func AllReady(path string) (bool, error) {
	bd, err := Load(path)
	if err != nil {
		return false, err
	}
	if len(bd.Ready) == 0 {
		return false, nil
	}
	for _, r := range bd.Ready {
		if !r {
			return false, nil
		}
	}
	return true, nil
}

// SetPhase sets the run phase.
func SetPhase(path, phase string) error {
	return update(path, func(b *Board) { b.Phase = phase })
}

// Phase returns the current run phase.
func Phase(path string) (string, error) {
	bd, err := Load(path)
	return bd.Phase, err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/blackboard/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/blackboard/blackboard.go internal/blackboard/blackboard_test.go
git commit -m "feat(blackboard): readiness barrier + phase get/set"
```

---

### Task 5: Blackboard `Signal` + `AddFinding` (dedup) + concurrency safety

**Files:**
- Modify: `internal/blackboard/blackboard.go`
- Test: `internal/blackboard/blackboard_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/blackboard/blackboard_test.go`:

```go
import "sync" // add to the import block at the top of the test file

func TestSignalRecordsRound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bb.json")
	require.NoError(t, Init(path, "r", []string{"a"}))
	require.NoError(t, Signal(path, "leader.invited", 1314530))

	bd, _ := Load(path)
	assert.Equal(t, 1314530, bd.Signals["leader.invited"])
}

func TestAddFindingAppendsAndDedups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bb.json")
	require.NoError(t, Init(path, "r", []string{"a"}))

	require.NoError(t, AddFinding(path, Finding{Agent: "a", Type: "BUG", Title: "map broken"}))
	require.NoError(t, AddFinding(path, Finding{Agent: "a", Type: "BUG", Title: "map broken"})) // dup
	require.NoError(t, AddFinding(path, Finding{Agent: "b", Type: "BUG", Title: "map broken"})) // diff agent

	bd, _ := Load(path)
	assert.Len(t, bd.Findings, 2, "same agent+title deduped; different agent kept")
}

func TestConcurrentAddFindingNoLostUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bb.json")
	require.NoError(t, Init(path, "r", []string{"a"}))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = AddFinding(path, Finding{Agent: "a", Type: "OBSERVATION", Title: "f" + string(rune('A'+n))})
		}(i)
	}
	wg.Wait()

	bd, _ := Load(path)
	assert.Len(t, bd.Findings, 20, "all distinct findings recorded under the lock")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/blackboard/ -run 'TestSignal|TestAddFinding|TestConcurrent' -v`
Expected: FAIL — `undefined: Signal`, `undefined: AddFinding`.

- [ ] **Step 3: Write the implementation**

Append to `internal/blackboard/blackboard.go`:

```go
// Signal records a named event and the beacon round it fired on.
func Signal(path, name string, round int) error {
	return update(path, func(b *Board) {
		if b.Signals == nil {
			b.Signals = map[string]int{}
		}
		b.Signals[name] = round
	})
}

// AddFinding appends a finding, skipping exact (agent,title) duplicates.
func AddFinding(path string, f Finding) error {
	return update(path, func(b *Board) {
		for _, e := range b.Findings {
			if e.Agent == f.Agent && e.Title == f.Title {
				return // dedup
			}
		}
		b.Findings = append(b.Findings, f)
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/blackboard/ -v`
Expected: PASS (including the concurrency test — run a few times if you want: `go test ./internal/blackboard/ -run TestConcurrent -count 5`).

- [ ] **Step 5: Commit**

```bash
git add internal/blackboard/blackboard.go internal/blackboard/blackboard_test.go
git commit -m "feat(blackboard): signals + deduped, concurrency-safe findings"
```

---

### Task 6: `ptorch` CLI (scenario + blackboard operations)

**Files:**
- Create: `cmd/ptorch/main.go`
- Test: `cmd/ptorch/main_test.go`

The conductor and agents shell out to this CLI for every scenario/blackboard
operation, so all mutations stay atomic and tested. `main` is a thin wrapper over
a testable `run(args, stdout, stderr) int`.

- [ ] **Step 1: Write the failing tests**

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenarioValidateGoodFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s.yaml")
	require.NoError(t, os.WriteFile(f, []byte(`
name: s
mode: party
roster:
  - {id: a, role: bug-finder, target: local}
  - {id: b, role: feel-tester, target: local}
`), 0o644))

	var out, errb bytes.Buffer
	code := run([]string{"scenario", "validate", f}, &out, &errb)
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "OK")
}

func TestScenarioValidateBadFileExitsNonZero(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s.yaml")
	require.NoError(t, os.WriteFile(f, []byte("name: s\nmode: nope\nroster: []\n"), 0o644))

	var out, errb bytes.Buffer
	code := run([]string{"scenario", "validate", f}, &out, &errb)
	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "invalid mode")
}

func TestScenarioPlanEmitsJSON(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s.yaml")
	require.NoError(t, os.WriteFile(f, []byte(`
name: s
mode: party
roster:
  - {id: a, role: bug-finder, target: local}
  - {id: b, role: feel-tester, target: local}
`), 0o644))

	var out, errb bytes.Buffer
	code := run([]string{"scenario", "plan", f}, &out, &errb)
	require.Equal(t, 0, code)

	var plan struct {
		Name           string `json:"name"`
		Mode           string `json:"mode"`
		MaxConnections int    `json:"max_connections"`
		Roster         []struct {
			ID, Role, Target string
		} `json:"roster"`
		Warnings []string `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &plan))
	assert.Equal(t, "party", plan.Mode)
	assert.Equal(t, 20, plan.MaxConnections)
	require.Len(t, plan.Roster, 2)
	assert.Equal(t, "a", plan.Roster[0].ID)
}

func TestBlackboardRoundTripThroughCLI(t *testing.T) {
	bb := filepath.Join(t.TempDir(), "bb.json")
	discard := &bytes.Buffer{}

	require.Equal(t, 0, run([]string{"bb", "init", bb, "--run", "r", "--ids", "a,b"}, discard, discard))

	// not all ready yet -> exit 3
	assert.Equal(t, 3, run([]string{"bb", "allready", bb}, discard, discard))

	require.Equal(t, 0, run([]string{"bb", "ready", bb, "--id", "a"}, discard, discard))
	require.Equal(t, 0, run([]string{"bb", "ready", bb, "--id", "b"}, discard, discard))
	assert.Equal(t, 0, run([]string{"bb", "allready", bb}, discard, discard))

	require.Equal(t, 0, run([]string{"bb", "phase", bb, "--set", "running"}, discard, discard))
	require.Equal(t, 0, run([]string{"bb", "signal", bb, "--name", "a.invited", "--round", "5"}, discard, discard))
	require.Equal(t, 0, run([]string{"bb", "finding", bb, "--agent", "a", "--type", "BUG", "--title", "x"}, discard, discard))

	var out bytes.Buffer
	require.Equal(t, 0, run([]string{"bb", "dump", bb}, &out, discard))
	assert.Contains(t, out.String(), `"phase": "running"`)
	assert.Contains(t, out.String(), `"a.invited": 5`)
	assert.Contains(t, out.String(), `"title": "x"`)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ptorch/ -v`
Expected: FAIL — `undefined: run`.

- [ ] **Step 3: Write the implementation**

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GoMudEngine/GoMud-Module-Playtest-Harness/internal/blackboard"
	"github.com/GoMudEngine/GoMud-Module-Playtest-Harness/internal/scenario"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: ptorch <scenario|bb> ...")
		return 2
	}
	switch args[0] {
	case "scenario":
		return runScenario(args[1:], stdout, stderr)
	case "bb":
		return runBB(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func loadScenario(path string, stderr io.Writer) (scenario.Scenario, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "read %s: %v\n", path, err)
		return scenario.Scenario{}, false
	}
	s, err := scenario.Parse(b)
	if err != nil {
		fmt.Fprintf(stderr, "parse %s: %v\n", path, err)
		return scenario.Scenario{}, false
	}
	return s, true
}

func runScenario(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: ptorch scenario <validate|plan> <file>")
		return 2
	}
	sub, path := args[0], args[1]
	s, ok := loadScenario(path, stderr)
	if !ok {
		return 1
	}
	if err := s.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch sub {
	case "validate":
		fmt.Fprintf(stdout, "OK: %q (%s, %d agents)\n", s.Name, s.Mode, len(s.Roster))
		for _, w := range s.Warnings() {
			fmt.Fprintln(stdout, "WARNING:", w)
		}
		return 0
	case "plan":
		type rosterOut struct {
			ID     string `json:"id"`
			Role   string `json:"role"`
			Target string `json:"target"`
		}
		out := struct {
			Name           string              `json:"name"`
			Mode           string              `json:"mode"`
			Summary        string              `json:"summary"`
			MaxConnections int                 `json:"max_connections"`
			Roster         []rosterOut         `json:"roster"`
			GroupGoals     []scenario.Goal     `json:"group_goals"`
			Requires       scenario.Requires   `json:"requires"`
			Warnings       []string            `json:"warnings"`
		}{
			Name: s.Name, Mode: s.Mode, Summary: s.Summary,
			MaxConnections: s.MaxConnections(),
			GroupGoals:     s.GroupGoals, Requires: s.Requires,
			Warnings: s.Warnings(),
		}
		for _, r := range s.Roster {
			out.Roster = append(out.Roster, rosterOut{r.ID, r.Role, r.Target})
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown scenario subcommand %q\n", sub)
		return 2
	}
}

func runBB(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: ptorch bb <init|ready|allready|phase|signal|finding|dump> <path> [flags]")
		return 2
	}
	sub, path := args[0], args[1]
	fs := flag.NewFlagSet("bb "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		runName = fs.String("run", "", "run name")
		ids     = fs.String("ids", "", "comma-separated agent ids")
		id      = fs.String("id", "", "agent id")
		setPh   = fs.String("set", "", "phase to set")
		name    = fs.String("name", "", "signal name")
		round   = fs.Int("round", 0, "beacon round")
		agent   = fs.String("agent", "", "finding agent id")
		ftype   = fs.String("type", "", "finding type")
		title   = fs.String("title", "", "finding title")
	)
	if err := fs.Parse(args[2:]); err != nil {
		return 2
	}

	switch sub {
	case "init":
		var list []string
		if *ids != "" {
			list = strings.Split(*ids, ",")
		}
		if err := blackboard.Init(path, *runName, list); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "ready":
		if err := blackboard.SetReady(path, *id); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "allready":
		ok, err := blackboard.AllReady(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if !ok {
			return 3 // not-all-ready: distinct exit code for shell branching
		}
		return 0
	case "phase":
		if *setPh != "" {
			if err := blackboard.SetPhase(path, *setPh); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		}
		p, err := blackboard.Phase(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, p)
		return 0
	case "signal":
		if err := blackboard.Signal(path, *name, *round); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "finding":
		f := blackboard.Finding{Agent: *agent, Type: *ftype, Title: *title, Round: *round}
		if err := blackboard.AddFinding(path, f); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "dump":
		bd, err := blackboard.Load(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(bd); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown bb subcommand %q\n", sub)
		return 2
	}
}
```

- [ ] **Step 4: Run tests + build to verify they pass**

Run: `go test ./cmd/ptorch/ -v && go build ./...`
Expected: PASS, build clean. (`bb dump` prints `phase` as `running` and findings; note the `dump` test asserts on the indented JSON substrings.)

- [ ] **Step 5: Commit**

```bash
git add cmd/ptorch/main.go cmd/ptorch/main_test.go
git commit -m "feat(ptorch): CLI over scenario + blackboard for the conductor/agents"
```

---

### Task 7: Scenario schema doc + generic template

**Files:**
- Create: `framework/scenarios/SCHEMA.md`
- Create: `framework/scenarios/template.yaml`

- [ ] **Step 1: Write `framework/scenarios/SCHEMA.md`**

````markdown
# Scenario File Schema

A **scenario file** defines a multi-agent playtest run. It is game-agnostic —
engine-specific commands still come from `engine-profile.yaml`. Run one with the
`/playtest-scenario <name>` conductor (see `.claude/commands/playtest-scenario.md`).

## Fields

- `name` (string, required) — run name; the combined report references it.
- `mode` (string, required) — one of:
  - `party` — testers coordinate toward a shared goal (invite/accept, group play).
  - `adversarial` — independent players interacting non-cooperatively (PvP swings,
    contested resources, trade, racing). Per-agent goals may conflict.
  - `parallel` — N testers each pursue their own goals; no interaction expected.
  - `scenario` — a scripted sequence driven by the `choreography` block.
- `summary` (string) — one line describing what the run validates.
- `requires` (map, optional) — server preconditions the conductor **verifies and
  surfaces** (it never changes server config):
  - `permadeath` (bool) — the server's `Death.PermaDeath` setting the run expects.
  - `death_protection` (bool) — the playtest module's `DeathProtection` setting.
  - `max_connections` (int) — your server's `Network.AI.MaxConnections` (default 20).
- `roster` (list, required) — the tester agents. Each entry:
  - `id` (string, required) — stable id used in goals/choreography/reports.
  - `role` (string, required) — an existing personality (`framework/personalities/`).
  - `target` (string, required) — a `targets.yaml` entry. Blank creds there means
    the agent creates a character on first run.
  - `goals` (list, optional) — per-agent goals in the standard `id`/`do`/`verify`
    shape (see `framework/goals/SCHEMA.md`).
- `group_goals` (list, optional) — interaction-level objectives, agent-judged, in
  the `id`/`do`/`verify` shape. Evidence may span multiple agents.
- `choreography` (list, optional) — ordered steps, mainly for `scenario` mode:
  - `who` (string, required) — a roster id.
  - `do` (string, required) — what that agent does.
  - `after` (string, optional) — a blackboard signal name to wait for first.
  - `round` (int, optional) — an absolute beacon round to act on.

## Verification model

Like single-agent goals, verification is **agent-judged** from observed `output`,
`gmcp`, and `beacon` events across agents — there is no assertion engine. Write
`verify` so an agent can tell from what it sees (and from the other agents' state
on the blackboard) whether the goal succeeded.

## Limits & cost

⚠️ Each roster agent is an independent LLM loop. **N agents cost roughly N× the
tokens and local processing of a single `/playtest` run.** Start with 2, watch
your usage rate, and keep rosters small. The server also caps concurrent AI
clients at `Network.AI.MaxConnections` (default 20) — raise it there if you need
more, and set `requires.max_connections` to match so the conductor can warn early.
````

- [ ] **Step 2: Write `framework/scenarios/template.yaml`** (generic, annotated)

```yaml
# Generic multi-agent scenario template. Copy this, fill it in, and run with:
#   /playtest-scenario <name>
# Full field docs: framework/scenarios/SCHEMA.md
#
# COST WARNING: each agent below is an independent LLM loop. N agents cost
# roughly N× a single /playtest run. Start with 2 and watch your usage rate.

name: my-scenario
mode: party                 # party | adversarial | parallel | scenario
summary: One line describing what this run validates.

requires:                   # server preconditions (verified + surfaced, not set)
  permadeath: false         # the server's Death.PermaDeath setting you expect
  death_protection: true    # the playtest module's DeathProtection setting
  max_connections: 20       # your server's Network.AI.MaxConnections

roster:
  - id: agent1
    role: feature-tester    # a personality under framework/personalities/
    target: local           # a targets.yaml entry (blank creds = creates a char)
    # goals:                # optional per-agent goals (id/do/verify)
    #   - id: solo
    #     do: <what this agent does alone>
    #     verify: <how to judge it>
  - id: agent2
    role: feel-tester
    target: local

group_goals:                # interaction-level objectives (agent-judged)
  - id: example
    do: <what the agents do together / to each other>
    verify: <cross-agent evidence that judges success>

# choreography:             # optional; ordered steps for `scenario` mode
#   - who: agent1
#     do: <first action>
#   - who: agent2
#     after: agent1.done    # wait for a blackboard signal an agent emits
#     do: <reaction>
```

- [ ] **Step 3: Validate the template parses**

Run: `go run ./cmd/ptorch scenario validate framework/scenarios/template.yaml`
Expected: `OK: "my-scenario" (party, 2 agents)` (no error exit).

- [ ] **Step 4: Commit**

```bash
git add framework/scenarios/SCHEMA.md framework/scenarios/template.yaml
git commit -m "docs(scenarios): scenario schema + generic annotated template"
```

---

### Task 8: The four worked example scenarios + party expected report

**Files:**
- Create: `framework/scenarios/examples/party-formation.yaml`
- Create: `framework/scenarios/examples/party-formation.expected-report.md`
- Create: `framework/scenarios/examples/adversarial-contested-pickup.yaml`
- Create: `framework/scenarios/examples/parallel-coverage.yaml`
- Create: `framework/scenarios/examples/scenario-trap-and-spring.yaml`

Each file starts with a header comment stating its validation status (the party
example is validated end-to-end in Task 12; the other three ship as adaptable
starting templates), per spec §11.

- [ ] **Step 1: Write `party-formation.yaml`** (the v1 validated scenario)

```yaml
# VALIDATED end-to-end (see docs/e2e/). A 2-agent party: the leader forms a party
# and invites the member, who accepts; both then confirm shared party state.
name: party-formation
mode: party
summary: Two testers form a party and verify shared party membership via GMCP.

requires:
  permadeath: false
  death_protection: true
  max_connections: 20

roster:
  - id: leader
    role: feature-tester
    target: local
  - id: member
    role: feel-tester
    target: local

group_goals:
  - id: form
    do: leader runs `party create` then `party invite member`; member runs `party accept`
    verify: both agents see the two-person party (GMCP PartyUpdated / party status shows the other member)
  - id: party-chat
    do: leader sends a party chat line; member confirms receipt
    verify: member's event stream shows the leader's party message
```

- [ ] **Step 2: Write `party-formation.expected-report.md`**

```markdown
# Multi-Agent Playtest Report: party-formation

**Date:** <YYYY-MM-DD>
**Scenario:** party-formation (mode: party)
**Agents:** leader (feature-tester), member (feel-tester)
**Server:** local (AI port)

## Summary
Two testers connected, each created a character, and met in the start area. The
leader created a party and invited the member; the member accepted. Both then
observed the two-person party in their GMCP state, and a party-chat line from the
leader reached the member.

## Group Goal Results
- [x] form — PASS: leader's GMCP party listed `member`; member's listed `leader`
      after `party accept` (PartyUpdated received by both).
- [x] party-chat — PASS: member's event stream showed the leader's party message.

## Per-Agent Outcomes
- leader (feature-tester): formed the party and invited successfully; no errors.
- member (feel-tester): accepted cleanly; onboarding-to-party flow felt clear.

## Findings
### PASS: Party formation and shared state
Invite → accept produced consistent two-sided GMCP party state and working party
chat.

## Stats
- Agents: 2
- Group goals: 2/2 PASS
- Bugs / Concerns / Observations: 0 / 0 / 0
```

- [ ] **Step 3: Write `adversarial-contested-pickup.yaml`**

```yaml
# STARTING TEMPLATE (not yet validated end-to-end). Two testers race for the same
# item; exactly one should get it. Adapt room/item to your world via the engine
# profile, then run it.
name: adversarial-contested-pickup
mode: adversarial
summary: Two testers race to grab the same single item; verify exactly one wins.

requires:
  permadeath: false
  death_protection: true
  max_connections: 20

roster:
  - id: racer-a
    role: bug-finder
    target: local
  - id: racer-b
    role: bug-finder
    target: local

group_goals:
  - id: contest
    do: both racers move to the room holding a single take-able item and each run `get <item>`
    verify: exactly one racer ends up with the item; the other sees a clean "not here / already taken" result (no dupes, no error)
```

- [ ] **Step 4: Write `parallel-coverage.yaml`**

```yaml
# STARTING TEMPLATE (not yet validated end-to-end). Independent testers each run
# their own short goals at once — coverage/concurrency, no interaction expected.
name: parallel-coverage
mode: parallel
summary: Several testers independently exercise different areas concurrently.

requires:
  permadeath: false
  death_protection: true
  max_connections: 20

roster:
  - id: explorer
    role: feel-tester
    target: local
    goals:
      - id: wander
        do: explore several rooms and read their descriptions
        verify: visited 5+ distinct rooms with no movement errors
  - id: shopper
    role: feature-tester
    target: local
    goals:
      - id: shop
        do: find a shop, list wares, buy and sell one item
        verify: inventory and currency change consistently with the transactions
```

- [ ] **Step 5: Write `scenario-trap-and-spring.yaml`**

```yaml
# STARTING TEMPLATE (not yet validated end-to-end). A scripted sequence: agent A
# stages something, then agent B reacts after A signals it is ready. Demonstrates
# the choreography + signals mechanism (the agent emits the signal via:
#   ptorch bb signal <bb> --name setter.ready --round <current round>).
name: scenario-trap-and-spring
mode: scenario
summary: One tester stages a situation; the other reacts on cue.

requires:
  permadeath: false
  death_protection: true
  max_connections: 20

roster:
  - id: setter
    role: feature-tester
    target: local
  - id: reactor
    role: bug-finder
    target: local

group_goals:
  - id: sequence
    do: setter prepares the situation and signals ready; reactor then acts on it
    verify: reactor only acts after setter.ready; the staged interaction resolves as intended

choreography:
  - who: setter
    do: move to the agreed room, prepare, then emit the `setter.ready` signal
  - who: reactor
    after: setter.ready
    do: enter and respond to what setter staged
```

- [ ] **Step 6: Validate every example parses (Validate gate)**

Run:
```bash
for f in framework/scenarios/examples/*.yaml; do
  echo "== $f =="; go run ./cmd/ptorch scenario validate "$f" || exit 1
done
```
Expected: each prints `OK: ...` (exit 0). Adversarial/parallel/scenario may also
print COST warnings only if rosters grow ≥3 — these have 2 agents, so no COST
warning; all must exit 0.

- [ ] **Step 7: Commit**

```bash
git add framework/scenarios/examples/
git commit -m "docs(scenarios): worked examples per mode + party expected report"
```

---

### Task 9: Combined report format + agent-runner instructions

**Files:**
- Create: `framework/multi-agent-report-format.md`
- Create: `framework/agent-runner.md`

- [ ] **Step 1: Write `framework/multi-agent-report-format.md`**

````markdown
# Multi-Agent (Scenario) Report Format

The conductor writes ONE combined report per scenario run, plus the usual
per-agent reports (see `report-format.md`) for each roster id. Combined report
shape:

```markdown
# Multi-Agent Playtest Report: <scenario name>

**Date:** <YYYY-MM-DD>
**Scenario:** <name> (mode: <mode>)
**Agents:** <id> (<role>), <id> (<role>), ...
**Server:** <target> (AI port)

## Summary
<2-4 sentences on the run arc across agents.>

## Group Goal Results
- [x] <goal id>: <do> — PASS: <cross-agent evidence>
- [ ] <goal id>: <do> — FAIL: <observed vs. expected, which agents>

## Per-Agent Outcomes
- <id> (<role>): <one-line outcome; link/reference its per-agent report>

## Findings
(Merged from all agents' blackboard findings, deduped, tagged by agent. Keep the
BUG/CONCERN/OBSERVATION/PASS/FAIL/BLOCKED categories from report-format.md.)
### BUG: <title> (<agent id>)
<repro: where, what was typed, what happened, what was expected>

## Stats
- Agents: <N>
- Group goals: <P>/<T> PASS
- Bugs / Concerns / Observations: <N> / <N> / <N>
```

## Conventions
- Name it `framework/reports/<date>-<scenario>.md`.
- Group-goal evidence should cite which agents observed what (e.g., "leader's GMCP
  party shows member; member's shows leader").
- Findings come from each agent's blackboard `findings` entries (the conductor
  reads them with `ptorch bb dump`), merged and deduped, each tagged by agent.
````

- [ ] **Step 2: Write `framework/agent-runner.md`** (per-agent instructions the conductor injects into each subagent)

````markdown
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
````

- [ ] **Step 3: Commit**

```bash
git add framework/multi-agent-report-format.md framework/agent-runner.md
git commit -m "docs(framework): combined report format + agent-runner instructions"
```

---

### Task 10: Conductor command `/playtest-scenario`

**Files:**
- Create: `.claude/commands/playtest-scenario.md`

- [ ] **Step 1: Write `.claude/commands/playtest-scenario.md`**

````markdown
---
description: Run a multi-agent (party / adversarial / parallel / scenario) playtest
argument-hint: <scenario-name>
---

# /playtest-scenario `<scenario-name>`

The reference conductor for **multi-agent** runs. It reads a scenario file, spawns
one independent agent per roster entry, coordinates them via the game + a small
shared blackboard, and writes a combined report. Auto-discovered from the repo —
no install. (Single-agent runs still use `/playtest`.)

> ⚠️ **Cost:** each roster agent is an independent LLM loop. **N agents cost
> roughly N× a single `/playtest` run** in tokens and local processing. Start with
> 2 agents, watch your usage rate, and keep rosters small. The server also caps AI
> clients at `Network.AI.MaxConnections` (default 20).

## 1. Load and check the scenario
- The scenario file is `framework/scenarios/<scenario-name>.yaml` (or an
  `examples/<...>.yaml`). Get its machine-readable plan:
  ```sh
  go run ./cmd/ptorch scenario plan framework/scenarios/<scenario-name>.yaml
  ```
  This emits JSON: `name`, `mode`, `max_connections`, `roster` (id/role/target),
  `group_goals`, `requires`, and `warnings`. If the command exits non-zero, the
  file is invalid — show the error and stop.
- **Surface every `warnings` entry to the user** (over-limit roster, COST). If the
  roster exceeds `max_connections`, stop and tell the user to raise
  `Network.AI.MaxConnections` (or lower the roster) before continuing.
- **Surface `requires` as preconditions to confirm** — the conductor does NOT
  change server config. If `requires.permadeath`/`death_protection` matter for the
  run (e.g., a lethal scenario), tell the user to set them on the server first.
  Where detectable in-game (e.g., the status panel showing Lives implies permadeath
  is on), note any mismatch.

## 2. Seed the blackboard
```sh
RUN="<scenario-name>-<date>"      # date passed in by you; do not invent timestamps in code
BB=".playtest/$RUN/blackboard.json"
go run ./cmd/ptorch bb init "$BB" --run "$RUN" --ids "<comma-separated roster ids>"
```

## 3. Spawn one agent per roster entry (background, independent)
For each roster entry, dispatch a **background subagent** whose instructions are
`framework/agent-runner.md`, parameterized with: that entry's `id`, `role`,
`target`, the relevant `group_goals` + per-agent `goals` + any `choreography`
lines naming it, the blackboard path `$BB`, and a private bridge dir
`.playtest/$RUN/<id>/`. Each agent connects, creates/logs in its character, and
marks itself ready.

(Other agent runtimes can spawn OS processes instead — the scenario file +
blackboard CLI are the engine-agnostic contract; subagents are just the reference.)

## 4. Readiness barrier
Wait for all agents to be present, then start the run:
```sh
until go run ./cmd/ptorch bb allready "$BB"; do sleep 1; done   # exit 0 = all ready
go run ./cmd/ptorch bb phase "$BB" --set running
```

## 5. Let agents run; wait for completion
Agents now play their assignments, interacting in-game and via signals. Wait for
all background subagents to finish (each writes its per-agent report and appends
its findings to the blackboard), then:
```sh
go run ./cmd/ptorch bb phase "$BB" --set done
```

## 6. Aggregate the combined report
Read the final blackboard and each per-agent report:
```sh
go run ./cmd/ptorch bb dump "$BB"
```
Write the combined report per `framework/multi-agent-report-format.md` to
`framework/reports/<date>-<scenario-name>.md`: scenario summary, group-goal
results with cross-agent evidence, per-agent outcomes, and the merged/deduped
findings (already deduped per agent+title on the blackboard).

## 7. Clean up
Quit any still-running `mudagent`s (each agent does this on finish; clean up
strays as in `.claude/commands/playtest.md` step 7). Report the combined-report
path to the user.
````

- [ ] **Step 2: Sanity-check it reads cleanly** (no code to run; verify the file exists and the front-matter matches the `/playtest.md` style)

Run: `head -5 .claude/commands/playtest-scenario.md`
Expected: shows the `---` front matter with `description:` and `argument-hint:`.

- [ ] **Step 3: Commit**

```bash
git add .claude/commands/playtest-scenario.md
git commit -m "feat(driver): /playtest-scenario multi-agent conductor command"
```

---

### Task 11: README multi-agent section (limit + cost) + followups update

**Files:**
- Modify: `README.md`
- Modify: `docs/followups.md`

- [ ] **Step 1: Add a multi-agent section to `README.md`**

Insert a new section immediately after the existing "Run a playtest with your AI
agent" section (before "Gotchas & troubleshooting"):

```markdown
---

## Run a MULTI-agent playtest (party / adversarial / parallel / scenario)

For testing multiplayer features — parties, PvP, contested resources, trade,
scripted social scenarios — the `/playtest-scenario` conductor runs several
independent tester agents from one **scenario file** and writes a combined report.

```
/playtest-scenario party-formation
```

- **Define the run in one file:** `framework/scenarios/<name>.yaml` — mode, a
  roster of agents (each with a role/personality and target), group goals, and an
  optional scripted choreography. Start from `framework/scenarios/template.yaml`
  or copy a worked example under `framework/scenarios/examples/` (one per mode;
  `party-formation` is validated end-to-end). Schema:
  [`framework/scenarios/SCHEMA.md`](framework/scenarios/SCHEMA.md).
- **How it coordinates:** agents interact *through the game* (real `party invite`,
  attacks, trades) plus a tiny shared blackboard for a readiness barrier, scripted
  timing, and findings collection. Reports follow
  [`framework/multi-agent-report-format.md`](framework/multi-agent-report-format.md).

> ⚠️ **Connection limit & cost — read this.**
> - GoMud caps concurrent AI clients at **`Network.AI.MaxConnections` (default
>   20)**. It's a preconfigured limit you can raise or lower in
>   `_datafiles/config.yaml` (or `config-overrides.yaml`). Set
>   `requires.max_connections` in your scenario to match so the conductor warns
>   early if your roster is too big.
> - **Running many orchestrated agents is expensive.** Each agent is an
>   independent LLM loop, so **N agents cost roughly N× the tokens and local
>   processing/time of a single `/playtest` run.** Use with caution: start with 2
>   agents, prefer the smallest roster that exercises the feature, and **watch your
>   usage rate.** Large rosters and long runs multiply quickly.
```

- [ ] **Step 2: Update `docs/followups.md`**

Replace the existing "Group / multi-tester runs (party mechanics)" bullet under
"What's next (v0.2 ideas)" with:

```markdown
- **Group / multi-tester runs (party mechanics).** DESIGNED + IN PROGRESS — spec
  `docs/superpowers/specs/2026-06-07-multi-agent-testing-design.md`, plan
  `docs/superpowers/plans/2026-06-07-multi-agent-testing.md`. v1 ships the general
  N-agent framework (scenario file, conductor, blackboard, combined report,
  starting templates) validated by a 2-agent party run. Deferred to follow-ups:
  lethal-PvP / per-agent death-protection (the only part that would touch
  `module/playtest/*` → a release); >2-agent soak tuning; tight turn-by-turn combat
  choreography.
```

- [ ] **Step 3: Commit**

```bash
git add README.md docs/followups.md
git commit -m "docs: README multi-agent section (limit + cost) + followups status"
```

---

### Task 12: Live 2-agent party E2E validation

**Files:**
- Create: `docs/e2e/2026-06-07-multiagent-party.md`

This is an integration/validation task (not TDD). It mirrors the single-agent E2E
already on record. **Server hygiene:** only boot a server if the user confirms
they're not smoke-testing; use a uniquely-named test binary and stop it by exact
name when done (never a broad `*mud*` kill).

- [ ] **Step 1: Boot a local server with the AI port + playtest module**

```bash
cd ~/GoMud
# ensure Network.AI.Port: 55555, then:
go build -o go-mud-server-e2e.exe .
./go-mud-server-e2e.exe > ~/workspace/srv-ma-e2e.log 2>&1 &
# wait for "MainWorker state=Started" and confirm 55555 is listening
```

- [ ] **Step 2: Run the conductor on the party example**

From `~/workspace/gomud-playtest-harness`, run `/playtest-scenario party-formation`
(or drive the conductor steps manually: `scenario plan` → `bb init` → spawn two
agent runners against `localhost:55555` with blank creds → `bb allready` →
`bb phase --set running` → agents form the party → `bb phase --set done` →
aggregate).

- [ ] **Step 3: Verify and record**

Confirm: both agents reached `logged_in`; the readiness barrier flipped to
`running`; `party create`/`invite`/`accept` produced two-sided GMCP party state;
party chat reached the member; the combined report was written to
`framework/reports/`. Capture the input, both agents' event streams, the final
`bb dump`, and the combined report into `docs/e2e/2026-06-07-multiagent-party.md`.

- [ ] **Step 4: Stop the server (exact name) and clean up**

```bash
# PowerShell: Get-Process -Name 'go-mud-server-e2e' | Stop-Process -Force
rm -f ~/GoMud/go-mud-server-e2e.exe ~/workspace/srv-ma-e2e.log
# revert ~/GoMud Network.AI.Port back to 0
```

- [ ] **Step 5: Commit**

```bash
cd ~/workspace/gomud-playtest-harness
git add docs/e2e/2026-06-07-multiagent-party.md
git commit -m "docs(e2e): 2-agent party run validated end-to-end"
```

---

### Task 13: Final validation pass

**Files:** none (verification + a possible `go mod tidy`)

- [ ] **Step 1: Full build, vet, and test**

```bash
cd ~/workspace/gomud-playtest-harness
go build ./...        # Expected: clean
go vet ./...          # Expected: clean
go test ./...         # Expected: all packages PASS (scenario, blackboard, ptorch, existing)
```

- [ ] **Step 2: Tidy modules if `yaml.v3` moved from indirect to direct**

```bash
go mod tidy
git diff --stat go.mod go.sum    # If changed, include them.
```

- [ ] **Step 3: Validate every shipped scenario file once more**

```bash
go run ./cmd/ptorch scenario validate framework/scenarios/template.yaml
for f in framework/scenarios/examples/*.yaml; do go run ./cmd/ptorch scenario validate "$f"; done
# Expected: each prints OK and exits 0.
```

- [ ] **Step 4: Commit any tidy changes**

```bash
git add -A
git commit -m "chore: go mod tidy after adding scenario/blackboard packages" || echo "nothing to commit"
```

---

## Self-Review (completed by plan author)

**Spec coverage:**
- §1 modes — `mode` enum + per-mode examples (Tasks 2, 8). ✓
- §2 decisions — independent agents + conductor (Task 10), in-game + blackboard
  (Tasks 3–5, 9), one scenario file (Tasks 1–2, 7), client-side death (requires
  surfaced, not set — Tasks 1, 10). ✓
- §3 architecture / §4 components — scenario pkg (1–2), blackboard pkg (3–5),
  ptorch CLI (6), conductor (10), agent runner (9), reports (9). ✓
- §5 lifecycle — barrier + phases in blackboard (4) and conductor (10). ✓
- §6 limits/cost/safety — Warnings (2), README + SCHEMA + conductor + template
  cost/limit notes (7, 10, 11); requires surfaced not set (10). ✓
- §7 v1 deliverable + validation — all build tasks + the 2-agent party E2E (12). ✓
- §9 testing — Go unit tests for parser (1–2) and blackboard incl. concurrency
  (3–5); live E2E (12). ✓
- §11 templates — generic template (7) + four examples + party expected report
  (8), with honest validation-status headers. ✓

**Placeholder scan:** No TBD/TODO; every code step has complete code; doc tasks
have full file contents; commands have expected output. ✓

**Type consistency:** `scenario.Parse/Validate/MaxConnections/Warnings`,
`Scenario/Requires/RosterEntry/Goal/ChoreographyStep`; `blackboard.Init(path,run,
ids)/SetReady/AllReady/SetPhase/Phase/Signal/AddFinding/Load`, `Board/Finding`,
`PhaseLobby/Running/Done`; `cmd/ptorch` `run(args,stdout,stderr) int` with
`scenario validate|plan` and `bb init|ready|allready|phase|signal|finding|dump` —
used consistently across Tasks 1–6, 10, and the agent runner. ✓
