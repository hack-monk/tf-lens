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
