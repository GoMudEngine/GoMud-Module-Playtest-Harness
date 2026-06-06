# PR: stop the web client from leaking the password on screen (GHSA-m8fw-4ccp-94jw)

> Canonical PR description for the web-client password-echo security fix
> (branch `fix/webclient-password-echo`, off `GoMudEngine/GoMud:master`).
> Addresses advisory **GHSA-m8fw-4ccp-94jw** (reported via private vulnerability
> reporting). Ported from the validated DOGMud fix.

**Title:** `fix(login): stop web client leaking the password on screen (GHSA-m8fw-4ccp-94jw)`

# Description

On the pure web client (`/webclient-pure`), a user's password was visible on
**their own screen** while logging in — a shoulder-surf / screen-share
disclosure (someone watching the screen, a screen share, or a recording sees
the password). Per advisory **GHSA-m8fw-4ccp-94jw**.

Two distinct causes, both in `internal/inputhandlers/login_prompt_handler.go`:

### 1. The submitted password was echoed into the main scrollback (cleartext)

Websocket clients get no per-character echo while typing (the client only sends
on Enter), so on Enter the handler echoes the submitted buffer once into the
main output window for visual feedback. That echo was **not gated on
`MaskInput`**, so passwords landed in the scrollback in cleartext — even though
the input field itself was obscured. (The `TEXTMASK` protocol only switches the
input field's type; it does nothing to the output stream.)

**Fix:** for masked steps, render the mask template once per buffer byte instead
of echoing the raw buffer. Non-masked steps echo as before.

### 2. A race left the input field plaintext until the prompt was answered

The prompt was sent synchronously, while the `TEXTMASK:true` command (which
switches the web input field to `type="password"`) was queued **asynchronously**
via `events.AddToQueue`. So the prompt could arrive first: the user saw
`Password:` and began typing into a still-plaintext field, with their keystrokes
mirrored on screen, until the event loop drained and `TEXTMASK` arrived.

**Fix:** send `TEXTMASK` **synchronously, before** the prompt, so the field is
in password mode by the time the prompt is visible. (The `events` import is no
longer needed in this file.)

## Scope / safety

- **Telnet clients are unaffected.** They use per-character echo with IAC `ECHO`
  suppression during typing and ignore `TEXTMASK`, so they never hit either
  branch.
- Both changes are confined to the websocket path in the login prompt handler;
  no protocol or API changes.

## Changes

- `internal/inputhandlers/login_prompt_handler.go` — mask the websocket
  Enter-echo for `MaskInput` steps; send `TEXTMASK` synchronously before the
  prompt; drop the now-unused `events` import.

## Testing

- `go build ./...`, `go vet ./internal/inputhandlers/`, and
  `go test ./internal/inputhandlers/` all pass; `gofmt` clean.
- Behavior verified live on **DOGMud** (the downstream fork where this was first
  fixed): on the web client the password no longer appears in the input field or
  the scrollback during login.

## Notes

A regression test would ideally assert that a masked step emits only mask bytes
on the websocket path, but the package currently has no harness for mocking a
websocket connection / capturing `connections.SendTo`. Happy to add one if a
maintainer can point me at the preferred way to fake a websocket connection in
tests. Reported and fixed by the same author (advisory reporter).
