package threat_test

import (
	"testing"

	"github.com/hack-monk/tf-lens/internal/parser"
	"github.com/hack-monk/tf-lens/internal/threat"
)

// ── Security Group tests ─────────────────────────────────────────────────────

func TestSG_OpenToInternet_AllPorts(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_security_group.test",
		Type:    "aws_security_group",
		Name:    "test",
		Attributes: map[string]any{
			"ingress": []any{
				map[string]any{
					"from_port":   float64(0),
					"to_port":     float64(0),
					"protocol":    "-1",
					"cidr_blocks": []any{"0.0.0.0/0"},
				},
			},
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "SG002")
	assertSeverity(t, findings, "SG002", threat.SeverityCritical)
}

func TestSG_WebPort_IsInfo(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_security_group.alb",
		Type:    "aws_security_group",
		Name:    "alb",
		Attributes: map[string]any{
			"ingress": []any{
				map[string]any{
					"from_port":   float64(443),
					"to_port":     float64(443),
					"protocol":    "tcp",
					"cidr_blocks": []any{"0.0.0.0/0"},
				},
			},
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "SG001")
	assertSeverity(t, findings, "SG001", threat.SeverityInfo)
}

func TestSG_NoFindings_PrivateCIDR(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_security_group.private",
		Type:    "aws_security_group",
		Name:    "private",
		Attributes: map[string]any{
			"ingress": []any{
				map[string]any{
					"from_port":   float64(5432),
					"to_port":     float64(5432),
					"protocol":    "tcp",
					"cidr_blocks": []any{"10.0.0.0/8"},
				},
			},
		},
	}}

	findings := threat.Analyse(resources)
	if len(findings) != 0 {
		t.Errorf("expected no findings for private CIDR, got %d", len(findings))
	}
}

// ── RDS tests ────────────────────────────────────────────────────────────────

func TestRDS_PubliclyAccessible(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_db_instance.prod",
		Type:    "aws_db_instance",
		Name:    "prod",
		Attributes: map[string]any{
			"storage_encrypted":    true,
			"publicly_accessible":  true,
			"backup_retention_period": float64(7),
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "RDS002")
	assertSeverity(t, findings, "RDS002", threat.SeverityCritical)
}

func TestRDS_NoEncryption(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_db_instance.db",
		Type:    "aws_db_instance",
		Name:    "db",
		Attributes: map[string]any{
			"storage_encrypted":    false,
			"publicly_accessible":  false,
			"backup_retention_period": float64(7),
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "RDS001")
	assertSeverity(t, findings, "RDS001", threat.SeverityHigh)
}

func TestRDS_NoBackups(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_db_instance.db",
		Type:    "aws_db_instance",
		Name:    "db",
		Attributes: map[string]any{
			"storage_encrypted":       true,
			"publicly_accessible":     false,
			"backup_retention_period": float64(0),
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "RDS003")
}

// ── S3 tests ─────────────────────────────────────────────────────────────────

func TestS3_PublicACL(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_s3_bucket.assets",
		Type:    "aws_s3_bucket",
		Name:    "assets",
		Attributes: map[string]any{
			"acl": "public-read",
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "S3001")
	assertSeverity(t, findings, "S3001", threat.SeverityCritical)
}

// ── IAM tests ────────────────────────────────────────────────────────────────

func TestIAM_WildcardTrust(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_iam_role.bad",
		Type:    "aws_iam_role",
		Name:    "bad",
		Attributes: map[string]any{
			"assume_role_policy": `{"Statement":[{"Principal":{"AWS":"*"}}]}`,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "IAM001")
	assertSeverity(t, findings, "IAM001", threat.SeverityCritical)
}

// ── Summary tests ─────────────────────────────────────────────────────────────

func TestSortedBySeverity(t *testing.T) {
	resources := []parser.Resource{
		{
			Address: "aws_security_group.sg",
			Type:    "aws_security_group",
			Name:    "sg",
			Attributes: map[string]any{
				"ingress": []any{
					map[string]any{
						"from_port": float64(22), "to_port": float64(22),
						"protocol": "tcp", "cidr_blocks": []any{"0.0.0.0/0"},
					},
				},
			},
		},
		{
			Address: "aws_db_instance.db",
			Type:    "aws_db_instance",
			Name:    "db",
			Attributes: map[string]any{
				"publicly_accessible": true,
				"storage_encrypted":   true,
				"backup_retention_period": float64(7),
			},
		},
	}

	findings := threat.Analyse(resources)
	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(findings))
	}

	// First finding should be at least as severe as the last
	if findings[0].Severity.Weight() < findings[len(findings)-1].Severity.Weight() {
		t.Errorf("findings not sorted by severity: first=%s last=%s",
			findings[0].Severity, findings[len(findings)-1].Severity)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func assertHasCode(t *testing.T, findings []threat.Finding, code string) {
	t.Helper()
	for _, f := range findings {
		if f.Code == code {
			return
		}
	}
	t.Errorf("expected finding with code %q, got: %v", code, findingCodes(findings))
}

func assertSeverity(t *testing.T, findings []threat.Finding, code string, want threat.Severity) {
	t.Helper()
	for _, f := range findings {
		if f.Code == code {
			if f.Severity != want {
				t.Errorf("finding %q: severity = %q, want %q", code, f.Severity, want)
			}
			return
		}
	}
	t.Errorf("finding %q not found", code)
}

func findingCodes(findings []threat.Finding) []string {
	codes := make([]string, len(findings))
	for i, f := range findings {
		codes[i] = f.Code
	}
	return codes
}