package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenarioValidateGoodFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s.yaml")
	require.NoError(t, os.WriteFile(f, []byte(`
name: s
mode: party
roster:
  - {id: a, role: bug-finder, target: local}
  - {id: b, role: feel-tester, target: local}
`), 0o644))

	var out, errb bytes.Buffer
	code := run([]string{"scenario", "validate", f}, &out, &errb)
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "OK")
}

func TestScenarioValidateBadFileExitsNonZero(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s.yaml")
	require.NoError(t, os.WriteFile(f, []byte("name: s\nmode: nope\nroster: []\n"), 0o644))

	var out, errb bytes.Buffer
	code := run([]string{"scenario", "validate", f}, &out, &errb)
	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "invalid mode")
}

func TestScenarioPlanEmitsJSON(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s.yaml")
	require.NoError(t, os.WriteFile(f, []byte(`
name: s
mode: party
roster:
  - {id: a, role: bug-finder, target: local}
  - {id: b, role: feel-tester, target: local}
`), 0o644))

	var out, errb bytes.Buffer
	code := run([]string{"scenario", "plan", f}, &out, &errb)
	require.Equal(t, 0, code)

	var plan struct {
		Name           string `json:"name"`
		Mode           string `json:"mode"`
		MaxConnections int    `json:"max_connections"`
		Roster         []struct {
			ID, Role, Target string
		} `json:"roster"`
		Warnings []string `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &plan))
	assert.Equal(t, "party", plan.Mode)
	assert.Equal(t, 20, plan.MaxConnections)
	require.Len(t, plan.Roster, 2)
	assert.Equal(t, "a", plan.Roster[0].ID)
}

func TestBBRejectsEmptyRequiredFlags(t *testing.T) {
	bb := filepath.Join(t.TempDir(), "bb.json")
	discard := &bytes.Buffer{}
	require.Equal(t, 0, run([]string{"bb", "init", bb, "--run", "r", "--ids", "a"}, discard, discard))

	// each missing required flag is a usage error (exit 2), and must not mutate the board
	assert.Equal(t, 2, run([]string{"bb", "ready", bb}, discard, &bytes.Buffer{}))
	assert.Equal(t, 2, run([]string{"bb", "signal", bb, "--round", "5"}, discard, &bytes.Buffer{}))
	assert.Equal(t, 2, run([]string{"bb", "finding", bb, "--agent", "a"}, discard, &bytes.Buffer{}))

	var out bytes.Buffer
	require.Equal(t, 0, run([]string{"bb", "dump", bb}, &out, discard))
	assert.NotContains(t, out.String(), `"": `, "no empty-string key should have been written")
}

func TestUsageErrorsExitTwo(t *testing.T) {
	d := &bytes.Buffer{}
	assert.Equal(t, 2, run(nil, d, d))
	assert.Equal(t, 2, run([]string{"bogus"}, d, d))
}

func TestBlackboardRoundTripThroughCLI(t *testing.T) {
	bb := filepath.Join(t.TempDir(), "bb.json")
	discard := &bytes.Buffer{}

	require.Equal(t, 0, run([]string{"bb", "init", bb, "--run", "r", "--ids", "a,b"}, discard, discard))

	// not all ready yet -> exit 3
	assert.Equal(t, 3, run([]string{"bb", "allready", bb}, discard, discard))

	require.Equal(t, 0, run([]string{"bb", "ready", bb, "--id", "a"}, discard, discard))
	require.Equal(t, 0, run([]string{"bb", "ready", bb, "--id", "b"}, discard, discard))
	assert.Equal(t, 0, run([]string{"bb", "allready", bb}, discard, discard))

	require.Equal(t, 0, run([]string{"bb", "phase", bb, "--set", "running"}, discard, discard))
	require.Equal(t, 0, run([]string{"bb", "signal", bb, "--name", "a.invited", "--round", "5"}, discard, discard))
	require.Equal(t, 0, run([]string{"bb", "finding", bb, "--agent", "a", "--type", "BUG", "--title", "x"}, discard, discard))

	var out bytes.Buffer
	require.Equal(t, 0, run([]string{"bb", "dump", bb}, &out, discard))
	assert.Contains(t, out.String(), `"phase": "running"`)
	assert.Contains(t, out.String(), `"a.invited": 5`)
	assert.Contains(t, out.String(), `"title": "x"`)
}
