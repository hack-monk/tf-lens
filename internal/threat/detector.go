package threat

import (
	"fmt"
	"strings"

	"github.com/hack-monk/tf-lens/internal/parser"
)

// Analyse runs all threat detectors against a slice of resources and
// returns every Finding discovered. Results are sorted by severity (desc).
func Analyse(resources []parser.Resource) []Finding {
	var findings []Finding

	for i := range resources {
		r := &resources[i]
		switch r.Type {
		case "aws_security_group":
			findings = append(findings, checkSecurityGroup(r)...)
		case "aws_s3_bucket":
			findings = append(findings, checkS3Bucket(r)...)
		case "aws_db_instance":
			findings = append(findings, checkRDSInstance(r)...)
		case "aws_iam_role":
			findings = append(findings, checkIAMRole(r)...)
		case "aws_lambda_function":
			findings = append(findings, checkLambda(r)...)
		case "aws_eks_cluster":
			findings = append(findings, checkEKSCluster(r)...)
		case "aws_elasticache_cluster", "aws_elasticache_replication_group":
			findings = append(findings, checkElastiCache(r)...)
		case "aws_sqs_queue":
			findings = append(findings, checkSQS(r)...)
		case "aws_sns_topic":
			findings = append(findings, checkSNS(r)...)
		case "aws_cloudfront_distribution":
			findings = append(findings, checkCloudFront(r)...)
		case "aws_kms_key":
			findings = append(findings, checkKMSKey(r)...)
		case "aws_rds_cluster":
			findings = append(findings, checkRDSCluster(r)...)
		case "aws_ecr_repository":
			findings = append(findings, checkECR(r)...)
		case "aws_api_gateway_rest_api", "aws_apigatewayv2_api":
			findings = append(findings, checkAPIGateway(r)...)
		case "aws_api_gateway_stage", "aws_apigatewayv2_stage":
			findings = append(findings, checkAPIGatewayStage(r)...)
		case "aws_elasticsearch_domain", "aws_opensearch_domain":
			findings = append(findings, checkOpenSearch(r)...)
		case "aws_redshift_cluster":
			findings = append(findings, checkRedshift(r)...)
		case "aws_ecs_task_definition":
			findings = append(findings, checkECSTaskDef(r)...)
		case "aws_instance":
			findings = append(findings, checkEC2Instance(r)...)
		case "aws_ebs_volume":
			findings = append(findings, checkEBSVolume(r)...)
		case "aws_efs_file_system":
			findings = append(findings, checkEFS(r)...)
		case "aws_cloudtrail":
			findings = append(findings, checkCloudTrail(r)...)
		case "aws_alb", "aws_lb":
			findings = append(findings, checkLoadBalancer(r)...)
		case "aws_launch_template":
			findings = append(findings, checkLaunchTemplate(r)...)
		case "aws_secretsmanager_secret":
			findings = append(findings, checkSecretsManager(r)...)
		case "aws_kinesis_stream":
			findings = append(findings, checkKinesis(r)...)
		case "aws_msk_cluster":
			findings = append(findings, checkMSK(r)...)
		case "aws_docdb_cluster":
			findings = append(findings, checkDocDB(r)...)
		case "aws_neptune_cluster":
			findings = append(findings, checkNeptune(r)...)
		case "aws_codebuild_project":
			findings = append(findings, checkCodeBuild(r)...)
		}
	}

	sortFindings(findings)
	return findings
}

// IndexByAddress returns a map from resource address to its findings.
// Used by the renderer to attach findings to nodes.
func IndexByAddress(findings []Finding) map[string][]Finding {
	idx := make(map[string][]Finding)
	for _, f := range findings {
		idx[f.ResourceAddress] = append(idx[f.ResourceAddress], f)
	}
	return idx
}

// ── Security Group ────────────────────────────────────────────────────────────

func checkSecurityGroup(r *parser.Resource) []Finding {
	var findings []Finding

	ingress := getList(r.Attributes, "ingress")
	for _, rule := range ingress {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		cidrs := getStringList(ruleMap, "cidr_blocks")
		ipv6 := getStringList(ruleMap, "ipv6_cidr_blocks")
		allCIDRs := append(cidrs, ipv6...)

		fromPort := getInt(ruleMap, "from_port")
		toPort := getInt(ruleMap, "to_port")
		protocol := getString(ruleMap, "protocol")

		for _, cidr := range allCIDRs {
			if cidr == "0.0.0.0/0" || cidr == "::/0" {
				// Allow 80/443 on ALB security groups — these are expected
				if isWebPort(fromPort, toPort) {
					findings = append(findings, Finding{
						ResourceAddress: r.Address,
						ResourceType:    r.Type,
						Severity:        SeverityInfo,
						Code:            "SG001",
						Title:           "Web port open to internet",
						Detail:          fmt.Sprintf("Ingress %s/%d-%d open to %s (web traffic)", protocol, fromPort, toPort, cidr),
						Remediation:     "Expected for public-facing load balancers. Ensure this SG is only attached to ALB/NLB, not to EC2 instances directly.",
					})
				} else if fromPort == 0 && toPort == 0 {
					findings = append(findings, Finding{
						ResourceAddress: r.Address,
						ResourceType:    r.Type,
						Severity:        SeverityCritical,
						Code:            "SG002",
						Title:           "All traffic open to internet",
						Detail:          fmt.Sprintf("Ingress allows ALL traffic from %s — full exposure to internet", cidr),
						Remediation:     "Remove this rule immediately. Restrict ingress to specific ports and trusted CIDR ranges.",
					})
				} else {
					findings = append(findings, Finding{
						ResourceAddress: r.Address,
						ResourceType:    r.Type,
						Severity:        SeverityHigh,
						Code:            "SG003",
						Title:           fmt.Sprintf("Port %d-%d open to internet", fromPort, toPort),
						Detail:          fmt.Sprintf("Ingress %s/%d-%d open to %s", protocol, fromPort, toPort, cidr),
						Remediation:     fmt.Sprintf("Restrict port %d-%d to specific trusted IP ranges. Avoid 0.0.0.0/0 unless serving public traffic.", fromPort, toPort),
					})
				}
			}
		}
	}

	// Check for unrestricted egress (informational only)
	egress := getList(r.Attributes, "egress")
	for _, rule := range egress {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		cidrs := getStringList(ruleMap, "cidr_blocks")
		fromPort := getInt(ruleMap, "from_port")
		toPort := getInt(ruleMap, "to_port")
		for _, cidr := range cidrs {
			if (cidr == "0.0.0.0/0") && fromPort == 0 && toPort == 0 {
				findings = append(findings, Finding{
					ResourceAddress: r.Address,
					ResourceType:    r.Type,
					Severity:        SeverityInfo,
					Code:            "SG004",
					Title:           "Unrestricted egress",
					Detail:          "All outbound traffic allowed to 0.0.0.0/0",
					Remediation:     "Consider restricting egress to known endpoints for defence-in-depth. For most workloads this is acceptable.",
				})
				break
			}
		}
	}

	return findings
}

