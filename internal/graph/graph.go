// Package graph builds the node/edge model from parsed Terraform resources.
// It applies AWS-aware grouping logic (VPC → Subnet → AZ → Instance)
// and assigns categories for colour coding.
package graph

import (
	"strings"

	"github.com/hack-monk/tf-lens/internal/parser"
)

// Category maps to a colour in the diagram.
type Category string

const (
	CategoryNetworking  Category = "networking"
	CategoryCompute     Category = "compute"
	CategoryStorage     Category = "storage"
	CategorySecurity    Category = "security"
	CategoryMessaging   Category = "messaging"
	CategoryUnknown     Category = "unknown"
)

// ChangeType is set by the diff package.
type ChangeType string

const (
	ChangeNone    ChangeType = ""
	ChangeAdded   ChangeType = "added"
	ChangeRemoved ChangeType = "removed"
	ChangeUpdated ChangeType = "updated"
)

// NodeDriftChange describes a single attribute that drifted from Terraform state.
type NodeDriftChange struct {
	Path     string // Attribute path, e.g. "tags.Name"
	Expected string // Value in Terraform state
	Actual   string // Value observed in cloud
}

// NodeFinding is a lightweight copy of a threat finding attached to a node.
// Defined here to avoid an import cycle between graph and threat.
type NodeFinding struct {
	Code        string // e.g. "SG001"
	Severity    string // "critical" | "high" | "medium" | "info"
	Title       string // Short human-readable description
	Detail      string // What was found and why it matters
	Remediation string // Concrete fix suggestion
}

// Node represents a single resource in the diagram.
type Node struct {
	ID           string            // Unique ID — same as resource Address
	Label        string            // Display label, e.g. "web (aws_instance)"
	Type         string            // Terraform resource type
	Name         string            // Resource name
	Provider     string            // "aws", "azurerm", etc.
	Category     Category          // For colour coding
	Attributes   map[string]any    // Full attribute map from parser
	Tags         map[string]string // Extracted tags
	ParentID     string            // ID of the compound (container) node, if any
	Module       string            // Terraform module path
	ChangeType   ChangeType        // Set by diff package
	// Threat fields — populated by threat.AnnotateGraph()
	ThreatCodes       []string       // e.g. ["SG003", "RDS001"]
	ThreatMaxSeverity string         // "critical" | "high" | "medium" | "info"
	ThreatFindings    []NodeFinding  // Full finding details for the detail panel
	// Cost fields — populated by cost.AnnotateGraph()
	MonthlyCost float64 // Estimated monthly cost in USD
	HourlyCost  float64 // Estimated hourly cost in USD
	// Drift fields — populated by drift.AnnotateGraph()
	DriftStatus  string            // "update", "delete", "create", or "" (no drift)
	DriftChanges []NodeDriftChange // Per-attribute changes
}

// Edge represents a dependency between two nodes.
type Edge struct {
	ID     string
	Source string
	Target string
	Label  string // Relationship label, e.g. "in subnet", "uses"
}

// FlowEdge represents an inferred runtime traffic/data flow path.
type FlowEdge struct {
	ID     string
	Source string
	Target string
	Label  string // e.g. "HTTPS", "queries", "triggers"
	Kind   string // "ingress", "data", "event", "internal"
}

// Graph is the complete node/edge model passed to the renderer.
type Graph struct {
	Nodes     []*Node
	Edges     []*Edge
	FlowEdges []*FlowEdge // populated by flow.AnnotateGraph()
}

