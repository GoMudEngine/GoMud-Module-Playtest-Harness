package playtest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBeaconPayloadJSON(t *testing.T) {
	b, err := json.Marshal(beaconPayload{Round: 5, HP: 10, HPMax: 20, SP: 3, SPMax: 8, RoomID: 1})
	require.NoError(t, err)
	assert.JSONEq(t, `{"round":5,"hp":10,"hp_max":20,"sp":3,"sp_max":8,"room_id":1}`, string(b))
}
