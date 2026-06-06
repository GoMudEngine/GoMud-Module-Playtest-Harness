package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventMarshalsOnePerLine(t *testing.T) {
	line, err := Marshal(Event{Type: "status", State: "connected"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"status","state":"connected"}`, line)
}

func TestOutputEventOmitsEmptyFields(t *testing.T) {
	line, err := Marshal(Event{Type: "output", Text: "hello"})
	require.NoError(t, err)
	assert.NotContains(t, line, `"package"`)
	assert.NotContains(t, line, `"state"`)
	assert.Contains(t, line, `"text":"hello"`)
}

func TestParseCommandPlainLine(t *testing.T) {
	cmd := ParseCommand("look around")
	assert.Equal(t, CommandKindText, cmd.Kind)
	assert.Equal(t, "look around", cmd.Text)
}

func TestParseCommandControlQuit(t *testing.T) {
	cmd := ParseCommand(`{"control":"quit"}`)
	assert.Equal(t, CommandKindControl, cmd.Kind)
	assert.Equal(t, "quit", cmd.Control)
}
