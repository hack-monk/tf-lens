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

// ── KMS tests ───────────────────────────────────────────────────────────────

func TestKMS_NoRotation(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_kms_key.main",
		Type:    "aws_kms_key",
		Name:    "main",
		Attributes: map[string]any{
			"enable_key_rotation": false,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "KMS001")
	assertSeverity(t, findings, "KMS001", threat.SeverityMedium)
}

func TestKMS_WithRotation_NoFinding(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_kms_key.good",
		Type:    "aws_kms_key",
		Name:    "good",
		Attributes: map[string]any{
			"enable_key_rotation": true,
		},
	}}

	findings := threat.Analyse(resources)
	for _, f := range findings {
		if f.Code == "KMS001" {
			t.Error("expected no KMS001 finding when rotation enabled")
		}
	}
}

// ── RDS Cluster tests ───────────────────────────────────────────────────────

func TestRDSCluster_NoEncryption(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_rds_cluster.aurora",
		Type:    "aws_rds_cluster",
		Name:    "aurora",
		Attributes: map[string]any{
			"storage_encrypted": false,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "RDSC001")
	assertSeverity(t, findings, "RDSC001", threat.SeverityHigh)
}

func TestRDSCluster_NoDeletionProtection(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_rds_cluster.aurora",
		Type:    "aws_rds_cluster",
		Name:    "aurora",
		Attributes: map[string]any{
			"storage_encrypted": true,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "RDSC002")
}

// ── ECR tests ───────────────────────────────────────────────────────────────

func TestECR_NoScanOnPush(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_ecr_repository.app",
		Type:    "aws_ecr_repository",
		Name:    "app",
		Attributes: map[string]any{
			"image_tag_mutability": "MUTABLE",
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "ECR001")
	assertHasCode(t, findings, "ECR002")
}

func TestECR_Immutable_WithScan_NoMutableFinding(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_ecr_repository.good",
		Type:    "aws_ecr_repository",
		Name:    "good",
		Attributes: map[string]any{
			"image_tag_mutability": "IMMUTABLE",
			"image_scanning_configuration": []any{
				map[string]any{"scan_on_push": true},
			},
		},
	}}

	findings := threat.Analyse(resources)
	for _, f := range findings {
		if f.Code == "ECR001" || f.Code == "ECR002" {
			t.Errorf("unexpected finding %s for well-configured ECR", f.Code)
		}
	}
}

// ── OpenSearch tests ────────────────────────────────────────────────────────

func TestOpenSearch_NoEncryption(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_opensearch_domain.es",
		Type:    "aws_opensearch_domain",
		Name:    "es",
		Attributes: map[string]any{},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "OS001")
	assertHasCode(t, findings, "OS002")
	assertHasCode(t, findings, "OS003")
}

// ── Redshift tests ──────────────────────────────────────────────────────────

func TestRedshift_PublicUnencrypted(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_redshift_cluster.dw",
		Type:    "aws_redshift_cluster",
		Name:    "dw",
		Attributes: map[string]any{
			"encrypted":            false,
			"publicly_accessible":  true,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "RS001")
	assertHasCode(t, findings, "RS002")
	assertSeverity(t, findings, "RS002", threat.SeverityCritical)
}

// ── ECS Task Definition tests ───────────────────────────────────────────────

func TestECS_PrivilegedContainer(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_ecs_task_definition.app",
		Type:    "aws_ecs_task_definition",
		Name:    "app",
		Attributes: map[string]any{
			"container_definitions": `[{"name":"app","image":"nginx","privileged":true}]`,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "ECS001")
	assertSeverity(t, findings, "ECS001", threat.SeverityHigh)
}

// ── API Gateway tests ───────────────────────────────────────────────────────

func TestAPIGW_NoAccessLogs(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_api_gateway_stage.prod",
		Type:    "aws_api_gateway_stage",
		Name:    "prod",
		Attributes: map[string]any{},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "APIGW002")
}

// ── EC2 Instance tests ──────────────────────────────────────────────────────

func TestEC2_NoIMDSv2(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_instance.web",
		Type:    "aws_instance",
		Name:    "web",
		Attributes: map[string]any{
			"associate_public_ip_address": true,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "EC2001")
	assertHasCode(t, findings, "EC2002")
	assertSeverity(t, findings, "EC2001", threat.SeverityHigh)
}

