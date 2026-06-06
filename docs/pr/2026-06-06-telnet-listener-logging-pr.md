# PR: log telnet/AI listeners at boot

> Engine PR (branch `feat/log-telnet-listeners`, off `GoMudEngine/GoMud:master`).
> Small operator-UX fix noticed by Volte6.

**Title:** `feat(net): log telnet/AI listeners at boot`

# Description

The server logs its **SSH** and **HTTP** listeners at boot, but not **telnet** —
so an operator who enables a telnet port (notably the opt-in **AI port**) gets no
confirmation at startup that it actually came up. The only way to tell today is
`netstat` or a connection attempt.

`TelnetListenOnPort` now logs each **successfully bound** listener, matching the
existing SSH log style:

```
INFO  Telnet  status=listening  port=33333  type=human
INFO  Telnet  status=listening  port=55555  type=AI
```

Human vs AI is distinguished by the connection type the listener was created
with. The line is emitted only after a successful `net.Listen`, so a failed bind
still surfaces the existing `Error creating server` log and nothing misleading.

## Changes

- `main.go` — `TelnetListenOnPort` logs the listener (`mudlog.Info("Telnet",
  "status", "listening", "port", …, "type", human|AI)`) on successful bind.

## Testing

- `go build ./...` passes; `gofmt` clean.
- Booted with the AI port enabled and the default telnet ports: confirmed the new
  lines appear for `33333`/`44444`/`9999` (human) and `55555` (AI), and that a
  port already in use still logs the existing bind error (no false "listening").

Reported by Volte6 while reviewing the playtest module.