// Build creates a Graph from a slice of parsed resources.
func Build(resources []parser.Resource) *Graph {
	g := &Graph{}

	nodeIndex := make(map[string]*Node, len(resources))

	// --- Pass 1: create all nodes -------------------------------------------
	for i := range resources {
		r := &resources[i]
		n := &Node{
			ID:         r.Address,
			Label:      r.Name + " (" + r.Type + ")",
			Type:       r.Type,
			Name:       r.Name,
			Provider:   r.Provider,
			Category:   classifyCategory(r.Type),
			Attributes: r.Attributes,
			Tags:       r.Tags,
			Module:     r.Module,
		}
		g.Nodes = append(g.Nodes, n)
		nodeIndex[n.ID] = n
	}

	// --- Pass 2: build edges from dependency data ---------------------------
	edgeSet := make(map[string]bool)
	for i := range resources {
		r := &resources[i]
		for _, dep := range r.Dependencies {
			edgeID := r.Address + "->" + dep
			if !edgeSet[edgeID] {
				edgeSet[edgeID] = true
				depType := ""
				if n, ok := nodeIndex[dep]; ok {
					depType = n.Type
				}
				g.Edges = append(g.Edges, &Edge{
					ID:     edgeID,
					Source: r.Address,
					Target: dep,
					Label:  edgeLabel(r.Type, depType),
				})
			}
		}
	}

	// --- Pass 3: apply module grouping (synthetic parent nodes) ------------
	applyModuleGrouping(g, nodeIndex)

	// --- Pass 4: apply container nesting (VPC → Subnet) --------------------
	applyNesting(g, nodeIndex)

	return g
}

// ---- module grouping --------------------------------------------------------

// applyModuleGrouping creates synthetic parent nodes for Terraform modules
// and sets ParentID on resources that belong to a module. Nested modules
// (e.g. module.vpc.module.subnets) produce a hierarchy of parent nodes.
func applyModuleGrouping(g *Graph, nodeIndex map[string]*Node) {
	// Collect all unique module paths
	modulePaths := make(map[string]bool)
	for _, n := range g.Nodes {
		if n.Module == "" {
			continue
		}
		// For nested modules, register each ancestor too.
		// e.g. "module.vpc.module.subnets" → ["module.vpc", "module.vpc.module.subnets"]
		parts := strings.Split(n.Module, ".")
		for i := 1; i < len(parts); i += 2 {
			ancestor := strings.Join(parts[:i+1], ".")
			modulePaths[ancestor] = true
		}
	}

	if len(modulePaths) == 0 {
		return
	}

	// Create synthetic nodes for each module path
	for modPath := range modulePaths {
		if _, exists := nodeIndex[modPath]; exists {
			continue // don't clobber real resources
		}

		// Extract short name: "module.vpc" → "vpc"
		parts := strings.Split(modPath, ".")
		shortName := parts[len(parts)-1]

		n := &Node{
			ID:       modPath,
			Label:    shortName,
			Type:     "module",
			Name:     shortName,
			Category: CategoryUnknown,
			Module:   parentModulePath(modPath),
		}
		g.Nodes = append(g.Nodes, n)
		nodeIndex[modPath] = n
	}

	// Set ParentID for module nodes (nested modules)
	for modPath := range modulePaths {
		n := nodeIndex[modPath]
		parent := parentModulePath(modPath)
		if parent != "" {
			if _, exists := nodeIndex[parent]; exists {
				n.ParentID = parent
			}
		}
	}

	// Set ParentID for resource nodes that belong to modules,
	// but only if they don't already have a parent from nesting rules.
	for _, n := range g.Nodes {
		if n.Module == "" || n.Type == "module" {
			continue
		}
		// Don't override existing parent (nesting runs after this, so ParentID is empty here)
		if n.ParentID == "" {
			if _, exists := nodeIndex[n.Module]; exists {
				n.ParentID = n.Module
			}
		}
	}
}

// parentModulePath returns the parent module path.
// "module.vpc.module.subnets" → "module.vpc"
// "module.vpc" → ""
func parentModulePath(modPath string) string {
	parts := strings.Split(modPath, ".")
	if len(parts) <= 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-2], ".")
}

// ---- nesting logic ----------------------------------------------------------