// ── S3 Bucket ─────────────────────────────────────────────────────────────────

func checkS3Bucket(r *parser.Resource) []Finding {
	var findings []Finding

	// Public ACL
	acl := getString(r.Attributes, "acl")
	if acl == "public-read" || acl == "public-read-write" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityCritical,
			Code:            "S3001",
			Title:           "Bucket is publicly readable",
			Detail:          fmt.Sprintf("Bucket ACL is %q — anyone on the internet can read objects", acl),
			Remediation:     `Set acl = "private" and use CloudFront or pre-signed URLs for public content.`,
		})
	}

	// Website hosting (makes bucket publicly accessible)
	if _, hasWebsite := r.Attributes["website"]; hasWebsite {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "S3002",
			Title:           "Static website hosting enabled",
			Detail:          "S3 website hosting exposes bucket contents publicly via HTTP (no HTTPS)",
			Remediation:     "Use CloudFront with HTTPS in front of S3 website buckets. Consider S3 Object Ownership settings.",
		})
	}

	// Server-side encryption
	if _, hasSSE := r.Attributes["server_side_encryption_configuration"]; !hasSSE {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "S3003",
			Title:           "Encryption not configured",
			Detail:          "No server_side_encryption_configuration block found — bucket may not enforce encryption at rest",
			Remediation:     "Add server_side_encryption_configuration with AES256 or aws:kms. Consider enabling S3 bucket key for cost reduction.",
		})
	}

	// Versioning
	versioning, _ := r.Attributes["versioning"].(map[string]any)
	if enabled, _ := versioning["enabled"].(bool); !enabled {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "S3004",
			Title:           "Versioning not enabled",
			Detail:          "S3 versioning is disabled — accidental deletes or overwrites are not recoverable",
			Remediation:     `Set versioning { enabled = true } for buckets containing important data.`,
		})
	}

	return findings
}

// ── RDS Instance ──────────────────────────────────────────────────────────────

func checkRDSInstance(r *parser.Resource) []Finding {
	var findings []Finding

	// Storage encryption
	if encrypted, _ := r.Attributes["storage_encrypted"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "RDS001",
			Title:           "Storage encryption disabled",
			Detail:          "storage_encrypted = false — database contents are not encrypted at rest",
			Remediation:     "Set storage_encrypted = true. Note: changing this requires creating a new instance from a snapshot.",
		})
	}

	// Publicly accessible
	if pub, _ := r.Attributes["publicly_accessible"].(bool); pub {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityCritical,
			Code:            "RDS002",
			Title:           "Database is publicly accessible",
			Detail:          "publicly_accessible = true — RDS endpoint is reachable from the internet",
			Remediation:     "Set publicly_accessible = false. Access the database through a bastion host or VPN.",
		})
	}

	// Backup retention
	retention := getInt(r.Attributes, "backup_retention_period")
	if retention == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "RDS003",
			Title:           "Automated backups disabled",
			Detail:          "backup_retention_period = 0 — no automated backups, point-in-time recovery unavailable",
			Remediation:     "Set backup_retention_period to at least 7 (days). For production, 14-35 days recommended.",
		})
	}

	// Deletion protection
	if delProt, ok := r.Attributes["deletion_protection"]; !ok || delProt == false {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "RDS004",
			Title:           "Deletion protection disabled",
			Detail:          "deletion_protection is not enabled — database can be accidentally destroyed by terraform destroy",
			Remediation:     "Set deletion_protection = true for production databases.",
		})
	}

	return findings
}

// ── IAM Role ──────────────────────────────────────────────────────────────────

func checkIAMRole(r *parser.Resource) []Finding {
	var findings []Finding

	// Check assume_role_policy for overly broad trust.
	// Real-world JSON may have spaces around colons or not — check both.
	policy := getString(r.Attributes, "assume_role_policy")
	wildcardPrincipal := strings.Contains(policy, `"AWS": "*"`) ||
		strings.Contains(policy, `"AWS":"*"`) ||
		strings.Contains(policy, `"Service": "*"`) ||
		strings.Contains(policy, `"Service":"*"`)
	if wildcardPrincipal {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityCritical,
			Code:            "IAM001",
			Title:           "Wildcard trust policy",
			Detail:          "assume_role_policy allows any AWS principal (*) to assume this role",
			Remediation:     "Restrict the Principal in the trust policy to specific AWS accounts, services, or roles.",
		})
	}

	// Check inline policy for wildcard actions/resources
	inlinePolicies := getList(r.Attributes, "inline_policy")
	for _, p := range inlinePolicies {
		pMap, ok := p.(map[string]any)
		if !ok {
			continue
		}
		policyDoc := getString(pMap, "policy")
		hasWildcardAction := strings.Contains(policyDoc, `"Action": "*"`) ||
			strings.Contains(policyDoc, `"Action":"*"`)
		hasWildcardResource := strings.Contains(policyDoc, `"Resource": "*"`) ||
			strings.Contains(policyDoc, `"Resource":"*"`)

		if hasWildcardAction {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityCritical,
				Code:            "IAM002",
				Title:           "Wildcard Action in inline policy",
				Detail:          `Inline policy contains "Action": "*" — grants all AWS permissions`,
				Remediation:     "Follow least-privilege: specify only the exact actions this role needs.",
			})
		}
		if hasWildcardResource {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityHigh,
				Code:            "IAM003",
				Title:           "Wildcard Resource in inline policy",
				Detail:          `Inline policy contains "Resource": "*" — applies to all AWS resources`,
				Remediation:     "Restrict Resource to specific ARNs wherever possible.",
			})
		}
	}

	return findings
}

