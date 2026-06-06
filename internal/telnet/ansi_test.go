package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripAnsi(t *testing.T) {
	assert.Equal(t, "red text", string(StripAnsi([]byte("\x1b[31mred\x1b[0m text"))))
	assert.Equal(t, "plain", string(StripAnsi([]byte("plain"))))
}
