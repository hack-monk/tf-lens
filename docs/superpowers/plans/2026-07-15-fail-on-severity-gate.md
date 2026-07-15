# --fail-on Severity/Drift Exit-Code Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `--fail-on=<severity>` flag to `tf-lens export` and `tf-lens serve` that exits 1 if threat findings at/above the given severity are found, or if any drift is detected (when `--drift` is also set).

**Architecture:** New shared file `cmd/failon.go` holds pure validation + gating functions with no CLI parsing concerns. `cmd/export.go` and `cmd/serve.go` each add one flag, one validation call after parsing, and two gate calls inside their existing `--threat` and `--drift` overlay blocks. `cmd/root.go`'s existing generic `error → print → os.Exit(1)` path handles all failures — no new exit-code scheme.

**Tech Stack:** Go stdlib `flag`, existing `internal/threat` (`Severity.Weight()`) and `internal/drift` (`DriftedResource`) packages. No new dependencies.

## Global Constraints

- `--fail-on` accepts exactly one of: `critical`, `high`, `medium`, `info`. Any other value is a validation error.
- `--fail-on` set without `--threat` is a validation error: `--fail-on requires --threat`.
- Threshold semantics are **≥** (at-or-above): `--fail-on=high` also fails on `critical` findings.
- If `--drift` is also set, **any** drift detected fails the gate, regardless of the `--fail-on` value (drift has no severity scale).
- No new exit codes — all gate/validation failures are plain `error` returns that flow through `cmd/root.go`'s existing `os.Exit(1)` path.
- `--fail-on` applies "at startup" for `serve --watch`: a gate failure on a `--watch`-triggered rebuild must NOT crash the running server — it should behave like any other rebuild error (log + keep serving the old graph), which happens automatically because `watchFiles()` already wraps its `buildServeGraph()` call in that exact error-handling pattern.
- No new Go dependencies.

---

### Task 1: Shared validation and gating logic

**Files:**
- Create: `cmd/failon.go`
- Test: `cmd/failon_test.go`

**Interfaces:**
- Consumes: `threat.Finding` (fields: `Severity threat.Severity`, `Code string`, `ResourceAddress string` — from `internal/threat/finding.go`), `threat.Severity.Weight() int` (critical=4, high=3, medium=2, info=1, unknown=0 — already exists), `drift.DriftedResource` (field: `Address string` — from `internal/drift/drift.go`).
- Produces: `validateFailOn(failOn string, threatEnabled bool) error`, `checkThreatGate(failOn string, findings []threat.Finding) error`, `checkDriftGate(failOn string, drifted []drift.DriftedResource) error` — all three consumed by Task 2.

- [ ] **Step 1: Write the failing tests**

Create `cmd/failon_test.go`:

```go
package cmd

import (
	"strings"
	"testing"

	"github.com/hack-monk/tf-lens/internal/drift"
	"github.com/hack-monk/tf-lens/internal/threat"
)

func TestValidateFailOn(t *testing.T) {
	tests := []struct {
		name          string
		failOn        string
		threatEnabled bool
		wantErr       string // substring expected in error, "" means no error
	}{
		{"empty is always valid", "", false, ""},
		{"empty is always valid even with threat on", "", true, ""},
		{"valid value requires threat", "critical", false, "--fail-on requires --threat"},
		{"valid value with threat enabled", "critical", true, ""},
		{"high with threat enabled", "high", true, ""},
		{"medium with threat enabled", "medium", true, ""},
		{"info with threat enabled", "info", true, ""},
		{"invalid value", "extreme", true, "invalid --fail-on value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFailOn(tt.failOn, tt.threatEnabled)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateFailOn(%q, %v) = %v, want nil", tt.failOn, tt.threatEnabled, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validateFailOn(%q, %v) = %v, want error containing %q", tt.failOn, tt.threatEnabled, err, tt.wantErr)
			}
		})
	}
}

func TestCheckThreatGate(t *testing.T) {
	findings := []threat.Finding{
		{ResourceAddress: "aws_s3_bucket.data", Code: "S3001", Severity: threat.SeverityMedium},
		{ResourceAddress: "aws_security_group.web", Code: "SG003", Severity: threat.SeverityHigh},
	}

	tests := []struct {
		name     string
		failOn   string
		findings []threat.Finding
		wantErr  bool
	}{
		{"empty fail-on never gates", "", findings, false},
		{"threshold above all findings passes", "critical", findings, false},
		{"threshold matches highest finding fails", "high", findings, true},
		{"threshold below highest finding fails", "medium", findings, true},
		{"threshold below lowest finding fails", "info", findings, true},
		{"no findings never fails", "critical", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkThreatGate(tt.failOn, tt.findings)
			if tt.wantErr && err == nil {
				t.Errorf("checkThreatGate(%q, ...) = nil, want error", tt.failOn)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("checkThreatGate(%q, ...) = %v, want nil", tt.failOn, err)
			}
		})
	}
}

func TestCheckDriftGate(t *testing.T) {
	drifted := []drift.DriftedResource{
		{Address: "aws_instance.web", Type: "aws_instance", Action: "update"},
	}

	tests := []struct {
		name    string
		failOn  string
		drifted []drift.DriftedResource
		wantErr bool
	}{
		{"empty fail-on never gates", "", drifted, false},
		{"no drift never fails", "critical", nil, false},
		{"any drift fails regardless of severity value", "info", drifted, true},
		{"any drift fails at critical value too", "critical", drifted, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDriftGate(tt.failOn, tt.drifted)
			if tt.wantErr && err == nil {
				t.Errorf("checkDriftGate(%q, ...) = nil, want error", tt.failOn)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("checkDriftGate(%q, ...) = %v, want nil", tt.failOn, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/... -run 'TestValidateFailOn|TestCheckThreatGate|TestCheckDriftGate' -v`