// ── Lambda ────────────────────────────────────────────────────────────────────

func checkLambda(r *parser.Resource) []Finding {
	var findings []Finding

	// No VPC config — Lambda runs in AWS-managed network
	vpcConfig, _ := r.Attributes["vpc_config"].([]any)
	subnetIDs, _ := r.Attributes["subnet_ids"].([]any)
	if len(vpcConfig) == 0 && len(subnetIDs) == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "LAM001",
			Title:           "Lambda not in VPC",
			Detail:          "Function has no vpc_config — it runs in the AWS-managed network without VPC-level network controls",
			Remediation:     "Consider deploying in a VPC if the function accesses private resources (RDS, ElastiCache). Note: VPC adds cold-start latency.",
		})
	}

	// Check environment for hardcoded secrets
	env, _ := r.Attributes["environment"].([]any)
	for _, e := range env {
		eMap, ok := e.(map[string]any)
		if !ok {
			continue
		}
		vars, _ := eMap["variables"].(map[string]any)
		for key, val := range vars {
			valStr, _ := val.(string)
			keyLower := strings.ToLower(key)
			if containsSecretKeyword(keyLower) && len(valStr) > 0 && !strings.HasPrefix(valStr, "arn:") {
				findings = append(findings, Finding{
					ResourceAddress: r.Address,
					ResourceType:    r.Type,
					Severity:        SeverityHigh,
					Code:            "LAM002",
					Title:           "Possible secret in environment variable",
					Detail:          fmt.Sprintf("Environment variable %q may contain a hardcoded secret", key),
					Remediation:     "Store secrets in AWS Secrets Manager or SSM Parameter Store. Fetch them at runtime or via Lambda environment variable injection.",
				})
				break
			}
		}
	}

	return findings
}

// ── EKS Cluster ───────────────────────────────────────────────────────────────

func checkEKSCluster(r *parser.Resource) []Finding {
	var findings []Finding

	// Public endpoint
	access, _ := r.Attributes["vpc_config"].([]any)
	for _, a := range access {
		aMap, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if pub, _ := aMap["endpoint_public_access"].(bool); pub {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityHigh,
				Code:            "EKS001",
				Title:           "Kubernetes API endpoint is public",
				Detail:          "endpoint_public_access = true — Kubernetes API server is reachable from the internet",
				Remediation:     "Set endpoint_public_access = false and use a VPN or bastion to access the API server. At minimum, restrict endpoint_public_access_cidrs.",
			})
		}
	}

	// Secrets encryption
	encryptionConfig := getList(r.Attributes, "encryption_config")
	if len(encryptionConfig) == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "EKS002",
			Title:           "Kubernetes secrets not encrypted with KMS",
			Detail:          "No encryption_config block — Kubernetes secrets are not encrypted with a customer-managed KMS key",
			Remediation:     "Add an encryption_config block with a KMS key for the secrets resource.",
		})
	}

	return findings
}

// ── ElastiCache ───────────────────────────────────────────────────────────────

func checkElastiCache(r *parser.Resource) []Finding {
	var findings []Finding

	if encrypted, _ := r.Attributes["at_rest_encryption_enabled"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "EC001",
			Title:           "Cache not encrypted at rest",
			Detail:          "at_rest_encryption_enabled = false",
			Remediation:     "Set at_rest_encryption_enabled = true. Note: requires cluster replacement.",
		})
	}

	if encrypted, _ := r.Attributes["transit_encryption_enabled"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "EC002",
			Title:           "Cache traffic not encrypted in transit",
			Detail:          "transit_encryption_enabled = false — data between app and cache is unencrypted",
			Remediation:     "Set transit_encryption_enabled = true and update clients to use TLS.",
		})
	}

	return findings
}

// ── SQS Queue ────────────────────────────────────────────────────────────────

func checkSQS(r *parser.Resource) []Finding {
	var findings []Finding

	kmsMaster := getString(r.Attributes, "kms_master_key_id")
	sqsManagedSSE := getString(r.Attributes, "sqs_managed_sse_enabled")
	if kmsMaster == "" && sqsManagedSSE != "true" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "SQS001",
			Title:           "Queue not encrypted",
			Detail:          "No KMS key or SQS managed SSE configured — messages stored unencrypted",
			Remediation:     `Set sqs_managed_sse_enabled = true for basic encryption, or kms_master_key_id for customer-managed keys.`,
		})
	}

	return findings
}

// ── SNS Topic ────────────────────────────────────────────────────────────────

func checkSNS(r *parser.Resource) []Finding {
	var findings []Finding

	kmsMaster := getString(r.Attributes, "kms_master_key_id")
	if kmsMaster == "" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "SNS001",
			Title:           "Topic not encrypted with KMS",
			Detail:          "No kms_master_key_id set — messages stored with AWS-managed encryption only",
			Remediation:     "Set kms_master_key_id for customer-managed encryption, especially for topics carrying sensitive data.",
		})
	}

	return findings
}

// ── CloudFront ───────────────────────────────────────────────────────────────