// nestingRules defines which resource types act as containers for others.
// Order matters: more specific rules should come first.
var nestingRules = []struct {
	container  string // resource type of the parent
	child      string // resource type of the child
	attrLink   string // attribute on the child that holds the parent's ID
	parentAttr string // attribute on the parent that is referenced
}{
	// Subnet-level nesting
	{"aws_subnet", "aws_instance", "subnet_id", "id"},
	{"aws_subnet", "aws_lambda_function", "subnet_ids", "id"},
	{"aws_subnet", "aws_ecs_service", "network_configuration.0.subnets", "id"},
	{"aws_subnet", "aws_nat_gateway", "subnet_id", "id"},
	{"aws_subnet", "aws_db_instance", "db_subnet_group_name", "id"},
	{"aws_subnet", "aws_elasticache_cluster", "subnet_group_name", "id"},

	// VPC-level nesting
	{"aws_vpc", "aws_subnet", "vpc_id", "id"},
	{"aws_vpc", "aws_internet_gateway", "vpc_id", "id"},
	{"aws_vpc", "aws_security_group", "vpc_id", "id"},
	{"aws_vpc", "aws_alb", "subnets", "id"},
	{"aws_vpc", "aws_lb", "subnets", "id"},
	{"aws_vpc", "aws_eks_cluster", "vpc_config.0.subnet_ids", "id"},
	{"aws_vpc", "aws_lb_target_group", "vpc_id", "id"},
	{"aws_vpc", "aws_route_table", "vpc_id", "id"},
	{"aws_vpc", "aws_db_subnet_group", "vpc_id", "id"},

	// ECS cluster → service
	{"aws_ecs_cluster", "aws_ecs_service", "cluster", "id"},
}

// applyNesting sets Node.ParentID for resources that live inside a container.
func applyNesting(g *Graph, nodeIndex map[string]*Node) {
	// Build a lookup: resource type → []*Node
	byType := make(map[string][]*Node)
	for _, n := range g.Nodes {
		byType[n.Type] = append(byType[n.Type], n)
	}

	for _, rule := range nestingRules {
		containers := byType[rule.container]
		if len(containers) == 0 {
			continue
		}

		// Build a map: parentAttrValue → containerNode
		parentByValue := make(map[string]*Node)
		for _, c := range containers {
			if v, ok := attrString(c.Attributes, rule.parentAttr); ok {
				parentByValue[v] = c
			}
		}

		// For each child type, check if its attrLink matches a container.
		// Infrastructure nesting (VPC→Subnet) takes priority over module grouping,
		// so we allow overriding a module-based parent.
		for _, child := range byType[rule.child] {
			if child.ParentID != "" && nodeIndex[child.ParentID] != nil && nodeIndex[child.ParentID].Type != "module" {
				continue // already nested by infrastructure rule
			}
			vals := attrStringList(child.Attributes, rule.attrLink)
			for _, v := range vals {
				if parent, ok := parentByValue[v]; ok {
					child.ParentID = parent.ID
					break
				}
			}
		}
	}
}

// ---- category classification ------------------------------------------------

