// Package protocol defines the line-oriented JSON contract between mudagent
// (stdout events / stdin commands) and an external AI agent.
package protocol

import (
	"encoding/json"
	"strings"
)

// Event is one JSON object emitted per line on stdout.
type Event struct {
	Type    string          `json:"type"`              // output | gmcp | beacon | status | error
	Text    string          `json:"text,omitempty"`    // output: cleaned text
	Raw     string          `json:"raw,omitempty"`     // output: original (with ANSI), optional
	Package string          `json:"package,omitempty"` // gmcp: package name, e.g. "Char.Vitals"
	Event   string          `json:"event,omitempty"`   // beacon: event name
	Data    json.RawMessage `json:"data,omitempty"`    // gmcp/beacon: structured payload
	State   string          `json:"state,omitempty"`   // status: connected | logged_in | disconnected
	Message string          `json:"message,omitempty"` // error: message
}

// Marshal renders an Event as a single JSON line (no trailing newline).
func Marshal(e Event) (string, error) {
	b, err := json.Marshal(e)
	return string(b), err
}

// CommandKind distinguishes a game command from an adapter control verb.
type CommandKind int

const (
	CommandKindText CommandKind = iota
	CommandKindControl
)

// Command is one parsed stdin line.
type Command struct {
	Kind    CommandKind
	Text    string // for CommandKindText
	Control string // for CommandKindControl, e.g. "quit"
}

type controlLine struct {
	Control string `json:"control"`
}

// ParseCommand interprets a stdin line: a JSON object with a "control" key is a
// control verb; anything else is sent verbatim to the MUD.
func ParseCommand(line string) Command {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "{") {
		var cl controlLine
		if err := json.Unmarshal([]byte(trimmed), &cl); err == nil && cl.Control != "" {
			return Command{Kind: CommandKindControl, Control: cl.Control}
		}
	}
	return Command{Kind: CommandKindText, Text: line}
}
