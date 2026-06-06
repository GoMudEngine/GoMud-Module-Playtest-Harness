package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest([]byte("target: localhost:55555\nuser: aitester\npassword: secret\n"))
	require.NoError(t, err)
	assert.Equal(t, "localhost:55555", m.Target)
	assert.Equal(t, "aitester", m.User)
	assert.Equal(t, "secret", m.Password)
}
