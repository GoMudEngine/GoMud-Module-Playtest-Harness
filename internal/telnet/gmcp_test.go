package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoGMCP(t *testing.T) {
	assert.Equal(t, []byte{IAC, DO, GMCP}, DoGMCP())
}

func TestFrameGMCP(t *testing.T) {
	got := FrameGMCP("Core.Hello", `{"client":"mudagent","version":"1"}`)
	want := append([]byte{IAC, SB, GMCP}, []byte(`Core.Hello {"client":"mudagent","version":"1"}`)...)
	want = append(want, IAC, SE)
	assert.Equal(t, want, got)
}
