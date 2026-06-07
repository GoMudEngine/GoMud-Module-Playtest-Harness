package scenario

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseValidPartyScenario(t *testing.T) {
	src := []byte(`
name: party-smoke
mode: party
summary: Two testers form a party.
requires:
  permadeath: false
  death_protection: true
roster:
  - id: leader
    role: feature-tester
    target: local
  - id: member
    role: feel-tester
    target: local
group_goals:
  - id: form
    do: leader invites member; member accepts
    verify: both see each other in GMCP party state
choreography:
  - who: leader
    do: party create then invite member
  - who: member
    after: leader.invited
    do: party accept
`)
	s, err := Parse(src)
	require.NoError(t, err)
	assert.Equal(t, "party-smoke", s.Name)
	assert.Equal(t, "party", s.Mode)
	require.Len(t, s.Roster, 2)
	assert.Equal(t, "leader", s.Roster[0].ID)
	assert.Equal(t, "feel-tester", s.Roster[1].Role)
	require.NotNil(t, s.Requires.Permadeath)
	assert.False(t, *s.Requires.Permadeath)
	require.Len(t, s.GroupGoals, 1)
	assert.Equal(t, "form", s.GroupGoals[0].ID)
	require.Len(t, s.Choreography, 2)
	assert.Equal(t, "leader.invited", s.Choreography[1].After)
}
