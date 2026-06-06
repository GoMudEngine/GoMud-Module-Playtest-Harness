# Playtest Report: Shop economy — buy/sell path  *(illustrative)*

> **Illustrative example — not a captured run.** The scenario *shape* is
> generalized from a DOGMud feature-tester goal (NPC market participation,
> chunk 5.4); the prose below shows what a **passing** report looks like so you
> can calibrate your own. Note: a freshly provisioned test account starts as a
> pre-tutorial "ghost" with no gold or items on stock GoMud, so a real run of
> this goal needs an account advanced through character creation first (see
> `docs/followups.md`). The **bug-finder** and **feel-tester** examples in this
> folder ARE real captured findings.

**Date:** 2026-06-06
**Target:** GoMud (stock) — Frostfang
**Personality:** feature-tester
**Account:** aitester (advanced past tutorial; has starting gold)
**Goals file:** feature-tester-shop-economy.yaml
**Duration:** ~8 minutes, 14 commands

## Summary
Exercised the merchant buy/sell path against a stock-GoMud shop. Buying, single
and bulk selling, refusal handling, and gold/inventory consistency all behaved
correctly. One minor wording observation on the bulk-sell message is noted.

## Goal Results
- [x] find-merchant — PASS: `list` at the shop renders wares with prices.
- [x] buy — PASS: "You buy a <item>"; gold decreased by the listed price; item appeared in inventory (confirmed via a Char.Inventory gmcp event).
- [x] sell-single — PASS: "You sell a <item> for N gold."; gold increased by N; item removed.
- [x] sell-bulk — PASS: "sell all <item>" sold the correct count; gold and inventory matched the total.
- [x] sell-refused — PASS: attempting to sell in an empty room returned "There's no merchant here."
- [x] stability — PASS: no crash/disconnect; gold and inventory stayed consistent across the session.

## Findings

### PASS: Buy/sell path is solid
Gold deltas matched listed prices exactly in both directions; inventory updated
atomically; refusal messages were specific rather than generic errors.

### OBSERVATION: Bulk-sell phrasing
"sell all" reports a combined total only; echoing the per-unit price too would
let a player sanity-check the math. Minor, not a defect.

## Stats
- Commands sent: 14
- Errors seen: 0
- Bugs / Concerns / Observations: 0 / 0 / 1

---

**Why this example matters.** Generalized from a DOGMud feature-tester goal that
validated a fork-specific economy refactor. The portable *shape* is the lesson:
exercise a feature's happy path, its bulk form, and its refusal paths, and assert
state (gold, inventory) via **GMCP** rather than scraping text. Feature-tester
runs are how you catch regressions in a system you just changed.