func checkCloudFront(r *parser.Resource) []Finding {
	var findings []Finding

	// Check viewer certificate
	cert, _ := r.Attributes["viewer_certificate"].([]any)
	for _, c := range cert {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if proto, _ := cMap["minimum_protocol_version"].(string); proto == "SSLv3" || proto == "TLSv1" {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityHigh,
				Code:            "CF001",
				Title:           "Outdated TLS version allowed",
				Detail:          fmt.Sprintf("minimum_protocol_version = %q — allows weak SSL/TLS", proto),
				Remediation:     `Set minimum_protocol_version = "TLSv1.2_2021" or higher.`,
			})
		}
	}

	// WAF not associated
	if _, hasWAF := r.Attributes["web_acl_id"]; !hasWAF {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "CF002",
			Title:           "No WAF associated",
			Detail:          "CloudFront distribution has no WAF web ACL — no protection against common web attacks",
			Remediation:     "Associate an aws_wafv2_web_acl for protection against OWASP Top 10 attacks.",
		})
	}

	return findings
}

// ── KMS Key ─────────────────────────────────────────────────────────────────

func checkKMSKey(r *parser.Resource) []Finding {
	var findings []Finding

	// Key rotation
	if rotation, _ := r.Attributes["enable_key_rotation"].(bool); !rotation {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "KMS001",
			Title:           "Key rotation not enabled",
			Detail:          "enable_key_rotation = false — key material is never automatically rotated",
			Remediation:     "Set enable_key_rotation = true. AWS rotates the backing key annually while keeping the key ID stable.",
		})
	}

	// Overly broad key policy
	policy := getString(r.Attributes, "policy")
	if strings.Contains(policy, `"*"`) && strings.Contains(policy, `"kms:*"`) {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "KMS002",
			Title:           "KMS key policy allows all actions",
			Detail:          `Key policy grants kms:* to principal "*" — any AWS principal can use or manage this key`,
			Remediation:     "Restrict the key policy to specific principals and actions. Separate key administrators from key users.",
		})
	}

	return findings
}

// ── RDS Cluster (Aurora) ────────────────────────────────────────────────────

func checkRDSCluster(r *parser.Resource) []Finding {
	var findings []Finding

	// Storage encryption
	if encrypted, _ := r.Attributes["storage_encrypted"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "RDSC001",
			Title:           "Aurora cluster storage not encrypted",
			Detail:          "storage_encrypted = false — cluster data is not encrypted at rest",
			Remediation:     "Set storage_encrypted = true. Must be set at cluster creation time.",
		})
	}

	// Deletion protection
	if delProt, ok := r.Attributes["deletion_protection"]; !ok || delProt == false {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "RDSC002",
			Title:           "Aurora deletion protection disabled",
			Detail:          "deletion_protection is not enabled — cluster can be accidentally destroyed",
			Remediation:     "Set deletion_protection = true for production Aurora clusters.",
		})
	}

	// Backup retention
	retention := getInt(r.Attributes, "backup_retention_period")
	if retention < 7 && retention > 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "RDSC003",
			Title:           "Short backup retention period",
			Detail:          fmt.Sprintf("backup_retention_period = %d days — consider longer retention for production", retention),
			Remediation:     "Set backup_retention_period to at least 7 days. For production, 14-35 days recommended.",
		})
	}

	// IAM authentication
	if iamAuth, _ := r.Attributes["iam_database_authentication_enabled"].(bool); !iamAuth {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "RDSC004",
			Title:           "IAM database authentication not enabled",
			Detail:          "iam_database_authentication_enabled = false — using password-only auth",
			Remediation:     "Enable IAM authentication for token-based access. Eliminates need to manage database passwords.",
		})
	}

	return findings
}

// ── ECR Repository ──────────────────────────────────────────────────────────

func checkECR(r *parser.Resource) []Finding {
	var findings []Finding

	// Image scan on push
	scanConfig := getList(r.Attributes, "image_scanning_configuration")
	scanEnabled := false
	for _, sc := range scanConfig {
		scMap, ok := sc.(map[string]any)
		if !ok {
			continue
		}
		if enabled, _ := scMap["scan_on_push"].(bool); enabled {
			scanEnabled = true
		}
	}
	if !scanEnabled {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "ECR001",
			Title:           "Image scanning on push not enabled",
			Detail:          "scan_on_push = false — container images are not scanned for vulnerabilities on push",
			Remediation:     "Set image_scanning_configuration { scan_on_push = true } to catch CVEs before deployment.",
		})
	}

	// Image tag immutability
	tagMutability := getString(r.Attributes, "image_tag_mutability")
	if tagMutability == "" || tagMutability == "MUTABLE" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "ECR002",
			Title:           "Image tags are mutable",
			Detail:          "image_tag_mutability = MUTABLE — tags can be overwritten, risk of deploying unexpected images",
			Remediation:     `Set image_tag_mutability = "IMMUTABLE" to prevent tag overwrites.`,
		})
	}

	// Encryption
	encryptionConfig := getList(r.Attributes, "encryption_configuration")
	if len(encryptionConfig) == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "ECR003",
			Title:           "No KMS encryption configured",
			Detail:          "No encryption_configuration block — images encrypted with default AES-256 only",
			Remediation:     "Add encryption_configuration with a KMS key for customer-managed encryption.",
		})
	}

	return findings
}

// ── API Gateway ─────────────────────────────────────────────────────────────

func checkAPIGateway(r *parser.Resource) []Finding {
	var findings []Finding

	// Check for missing authorization (REST API v1)
	if r.Type == "aws_api_gateway_rest_api" {
		policy := getString(r.Attributes, "policy")
		if policy == "" {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityInfo,
				Code:            "APIGW001",
				Title:           "No resource policy configured",
				Detail:          "API has no resource policy — no IP or account-level access restrictions",
				Remediation:     "Add a resource policy to restrict access by source IP, VPC, or AWS account.",
			})
		}
	}

	return findings
}