var categoryPrefixes = map[string]Category{
	"aws_vpc":                 CategoryNetworking,
	"aws_subnet":              CategoryNetworking,
	"aws_internet_gateway":    CategoryNetworking,
	"aws_nat_gateway":         CategoryNetworking,
	"aws_alb":                 CategoryNetworking,
	"aws_lb":                  CategoryNetworking,
	"aws_route53":             CategoryNetworking,
	"aws_instance":            CategoryCompute,
	"aws_lambda":              CategoryCompute,
	"aws_ecs":                 CategoryCompute,
	"aws_eks":                 CategoryCompute,
	"aws_autoscaling":         CategoryCompute,
	"aws_cloudfront":          CategoryCompute,
	"aws_s3":                  CategoryStorage,
	"aws_db":                  CategoryStorage,
	"aws_dynamodb":            CategoryStorage,
	"aws_elasticache":         CategoryStorage,
	"aws_ebs":                 CategoryStorage,
	"aws_efs":                 CategoryStorage,
	"aws_security_group":      CategorySecurity,
	"aws_iam":                 CategorySecurity,
	"aws_kms":                 CategorySecurity,
	"aws_secretsmanager":      CategorySecurity,
	"aws_wafv2":               CategorySecurity,
	"aws_ecr":                 CategoryCompute,
	"aws_codebuild":           CategoryCompute,
	"aws_launch_template":     CategoryCompute,
	"aws_redshift":            CategoryStorage,
	"aws_rds_cluster":         CategoryStorage,
	"aws_opensearch":          CategoryStorage,
	"aws_elasticsearch":       CategoryStorage,
	"aws_docdb":               CategoryStorage,
	"aws_neptune":             CategoryStorage,
	"aws_cloudtrail":          CategorySecurity,
	"aws_msk":                 CategoryMessaging,
	"aws_kinesis":             CategoryMessaging,
	"aws_sns":                 CategoryMessaging,
	"aws_sqs":                 CategoryMessaging,
	"aws_api_gateway":         CategoryMessaging,
	"aws_sfn":                 CategoryMessaging,
	"aws_cloudwatch":          CategoryMessaging,
}

// classifyCategory returns the Category for a given resource type.
func classifyCategory(resourceType string) Category {
	// Exact match first
	if c, ok := categoryPrefixes[resourceType]; ok {
		return c
	}
	// Prefix match: walk from longest prefix to shortest
	for prefix, cat := range categoryPrefixes {
		if strings.HasPrefix(resourceType, prefix) {
			return cat
		}
	}
	return CategoryUnknown
}

// ---- edge labels ------------------------------------------------------------

