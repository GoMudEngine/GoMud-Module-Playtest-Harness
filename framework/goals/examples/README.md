# Worked examples

Three end-to-end examples — one per standard personality — showing the whole
loop: **the scenario** (why you'd reach for this tester), **the goals file** you
hand the agent, and **the report** you should expect back.

Use them as templates: copy a `.yaml`, point your run at it (see the
[Quick Start](../../../README.md#quick-start-operator)), and compare what your
agent produces against the matching `*.expected-report.md`.

| Personality | Scenario | Goals file | Expected report |
|-------------|----------|------------|-----------------|
| **bug-finder** | Catch malformed player-facing text (format-string artifacts, broken templates) that a human skims past. | [`bug-finder-map-rendering.yaml`](bug-finder-map-rendering.yaml) | [report](bug-finder-map-rendering.expected-report.md) |
| **feature-tester** | Validate a specific system end to end — here, the shop buy/sell economy — including bulk and refusal paths. | [`feature-tester-shop-economy.yaml`](feature-tester-shop-economy.yaml) | [report](feature-tester-shop-economy.expected-report.md) |
| **feel-tester** | Judge the new-player experience: is onboarding clear, immersive, and free of dead-ends? | [`feel-tester-new-player-onboarding.yaml`](feel-tester-new-player-onboarding.yaml) | [report](feel-tester-new-player-onboarding.expected-report.md) |

## How real is each report?

- **bug-finder** and **feel-tester** reports are **real captured findings** from
  driving the harness against **stock GoMud** (see
  [`docs/e2e/2026-06-06-three-profiles-sanity.md`](../../../docs/e2e/2026-06-06-three-profiles-sanity.md)).
  The bug-finder map-rendering bug became upstream fix
  **GoMudEngine/GoMud PR #602.**
- The **feature-tester** report is **illustrative** — the buy/sell scenario
  shape is generalized from a DOGMud goal, and a freshly provisioned account is a
  pre-tutorial "ghost" with no gold/items, so a real run needs a character
  advanced past creation first (see [`docs/followups.md`](../../../docs/followups.md)).
  It's clearly marked at the top of that report.

## What each demonstrates

- **bug-finder** — reading every line *as text* (not as a game) surfaces
  format-string artifacts on surfaces players never normally see (a map sign, a
  panel footer). High-signal, low-false-positive.
- **feature-tester** — the portable shape: exercise a feature's happy path, its
  bulk form, and its refusal paths, and assert state (gold/inventory) via
  **GMCP** rather than scraping text. This is how you catch regressions in a
  system you just changed.
- **feel-tester** — qualitative, graded on named axes, written from a newcomer's
  seat. Catches the friction functional tests miss — a hint that dead-ends,
  curiosity that goes unrewarded.

These three are the standard lenses; mix and match goals across them for your own
runs. See [`framework/personalities/`](../../personalities/) for the role prompts
and [`framework/goals/SCHEMA.md`](../SCHEMA.md) for the goals file format.

> The two DOGMud-sourced examples were **generalized to stock GoMud** on purpose:
> this harness is engine-agnostic, so the examples stay portable rather than
> baking one server's content in. To adapt any of them to your world, change the
> command names and locations in your `engine-profile.yaml`, not the goal prose.
