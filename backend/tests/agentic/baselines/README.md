# Agentic Test Baselines

This directory holds the known-good reports for each persona. The
orchestrator compares each fresh run against the baseline to surface
regressions automatically.

## Layout

Each persona gets one baseline file named after its ID:

```
baselines/
  R-02.json
  R-03.json
  ...
  N-16.json
```

Every file is a single AgentReport object that validates against
`tests/agentic/report-schema.json`.

## Update workflow

1. Run the full agentic test suite.
2. Review the generated reports.
3. For any persona whose report represents the new desired baseline
   (e.g. a fix shipped and the behaviour is now correct), copy that
   report into the matching baseline file.
4. Commit the baseline update alongside the fix PR.

## Why this exists

Before Run 6 we synthesised every run from scratch and manually built
the ↑/↓/= table. The baseline diff lets the orchestrator compute that
table automatically and flag any persona whose score regressed from
baseline, so reviewers can focus on deltas instead of raw numbers.

## `baseline-compare` command

```bash
go run ./cmd/testctl baseline-compare \
  --baselines tests/agentic/baselines \
  --run /path/to/run-6/
```

Accepts either a single run JSON (containing `reports: [...]`) or a
directory of per-persona JSON files. Output is a markdown table with
one row per persona showing baseline → current score, regressions in
bold, new personas marked `NEW`, and missing personas marked `MISSING`.

## Initial seeding

The baselines directory ships empty. Seed it from the first clean run
of each persona — typically the Run 6 reports after PR #198 landed.
