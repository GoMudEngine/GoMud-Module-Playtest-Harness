package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRawJSONAlwaysValid(t *testing.T) {
	cases := map[string]string{
		`{"hp":10}`: `{"hp":10}`,  // valid object -> verbatim
		``:          `null`,       // empty -> null
		`not json`:  `"not json"`, // non-JSON -> JSON string
		`[1,2,3]`:   `[1,2,3]`,    // valid array -> verbatim
	}
	for in, want := range cases {
		got := rawJSON([]byte(in))
		assert.True(t, json.Valid(got), "rawJSON(%q) must be valid JSON, got %s", in, got)
		assert.JSONEq(t, want, string(got))
	}
}
