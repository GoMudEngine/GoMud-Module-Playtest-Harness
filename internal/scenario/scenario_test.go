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
  perma_death_protection: true
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
	require.NotNil(t, s.Requires.PermaDeathProtection)
	assert.True(t, *s.Requires.PermaDeathProtection)
	require.Len(t, s.GroupGoals, 1)
	assert.Equal(t, "form", s.GroupGoals[0].ID)
	require.Len(t, s.Choreography, 2)
	assert.Equal(t, "leader.invited", s.Choreography[1].After)
}

func validScenario() Scenario {
	return Scenario{
		Name: "s", Mode: "party",
		Roster: []RosterEntry{
			{ID: "a", Role: "bug-finder", Target: "local"},
			{ID: "b", Role: "feel-tester", Target: "local"},
		},
	}
}

func TestValidatePasses(t *testing.T) {
	require.NoError(t, validScenario().Validate())
}

func TestValidateRejectsBadMode(t *testing.T) {
	s := validScenario()
	s.Mode = "co-op"
	assert.ErrorContains(t, s.Validate(), "invalid mode")
}

func TestValidateRejectsEmptyRoster(t *testing.T) {
	s := validScenario()
	s.Roster = nil
	assert.ErrorContains(t, s.Validate(), "at least 1 agent")
}

func TestValidateRejectsDuplicateIDs(t *testing.T) {
	s := validScenario()
	s.Roster[1].ID = "a"
	assert.ErrorContains(t, s.Validate(), "duplicate roster id")
}

func TestValidateRejectsMissingRole(t *testing.T) {
	s := validScenario()
	s.Roster[0].Role = ""
	assert.ErrorContains(t, s.Validate(), "missing role")
}

func TestValidateRejectsMissingTarget(t *testing.T) {
	s := validScenario()
	s.Roster[0].Target = ""
	assert.ErrorContains(t, s.Validate(), "missing target")
}

func TestValidateRejectsUnknownChoreographyWho(t *testing.T) {
	s := validScenario()
	s.Choreography = []ChoreographyStep{{Who: "ghost", Do: "wave"}}
	assert.ErrorContains(t, s.Validate(), "not in roster")
}

func TestValidateRejectsMissingChoreographyWho(t *testing.T) {
	s := validScenario()
	s.Choreography = []ChoreographyStep{{Do: "wave"}}
	assert.ErrorContains(t, s.Validate(), "missing who")
}

func TestValidateRejectsMissingName(t *testing.T) {
	s := validScenario()
	s.Name = ""
	assert.ErrorContains(t, s.Validate(), "name is required")
}

func TestValidateRejectsMissingRosterID(t *testing.T) {
	s := validScenario()
	s.Roster[0].ID = ""
	assert.ErrorContains(t, s.Validate(), "missing id")
}

func TestWarningsNoneForSmallRoster(t *testing.T) {
	assert.Empty(t, validScenario().Warnings())
}

func TestMaxConnectionsDefaultsTo20(t *testing.T) {
	assert.Equal(t, 20, validScenario().MaxConnections())
	s := validScenario()
	s.Requires.MaxConnections = 5
	assert.Equal(t, 5, s.MaxConnections())
}

func TestWarningsFlagOverLimitAndCost(t *testing.T) {
	s := validScenario()
	s.Requires.MaxConnections = 1 // roster of 2 exceeds it
	w := s.Warnings()
	joined := ""
	for _, x := range w {
		joined += x + "\n"
	}
	assert.Contains(t, joined, "max_connections")

	big := validScenario()
	for i := 0; i < 3; i++ {
		big.Roster = append(big.Roster, RosterEntry{ID: "x" + string(rune('a'+i)), Role: "bug-finder", Target: "local"})
	}
	costJoined := ""
	for _, x := range big.Warnings() {
		costJoined += x + "\n"
	}
	assert.Contains(t, costJoined, "COST")
}

func TestParsePvpAndOnboarding(t *testing.T) {
	src := []byte(`
name: pvp
mode: adversarial
requires:
  pvp: enabled
  minimum_level: 1
  permadeath: false
  perma_death_protection: false
roster:
  - id: a
    role: bug-finder
    target: local
    onboarding: full
  - id: b
    role: bug-finder
    target: local
`)
	s, err := Parse(src)
	require.NoError(t, err)
	assert.Equal(t, "enabled", s.Requires.PVP)
	assert.Equal(t, 1, s.Requires.MinimumLevel)
	require.NotNil(t, s.Requires.PermaDeathProtection)
	assert.False(t, *s.Requires.PermaDeathProtection)
	assert.Equal(t, "full", s.Roster[0].Onboarding)
	assert.Equal(t, "", s.Roster[1].Onboarding)
	require.NoError(t, s.Validate())
}

func TestValidateRejectsBadOnboarding(t *testing.T) {
	s := validScenario()
	s.Roster[0].Onboarding = "skip"
	assert.ErrorContains(t, s.Validate(), "invalid onboarding")
}

func TestValidateRejectsBadPvp(t *testing.T) {
	s := validScenario()
	s.Requires.PVP = "sometimes"
	assert.ErrorContains(t, s.Validate(), "invalid pvp")
}
