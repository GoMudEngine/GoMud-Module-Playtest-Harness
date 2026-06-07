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
