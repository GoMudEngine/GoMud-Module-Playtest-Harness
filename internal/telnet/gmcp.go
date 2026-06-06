package telnet

// DoGMCP is the client's acceptance of the server's WILL GMCP.
func DoGMCP() []byte { return []byte{IAC, DO, GMCP} }

// FrameGMCP builds a GMCP sub-negotiation: IAC SB GMCP "<pkg> <json>" IAC SE.
func FrameGMCP(pkg, json string) []byte {
	out := []byte{IAC, SB, GMCP}
	out = append(out, []byte(pkg)...)
	out = append(out, ' ')
	out = append(out, []byte(json)...)
	out = append(out, IAC, SE)
	return out
}

// SupportedPackages is the default Core.Supports.Set list the adapter enables.
var SupportedPackages = []string{
	"Char 1", "Char.Info 1", "Char.Vitals 1", "Char.Inventory 1",
	"Char.Stats 1", "Char.Affects 1", "Room 1", "Room.Info 1",
}
