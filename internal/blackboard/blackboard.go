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
