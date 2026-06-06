# Phase 3 — Publish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax. Several steps are **outward-facing** (GitHub releases, PRs) — confirm with the human before executing those.

**Goal:** Publish the harness so any GoMud operator can `module install playtest`, build, and drive it with `mudagent` — i.e. land the registry entry, the release artifacts, and a validated consumer path.

**Architecture:** Three shipping channels (recap): the **engine PR** → `GoMudEngine/GoMud` (Track 1, separate), the **`playtest` module** → the GoMud module registry, and the **`mudagent` binary + framework content** → releases of `pruuk/gomud-playtest-harness`. Phase 3 wires up the latter two and validates the whole install→use path.

## ⚠️ Hard prerequisite — the engine PR must be merged upstream first

The `playtest` module imports the engine's `IsAI` field and targets the AI port, both added by the **Track-1 engine PR**. Until that PR is merged into `GoMudEngine/GoMud` (or in a GoMud release):

- The module **will not compile** on stock GoMud (`IsAI undefined`).
- A registry entry would point users at a module that breaks their build.
- The consumer-path validation (the acceptance test) needs a GoMud that has the primitives.

So Phase 3 splits into **"prep now" (safe before the PR lands)** and **"publish after"** (blocked on the merge). Do the prep tasks now; gate the publish tasks on the engine PR landing. Do not open the registry PR until a consumer can actually build the module against a public GoMud.

**Test target:** a clean GoMud checkout that includes the engine primitives — post-merge that's a fresh `GoMudEngine/GoMud`; pre-merge, simulate with the `feature/ai-port` branch.

---

## Part 1 — Prep (safe to do now)

### Task 1: Finalize the module for release

**Files:** `module/playtest/` (developed in `~/GoMud/modules/playtest/`)

- [ ] **Step 1: Version bump.** In `playtest.go`, change `plugins.New("playtest", "0.1")` to `plugins.New("playtest", "0.1.0")` (semver; stay pre-1.0 until the engine API settles post-merge). Sync + `go build ./...` in `~/GoMud`.
- [ ] **Step 2: Confirm no `go.mod` in the packaged dir.** `module package` archives *everything* under `modules/playtest/`. The nested `go.mod` exists ONLY in the harness repo (so `go ./...` skips it there); it must NOT be present in `~/GoMud/modules/playtest/`. Verify: `ls ~/GoMud/modules/playtest/go.mod` should not exist.
- [ ] **Step 3: Decide on test files.** `_test.go` files import `testify` and will be included in the archive. GoMud bundles `testify`, so a consumer's `go build` is unaffected (test files only compile under `go test`). **Decision:** keep them (auditable, and GoMud has the dep) — but note it in the release notes. (If a leaner archive is wanted, package from a copy with `*_test.go` removed.)
- [ ] **Step 4: Final README pass.** Ensure `module/playtest/README.md` states the **`gmcp` module dependency** (for beacons) and the **engine-PR/GoMud-version requirement** prominently, since the registry has no dependency field.

### Task 2: Package dry-run

- [ ] **Step 1:** From `~/GoMud`: `go run . module package playtest`. Capture the printed **Archive** (`playtest.tar.gz`) and **SHA256**.
- [ ] **Step 2: Inspect the archive.** `tar -tzf playtest.tar.gz` — confirm: top-level dir is `playtest/`; contains `playtest.go`, the other `.go` files, `README.md`, `files/data-overlays/config.yaml`, `files/...`; and **no `go.mod`**. Note whether `_test.go` are present (expected).
- [ ] **Step 3:** Record the sha256 (it must match the eventual release asset exactly — re-run package against the *final* committed source, since any byte change changes the hash).

### Task 3: Cross-compile `mudagent` release binaries

- [ ] From the harness repo, build for each target (one binary per OS/arch):

```sh
for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os=${t%/*}; arch=${t#*/}; ext=""; [ "$os" = windows ] && ext=".exe"
  GOOS=$os GOARCH=$arch go build -o dist/mudagent-$os-$arch$ext ./cmd/mudagent
done
```
(`dist/` is git-ignored; these become release assets.) Confirm each builds.

### Task 4: Draft the registry entry

- [ ] Draft the `module-registry.yaml` entry (do not submit yet) and save it to `docs/pr/registry-entry-playtest.yaml`:

```yaml
- name: playtest
  description: >-
    AI playtest harness — provisions a flagged AI test account, structural
    safe-mode, and per-round Playtest.Round GMCP beacons for structured goal
    verification. Requires a GoMud with the AI-port primitives (IsAI flag) and
    the bundled gmcp module. Pairs with the mudagent adapter + framework content
    at github.com/pruuk/gomud-playtest-harness.
  version: 0.1.0
  author: pruuk
  url: https://github.com/pruuk/gomud-playtest-harness/releases/download/v0.1.0/playtest.tar.gz
  sha256: <from Task 2 Step 1, against final source>
```
(The `description` carries the `gmcp` + GoMud-version prerequisites because the schema has no dependency field.)

