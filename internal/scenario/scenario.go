package scenario

import "gopkg.in/yaml.v3"

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
	Permadeath      *bool `yaml:"permadeath"`
	DeathProtection *bool `yaml:"death_protection"`
	MaxConnections  int   `yaml:"max_connections"` // 0 → DefaultMaxConnections
}

// RosterEntry is one tester agent in the run.
type RosterEntry struct {
	ID     string `yaml:"id"`     // stable id used in goals/choreography/reports
	Role   string `yaml:"role"`   // an existing personality name
	Target string `yaml:"target"` // a targets.yaml entry
	Goals  []Goal `yaml:"goals"`  // optional per-agent goals
}

// Goal reuses the single-agent do/verify shape.
type Goal struct {
	ID     string `yaml:"id"`
	Do     string `yaml:"do"`
	Verify string `yaml:"verify"`
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
