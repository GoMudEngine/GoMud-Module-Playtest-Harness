# Beacon Regression: Provisioned "Phantom" Active User (root cause + fix)

**Date:** 2026-06-06
**Severity:** blocker for publish — the headline `Playtest.Round` beacon feature
did not reach the agent.
**Status:** ✅ FIXED and verified end-to-end on real upstream master.

## Symptom

Driving `mudagent` against the packaged module on merged GoMud master,
**0 `Playtest.Round` beacons** reached the agent (verified server-side: the raw
stream contained no `Playtest` at all). Intermittently the engine also logged
`Warning: this account is not flagged as AI but connected on the AI port` at
login, and the agent sometimes stalled at `connected` (login rejected).

## Root cause

`ensureTestAccount` (new-account path) used `users.CreateUser`, which is the
**online** account-creation call. Besides writing the record to disk + index,
`CreateUser` registers the account as a **live active user** in the engine's
`userManager` under its connection id — which is `0` for a provisioned record
(`NewUserRecord(0, 0)`). That left a **phantom logged-in `aitester`** from boot:

- `onNewRound` iterates `users.GetAllActiveUsers()` and beacons each `IsAI`
  user. The phantom matched, so every beacon was sent to **connection 0 (a dead
  connection)** — the real agent got nothing. (`beacon-dbg` showed `aitester`
  active from the first round, before any client connected.)
- When the real AI client logged in, the engine saw the account already present
  in `userManager` → "already logged in" / the stale-user **reconnect-swap**
  path (`LoginUser`), which replaced the freshly disk-loaded record (with
  `IsAI=true`) and dropped the flag → the "not flagged as AI" warning.

It was **intermittent** because only the boot that *creates* the account hits
`CreateUser`; later boots hit `flagExisting` (`LoadUser`+`SaveUser`), which does
**not** register a phantom — so beacons appeared to work on "second run" test
beds and broke on fresh installs (the real `module install` scenario).

## Fix

After `CreateUser`, evict the phantom from the active manager:

```go
if err := users.LogOutUserByConnectionId(u.ConnectionId()); err != nil {
    mudlog.Warn("playtest", "msg", "evict provisioned phantom user", "error", err)
}
```

The account remains on disk + index (findable, `IsAI`-flagged); it is simply no
longer a live session. The real client then logs in via the **normal** path
(verified: `loaded-from-disk isAI=true` → `after-LoginUser isAI=true,
swapped=false`), and beacons flow to the live connection.

`module/playtest/provision.go` (and the `~/GoMud` dev copy). Repackaged archive
sha256: `c65ccf45663cb093c71d0573b007dc0330b647b355b41967a4da23652b0e09e4`.

## Verification (clean fresh-checkout, packaged artifact, real master)

Fresh worktree off `GoMudEngine/GoMud:master` (`dd27b4f1`), pristine users dir,
the **packaged** `playtest.tar.gz` extracted (no dev sources, no debug),
single connection:

- `provisioned: 1`, login `connected → logged_in → disconnected`
- **5 `Playtest.Round` beacons** received
- no "not flagged as AI" warning; `beacon-dbg`: `aitester active, isAI=true`

## Engine follow-up (optional, for a later PR to GoMud)

Two engine rough edges surfaced (not required for this module fix, but worth a
note to the maintainer):
1. `users.CreateUser` couples disk/index creation with *online* session
   registration — an offline "create account" helper would make programmatic
   provisioning cleaner (no evict dance).
2. `UserIndex.AddUser` opens the index file `O_RDWR` with no create, so it fails
   silently if the index file doesn't exist yet (only surfaced via our own test
   when the file was deleted; a fresh checkout's index is created at boot).
