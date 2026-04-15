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