package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGMCPEventClassification(t *testing.T) {
	// Playtest.* -> beacon event, with the suffix as the event name.
	b := gmcpEvent("Playtest.Round", []byte(`{"round":3}`))
	assert.Equal(t, "beacon", b.Type)
	assert.Equal(t, "Round", b.Event)
	assert.JSONEq(t, `{"round":3}`, string(b.Data))

	// Anything else -> gmcp event, unchanged.
	g := gmcpEvent("Char.Vitals", []byte(`{"hp":10}`))
	assert.Equal(t, "gmcp", g.Type)
	assert.Equal(t, "Char.Vitals", g.Package)
	assert.JSONEq(t, `{"hp":10}`, string(g.Data))
}