func TestEC2_IMDSv2_Enforced(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_instance.good",
		Type:    "aws_instance",
		Name:    "good",
		Attributes: map[string]any{
			"monitoring": true,
			"metadata_options": []any{
				map[string]any{"http_tokens": "required"},
			},
			"root_block_device": []any{
				map[string]any{"encrypted": true},
			},
		},
	}}

	findings := threat.Analyse(resources)
	for _, f := range findings {
		if f.Code == "EC2001" || f.Code == "EC2004" {
			t.Errorf("unexpected finding %s for well-configured EC2", f.Code)
		}
	}
}

// ── EBS Volume tests ────────────────────────────────────────────────────────

func TestEBS_Unencrypted(t *testing.T) {
	resources := []parser.Resource{{
		Address:    "aws_ebs_volume.data",
		Type:       "aws_ebs_volume",
		Name:       "data",
		Attributes: map[string]any{"encrypted": false},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "EBS001")
	assertSeverity(t, findings, "EBS001", threat.SeverityHigh)
}

// ── CloudTrail tests ────────────────────────────────────────────────────────

func TestCloudTrail_Weak(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_cloudtrail.main",
		Type:    "aws_cloudtrail",
		Name:    "main",
		Attributes: map[string]any{
			"is_multi_region_trail":      false,
			"enable_log_file_validation": false,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "CT001")
	assertHasCode(t, findings, "CT002")
	assertHasCode(t, findings, "CT003")
}

// ── Load Balancer tests ─────────────────────────────────────────────────────

func TestALB_NoDropHeaders(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_alb.web",
		Type:    "aws_alb",
		Name:    "web",
		Attributes: map[string]any{
			"drop_invalid_header_fields": false,
			"internal":                   false,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "ALB001")
	assertHasCode(t, findings, "ALB003")
}

// ── Launch Template tests ───────────────────────────────────────────────────

func TestLaunchTemplate_NoIMDSv2(t *testing.T) {
	resources := []parser.Resource{{
		Address:    "aws_launch_template.app",
		Type:       "aws_launch_template",
		Name:       "app",
		Attributes: map[string]any{},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "LT001")
}

// ── Kinesis tests ───────────────────────────────────────────────────────────

func TestKinesis_NoEncryption(t *testing.T) {
	resources := []parser.Resource{{
		Address:    "aws_kinesis_stream.events",
		Type:       "aws_kinesis_stream",
		Name:       "events",
		Attributes: map[string]any{"encryption_type": "NONE"},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "KIN001")
}

// ── MSK tests ───────────────────────────────────────────────────────────────

func TestMSK_NoEncryption(t *testing.T) {
	resources := []parser.Resource{{
		Address:    "aws_msk_cluster.kafka",
		Type:       "aws_msk_cluster",
		Name:       "kafka",
		Attributes: map[string]any{},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "MSK001")
	assertHasCode(t, findings, "MSK003")
}

// ── DocumentDB tests ────────────────────────────────────────────────────────

func TestDocDB_Unencrypted(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_docdb_cluster.docs",
		Type:    "aws_docdb_cluster",
		Name:    "docs",
		Attributes: map[string]any{
			"storage_encrypted": false,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "DOC001")
	assertHasCode(t, findings, "DOC002")
}

// ── Neptune tests ───────────────────────────────────────────────────────────

func TestNeptune_Unencrypted(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_neptune_cluster.graph",
		Type:    "aws_neptune_cluster",
		Name:    "graph",
		Attributes: map[string]any{
			"storage_encrypted": false,
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "NEP001")
	assertHasCode(t, findings, "NEP002")
}

// ── CodeBuild tests ─────────────────────────────────────────────────────────

func TestCodeBuild_Privileged(t *testing.T) {
	resources := []parser.Resource{{
		Address: "aws_codebuild_project.build",
		Type:    "aws_codebuild_project",
		Name:    "build",
		Attributes: map[string]any{
			"environment": []any{
				map[string]any{"privileged_mode": true},
			},
		},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "CB001")
	assertSeverity(t, findings, "CB001", threat.SeverityHigh)
}

// ── EFS tests ───────────────────────────────────────────────────────────────

func TestEFS_Unencrypted(t *testing.T) {
	resources := []parser.Resource{{
		Address:    "aws_efs_file_system.data",
		Type:       "aws_efs_file_system",
		Name:       "data",
		Attributes: map[string]any{"encrypted": false},
	}}

	findings := threat.Analyse(resources)
	assertHasCode(t, findings, "EFS001")
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