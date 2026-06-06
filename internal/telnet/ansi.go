package telnet

import "regexp"

// ansiRegexp matches CSI escape sequences (color/style and common cursor ops).
var ansiRegexp = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

// StripAnsi removes ANSI escape sequences, returning clean text. The adapter
// always strips, so it is correct whether or not the server also stripped
// (stock GoMud does not strip; our AI-port engine PR / DOGMud do — double-strip
// is a no-op).
func StripAnsi(p []byte) []byte {
	return ansiRegexp.ReplaceAll(p, nil)
}