---

## Part 2 — Publish (GATED on the engine PR being merged upstream)

> Do not start Part 2 until the Track-1 engine PR is merged into
> `GoMudEngine/GoMud` (or available in a GoMud release). Each step here is
> outward-facing — confirm with the human first.

### Task 5: Cut a GitHub release of `pruuk/gomud-playtest-harness`

- [ ] **Step 1:** Re-run `module package playtest` against the **final committed** module source and record the definitive sha256.
- [ ] **Step 2:** Tag and create release `v0.1.0` on `pruuk/gomud-playtest-harness` (GitHub UI, since `gh` isn't installed). Upload assets:
  - `playtest.tar.gz` (the module archive) — its URL becomes the registry `url`.
  - the cross-compiled `mudagent-*` binaries (Task 3).
  - (optional) `framework.tar.gz` of `framework/` for convenience (it also lives in the repo).
- [ ] **Step 3:** Confirm the asset download URL matches the registry entry's `url`, and the asset's sha256 matches Task 5 Step 1.

### Task 6: Open the module-registry PR

- [ ] **Step 1:** Fork `GoMudEngine/GoMud-Modules` → `pruuk/GoMud-Modules` (a different repo network than GoMud, so the one-fork-per-network limit does not apply).
- [ ] **Step 2:** On a branch, add the Task-4 entry (with the final `url` + `sha256`) to `module-registry.yaml`. Keep the diff to just that entry.
- [ ] **Step 3:** Open the PR `pruuk/GoMud-Modules → GoMudEngine/GoMud-Modules`. In the PR body, link the **E2E smoke docs** (`docs/e2e/2026-06-05-mudagent-smoke.md` and `…-beacons-smoke.md`) as evidence, state the `gmcp` + GoMud-version prerequisites, and note the companion `mudagent`/framework release.

### Task 7: Consumer-path validation — the acceptance test

> Run against a **clean** GoMud checkout that includes the engine primitives. A
> pre-merge dry-run (Task 7a) can validate everything except the live registry
> fetch; Task 7b is the real post-publish validation.

- [ ] **Task 7a (pre-publish dry-run):** In a fresh GoMud checkout on a branch with the engine primitives, manually extract `playtest.tar.gz` into `modules/` (simulating what `module install` does after download+verify), then `go generate && go build`. Enable the AI port + set the module config (admin UI), boot, and drive `mudagent` — confirm login + a `Playtest.Round` beacon. This proves the *packaged* module (not the dev copy) is complete and builds cleanly.
- [ ] **Task 7b (post-publish, after Tasks 5 & 6 merge):** On a clean checkout: `go run . module install playtest` → confirm it fetches the registry, downloads the asset, **verifies the sha256**, extracts to `modules/playtest/`, and records `modules/modules.lock.yaml`. Then `go generate && go build -o go-mud-server` → run → drive `mudagent` → confirm a beacon flows. This validates the *entire* published path end to end.

### Task 8: Docs + release notes

- [ ] **Step 1:** Update the repo `README.md` Quick Start to the real published install (`go run . module install playtest`, the `v0.1.0` adapter download links, the GoMud-version requirement). Replace any "until the PR is merged" hedging once it is merged.
- [ ] **Step 2:** Write `v0.1.0` release notes: what's included (module + adapter + framework), prerequisites (GoMud with the AI port, `gmcp` module), the two E2E smokes, and the known follow-ups from `docs/followups.md`.
- [ ] **Step 3:** Reconcile `docs/design/…` Phase-3 notes and `docs/usage/playtest-module.md` install section with the shipped reality.

---

## Self-Review checklist

- The registry entry's `sha256` is computed against the **exact** bytes of the released `playtest.tar.gz` (re-package after any source change — the hash is byte-exact).
- The packaged archive contains **no `go.mod`** (harness-only artifact) and the correct `playtest/`-rooted layout.
- The `gmcp` + GoMud-version prerequisites are stated in the registry `description` AND the module README (no dependency field exists to enforce them).
- Part 2 was not started before the engine PR merged; the consumer-path validation (Task 7b) actually ran `module install` on a clean checkout and a beacon flowed.
- Outward-facing steps (release, registry PR) were confirmed with the human before execution.

## Open questions

- **Adapter distribution.** v0.1.0 ships `mudagent` as release binaries. Consider `go install github.com/pruuk/gomud-playtest-harness/cmd/mudagent@v0.1.0` as an alternative once the repo is public and tagged (no separate binary hosting needed for Go users).
- **Repo transfer.** Per the design's deferred question, decide post-publish whether to offer the repo to the `GoMudEngine` org once it has proven out.