Expected: FAIL — `undefined: validateFailOn` (and the other two functions), since `cmd/failon.go` doesn't exist yet.

- [ ] **Step 3: Write the implementation**

Create `cmd/failon.go`:

```go
package cmd

import (
	"fmt"

	"github.com/hack-monk/tf-lens/internal/drift"
	"github.com/hack-monk/tf-lens/internal/threat"
)

var validFailOnSeverities = map[string]bool{
	"critical": true,
	"high":     true,
	"medium":   true,
	"info":     true,
}

// validateFailOn checks that --fail-on, if set, names a valid severity and
// that --threat is enabled. --fail-on gates threat findings, so it's
// meaningless without threat modelling turned on.
func validateFailOn(failOn string, threatEnabled bool) error {
	if failOn == "" {
		return nil
	}
	if !threatEnabled {
		return fmt.Errorf("--fail-on requires --threat")
	}
	if !validFailOnSeverities[failOn] {
		return fmt.Errorf("invalid --fail-on value %q: must be one of critical, high, medium, info", failOn)
	}
	return nil
}

// checkThreatGate returns an error if any finding's severity is at or above
// the --fail-on threshold (e.g. --fail-on=high also fails on critical).
func checkThreatGate(failOn string, findings []threat.Finding) error {
	if failOn == "" {
		return nil
	}
	threshold := threat.Severity(failOn).Weight()
	for _, f := range findings {
		if f.Severity.Weight() >= threshold {
			return fmt.Errorf("fail-on gate: %s finding %s on %s (threshold: %s)",
				f.Severity, f.Code, f.ResourceAddress, failOn)
		}
	}
	return nil
}

// checkDriftGate returns an error if --fail-on is set and any drift was
// detected. Drift has no severity scale, so any drift at all fails the
// gate regardless of the --fail-on value.
func checkDriftGate(failOn string, drifted []drift.DriftedResource) error {
	if failOn == "" || len(drifted) == 0 {
		return nil
	}
	return fmt.Errorf("fail-on gate: %d resource(s) drifted from state", len(drifted))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/... -run 'TestValidateFailOn|TestCheckThreatGate|TestCheckDriftGate' -v`
Expected: PASS — all subtests green.

- [ ] **Step 5: Commit**

```bash
git add cmd/failon.go cmd/failon_test.go
git commit -m "feat: add --fail-on validation and gating logic"
```

---

### Task 2: Wire `--fail-on` into export and serve commands

**Files:**
- Modify: `cmd/export.go` (var block, flag registration, validation call, two gate calls)
- Modify: `cmd/serve.go` (var block, flag registration, validation call, two gate calls)

**Interfaces:**
- Consumes: `validateFailOn`, `checkThreatGate`, `checkDriftGate` from Task 1 (exact signatures above).
- Produces: nothing further consumed by other tasks — this is the last task.

- [ ] **Step 1: Add the flag and validation call to `cmd/export.go`**

In `cmd/export.go:20-31`, change:

