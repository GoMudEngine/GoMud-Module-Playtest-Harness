# Report Format

The agent writes one Markdown report per session, in this shape. Keep the
finding categories (`BUG`/`CONCERN`/`OBSERVATION`/`PASS`/`FAIL`/`BLOCKED`)
consistent so reports are skimmable and comparable across runs.

## Template

```markdown
# Playtest Report: <session name or "Exploratory">

**Date:** <YYYY-MM-DD>
**Target:** <target name>
**Personality:** <bug-finder | feature-tester | feel-tester>
**Account:** <username>
**Goals file:** <filename or "none">
**Duration:** <~minutes>, <N> commands

## Summary
<2-4 sentence narrative of the session arc.>

## Goal Results
(Omit this section for exploratory runs with no goals file.)
- [x] <goal id>: <do> — PASS: <evidence>
- [ ] <goal id>: <do> — FAIL: <observed vs. expected>
- [ ] <goal id>: <do> — BLOCKED: <why>

## Findings
### BUG: <title>
<what you did, what happened, what should have happened (with repro steps)>

### CONCERN: <title>
<what seemed wrong and why>

### OBSERVATION: <title>
<notable behavior worth recording>

### PASS: <title>
<a feature/experience confirmed working>

## Stats
- Commands sent: <N>
- Errors seen: <N>
- Bugs / Concerns / Observations: <N> / <N> / <N>
```

## Conventions

- One report per session; name it
  `<date>-<target>-<personality>[-<goals>].md`.
- Every BUG should be reproducible from the report alone: where, what you typed,
  what happened, what you expected.
- For goal runs, every goal in the goals file gets a line in Goal Results.
- Keep findings concrete; avoid vague "feels off" without saying why.
