package blackboard

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitClearsStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bb.json")
	// Simulate a lock left behind by a crashed prior run.
	require.NoError(t, os.WriteFile(lockPath(path), []byte("stale"), 0o644))

	// Init must clear the stale lock and succeed (not block for the full timeout).
	require.NoError(t, Init(path, "r", []string{"a"}))
	_, err := os.Stat(lockPath(path))
	assert.True(t, os.IsNotExist(err), "stale lock should be gone after Init")

	// And the board is usable afterwards.
	require.NoError(t, SetReady(path, "a"))
}

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