func checkAPIGatewayStage(r *parser.Resource) []Finding {
	var findings []Finding

	// Access logging
	accessLog := getList(r.Attributes, "access_log_settings")
	if len(accessLog) == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "APIGW002",
			Title:           "Access logging not configured",
			Detail:          "No access_log_settings — API calls are not logged for audit or troubleshooting",
			Remediation:     "Add access_log_settings with a CloudWatch log group destination.",
		})
	}

	// Client certificate
	if r.Type == "aws_api_gateway_stage" {
		certID := getString(r.Attributes, "client_certificate_id")
		if certID == "" {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityInfo,
				Code:            "APIGW003",
				Title:           "No client certificate for backend",
				Detail:          "No client_certificate_id — API Gateway does not use mTLS to call backend integrations",
				Remediation:     "Configure a client certificate for backend authentication if backend supports mTLS.",
			})
		}
	}

	// WAF (v2 stage)
	if r.Type == "aws_apigatewayv2_stage" {
		// xray tracing
		if tracing, _ := r.Attributes["xray_tracing_enabled"].(bool); !tracing {
			// Note: this is only for HTTP APIs in v2 — not always applicable
		}
	}

	return findings
}

// ── OpenSearch / Elasticsearch ──────────────────────────────────────────────

func checkOpenSearch(r *parser.Resource) []Finding {
	var findings []Finding

	// Encryption at rest
	encryptAtRest := getList(r.Attributes, "encrypt_at_rest")
	encrypted := false
	for _, e := range encryptAtRest {
		eMap, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if enabled, _ := eMap["enabled"].(bool); enabled {
			encrypted = true
		}
	}
	if !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "OS001",
			Title:           "Encryption at rest not enabled",
			Detail:          "encrypt_at_rest is not enabled — index data stored unencrypted",
			Remediation:     "Set encrypt_at_rest { enabled = true }.",
		})
	}

	// Node-to-node encryption
	nodeToNode := getList(r.Attributes, "node_to_node_encryption")
	n2nEnabled := false
	for _, n := range nodeToNode {
		nMap, ok := n.(map[string]any)
		if !ok {
			continue
		}
		if enabled, _ := nMap["enabled"].(bool); enabled {
			n2nEnabled = true
		}
	}
	if !n2nEnabled {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "OS002",
			Title:           "Node-to-node encryption disabled",
			Detail:          "node_to_node_encryption not enabled — inter-node traffic is unencrypted",
			Remediation:     "Set node_to_node_encryption { enabled = true }.",
		})
	}

	// Domain endpoint options (enforce HTTPS)
	domainEndpoint := getList(r.Attributes, "domain_endpoint_options")
	httpsEnforced := false
	for _, d := range domainEndpoint {
		dMap, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if enforced, _ := dMap["enforce_https"].(bool); enforced {
			httpsEnforced = true
		}
	}
	if !httpsEnforced {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "OS003",
			Title:           "HTTPS not enforced on domain endpoint",
			Detail:          "enforce_https is not set — clients can connect over unencrypted HTTP",
			Remediation:     "Set domain_endpoint_options { enforce_https = true, tls_security_policy = \"Policy-Min-TLS-1-2-2019-07\" }.",
		})
	}

	// Logging
	logPublishing := getList(r.Attributes, "log_publishing_options")
	if len(logPublishing) == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "OS004",
			Title:           "Audit logging not configured",
			Detail:          "No log_publishing_options — search and index operations are not logged",
			Remediation:     "Add log_publishing_options for SEARCH_SLOW_LOGS, INDEX_SLOW_LOGS, and ES_APPLICATION_LOGS.",
		})
	}

	return findings
}

// ── Redshift Cluster ────────────────────────────────────────────────────────

func checkRedshift(r *parser.Resource) []Finding {
	var findings []Finding

	// Encryption
	if encrypted, _ := r.Attributes["encrypted"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "RS001",
			Title:           "Redshift cluster not encrypted",
			Detail:          "encrypted = false — data warehouse contents stored unencrypted",
			Remediation:     "Set encrypted = true with a KMS key. Requires cluster recreation.",
		})
	}

	// Publicly accessible
	if pub, _ := r.Attributes["publicly_accessible"].(bool); pub {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityCritical,
			Code:            "RS002",
			Title:           "Redshift cluster is publicly accessible",
			Detail:          "publicly_accessible = true — cluster endpoint reachable from internet",
			Remediation:     "Set publicly_accessible = false. Access via VPN or bastion host.",
		})
	}

	// Audit logging
	logging := getList(r.Attributes, "logging")
	loggingEnabled := false
	for _, l := range logging {
		lMap, ok := l.(map[string]any)
		if !ok {
			continue
		}
		if enabled, _ := lMap["enable"].(bool); enabled {
			loggingEnabled = true
		}
	}
	if !loggingEnabled {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "RS003",
			Title:           "Audit logging not enabled",
			Detail:          "No logging configuration — query and connection activity not logged",
			Remediation:     "Set logging { enable = true, bucket_name = \"...\" } for audit trail.",
		})
	}

	return findings
}

// ── ECS Task Definition ────────────────────────────────────────────────────

func checkECSTaskDef(r *parser.Resource) []Finding {
	var findings []Finding

	// Check for privileged containers or hardcoded secrets in container definitions
	containerDefs := getString(r.Attributes, "container_definitions")

	if strings.Contains(containerDefs, `"privileged":true`) || strings.Contains(containerDefs, `"privileged": true`) {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "ECS001",
			Title:           "Privileged container detected",
			Detail:          "Container runs in privileged mode — has full access to host kernel capabilities",
			Remediation:     "Remove privileged = true. Use specific Linux capabilities instead if needed.",
		})
	}

	// Read-only root filesystem
	if !strings.Contains(containerDefs, `"readonlyRootFilesystem"`) {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "ECS002",
			Title:           "Root filesystem not read-only",
			Detail:          "readonlyRootFilesystem not set — container can write to its filesystem",
			Remediation:     "Set readonlyRootFilesystem = true and use volumes for writable paths.",
		})
	}

	// Hardcoded secrets in environment
	envSecretKeywords := []string{`"PASSWORD"`, `"SECRET"`, `"API_KEY"`, `"TOKEN"`, `"PRIVATE_KEY"`}
	for _, kw := range envSecretKeywords {
		if strings.Contains(strings.ToUpper(containerDefs), kw) &&
			!strings.Contains(containerDefs, `"valueFrom"`) {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityHigh,
				Code:            "ECS003",
				Title:           "Possible hardcoded secret in container environment",
				Detail:          "Container definition has secret-like environment variables without valueFrom",
				Remediation:     "Use secrets with valueFrom to reference SSM Parameter Store or Secrets Manager ARNs.",
			})
			break
		}
	}

	return findings
}

