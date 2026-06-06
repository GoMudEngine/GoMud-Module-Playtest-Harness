// This directory holds the playtest module's source for distribution. It only
// compiles inside a GoMud checkout (it imports GoMud-internal packages). This
// go.mod exists solely so the harness repo's `go ./...` skips this directory.
// Do NOT copy this go.mod into a GoMud `modules/` directory.
module github.com/GoMudEngine/GoMud/modules/playtest

go 1.25
