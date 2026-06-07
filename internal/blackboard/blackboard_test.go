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
