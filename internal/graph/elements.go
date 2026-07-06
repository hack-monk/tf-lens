package graph

import "strings"

// ── Abbreviations ─────────────────────────────────────────────────────────────

// AbbrevMap maps Terraform resource types to short display labels.
var AbbrevMap = map[string]string{
	"aws_vpc": "VPC", "aws_subnet": "SN", "aws_internet_gateway": "IGW",
	"aws_nat_gateway": "NAT", "aws_alb": "ALB", "aws_lb": "ALB",
	"aws_route53_zone": "R53", "aws_instance": "EC2", "aws_lambda_function": "λ",
	"aws_ecs_service": "ECS", "aws_eks_cluster": "EKS", "aws_autoscaling_group": "ASG",
	"aws_cloudfront_distribution": "CF", "aws_s3_bucket": "S3", "aws_db_instance": "RDS",
	"aws_dynamodb_table": "DDB", "aws_elasticache_cluster": "EC", "aws_ebs_volume": "EBS",
	"aws_efs_file_system": "EFS", "aws_security_group": "SG", "aws_iam_role": "IAM",
	"aws_kms_key": "KMS", "aws_secretsmanager_secret": "SM", "aws_sns_topic": "SNS",
	"aws_sqs_queue": "SQS", "aws_api_gateway_rest_api": "API", "aws_cloudwatch_log_group": "CW",
	"aws_rds_cluster": "RDS", "aws_ecr_repository": "ECR", "aws_redshift_cluster": "RS",
	"aws_opensearch_domain": "OS", "aws_elasticsearch_domain": "ES",
	"aws_ecs_task_definition": "ECS", "aws_ecs_cluster": "ECS",
	"aws_api_gateway_stage": "API", "aws_apigatewayv2_api": "API", "aws_apigatewayv2_stage": "API",
	"aws_cloudtrail": "CT", "aws_launch_template": "LT", "aws_kinesis_stream": "KDS",
	"aws_msk_cluster": "MSK", "aws_docdb_cluster": "DOC", "aws_neptune_cluster": "NEP",
	"aws_codebuild_project": "CB",
	"module": "MOD",
}

// Abbrev returns a short display abbreviation for a Terraform resource type.
func Abbrev(t string) string {
	if a, ok := AbbrevMap[t]; ok {
		return a
	}
	p := strings.Split(t, "_")
	last := p[len(p)-1]
	if len(last) > 3 {
		last = last[:3]
	}
	return strings.ToUpper(last)
}

// ── Wire types (Cytoscape.js element format) ──────────────────────────────────

// Element is one Cytoscape.js node or edge, as serialised to JSON.
type Element struct {
	Group string      `json:"group"`
	Data  interface{} `json:"data"`
}

// NodeData is the data payload for a node Element.
type NodeData struct {
	ID             string        `json:"id"`
	Label          string        `json:"label"`
	Parent         string        `json:"parent,omitempty"`
	Type           string        `json:"type"`
	Category       string        `json:"category"`
	ChangeType     string        `json:"changeType,omitempty"`
	Abbrev         string        `json:"abbrev"`
	IsParent       bool          `json:"isParent"`
	ThreatSeverity string        `json:"threatSeverity,omitempty"`
	ThreatCodes    []string      `json:"threatCodes,omitempty"`
	ThreatFindings []FindingData `json:"threatFindings,omitempty"`
	MonthlyCost    float64       `json:"monthlyCost,omitempty"`
	DriftStatus    string        `json:"driftStatus,omitempty"`
	DriftChanges   []DriftChangeData `json:"driftChanges,omitempty"`
}

// EdgeData is the data payload for an edge Element.
type EdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
	Flow   bool   `json:"flow,omitempty"`
	Kind   string `json:"flowKind,omitempty"`
}

// FindingData is a lightweight threat finding for JSON serialisation.
type FindingData struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation"`
}

// DriftChangeData is a single drifted attribute for JSON serialisation.
type DriftChangeData struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

// ── BuildElements ─────────────────────────────────────────────────────────────

// BuildElements converts a Graph into the Cytoscape.js element slice used by
// both the HTML renderer and the serve-mode JSON API.
func BuildElements(g *Graph) []Element {
	parentIDs := map[string]bool{}
	for _, n := range g.Nodes {
		if n.ParentID != "" {
			parentIDs[n.ParentID] = true
		}
	}

	var elems []Element
	for _, n := range g.Nodes {
		elems = append(elems, Element{
			Group: "nodes",
			Data: NodeData{
				ID: n.ID, Label: n.Name, Parent: n.ParentID,
				Type: n.Type, Category: string(n.Category),
				ChangeType:     string(n.ChangeType),
				Abbrev:         Abbrev(n.Type),
				IsParent:       parentIDs[n.ID],
				ThreatSeverity: n.ThreatMaxSeverity,
				ThreatCodes:    n.ThreatCodes,
				ThreatFindings: toFindingData(n.ThreatFindings),
				MonthlyCost:    n.MonthlyCost,
				DriftStatus:    n.DriftStatus,
				DriftChanges:   toDriftChangeData(n.DriftChanges),
			},
		})
	}

	nodeIDs := map[string]bool{}
	for _, n := range g.Nodes {
		nodeIDs[n.ID] = true
	}
	for _, e := range g.Edges {
		if nodeIDs[e.Source] && nodeIDs[e.Target] {
			elems = append(elems, Element{
				Group: "edges",
				Data:  EdgeData{ID: e.ID, Source: e.Source, Target: e.Target, Label: e.Label},
			})
		}
	}
	for _, f := range g.FlowEdges {
		if nodeIDs[f.Source] && nodeIDs[f.Target] {
			elems = append(elems, Element{
				Group: "edges",
				Data:  EdgeData{ID: f.ID, Source: f.Source, Target: f.Target, Label: f.Label, Flow: true, Kind: f.Kind},
			})
		}
	}

	return elems
}

func toFindingData(nf []NodeFinding) []FindingData {
	if len(nf) == 0 {
		return nil
	}
	out := make([]FindingData, len(nf))
	for i, f := range nf {
		out[i] = FindingData{Code: f.Code, Severity: f.Severity, Title: f.Title, Detail: f.Detail, Remediation: f.Remediation}
	}
	return out
}

func toDriftChangeData(dc []NodeDriftChange) []DriftChangeData {
	if len(dc) == 0 {
		return nil
	}
	out := make([]DriftChangeData, len(dc))
	for i, c := range dc {
		out[i] = DriftChangeData{Path: c.Path, Expected: c.Expected, Actual: c.Actual}
	}
	return out
}