```go
var (
	exportPlan        string
	exportState       string
	exportOut         string
	exportDiff        string
	exportThreat      bool
	exportCost        string
	exportDrift       string
	exportFormat      string
	exportFlow        bool
	exportAnnotations string
)
```

to:

```go
var (
	exportPlan        string
	exportState       string
	exportOut         string
	exportDiff        string
	exportThreat      bool
	exportCost        string
	exportDrift       string
	exportFormat      string
	exportFlow        bool
	exportAnnotations string
	exportFailOn      string
)
```

In `cmd/export.go:52-55`, change:

```go
	fs.StringVar(&exportAnnotations, "annotations", "",
		"Path to tf-lens.yaml annotation file with human-readable labels and tour steps")
	fs.Parse(args)

	// ── 1. Parse primary input ────────────────────────────────────────────
```

to:

```go
	fs.StringVar(&exportAnnotations, "annotations", "",
		"Path to tf-lens.yaml annotation file with human-readable labels and tour steps")
	fs.StringVar(&exportFailOn, "fail-on", "",
		"Exit 1 if threat findings at/above this severity are found, or if any drift is detected: critical, high, medium, info (requires --threat)")
	fs.Parse(args)

	if err := validateFailOn(exportFailOn, exportThreat); err != nil {
		return err
	}

	// ── 1. Parse primary input ────────────────────────────────────────────
```

- [ ] **Step 2: Add the threat gate call to `cmd/export.go`**

In `cmd/export.go:87-115`, change the end of the `--threat` block from:

```go
		if len(findings) == 0 {
			fmt.Println("    ✅ No issues found")
		}
		fmt.Println()
	}
```

to:

```go
		if len(findings) == 0 {
			fmt.Println("    ✅ No issues found")
		}
		fmt.Println()

		if err := checkThreatGate(exportFailOn, findings); err != nil {
			return err
		}
	}
```

- [ ] **Step 3: Add the drift gate call to `cmd/export.go`**

In `cmd/export.go:133-155`, change the end of the `--drift` block from:

```go
		if summary["create"] > 0 {
			fmt.Printf("    Created:  %d\n", summary["create"])
		}
		fmt.Println()
	}
```

to:

```go
		if summary["create"] > 0 {
			fmt.Printf("    Created:  %d\n", summary["create"])
		}
		fmt.Println()

		if err := checkDriftGate(exportFailOn, drifted); err != nil {
			return err
		}
	}
```

- [ ] **Step 4: Confirm export.go compiles**

Run: `go build ./...`
Expected: exits 0, no output.

- [ ] **Step 5: Add the flag and validation call to `cmd/serve.go`**

In `cmd/serve.go:23-35`, change:

```go
var (
	servePort        int
	servePlan        string
	serveState       string
	serveDiff        string
	serveNoOpen      bool
	serveThreat      bool
	serveCost        string
	serveDriftPath   string
	serveWatch       bool
	serveFlow        bool
	serveAnnotations string
)
```

to:

```go
var (
	servePort        int
	servePlan        string
	serveState       string
	serveDiff        string
	serveNoOpen      bool
	serveThreat      bool
	serveCost        string
	serveDriftPath   string
	serveWatch       bool
	serveFlow        bool
	serveAnnotations string
	serveFailOn      string
)
```

In `cmd/serve.go:55-63`, change:

```go
	fs.StringVar(&serveAnnotations, "annotations", "",
		"Path to tf-lens.yaml annotation file with human-readable labels and tour steps")
	fs.Parse(args)

	// ── 1. Build initial graph ────────────────────────────────────────────
	g, err := buildServeGraph()
	if err != nil {
		return err
	}
```

to:

```go
	fs.StringVar(&serveAnnotations, "annotations", "",
		"Path to tf-lens.yaml annotation file with human-readable labels and tour steps")
	fs.StringVar(&serveFailOn, "fail-on", "",
		"Exit 1 at startup if threat findings at/above this severity are found, or if any drift is detected: critical, high, medium, info (requires --threat)")
	fs.Parse(args)

	if err := validateFailOn(serveFailOn, serveThreat); err != nil {
		return err
	}

	// ── 1. Build initial graph ────────────────────────────────────────────
	g, err := buildServeGraph()
	if err != nil {
		return err
	}
```

- [ ] **Step 6: Add the threat and drift gate calls inside `buildServeGraph`**

In `cmd/serve.go:115-124`, change:

