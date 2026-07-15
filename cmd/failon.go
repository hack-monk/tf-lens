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