// ── EC2 Instance ────────────────────────────────────────────────────────────

func checkEC2Instance(r *parser.Resource) []Finding {
	var findings []Finding

	// IMDSv2 enforcement
	metadataOpts := getList(r.Attributes, "metadata_options")
	imdsv2 := false
	for _, m := range metadataOpts {
		mMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if getString(mMap, "http_tokens") == "required" {
			imdsv2 = true
		}
	}
	if !imdsv2 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "EC2001",
			Title:           "IMDSv2 not enforced",
			Detail:          "Instance metadata service v1 is accessible — vulnerable to SSRF credential theft",
			Remediation:     `Set metadata_options { http_tokens = "required" } to enforce IMDSv2.`,
		})
	}

	// Public IP association
	if pub, _ := r.Attributes["associate_public_ip_address"].(bool); pub {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "EC2002",
			Title:           "Public IP associated",
			Detail:          "associate_public_ip_address = true — instance is directly internet-reachable",
			Remediation:     "Use a load balancer or NAT gateway instead of assigning public IPs to instances.",
		})
	}

	// Monitoring
	if monitoring, _ := r.Attributes["monitoring"].(bool); !monitoring {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "EC2003",
			Title:           "Detailed monitoring disabled",
			Detail:          "monitoring = false — only basic 5-minute CloudWatch metrics collected",
			Remediation:     "Set monitoring = true for 1-minute metric granularity.",
		})
	}

	// Root block device encryption
	rootBlock := getList(r.Attributes, "root_block_device")
	for _, rb := range rootBlock {
		rbMap, ok := rb.(map[string]any)
		if !ok {
			continue
		}
		if encrypted, _ := rbMap["encrypted"].(bool); !encrypted {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityHigh,
				Code:            "EC2004",
				Title:           "Root volume not encrypted",
				Detail:          "Root block device encryption is not enabled",
				Remediation:     "Set root_block_device { encrypted = true } or enable EBS encryption by default in account settings.",
			})
		}
	}

	return findings
}

// ── EBS Volume ──────────────────────────────────────────────────────────────

func checkEBSVolume(r *parser.Resource) []Finding {
	var findings []Finding

	if encrypted, _ := r.Attributes["encrypted"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "EBS001",
			Title:           "EBS volume not encrypted",
			Detail:          "encrypted = false — data at rest is unencrypted",
			Remediation:     "Set encrypted = true. Enable EBS encryption by default in account settings for all new volumes.",
		})
	}

	return findings
}

// ── EFS File System ─────────────────────────────────────────────────────────

func checkEFS(r *parser.Resource) []Finding {
	var findings []Finding

	if encrypted, _ := r.Attributes["encrypted"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "EFS001",
			Title:           "EFS not encrypted at rest",
			Detail:          "encrypted = false — file system data stored unencrypted",
			Remediation:     "Set encrypted = true with a KMS key. Must be set at creation time.",
		})
	}

	// Lifecycle policy
	lifecycle := getList(r.Attributes, "lifecycle_policy")
	if len(lifecycle) == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "EFS002",
			Title:           "No lifecycle policy configured",
			Detail:          "No lifecycle_policy — infrequently accessed files not auto-transitioned to IA storage class",
			Remediation:     "Add lifecycle_policy to transition files to Infrequent Access after 30 days for cost savings.",
		})
	}

	return findings
}

// ── CloudTrail ──────────────────────────────────────────────────────────────

func checkCloudTrail(r *parser.Resource) []Finding {
	var findings []Finding

	// Multi-region
	if multiRegion, _ := r.Attributes["is_multi_region_trail"].(bool); !multiRegion {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "CT001",
			Title:           "Trail is not multi-region",
			Detail:          "is_multi_region_trail = false — API activity in other regions is not captured",
			Remediation:     "Set is_multi_region_trail = true for comprehensive audit coverage.",
		})
	}

	// Log file validation
	if validation, _ := r.Attributes["enable_log_file_validation"].(bool); !validation {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "CT002",
			Title:           "Log file validation disabled",
			Detail:          "enable_log_file_validation = false — log tampering cannot be detected",
			Remediation:     "Set enable_log_file_validation = true to detect unauthorized log modifications.",
		})
	}

	// KMS encryption
	kmsKey := getString(r.Attributes, "kms_key_id")
	if kmsKey == "" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "CT003",
			Title:           "CloudTrail logs not encrypted with KMS",
			Detail:          "No kms_key_id — logs encrypted with S3 default encryption only",
			Remediation:     "Set kms_key_id to a customer-managed KMS key for stronger encryption controls.",
		})
	}

	// CloudWatch integration
	cwLogGroup := getString(r.Attributes, "cloud_watch_logs_group_arn")
	if cwLogGroup == "" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "CT004",
			Title:           "No CloudWatch Logs integration",
			Detail:          "No cloud_watch_logs_group_arn — cannot set up metric filters or alarms on API activity",
			Remediation:     "Configure cloud_watch_logs_group_arn for real-time alerting on suspicious API calls.",
		})
	}

	return findings
}

// ── Load Balancer (ALB/NLB) ─────────────────────────────────────────────────

