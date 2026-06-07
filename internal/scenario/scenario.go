package scenario

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Scenario is a multi-agent playtest run definition. Engine-specific behavior
// still comes from engine-profile.yaml; this file is game-agnostic.
type Scenario struct {
	Name         string             `yaml:"name"`
	Mode         string             `yaml:"mode"` // party | adversarial | parallel | scenario
	Summary      string             `yaml:"summary"`
	Requires     Requires           `yaml:"requires"`
	Roster       []RosterEntry      `yaml:"roster"`
	GroupGoals   []Goal             `yaml:"group_goals"`
	Choreography []ChoreographyStep `yaml:"choreography"`
}

// Requires declares server preconditions. The harness VERIFIES/surfaces these;
// it never mutates server config. Pointers distinguish "unset" from "false".
type Requires struct {
	Permadeath           *bool  `yaml:"permadeath" json:"permadeath"`
	// PermaDeathProtection maps to the playtest module's DeathProtection setting,
	// which only matters under Death.PermaDeath (it guards perma-death only).
	PermaDeathProtection *bool  `yaml:"perma_death_protection" json:"perma_death_protection"`
	PVP                  string `yaml:"pvp" json:"pvp,omitempty"`           // enabled | limited | disabled
	MinimumLevel         int    `yaml:"minimum_level" json:"minimum_level,omitempty"`
	MaxConnections       int    `yaml:"max_connections" json:"max_connections"` // 0 → DefaultMaxConnections
}

// RosterEntry is one tester agent in the run.
type RosterEntry struct {
	ID         string `yaml:"id"`         // stable id used in goals/choreography/reports
	Role       string `yaml:"role"`       // an existing personality name
	Target     string `yaml:"target"`     // a targets.yaml entry
	Onboarding string `yaml:"onboarding"` // auto (default) | full (real new-player flow)
	Goals      []Goal `yaml:"goals"`      // optional per-agent goals
}

// Goal reuses the single-agent do/verify shape.
type Goal struct {
	ID     string `yaml:"id" json:"id"`
	Do     string `yaml:"do" json:"do"`
	Verify string `yaml:"verify" json:"verify"`
}

// ChoreographyStep is one ordered step (mainly for `scenario` mode).
type ChoreographyStep struct {
	Who   string `yaml:"who"`   // a roster id
	Do    string `yaml:"do"`    // what that agent does
	After string `yaml:"after"` // optional blackboard signal name to wait for
	Round int    `yaml:"round"` // optional absolute beacon round
}

// Parse decodes a scenario file from YAML bytes.
func Parse(b []byte) (Scenario, error) {
	var s Scenario
	if err := yaml.Unmarshal(b, &s); err != nil {
		return Scenario{}, err
	}
	return s, nil
}

// DefaultMaxConnections mirrors GoMud's Network.AI.MaxConnections default.
const DefaultMaxConnections = 20

var validModes = map[string]bool{
	"party": true, "adversarial": true, "parallel": true, "scenario": true,
}

// Validate returns the first structural error, or nil. Cost/limit advisories
// are non-fatal; see Warnings.
func (s Scenario) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("scenario: name is required")
	}
	if !validModes[s.Mode] {
		return fmt.Errorf("scenario %q: invalid mode %q (want party|adversarial|parallel|scenario)", s.Name, s.Mode)
	}
	if len(s.Roster) < 1 {
		return fmt.Errorf("scenario %q: roster must have at least 1 agent", s.Name)
	}
	seen := map[string]bool{}
	for i, r := range s.Roster {
		if r.ID == "" {
			return fmt.Errorf("scenario %q: roster[%d] missing id", s.Name, i)
		}
		if seen[r.ID] {
			return fmt.Errorf("scenario %q: duplicate roster id %q", s.Name, r.ID)
		}
		seen[r.ID] = true
		if r.Role == "" {
			return fmt.Errorf("scenario %q: roster %q missing role", s.Name, r.ID)
		}
		if r.Target == "" {
			return fmt.Errorf("scenario %q: roster %q missing target", s.Name, r.ID)
		}
		if r.Onboarding != "" && r.Onboarding != "auto" && r.Onboarding != "full" {
			return fmt.Errorf("scenario %q: roster %q invalid onboarding %q (want auto|full)", s.Name, r.ID, r.Onboarding)
		}
	}
	switch s.Requires.PVP {
	case "", "enabled", "limited", "disabled":
	default:
		return fmt.Errorf("scenario %q: invalid pvp %q (want enabled|limited|disabled)", s.Name, s.Requires.PVP)
	}
	for i, c := range s.Choreography {
		if c.Who == "" {
			return fmt.Errorf("scenario %q: choreography[%d] missing who", s.Name, i)
		}
		if !seen[c.Who] {
			return fmt.Errorf("scenario %q: choreography[%d] who %q not in roster", s.Name, i, c.Who)
		}
	}
	return nil
}

// MaxConnections is the scenario's connection cap (default DefaultMaxConnections).
func (s Scenario) MaxConnections() int {
	if s.Requires.MaxConnections > 0 {
		return s.Requires.MaxConnections
	}
	return DefaultMaxConnections
}

// Warnings returns non-fatal advisories: roster over the connection limit, and
// the token/processing cost of running many independent agents.
func (s Scenario) Warnings() []string {
	var w []string
	if n, max := len(s.Roster), s.MaxConnections(); n > max {
		w = append(w, fmt.Sprintf(
			"roster has %d agents but max_connections is %d — raise Network.AI.MaxConnections on the server (default %d) or it will refuse extra connections",
			n, max, DefaultMaxConnections))
	}
	if n := len(s.Roster); n >= 3 {
		w = append(w, fmt.Sprintf(
			"COST: %d independent agents ≈ %dx the tokens/processing of a single run — use with caution and watch your usage rate", n, n))
	}
	return w
}
