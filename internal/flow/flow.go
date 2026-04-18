// Package flow infers traffic and data flow paths through infrastructure.
// Unlike Terraform dependency edges (which show resource creation order),
// flow edges show how requests, events, and data move at runtime:
//
//	Internet → CloudFront → ALB → EC2 → RDS
//	User → API Gateway → Lambda → DynamoDB
//	Events → SQS → Lambda → S3
//
// Flow is inferred from resource types, attributes, and connectivity patterns.
package flow

import (
	"strings"

	"github.com/hack-monk/tf-lens/internal/graph"
)

// Edge represents a single inferred traffic/data flow path.
type Edge struct {
	ID     string
	Source string // node ID (resource address)
	Target string // node ID (resource address)
	Label  string // e.g. "HTTPS", "events", "queries"
	Kind   string // "ingress", "data", "event", "internal"
}

// Infer analyses the graph and returns flow edges showing runtime traffic paths.
func Infer(g *graph.Graph) []Edge {
	nodeByID := make(map[string]*graph.Node, len(g.Nodes))
	byType := make(map[string][]*graph.Node)
	for _, n := range g.Nodes {
		nodeByID[n.ID] = n
		byType[n.Type] = append(byType[n.Type], n)
	}

	var edges []Edge
	edgeSet := make(map[string]bool)

	addEdge := func(src, tgt, label, kind string) {
		key := src + "→" + tgt
		if edgeSet[key] || src == tgt {
			return
		}
		// Both nodes must exist
		if _, ok := nodeByID[src]; !ok {
			return
		}
		if _, ok := nodeByID[tgt]; !ok {
			return
		}
		edgeSet[key] = true
		edges = append(edges, Edge{
			ID:     "flow:" + src + "→" + tgt,
			Source: src,
			Target: tgt,
			Label:  label,
			Kind:   kind,
		})
	}

	// ── Rule-based flow inference ──────────────────────────────────────────

	// 1. ALB/NLB → target instances/ECS services (via target group or subnet co-location)
	for _, lb := range append(byType["aws_alb"], byType["aws_lb"]...) {
		// LB → instances in same subnets
		lbSubnets := attrStringList(lb.Attributes, "subnets")
		for _, inst := range byType["aws_instance"] {
			instSubnet := attrString(inst.Attributes, "subnet_id")
			if instSubnet != "" && containsStr(lbSubnets, instSubnet) {
				addEdge(lb.ID, inst.ID, "HTTP/HTTPS", "ingress")
			}
		}
		// LB → ECS services in same VPC
		for _, svc := range byType["aws_ecs_service"] {
			addEdge(lb.ID, svc.ID, "HTTP/HTTPS", "ingress")
		}
	}

	// 2. CloudFront → origin (S3 or ALB)
	for _, cf := range byType["aws_cloudfront_distribution"] {
		// CloudFront typically fronts ALBs or S3
		for _, lb := range append(byType["aws_alb"], byType["aws_lb"]...) {
			addEdge(cf.ID, lb.ID, "origin", "ingress")
		}
		for _, s3 := range byType["aws_s3_bucket"] {
			// Check if S3 is configured as website or has same-VPC connection
			if _, hasWebsite := s3.Attributes["website"]; hasWebsite {
				addEdge(cf.ID, s3.ID, "origin", "ingress")
			}
		}
	}

	// 3. API Gateway → Lambda (integration)
	for _, apigw := range append(byType["aws_api_gateway_rest_api"], byType["aws_apigatewayv2_api"]...) {
		for _, fn := range byType["aws_lambda_function"] {
			addEdge(apigw.ID, fn.ID, "invoke", "ingress")
		}
	}

	// 4. Lambda → downstream services (inferred from IAM + env vars)
	for _, fn := range byType["aws_lambda_function"] {
		envVars := extractEnvVars(fn.Attributes)

		// Lambda → DynamoDB
		for _, ddb := range byType["aws_dynamodb_table"] {
			if referencesResource(envVars, ddb) {
				addEdge(fn.ID, ddb.ID, "read/write", "data")
			}
		}
		// Lambda → S3
		for _, s3 := range byType["aws_s3_bucket"] {
			if referencesResource(envVars, s3) {
				addEdge(fn.ID, s3.ID, "read/write", "data")
			}
		}
		// Lambda → SQS (publish)
		for _, sqs := range byType["aws_sqs_queue"] {
			if referencesResource(envVars, sqs) {
				addEdge(fn.ID, sqs.ID, "send", "event")
			}
		}
		// Lambda → SNS (publish)
		for _, sns := range byType["aws_sns_topic"] {
			if referencesResource(envVars, sns) {
				addEdge(fn.ID, sns.ID, "publish", "event")
			}
		}
		// Lambda → RDS/Aurora
		for _, db := range append(byType["aws_db_instance"], byType["aws_rds_cluster"]...) {
			if sameVPCSubnets(fn, db) || referencesResource(envVars, db) {
				addEdge(fn.ID, db.ID, "queries", "data")
			}
		}
	}

	// 5. SQS → Lambda (event source)
	for _, sqs := range byType["aws_sqs_queue"] {
		for _, fn := range byType["aws_lambda_function"] {
			// Check if lambda has SQS as event source (via dependencies)
			if hasDependency(g, fn.ID, sqs.ID) {
				addEdge(sqs.ID, fn.ID, "triggers", "event")
			}
		}
	}

	// 6. SNS → SQS / Lambda (subscriptions)
	for _, sns := range byType["aws_sns_topic"] {
		for _, sqs := range byType["aws_sqs_queue"] {
			if hasDependency(g, sqs.ID, sns.ID) {
				addEdge(sns.ID, sqs.ID, "delivers", "event")
			}
		}
		for _, fn := range byType["aws_lambda_function"] {
			if hasDependency(g, fn.ID, sns.ID) {
				addEdge(sns.ID, fn.ID, "triggers", "event")
			}
		}
	}

	// 7. EC2/ECS → database tier (subnet co-location or dependency)
	for _, inst := range append(byType["aws_instance"], byType["aws_ecs_service"]...) {
		for _, db := range append(byType["aws_db_instance"], byType["aws_rds_cluster"]...) {
			if sameVPCSubnets(inst, db) || hasDependency(g, inst.ID, db.ID) {
				addEdge(inst.ID, db.ID, "queries", "data")
			}
		}
		for _, cache := range append(byType["aws_elasticache_cluster"], byType["aws_elasticache_replication_group"]...) {
			if hasDependency(g, inst.ID, cache.ID) || sameVPCSubnets(inst, cache) {
				addEdge(inst.ID, cache.ID, "cache ops", "data")
			}
		}
		for _, ddb := range byType["aws_dynamodb_table"] {
			if hasDependency(g, inst.ID, ddb.ID) {
				addEdge(inst.ID, ddb.ID, "read/write", "data")
			}
		}
	}

	// 8. Kinesis → Lambda / Firehose
	for _, stream := range byType["aws_kinesis_stream"] {
		for _, fn := range byType["aws_lambda_function"] {
			if hasDependency(g, fn.ID, stream.ID) {
				addEdge(stream.ID, fn.ID, "stream", "event")
			}
		}
	}

	// 9. CloudTrail → S3 (log delivery)
	for _, ct := range byType["aws_cloudtrail"] {
		bucket := attrString(ct.Attributes, "s3_bucket_name")
		if bucket != "" {
			for _, s3 := range byType["aws_s3_bucket"] {
				if s3.Name == bucket || attrString(s3.Attributes, "bucket") == bucket {
					addEdge(ct.ID, s3.ID, "logs", "data")
				}
			}
		}
	}

	// 10. MSK → Lambda consumers
	for _, msk := range byType["aws_msk_cluster"] {
		for _, fn := range byType["aws_lambda_function"] {
			if hasDependency(g, fn.ID, msk.ID) {
				addEdge(msk.ID, fn.ID, "consume", "event")
			}
		}
	}

	return edges
}