```go
	if serveThreat {
		findings := threat.Analyse(resources)
		threat.AnnotateGraph(g, findings)
		counts := map[string]int{}
		for _, f := range findings {
			counts[string(f.Severity)]++
		}
		fmt.Printf("🔒  Threat model: %d critical, %d high, %d medium, %d info\n",
			counts["critical"], counts["high"], counts["medium"], counts["info"])
	}
```

to:

```go
	if serveThreat {
		findings := threat.Analyse(resources)
		threat.AnnotateGraph(g, findings)
		counts := map[string]int{}
		for _, f := range findings {
			counts[string(f.Severity)]++
		}
		fmt.Printf("🔒  Threat model: %d critical, %d high, %d medium, %d info\n",
			counts["critical"], counts["high"], counts["medium"], counts["info"])

		if err := checkThreatGate(serveFailOn, findings); err != nil {
			return nil, err
		}
	}
```

In `cmd/serve.go:137-144`, change:

```go
	if serveDriftPath != "" {
		drifted, err := resolveDrift(serveDriftPath)
		if err != nil {
			return nil, fmt.Errorf("drift detection: %w", err)
		}
		drift.AnnotateGraph(g, drifted)
		fmt.Printf("🔀  Drift: %d resources drifted from state\n", len(drifted))
	}
```

to:

```go
	if serveDriftPath != "" {
		drifted, err := resolveDrift(serveDriftPath)
		if err != nil {
			return nil, fmt.Errorf("drift detection: %w", err)
		}
		drift.AnnotateGraph(g, drifted)
		fmt.Printf("🔀  Drift: %d resources drifted from state\n", len(drifted))

		if err := checkDriftGate(serveFailOn, drifted); err != nil {
			return nil, err
		}
	}
```

- [ ] **Step 7: Confirm serve.go compiles and run the full test suite**

Run: `go build ./... && go test ./... -count=1`
Expected: build exits 0; all packages report `ok`, including the new `cmd` package (7 tests from Task 1, all passing).

- [ ] **Step 8: Manual CLI verification**

These fixtures already exist and produce known results (verified during planning):
- `testdata/plan_basic.json` with `--threat` produces 0 critical, 4 high, 3 medium, 5 info findings.
- `testdata/drift.json` (used with `--drift`) produces 2 drifted resources.

Build and check the passing case (threshold above all findings):

```bash
go build -o /tmp/tf-lens-verify .
/tmp/tf-lens-verify export --plan testdata/plan_basic.json --threat --fail-on=critical --out /tmp/gate-pass.html
echo "exit code: $?"
```
Expected: `exit code: 0` (no critical findings exist).

Check the failing case (threshold at the highest present severity):

```bash
/tmp/tf-lens-verify export --plan testdata/plan_basic.json --threat --fail-on=high --out /tmp/gate-fail.html
echo "exit code: $?"
```
Expected: non-zero exit code, stderr shows a `fail-on gate: high finding ...` message, and `/tmp/gate-fail.html` is NOT created (the function returns before reaching the write step).

Check the drift gate:

```bash
/tmp/tf-lens-verify export --plan testdata/plan_basic.json --threat --fail-on=critical --drift testdata/drift.json --out /tmp/gate-drift.html
echo "exit code: $?"
```
Expected: non-zero exit code, stderr shows `fail-on gate: 2 resource(s) drifted from state` (drift gate fires even though `--fail-on=critical` and there are no critical threat findings).

Check the validation error:

```bash
/tmp/tf-lens-verify export --plan testdata/plan_basic.json --fail-on=critical --out /tmp/gate-noThreat.html
echo "exit code: $?"
```
Expected: non-zero exit code, stderr shows `--fail-on requires --threat`.

Clean up:

```bash
rm -f /tmp/tf-lens-verify /tmp/gate-pass.html /tmp/gate-fail.html /tmp/gate-drift.html /tmp/gate-noThreat.html
```

- [ ] **Step 9: Update README**

In `README.md`, add a bullet near the other CLI-flag-driven features (e.g. near the Threat modelling or Cost overlay bullets around line 41-43):

```
**CI severity gate** — `--fail-on=critical|high|medium|info` exits 1 if threat findings at/above that severity are found, or if any drift is detected (requires `--threat`)
```

And add to the Shipped checklist (near the Threat modelling / drift entries around line 468/474):

```
- [x] `--fail-on` CI exit-code gate (threat severity threshold + any-drift)
```

- [ ] **Step 10: Commit**

```bash
git add cmd/export.go cmd/serve.go README.md
git commit -m "feat: wire --fail-on into export and serve commands"
```
