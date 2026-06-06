package playtest

import (
	"embed"

	"github.com/GoMudEngine/GoMud/internal/plugins"
)

//go:embed files/*
var files embed.FS

// PlaytestModule wires AI-playtest policy on top of the engine's AI-port
// primitives: account provisioning, structural safe-mode, and admin commands.
type PlaytestModule struct {
	plug *plugins.Plugin
	cfg  Config
}

var module PlaytestModule

func init() {
	module = PlaytestModule{
		plug: plugins.New(`playtest`, `0.1`),
	}
	if err := module.plug.AttachFileSystem(files); err != nil {
		panic(err)
	}

	module.plug.Callbacks.SetOnLoad(module.onLoad)
}

// onLoad runs after the world and users are loaded.
func (m *PlaytestModule) onLoad() {
	m.cfg = loadConfig(m.plug)
	// provisioning + listeners + commands are wired in later tasks
}