// AnnotateGraph adds flow edges to the graph.
func AnnotateGraph(g *graph.Graph, flows []Edge) {
	for _, f := range flows {
		g.FlowEdges = append(g.FlowEdges, &graph.FlowEdge{
			ID:     f.ID,
			Source: f.Source,
			Target: f.Target,
			Label:  f.Label,
			Kind:   f.Kind,
		})
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func attrString(attrs map[string]any, key string) string {
	v, _ := attrs[key].(string)
	return v
}

func attrStringList(attrs map[string]any, key string) []string {
	list, _ := attrs[key].([]any)
	var result []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// extractEnvVars pulls environment variable keys and values from a Lambda function.
func extractEnvVars(attrs map[string]any) map[string]string {
	vars := make(map[string]string)
	env, _ := attrs["environment"].([]any)
	for _, e := range env {
		eMap, ok := e.(map[string]any)
		if !ok {
			continue
		}
		v, _ := eMap["variables"].(map[string]any)
		for k, val := range v {
			if s, ok := val.(string); ok {
				vars[k] = s
			}
		}
	}
	return vars
}

// referencesResource checks if env vars mention a resource by name or ID.
func referencesResource(envVars map[string]string, n *graph.Node) bool {
	for _, v := range envVars {
		lower := strings.ToLower(v)
		if strings.Contains(lower, strings.ToLower(n.Name)) {
			return true
		}
		// Check for resource ID in attribute values
		if id := attrString(n.Attributes, "id"); id != "" && strings.Contains(v, id) {
			return true
		}
		if arn := attrString(n.Attributes, "arn"); arn != "" && strings.Contains(v, arn) {
			return true
		}
	}
	return false
}

// sameVPCSubnets checks if two resources share a VPC via subnet attributes.
func sameVPCSubnets(a, b *graph.Node) bool {
	aVPC := attrString(a.Attributes, "vpc_id")
	bVPC := attrString(b.Attributes, "vpc_id")
	if aVPC != "" && bVPC != "" && aVPC == bVPC {
		return true
	}
	return false
}

// hasDependency checks if source depends on target in the graph.
func hasDependency(g *graph.Graph, sourceID, targetID string) bool {
	for _, e := range g.Edges {
		if e.Source == sourceID && e.Target == targetID {
			return true
		}
	}
	return false
}
