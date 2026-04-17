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
}

// Edge represents a dependency between two nodes.
type Edge struct {
	ID     string
	Source string
	Target string
}

// Graph is the complete node/edge model passed to the renderer.
type Graph struct {
	Nodes []*Node
	Edges []*Edge
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
				g.Edges = append(g.Edges, &Edge{
					ID:     edgeID,
					Source: r.Address,
					Target: dep,
				})
			}
		}
	}

	// --- Pass 3: apply container nesting (VPC → Subnet) --------------------
	applyNesting(g, nodeIndex)

	return g
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
	{"aws_subnet", "aws_instance", "subnet_id", "id"},
	{"aws_subnet", "aws_lambda_function", "subnet_ids", "id"}, // lambda uses a list
	{"aws_subnet", "aws_ecs_service", "network_configuration.0.subnets", "id"},
	{"aws_vpc", "aws_subnet", "vpc_id", "id"},
	{"aws_vpc", "aws_internet_gateway", "vpc_id", "id"},
	{"aws_vpc", "aws_nat_gateway", "subnet_id", "id"}, // nat is in a subnet in a vpc; handled transitively
	{"aws_vpc", "aws_security_group", "vpc_id", "id"},
	{"aws_vpc", "aws_alb", "subnets", "id"}, // ALB spans multiple subnets
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

		// For each child type, check if its attrLink matches a container
		for _, child := range byType[rule.child] {
			if child.ParentID != "" {
				continue // already nested
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
	"aws_sns":                 CategoryMessaging,
	"aws_sqs":                 CategoryMessaging,
	"aws_api_gateway":         CategoryMessaging,
	"aws_kinesis":             CategoryMessaging,
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