// edgeLabelRules maps (source type, target type) → label.
// Order: checked first-match, so put specific rules before broad ones.
var edgeLabelRules = []struct {
	source string // prefix match on source type
	target string // prefix match on target type
	label  string
}{
	// Networking
	{"aws_instance", "aws_subnet", "in subnet"},
	{"aws_instance", "aws_security_group", "attached"},
	{"aws_instance", "aws_iam", "assumes"},
	{"aws_subnet", "aws_vpc", "in VPC"},
	{"aws_subnet", "aws_route_table", "routed by"},
	{"aws_internet_gateway", "aws_vpc", "attached"},
	{"aws_nat_gateway", "aws_subnet", "in subnet"},
	{"aws_nat_gateway", "aws_eip", "uses EIP"},
	{"aws_route_table", "aws_vpc", "in VPC"},
	{"aws_route", "aws_internet_gateway", "via IGW"},
	{"aws_route", "aws_nat_gateway", "via NAT"},

	// Load balancing
	{"aws_alb", "aws_subnet", "in subnet"},
	{"aws_alb", "aws_security_group", "attached"},
	{"aws_lb", "aws_subnet", "in subnet"},
	{"aws_lb", "aws_security_group", "attached"},
	{"aws_lb_target_group", "aws_vpc", "in VPC"},
	{"aws_lb_listener", "aws_lb", "listens on"},
	{"aws_lb_target_group_attachment", "aws_instance", "targets"},

	// Compute
	{"aws_lambda_function", "aws_iam_role", "assumes"},
	{"aws_lambda_function", "aws_subnet", "in subnet"},
	{"aws_lambda_function", "aws_security_group", "attached"},
	{"aws_lambda_function", "aws_sqs_queue", "triggered by"},
	{"aws_lambda_function", "aws_sns_topic", "subscribed"},
	{"aws_ecs_service", "aws_ecs_cluster", "runs in"},
	{"aws_ecs_service", "aws_ecs_task_definition", "runs task"},
	{"aws_ecs_service", "aws_lb_target_group", "registered"},
	{"aws_ecs_task_definition", "aws_iam_role", "assumes"},
	{"aws_eks_cluster", "aws_subnet", "in subnet"},
	{"aws_eks_cluster", "aws_iam_role", "assumes"},

	// Storage
	{"aws_db_instance", "aws_db_subnet_group", "in subnet group"},
	{"aws_db_instance", "aws_security_group", "attached"},
	{"aws_db_instance", "aws_kms_key", "encrypted by"},
	{"aws_s3_bucket", "aws_kms_key", "encrypted by"},

	// Security
	{"aws_security_group", "aws_vpc", "in VPC"},
	{"aws_iam_role_policy", "aws_iam_role", "attached"},
	{"aws_iam_role_policy_attachment", "aws_iam_role", "attached"},

	// Messaging
	{"aws_sns_topic_subscription", "aws_sns_topic", "subscribes"},
	{"aws_sqs_queue_policy", "aws_sqs_queue", "policy for"},

	// Databases
	{"aws_docdb_cluster", "aws_db_subnet_group", "in subnet group"},
	{"aws_docdb_cluster", "aws_kms_key", "encrypted by"},
	{"aws_neptune_cluster", "aws_db_subnet_group", "in subnet group"},
	{"aws_neptune_cluster", "aws_kms_key", "encrypted by"},
	{"aws_redshift_cluster", "aws_iam_role", "assumes"},
	{"aws_redshift_cluster", "aws_kms_key", "encrypted by"},

	// CI/CD
	{"aws_codebuild_project", "aws_iam_role", "assumes"},
	{"aws_codebuild_project", "aws_s3_bucket", "artifacts in"},
	{"aws_codebuild_project", "aws_vpc", "in VPC"},

	// Streaming
	{"aws_kinesis_stream", "aws_kms_key", "encrypted by"},
	{"aws_msk_cluster", "aws_subnet", "in subnet"},
	{"aws_msk_cluster", "aws_security_group", "attached"},
	{"aws_msk_cluster", "aws_kms_key", "encrypted by"},

	// Monitoring
	{"aws_cloudwatch_metric_alarm", "aws_sns_topic", "notifies"},
	{"aws_cloudtrail", "aws_s3_bucket", "logs to"},
	{"aws_cloudtrail", "aws_kms_key", "encrypted by"},
	{"aws_cloudtrail", "aws_cloudwatch_log_group", "streams to"},

	// Catch-all patterns by target type
	{"", "aws_iam_role", "uses role"},
	{"", "aws_security_group", "uses SG"},
	{"", "aws_kms_key", "uses key"},
	{"", "aws_subnet", "in subnet"},
	{"", "aws_vpc", "in VPC"},
}

// edgeLabel returns a human-readable label for an edge between two resource types.
func edgeLabel(sourceType, targetType string) string {
	if sourceType == "" || targetType == "" {
		return ""
	}
	for _, rule := range edgeLabelRules {
		sourceMatch := rule.source == "" || strings.HasPrefix(sourceType, rule.source)
		targetMatch := strings.HasPrefix(targetType, rule.target)
		if sourceMatch && targetMatch {
			return rule.label
		}
	}
	return ""
}

// ---- attribute helpers ------------------------------------------------------

// attrString extracts a string value from a nested attribute path (dot-separated).
func attrString(attrs map[string]any, path string) (string, bool) {
	parts := strings.SplitN(path, ".", 2)
	v, ok := attrs[parts[0]]
	if !ok {
		return "", false
	}
	if len(parts) == 1 {
		s, ok := v.(string)
		return s, ok
	}
	// Recurse into nested map
	if sub, ok := v.(map[string]any); ok {
		return attrString(sub, parts[1])
	}
	return "", false
}

// attrStringList extracts one or more string values from an attribute that may
// be a string or a []any of strings (e.g. subnet_ids).
func attrStringList(attrs map[string]any, path string) []string {
	parts := strings.SplitN(path, ".", 2)
	v, ok := attrs[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) > 1 {
		if sub, ok := v.(map[string]any); ok {
			return attrStringList(sub, parts[1])
		}
		return nil
	}
	switch val := v.(type) {
	case string:
		return []string{val}
	case []any:
		var out []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}