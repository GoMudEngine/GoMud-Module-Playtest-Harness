package session

import "gopkg.in/yaml.v3"

// Manifest is the run configuration (connection + account). Personality and
// goals are consumed by the agent/driver, not the adapter, so they are not
// required here; unknown keys are ignored.
type Manifest struct {
	Target   string `yaml:"target"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// ParseManifest decodes a run manifest from YAML bytes.
func ParseManifest(b []byte) (Manifest, error) {
	var m Manifest
	err := yaml.Unmarshal(b, &m)
	return m, err
}