func checkLoadBalancer(r *parser.Resource) []Finding {
	var findings []Finding

	// Drop invalid headers (ALB only)
	lbType := getString(r.Attributes, "load_balancer_type")
	if lbType == "" || lbType == "application" {
		if drop, _ := r.Attributes["drop_invalid_header_fields"].(bool); !drop {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityMedium,
				Code:            "ALB001",
				Title:           "Invalid header fields not dropped",
				Detail:          "drop_invalid_header_fields = false — ALB forwards malformed HTTP headers to targets",
				Remediation:     "Set drop_invalid_header_fields = true to prevent HTTP request smuggling attacks.",
			})
		}
	}

	// Internal vs internet-facing
	if internal, _ := r.Attributes["internal"].(bool); !internal {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "ALB002",
			Title:           "Internet-facing load balancer",
			Detail:          "internal = false — load balancer is accessible from the internet",
			Remediation:     "Ensure this is intentional. Use internal = true for backend services.",
		})
	}

	// Access logs
	accessLogs := getList(r.Attributes, "access_logs")
	logsEnabled := false
	for _, al := range accessLogs {
		alMap, ok := al.(map[string]any)
		if !ok {
			continue
		}
		if enabled, _ := alMap["enabled"].(bool); enabled {
			logsEnabled = true
		}
	}
	if !logsEnabled {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "ALB003",
			Title:           "Access logging not enabled",
			Detail:          "No access_logs enabled — request-level audit trail unavailable",
			Remediation:     "Set access_logs { enabled = true, bucket = \"...\" } for request logging.",
		})
	}

	// Deletion protection
	if delProt, _ := r.Attributes["enable_deletion_protection"].(bool); !delProt {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "ALB004",
			Title:           "Deletion protection disabled",
			Detail:          "enable_deletion_protection = false — load balancer can be accidentally deleted",
			Remediation:     "Set enable_deletion_protection = true for production load balancers.",
		})
	}

	return findings
}

// ── Launch Template ─────────────────────────────────────────────────────────

func checkLaunchTemplate(r *parser.Resource) []Finding {
	var findings []Finding

	// IMDSv2
	metadataOpts := getList(r.Attributes, "metadata_options")
	imdsv2 := false
	for _, m := range metadataOpts {
		mMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if getString(mMap, "http_tokens") == "required" {
			imdsv2 = true
		}
	}
	if !imdsv2 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "LT001",
			Title:           "IMDSv2 not enforced in launch template",
			Detail:          "Instances launched from this template can use IMDSv1 — vulnerable to SSRF",
			Remediation:     `Set metadata_options { http_tokens = "required" }.`,
		})
	}

	// EBS encryption in block device mappings
	blockDevices := getList(r.Attributes, "block_device_mappings")
	for _, bd := range blockDevices {
		bdMap, ok := bd.(map[string]any)
		if !ok {
			continue
		}
		ebsList := getList(bdMap, "ebs")
		for _, ebs := range ebsList {
			ebsMap, ok := ebs.(map[string]any)
			if !ok {
				continue
			}
			if encrypted, _ := ebsMap["encrypted"].(string); encrypted != "true" {
				findings = append(findings, Finding{
					ResourceAddress: r.Address,
					ResourceType:    r.Type,
					Severity:        SeverityHigh,
					Code:            "LT002",
					Title:           "EBS volume in launch template not encrypted",
					Detail:          "Block device mapping has unencrypted EBS volume",
					Remediation:     "Set encrypted = true in ebs block of block_device_mappings.",
				})
				break
			}
		}
	}

	return findings
}

// ── Secrets Manager ─────────────────────────────────────────────────────────

func checkSecretsManager(r *parser.Resource) []Finding {
	var findings []Finding

	// Recovery window
	recoveryDays := getInt(r.Attributes, "recovery_window_in_days")
	if recoveryDays == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "SM001",
			Title:           "Default recovery window",
			Detail:          "recovery_window_in_days not set — defaults to 30 days, which may be too long or too short",
			Remediation:     "Set recovery_window_in_days explicitly based on your recovery requirements.",
		})
	}

	// KMS encryption
	kmsKey := getString(r.Attributes, "kms_key_id")
	if kmsKey == "" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "SM002",
			Title:           "Using AWS-managed encryption key",
			Detail:          "No kms_key_id — secret encrypted with default aws/secretsmanager key",
			Remediation:     "Set kms_key_id to a customer-managed KMS key for cross-account access and key policy control.",
		})
	}

	return findings
}

// ── Kinesis Stream ──────────────────────────────────────────────────────────

func checkKinesis(r *parser.Resource) []Finding {
	var findings []Finding

	// Encryption
	encType := getString(r.Attributes, "encryption_type")
	if encType != "KMS" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "KIN001",
			Title:           "Stream not encrypted with KMS",
			Detail:          fmt.Sprintf("encryption_type = %q — data in stream is not encrypted at rest", encType),
			Remediation:     `Set encryption_type = "KMS" and provide a kms_key_id.`,
		})
	}

	return findings
}

// ── MSK (Managed Kafka) ────────────────────────────────────────────────────

func checkMSK(r *parser.Resource) []Finding {
	var findings []Finding

	// Encryption in transit
	encInTransit := getList(r.Attributes, "encryption_info")
	transitTLS := false
	atRestEnabled := false
	for _, ei := range encInTransit {
		eiMap, ok := ei.(map[string]any)
		if !ok {
			continue
		}
		transit := getList(eiMap, "encryption_in_transit")
		for _, t := range transit {
			tMap, ok := t.(map[string]any)
			if !ok {
				continue
			}
			if getString(tMap, "client_broker") == "TLS" {
				transitTLS = true
			}
			if inCluster, _ := tMap["in_cluster"].(bool); inCluster {
				transitTLS = true
			}
		}
		if _, hasKey := eiMap["encryption_at_rest_kms_key_arn"]; hasKey {
			atRestEnabled = true
		}
	}

	if !transitTLS {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "MSK001",
			Title:           "Kafka traffic not encrypted in transit",
			Detail:          "Client-broker communication not using TLS — data transmitted in plaintext",
			Remediation:     `Set encryption_in_transit { client_broker = "TLS" } in encryption_info.`,
		})
	}

	if !atRestEnabled {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "MSK002",
			Title:           "Kafka data not encrypted at rest with KMS",
			Detail:          "No encryption_at_rest_kms_key_arn — using AWS-managed encryption only",
			Remediation:     "Set encryption_at_rest_kms_key_arn in encryption_info for customer-managed encryption.",
		})
	}

	// Logging
	loggingInfo := getList(r.Attributes, "logging_info")
	if len(loggingInfo) == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "MSK003",
			Title:           "Broker logging not configured",
			Detail:          "No logging_info — broker logs not shipped to CloudWatch, S3, or Firehose",
			Remediation:     "Configure logging_info with broker_logs destinations for audit and troubleshooting.",
		})
	}

	return findings
}

