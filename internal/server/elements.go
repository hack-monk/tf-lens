package server

import (
	"strings"

	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
)

// abbrevMap maps Terraform resource types to short display abbreviations.
var abbrevMap = map[string]string{
	"aws_vpc": "VPC", "aws_subnet": "SN", "aws_internet_gateway": "IGW",
	"aws_nat_gateway": "NAT", "aws_alb": "ALB", "aws_lb": "ALB",
	"aws_route53_zone": "R53", "aws_instance": "EC2", "aws_lambda_function": "λ",
	"aws_ecs_service": "ECS", "aws_eks_cluster": "EKS", "aws_autoscaling_group": "ASG",
	"aws_cloudfront_distribution": "CF", "aws_s3_bucket": "S3", "aws_db_instance": "RDS",
	"aws_dynamodb_table": "DDB", "aws_elasticache_cluster": "EC", "aws_ebs_volume": "EBS",
	"aws_efs_file_system": "EFS", "aws_security_group": "SG", "aws_iam_role": "IAM",
	"aws_kms_key": "KMS", "aws_secretsmanager_secret": "SM", "aws_sns_topic": "SNS",
	"aws_sqs_queue": "SQS", "aws_api_gateway_rest_api": "API", "aws_cloudwatch_log_group": "CW",
}

func getAbbrev(resourceType string) string {
	if a, ok := abbrevMap[resourceType]; ok {
		return a
	}
	parts := strings.Split(resourceType, "_")
	last := parts[len(parts)-1]
	if len(last) > 3 {
		last = last[:3]
	}
	return strings.ToUpper(last)
}

// buildElements converts a Graph into the element slice served as JSON.
func buildElements(g *graph.Graph) []element {
	// Mark compound (parent) nodes
	parentIDs := map[string]bool{}
	for _, n := range g.Nodes {
		if n.ParentID != "" {
			parentIDs[n.ParentID] = true
		}
	}

	var elems []element
	for _, n := range g.Nodes {
		elems = append(elems, element{
			Group: "nodes",
			Data: nodeData{
				ID: n.ID, Label: n.Name, Parent: n.ParentID,
				Type: n.Type, Category: string(n.Category),
				ChangeType: string(n.ChangeType),
				Abbrev:         getAbbrev(n.Type),
				IsParent:        parentIDs[n.ID],
				ThreatSeverity:  n.ThreatMaxSeverity,
				ThreatCodes:     n.ThreatCodes,
			},
		})
	}

	nodeIDs := map[string]bool{}
	for _, n := range g.Nodes {
		nodeIDs[n.ID] = true
	}
	for _, e := range g.Edges {
		if nodeIDs[e.Source] && nodeIDs[e.Target] {
			elems = append(elems, element{
				Group: "edges",
				Data:  edgeData{ID: e.ID, Source: e.Source, Target: e.Target},
			})
		}
	}
	return elems
}

// Unused in serve mode — icons are resolved client-side via CSS classes.
// Kept to satisfy the icons import path.
var _ = (*icons.Resolver)(nil)