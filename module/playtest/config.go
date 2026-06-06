package playtest

import "github.com/GoMudEngine/GoMud/internal/plugins"

// Config is the resolved module configuration.
type Config struct {
	Enabled         bool
	AccountName     string
	AccountPassword string
	SafeMode        bool
	SandboxZoneTag  string
	DeathProtection bool
}

func loadConfig(p *plugins.Plugin) Config { return Config{} } // implemented in Task 1
