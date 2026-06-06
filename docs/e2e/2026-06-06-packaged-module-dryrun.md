# Packaged-Module Dry-Run (Phase-3 Task 7a)

**Date:** 2026-06-06
**Goal:** Prove the **packaged** `playtest.tar.gz` artifact — not the dev copy —
is complete and self-contained: it builds, provisions, and emits beacons in a
**clean** GoMud checkout that has the engine AI-port primitives.
**Result:** ✅ PASS.

## Method (simulates `module install` post download+verify)

1. `git worktree add` a fresh checkout off `feature/ai-port` (engine primitives
   present; **no** `modules/playtest/`, but the default `gmcp` module is) — a
   clean slate, isolated from the dev `~/GoMud`.
2. `tar -xzf playtest.tar.gz` into the worktree's `modules/` — i.e. install the
   **packaged** archive (sha256 `733fbe8c…782f4`), not the working copy.
3. Set the operator password in the extracted overlay
   (`files/data-overlays/config.yaml` → `AccountPassword: "testpass123"`) —
   stands in for the admin-UI config step a real operator performs.
4. Enable the AI port (`Network.AI.Port: 55555`).
5. `go run cmd/generate/module-imports.go` → `all-modules.go` now imports both
   `modules/gmcp` and `modules/playtest`.
6. `go build` → **`PACKAGED_BUILD_OK`** (the packaged sources compile with no
   `go.mod`, no dev-only files).
7. Boot, then drive `mudagent`.

## Evidence

| Check | Result |
|-------|--------|
| Packaged sources compile in a clean checkout | `PACKAGED_BUILD_OK` |
| Generation wires the module in | `all-modules.go` imports `modules/playtest` (+ `gmcp`) |
| AI listener opens | `0.0.0.0:55555 LISTENING` |
| **Packaged module provisions the account** | boot log: `playtest msg="provisioned AI test account" name="aitester"` (created fresh — clean checkout had no prior account) |
| Login round-trips | `connected → logged_in → disconnected` |
| Command round-trips | `status` → full Attributes panel |
| **Packaged module emits beacons** | **4** `Playtest.Round` beacons; payload `{"round":…, "hp":0, "hp_max":6, "sp":0, "sp_max":5, "room_id":1}` |
| No module-related panics | only unrelated default warnings (first-run `.roundcount`; SSH host-key not configured) |

Raw capture: `/tmp/proof7a.jsonl` (ephemeral). Worktree removed and pruned
after the run; `~/GoMud` untouched.

## Conclusion

The shipped archive is complete and self-contained — it installs, builds,
provisions, and beacons end-to-end on a clean GoMud with the engine primitives.
This is everything Task 7a can validate **pre-publish**. The only remaining gap
is the live registry fetch (Task 7b: `module install playtest` against the real
registry), which is gated on the engine PR merging + the `v0.1.0` release.