// ── DocumentDB Cluster ──────────────────────────────────────────────────────

func checkDocDB(r *parser.Resource) []Finding {
	var findings []Finding

	if encrypted, _ := r.Attributes["storage_encrypted"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "DOC001",
			Title:           "DocumentDB cluster not encrypted",
			Detail:          "storage_encrypted = false — document data stored unencrypted at rest",
			Remediation:     "Set storage_encrypted = true. Must be set at cluster creation time.",
		})
	}

	if delProt, ok := r.Attributes["deletion_protection"]; !ok || delProt == false {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "DOC002",
			Title:           "Deletion protection disabled",
			Detail:          "deletion_protection not enabled — cluster can be accidentally destroyed",
			Remediation:     "Set deletion_protection = true for production clusters.",
		})
	}

	// Audit logging
	enabledLogs := getStringList(r.Attributes, "enabled_cloudwatch_logs_exports")
	hasAudit := false
	for _, l := range enabledLogs {
		if l == "audit" {
			hasAudit = true
		}
	}
	if !hasAudit {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "DOC003",
			Title:           "Audit logging not enabled",
			Detail:          "enabled_cloudwatch_logs_exports does not include 'audit'",
			Remediation:     `Add "audit" to enabled_cloudwatch_logs_exports for database activity monitoring.`,
		})
	}

	return findings
}

// ── Neptune Cluster ─────────────────────────────────────────────────────────

func checkNeptune(r *parser.Resource) []Finding {
	var findings []Finding

	if encrypted, _ := r.Attributes["storage_encrypted"].(bool); !encrypted {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityHigh,
			Code:            "NEP001",
			Title:           "Neptune cluster not encrypted",
			Detail:          "storage_encrypted = false — graph data stored unencrypted",
			Remediation:     "Set storage_encrypted = true. Must be set at creation time.",
		})
	}

	if delProt, ok := r.Attributes["deletion_protection"]; !ok || delProt == false {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "NEP002",
			Title:           "Deletion protection disabled",
			Detail:          "deletion_protection not enabled — cluster can be accidentally destroyed",
			Remediation:     "Set deletion_protection = true for production clusters.",
		})
	}

	// IAM authentication
	if iamAuth, _ := r.Attributes["iam_database_authentication_enabled"].(bool); !iamAuth {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "NEP003",
			Title:           "IAM database authentication not enabled",
			Detail:          "iam_database_authentication_enabled = false — using password-only auth",
			Remediation:     "Enable IAM authentication for token-based, credential-free access.",
		})
	}

	return findings
}

// ── CodeBuild Project ───────────────────────────────────────────────────────

func checkCodeBuild(r *parser.Resource) []Finding {
	var findings []Finding

	// Privileged mode
	env := getList(r.Attributes, "environment")
	for _, e := range env {
		eMap, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if priv, _ := eMap["privileged_mode"].(bool); priv {
			findings = append(findings, Finding{
				ResourceAddress: r.Address,
				ResourceType:    r.Type,
				Severity:        SeverityHigh,
				Code:            "CB001",
				Title:           "Privileged mode enabled",
				Detail:          "privileged_mode = true — build container has elevated Docker daemon access",
				Remediation:     "Disable privileged_mode unless building Docker images. Use buildx or kaniko instead.",
			})
		}
	}

	// Encryption
	encKey := getString(r.Attributes, "encryption_key")
	if encKey == "" {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityInfo,
			Code:            "CB002",
			Title:           "Using AWS-managed encryption key",
			Detail:          "No encryption_key — build artifacts encrypted with default AWS key",
			Remediation:     "Set encryption_key to a customer-managed KMS key for stricter access control.",
		})
	}

	// Logging
	logsConfig := getList(r.Attributes, "logs_config")
	if len(logsConfig) == 0 {
		findings = append(findings, Finding{
			ResourceAddress: r.Address,
			ResourceType:    r.Type,
			Severity:        SeverityMedium,
			Code:            "CB003",
			Title:           "Build logging not configured",
			Detail:          "No logs_config — build output logs may not be persisted",
			Remediation:     "Configure logs_config with CloudWatch Logs or S3 for build audit trail.",
		})
	}

	return findings
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func getString(attrs map[string]any, key string) string {
	v, _ := attrs[key].(string)
	return v
}

func getInt(attrs map[string]any, key string) int {
	switch v := attrs[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return 0
}

func getList(attrs map[string]any, key string) []any {
	v, _ := attrs[key].([]any)
	return v
}

func getStringList(attrs map[string]any, key string) []string {
	list := getList(attrs, key)
	var result []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func isWebPort(from, to int) bool {
	webPorts := []int{80, 443, 8080, 8443}
	for _, p := range webPorts {
		if from == p && to == p {
			return true
		}
	}
	return false
}

func containsSecretKeyword(key string) bool {
	keywords := []string{"password", "secret", "token", "key", "pwd", "credential", "api_key", "auth"}
	for _, kw := range keywords {
		if strings.Contains(key, kw) {
			return true
		}
	}
	return false
}

func sortFindings(findings []Finding) {
	// Simple insertion sort by severity weight descending
	for i := 1; i < len(findings); i++ {
		for j := i; j > 0 && findings[j].Severity.Weight() > findings[j-1].Severity.Weight(); j-- {
			findings[j], findings[j-1] = findings[j-1], findings[j]
		}
	}
}