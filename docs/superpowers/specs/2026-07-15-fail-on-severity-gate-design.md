# --fail-on Severity/Drift Exit-Code Gate — Design

## Problem

tf-lens's threat and drift overlays already detect and print security
findings and infrastructure drift, but nothing acts on that information —
the CLI always exits 0. CI pipelines that want to block a deploy on
critical findings, or on any detected drift, have no way to do that today
short of parsing stdout text.

## Scope

New `--fail-on=<severity>` flag on both `tf-lens export` and `tf-lens
serve`, where `<severity>` is one of `critical`, `high`, `medium`, `info`.
New shared file `cmd/failon.go` holds validation and gating logic (first
shared helper file in `cmd/`, and the first test file in that package).

No new Go dependencies. No changes to `internal/threat` or `internal/drift`
beyond reusing what already exists (`threat.Severity.Weight()`,
`drift.DriftedResource`).

Out of scope: a separate/custom exit code for gate failures (reuses the
existing generic error → exit 1 path in `cmd/root.go`); gating on cost or
diff overlays (not requested); a `--fail-on=none` or list-of-severities
syntax (single threshold value is sufficient).

## Validation

Runs immediately after flag parsing, in both `runExport` and `runServe`:

- `--fail-on` set but `--threat` not set → error: `--fail-on requires --threat`.
- `--fail-on` set to anything other than `critical`, `high`, `medium`, `info`
  → error listing the valid values.

Both are plain `error` returns. `cmd/root.go`'s existing `Execute()` already
prints any returned error to stderr and calls `os.Exit(1)` — no new exit
code or error-handling path needed.

## Threat gate

After `threat.Analyse()` produces findings (in the existing `--threat`
block in `export.go` / `buildServeGraph` in `serve.go`), if `--fail-on` is
set: compare each finding's `Severity.Weight()` against
`threat.Severity(failOn).Weight()`. `Weight()` already exists
(critical=4, high=3, medium=2, info=1 — internal/threat/finding.go:42-54).
If any finding's weight is **≥** the threshold's weight, return an error
naming the offending finding (its code, resource address, and severity).
`--fail-on=high` therefore also fails on `critical` findings — threshold
means "this or worse," not "exactly this."

## Drift gate

Bundled into the same `--fail-on` flag: if `--drift` is also set and the
drift check finds **any** drifted resources at all, that fails the gate
too — regardless of which severity value `--fail-on` was set to. Drift has
no severity scale in this codebase (it's presence/absence, not
critical/high/medium/info), so this is unconditional: `--fail-on` +
`--drift` + any drift found = failure. If `--drift` isn't passed, there is
nothing to gate on and this check doesn't run.

## Serve + `--watch` interaction

The gate must apply "at startup" but not crash a running `--watch` server
on every rebuild. This falls out of existing code structure without new
branching:

- `runServe` calls `buildServeGraph()` once at startup. If the gate check
  (placed inside `buildServeGraph`) returns an error, it propagates up
  through `runServe` and `cmd/root.go` exits 1 before the HTTP server ever
  starts.
- `--watch` mode's `watchFiles()` loop also calls `buildServeGraph()` on
  every detected file change, but it already wraps that call:
  ```go
  g, err := buildServeGraph()
  if err != nil {
      log.Printf("⚠  Rebuild failed: %v (keeping previous graph)\n", err)
      continue
  }
  ```
  So a gate failure on a later rebuild degrades to a log line and the
  server keeps serving the last-known-good graph — it does not crash the
  running process.

This is a deliberate reuse of `watchFiles`'s existing error handling, not
new gate-specific logic. The one consequence: a user running `--watch`
gets no exit code at all for gate failures introduced after startup — only
the log line. That matches "gate on startup" as specified; if a future
need arises for the running server to signal a post-startup gate breach
more loudly (e.g. a banner in the UI), that's a separate, unrequested
feature.

## Testing

`cmd/failon_test.go` — table-driven tests covering:
- Validation: `--fail-on` without `--threat` → error; invalid severity
  string → error; valid combinations → no error.
- Threat gate: finding below threshold → no error; finding at or above
  threshold → error; empty findings → no error.
- Drift gate: no drift → no error; any drift present → error, independent
  of the `--fail-on` value.

Same style as the codebase's existing test files — plain `testing.T`
table tests, no new test framework.

## Non-goals (explicitly deferred)

- Custom exit codes distinguishing "validation error" from "gate
  triggered" — both use the existing generic exit-1 path.
- Gating on cost or diff overlays.
- Multi-severity or "none" syntax for `--fail-on`.
- Any change to `--watch`'s behavior beyond what already exists (no new
  UI signal for post-startup gate breaches).
