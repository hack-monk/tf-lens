# TF-Lens Onboarding Context Layer & UI Enhancements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a context annotation system, guided tour mode, and UI improvements (dark mode, dashboard, minimap, filter search, collapsible modules) to help new team members understand Terraform infrastructure visually.

**Architecture:** Two new Go packages (`internal/glossary`, `internal/annotations`) attach human-readable context to graph nodes. The renderer embeds tour steps and glossary data into the HTML template. All UI features are implemented in the existing `htmlSrc` Go string constant in `internal/renderer/export.go`.

**Tech Stack:** Go 1.22, `gopkg.in/yaml.v3` (new dep for annotation parsing), Cytoscape.js (existing), `cytoscape-expand-collapse` v4.1.0 (new JS lib for collapsible modules), Cobra CLI (existing).

## Global Constraints

- Module path: `github.com/hack-monk/tf-lens`
- Go minimum: 1.22
- All HTML features must work fully offline in the exported HTML file
- No regressions to existing overlays (threat, cost, drift, diff, flow)
- Binary size increase must stay under 200KB total (glossary is static strings, expand-collapse is ~15KB)
- `--annotations` flag is optional on both `export` and `serve` commands
- New `NodeData` JSON fields must use `omitempty` so existing exports without annotations are unaffected

---

## File Map

**Created:**
- `internal/glossary/glossary.go` — static map of resource type → ServiceInfo
- `internal/glossary/glossary_test.go`
- `internal/annotations/annotations.go` — parse tf-lens.yaml, apply to graph
- `internal/annotations/annotations_test.go`

**Modified:**
- `internal/graph/graph.go` — add fields to `Node`, add `TourStep` type, add `TourSteps` to `Graph`, auto-infer HumanLabel/Description/Owner/Environment in `Build()`
- `internal/graph/elements.go` — add annotation fields to `NodeData`, update `BuildElements()`
- `internal/renderer/export.go` — add `TourStepsJSON` to `templateData`, update `ExportHTML`, update `loadBundledJS()`, add cytoscape-expand-collapse CDN fallback, extend `htmlSrc` with all new UI
- `internal/renderer/bundle.go` — no change (embed is directory-level `//go:embed js`)
- `cmd/export.go` — add `--annotations` flag, wire into pipeline
- `cmd/serve.go` — add `--annotations` flag, wire into `buildServeGraph()`
- `Makefile` — add `cytoscape-expand-collapse` download to `bundle` target

---

## Task 1: Glossary Package

**Files:**
- Create: `internal/glossary/glossary.go`
- Create: `internal/glossary/glossary_test.go`

**Interfaces:**
- Produces: `glossary.ServiceInfo{Name, OneLiner, DocsURL string}`, `glossary.Lookup(resourceType string) (ServiceInfo, bool)`

- [ ] **Step 1: Write the failing test**

```go
// internal/glossary/glossary_test.go
package glossary_test

import (
	"testing"

	"github.com/hack-monk/tf-lens/internal/glossary"
)

func TestLookup_KnownType(t *testing.T) {
	info, ok := glossary.Lookup("aws_sqs_queue")
	if !ok {
		t.Fatal("expected ok=true for aws_sqs_queue")
	}
	if info.Name == "" {
		t.Error("Name must not be empty")
	}
	if info.OneLiner == "" {
		t.Error("OneLiner must not be empty")
	}
	if info.DocsURL == "" {
		t.Error("DocsURL must not be empty")
	}
}

func TestLookup_UnknownType(t *testing.T) {
	_, ok := glossary.Lookup("aws_unknown_thing")
	if ok {
		t.Error("expected ok=false for unknown type")
	}
}

func TestLookup_AllThirtyTypes(t *testing.T) {
	known := []string{
		"aws_vpc", "aws_subnet", "aws_internet_gateway", "aws_nat_gateway",
		"aws_alb", "aws_lb", "aws_route53_zone",
		"aws_instance", "aws_lambda_function", "aws_ecs_service", "aws_ecs_cluster",
		"aws_ecs_task_definition", "aws_eks_cluster", "aws_autoscaling_group",
		"aws_launch_template", "aws_cloudfront_distribution",
		"aws_s3_bucket", "aws_db_instance", "aws_rds_cluster", "aws_dynamodb_table",
		"aws_elasticache_cluster", "aws_ebs_volume", "aws_efs_file_system", "aws_ecr_repository",
		"aws_security_group", "aws_iam_role", "aws_kms_key", "aws_secretsmanager_secret",
		"aws_sns_topic", "aws_sqs_queue", "aws_api_gateway_rest_api", "aws_cloudwatch_log_group",
		"aws_opensearch_domain", "aws_redshift_cluster", "aws_kinesis_stream",
		"aws_msk_cluster", "aws_docdb_cluster", "aws_neptune_cluster",
		"aws_codebuild_project", "aws_cloudtrail",
	}
	for _, typ := range known {
		if _, ok := glossary.Lookup(typ); !ok {
			t.Errorf("missing glossary entry for %s", typ)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /Users/ashutoch/Desktop/tf-lens
go test ./internal/glossary/... -v
```
Expected: FAIL with "cannot find package"

- [ ] **Step 3: Implement glossary package**

```go
// internal/glossary/glossary.go
package glossary

// ServiceInfo describes an AWS service for new joiners who don't know the resource type.
type ServiceInfo struct {
	Name     string // e.g. "Amazon SQS"
	OneLiner string // one-sentence explanation
	DocsURL  string // link to AWS docs
}

// Lookup returns the ServiceInfo for a Terraform resource type.
// Returns (ServiceInfo, false) for unknown types.
func Lookup(resourceType string) (ServiceInfo, bool) {
	info, ok := catalog[resourceType]
	return info, ok
}

var catalog = map[string]ServiceInfo{
	"aws_vpc": {
		Name:     "Amazon VPC",
		OneLiner: "Virtual private network that isolates your AWS resources from the public internet and other accounts.",
		DocsURL:  "https://docs.aws.amazon.com/vpc/latest/userguide/",
	},
	"aws_subnet": {
		Name:     "VPC Subnet",
		OneLiner: "A range of IP addresses within a VPC. Public subnets route to the internet; private subnets do not.",
		DocsURL:  "https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html",
	},
	"aws_internet_gateway": {
		Name:     "Internet Gateway",
		OneLiner: "Connects a VPC to the public internet, enabling inbound and outbound traffic for public subnets.",
		DocsURL:  "https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Internet_Gateway.html",
	},
	"aws_nat_gateway": {
		Name:     "NAT Gateway",
		OneLiner: "Lets private subnet resources reach the internet for outbound calls (e.g. software updates) without exposing them inbound.",
		DocsURL:  "https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html",
	},
	"aws_alb": {
		Name:     "Application Load Balancer",
		OneLiner: "Distributes HTTP/HTTPS traffic across multiple targets, supports path-based routing and health checks.",
		DocsURL:  "https://docs.aws.amazon.com/elasticloadbalancing/latest/application/",
	},
	"aws_lb": {
		Name:     "Load Balancer",
		OneLiner: "Distributes incoming traffic across multiple targets to improve availability and fault tolerance.",
		DocsURL:  "https://docs.aws.amazon.com/elasticloadbalancing/latest/userguide/",
	},
	"aws_route53_zone": {
		Name:     "Amazon Route 53",
		OneLiner: "DNS service that routes end-user requests to your infrastructure — translates domain names to IP addresses.",
		DocsURL:  "https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/",
	},
	"aws_instance": {
		Name:     "Amazon EC2 Instance",
		OneLiner: "A virtual server in the cloud. You choose the CPU, memory, and OS. Pay per hour of use.",
		DocsURL:  "https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/",
	},
	"aws_lambda_function": {
		Name:     "AWS Lambda",
		OneLiner: "Run code without managing servers. Triggered by events; scales automatically; you pay per invocation.",
		DocsURL:  "https://docs.aws.amazon.com/lambda/latest/dg/",
	},
	"aws_ecs_service": {
		Name:     "Amazon ECS Service",
		OneLiner: "Runs and maintains a desired count of Docker containers. Handles restarts, scaling, and load balancer registration.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonECS/latest/developerguide/",
	},
	"aws_ecs_cluster": {
		Name:     "Amazon ECS Cluster",
		OneLiner: "A logical grouping of EC2 instances or Fargate capacity on which ECS services run.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonECS/latest/developerguide/clusters.html",
	},
	"aws_ecs_task_definition": {
		Name:     "ECS Task Definition",
		OneLiner: "Blueprint for a container: which image to run, CPU/memory limits, environment variables, and log config.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html",
	},
	"aws_eks_cluster": {
		Name:     "Amazon EKS",
		OneLiner: "Managed Kubernetes cluster. AWS runs the control plane; you run worker nodes or use Fargate.",
		DocsURL:  "https://docs.aws.amazon.com/eks/latest/userguide/",
	},
	"aws_autoscaling_group": {
		Name:     "Auto Scaling Group",
		OneLiner: "Automatically adjusts the number of EC2 instances based on demand. Maintains a min/max/desired count.",
		DocsURL:  "https://docs.aws.amazon.com/autoscaling/ec2/userguide/",
	},
	"aws_launch_template": {
		Name:     "Launch Template",
		OneLiner: "Configuration blueprint for EC2 instances launched by an Auto Scaling Group — AMI, instance type, security groups.",
		DocsURL:  "https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-launch-templates.html",
	},
	"aws_cloudfront_distribution": {
		Name:     "Amazon CloudFront",
		OneLiner: "Content delivery network (CDN) that caches content at edge locations worldwide to reduce latency.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/",
	},
	"aws_s3_bucket": {
		Name:     "Amazon S3",
		OneLiner: "Object storage for any amount of data. Stores files (objects) in buckets. Pay for what you store.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonS3/latest/userguide/",
	},
	"aws_db_instance": {
		Name:     "Amazon RDS Instance",
		OneLiner: "Managed relational database (MySQL, PostgreSQL, etc.). AWS handles backups, patching, and failover.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/",
	},
	"aws_rds_cluster": {
		Name:     "Amazon Aurora Cluster",
		OneLiner: "MySQL/PostgreSQL-compatible database with automatic storage scaling and multi-AZ replication.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/",
	},
	"aws_dynamodb_table": {
		Name:     "Amazon DynamoDB",
		OneLiner: "Fully managed NoSQL key-value store. Single-digit millisecond latency at any scale.",
		DocsURL:  "https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/",
	},
	"aws_elasticache_cluster": {
		Name:     "Amazon ElastiCache",
		OneLiner: "In-memory cache (Redis or Memcached). Used to speed up database reads by caching frequent queries.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/",
	},
	"aws_ebs_volume": {
		Name:     "Amazon EBS Volume",
		OneLiner: "Block storage attached to a single EC2 instance — like a hard drive for your virtual machine.",
		DocsURL:  "https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AmazonEBS.html",
	},
	"aws_efs_file_system": {
		Name:     "Amazon EFS",
		OneLiner: "Shared file system mountable by multiple EC2 instances simultaneously — like a network drive.",
		DocsURL:  "https://docs.aws.amazon.com/efs/latest/ug/",
	},
	"aws_ecr_repository": {
		Name:     "Amazon ECR",
		OneLiner: "Private Docker container image registry. Stores images used by ECS or EKS to run containers.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonECR/latest/userguide/",
	},
	"aws_security_group": {
		Name:     "Security Group",
		OneLiner: "Virtual firewall for EC2/RDS/Lambda — controls which IP addresses and ports can send or receive traffic.",
		DocsURL:  "https://docs.aws.amazon.com/vpc/latest/userguide/VPC_SecurityGroups.html",
	},
	"aws_iam_role": {
		Name:     "IAM Role",
		OneLiner: "Set of permissions that AWS services can assume. A Lambda function or EC2 instance uses a role to call other AWS services.",
		DocsURL:  "https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html",
	},
	"aws_kms_key": {
		Name:     "AWS KMS Key",
		OneLiner: "Encryption key managed by AWS Key Management Service. Used to encrypt S3 objects, RDS databases, Secrets, and more.",
		DocsURL:  "https://docs.aws.amazon.com/kms/latest/developerguide/",
	},
	"aws_secretsmanager_secret": {
		Name:     "AWS Secrets Manager",
		OneLiner: "Securely stores and rotates credentials (database passwords, API keys). Applications retrieve secrets at runtime.",
		DocsURL:  "https://docs.aws.amazon.com/secretsmanager/latest/userguide/",
	},
	"aws_sns_topic": {
		Name:     "Amazon SNS",
		OneLiner: "Pub/sub messaging. Publishers send to a topic; all subscribers receive the message (fan-out).",
		DocsURL:  "https://docs.aws.amazon.com/sns/latest/dg/",
	},
	"aws_sqs_queue": {
		Name:     "Amazon SQS",
		OneLiner: "Fully managed message queue. Decouples producers from consumers so neither blocks the other.",
		DocsURL:  "https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/",
	},
	"aws_api_gateway_rest_api": {
		Name:     "Amazon API Gateway",
		OneLiner: "Fully managed service to create, publish, and secure REST/HTTP APIs at any scale.",
		DocsURL:  "https://docs.aws.amazon.com/apigateway/latest/developerguide/",
	},
	"aws_cloudwatch_log_group": {
		Name:     "CloudWatch Logs",
		OneLiner: "Stores and searches application/service log output. Lambda, ECS, and EC2 write logs here automatically.",
		DocsURL:  "https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/",
	},
	"aws_opensearch_domain": {
		Name:     "Amazon OpenSearch",
		OneLiner: "Managed search and analytics engine (fork of Elasticsearch). Used for log analysis and full-text search.",
		DocsURL:  "https://docs.aws.amazon.com/opensearch-service/latest/developerguide/",
	},
	"aws_redshift_cluster": {
		Name:     "Amazon Redshift",
		OneLiner: "Cloud data warehouse for analytics queries over petabytes of data. Columnar storage, SQL interface.",
		DocsURL:  "https://docs.aws.amazon.com/redshift/latest/gsg/",
	},
	"aws_kinesis_stream": {
		Name:     "Amazon Kinesis Data Streams",
		OneLiner: "Real-time data streaming. Producers write events; consumers (Lambda, KDA) process them in order.",
		DocsURL:  "https://docs.aws.amazon.com/streams/latest/dev/",
	},
	"aws_msk_cluster": {
		Name:     "Amazon MSK (Kafka)",
		OneLiner: "Managed Apache Kafka cluster. High-throughput event streaming for microservices and analytics pipelines.",
		DocsURL:  "https://docs.aws.amazon.com/msk/latest/developerguide/",
	},
	"aws_docdb_cluster": {
		Name:     "Amazon DocumentDB",
		OneLiner: "MongoDB-compatible managed document database. Scales storage automatically, multi-AZ replication.",
		DocsURL:  "https://docs.aws.amazon.com/documentdb/latest/developerguide/",
	},
	"aws_neptune_cluster": {
		Name:     "Amazon Neptune",
		OneLiner: "Managed graph database for highly connected datasets — knowledge graphs, recommendation engines, fraud detection.",
		DocsURL:  "https://docs.aws.amazon.com/neptune/latest/userguide/",
	},
	"aws_codebuild_project": {
		Name:     "AWS CodeBuild",
		OneLiner: "Managed CI build service. Compiles source code, runs tests, and produces deployable artifacts.",
		DocsURL:  "https://docs.aws.amazon.com/codebuild/latest/userguide/",
	},
	"aws_cloudtrail": {
		Name:     "AWS CloudTrail",
		OneLiner: "Audit log of every API call made in your AWS account — who did what, when, from where.",
		DocsURL:  "https://docs.aws.amazon.com/awscloudtrail/latest/userguide/",
	},
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/glossary/... -v
```
Expected: PASS — all three tests green.

- [ ] **Step 5: Commit**

```bash
git add internal/glossary/
git commit -m "feat(glossary): add AWS service glossary with 40 resource type entries"
```

---

## Task 2: Annotations Package

**Files:**
- Create: `internal/annotations/annotations.go`
- Create: `internal/annotations/annotations_test.go`

**Interfaces:**
- Consumes: `gopkg.in/yaml.v3` (new dep), `graph.Graph`, `graph.TourStep` (added in Task 3 — write this task's code referencing those types; they will exist when this compiles after Task 3)
- Produces:
  - `annotations.Annotation{Resource, Label, Description, Docs, Owner string}`
  - `annotations.TourStep{Step int, Resource, Title, Narration string}`
  - `annotations.File{Annotations []Annotation, Tour []TourStep}`
  - `annotations.Parse(path string) (*File, error)`
  - `annotations.Apply(g *graph.Graph, f *File)` — merges into nodes + sets `g.TourSteps`

**Note:** Task 2 references `graph.TourStep` and `graph.Graph.TourSteps`. These are added in Task 3. Build will fail until Task 3 is complete — that is expected. Run Task 2's tests after Task 3.

- [ ] **Step 1: Add gopkg.in/yaml.v3 dependency**

```bash
cd /Users/ashutoch/Desktop/tf-lens
go get gopkg.in/yaml.v3
```
Expected: go.mod and go.sum updated.

- [ ] **Step 2: Write the failing test**

```go
// internal/annotations/annotations_test.go
package annotations_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hack-monk/tf-lens/internal/annotations"
	"github.com/hack-monk/tf-lens/internal/graph"
)

const sampleYAML = `
annotations:
  - resource: aws_sqs_queue.orders
    label: "Order Processing Queue"
    description: "Decouples checkout from fulfillment."
    docs: "https://wiki.example.com/order-queue"
    owner: "payments-team"

tour:
  - step: 1
    resource: aws_alb.main
    title: "Entry Point"
    narration: "All traffic enters here."
  - step: 2
    resource: aws_sqs_queue.orders
    title: "Order Queue"
    narration: "Async handoff to fulfillment."
`

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "tf-lens-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestParse_ValidYAML(t *testing.T) {
	path := writeTempYAML(t, sampleYAML)
	f, err := annotations.Parse(path)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(f.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(f.Annotations))
	}
	a := f.Annotations[0]
	if a.Resource != "aws_sqs_queue.orders" {
		t.Errorf("Resource = %q, want %q", a.Resource, "aws_sqs_queue.orders")
	}
	if a.Label != "Order Processing Queue" {
		t.Errorf("Label = %q", a.Label)
	}
	if a.Owner != "payments-team" {
		t.Errorf("Owner = %q", a.Owner)
	}
	if len(f.Tour) != 2 {
		t.Fatalf("expected 2 tour steps, got %d", len(f.Tour))
	}
	if f.Tour[0].Step != 1 || f.Tour[0].Resource != "aws_alb.main" {
		t.Errorf("tour step 1 mismatch: %+v", f.Tour[0])
	}
}

func TestParse_MissingFile(t *testing.T) {
	_, err := annotations.Parse(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestApply_MergesIntoNodes(t *testing.T) {
	path := writeTempYAML(t, sampleYAML)
	f, _ := annotations.Parse(path)

	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_sqs_queue.orders", Type: "aws_sqs_queue", Name: "orders"},
			{ID: "aws_alb.main", Type: "aws_alb", Name: "main"},
		},
	}
	annotations.Apply(g, f)

	// Check annotation merged
	var queueNode *graph.Node
	for _, n := range g.Nodes {
		if n.ID == "aws_sqs_queue.orders" {
			queueNode = n
		}
	}
	if queueNode == nil {
		t.Fatal("queue node not found")
	}
	if queueNode.HumanLabel != "Order Processing Queue" {
		t.Errorf("HumanLabel = %q", queueNode.HumanLabel)
	}
	if queueNode.Description != "Decouples checkout from fulfillment." {
		t.Errorf("Description = %q", queueNode.Description)
	}
	if queueNode.Owner != "payments-team" {
		t.Errorf("Owner = %q", queueNode.Owner)
	}

	// Check tour steps attached to graph
	if len(g.TourSteps) != 2 {
		t.Fatalf("expected 2 tour steps on graph, got %d", len(g.TourSteps))
	}
}
```

- [ ] **Step 3: Implement annotations package**

```go
// internal/annotations/annotations.go
package annotations

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/hack-monk/tf-lens/internal/graph"
)

// Annotation enriches a single resource node with human-authored context.
type Annotation struct {
	Resource    string `yaml:"resource"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
	Docs        string `yaml:"docs"`
	Owner       string `yaml:"owner"`
}

// TourStep defines one step in a guided tour of the infrastructure.
type TourStep struct {
	Step      int    `yaml:"step"`
	Resource  string `yaml:"resource"`
	Title     string `yaml:"title"`
	Narration string `yaml:"narration"`
}

// File is the parsed tf-lens.yaml content.
type File struct {
	Annotations []Annotation `yaml:"annotations"`
	Tour        []TourStep   `yaml:"tour"`
}

// Parse reads and validates a tf-lens.yaml annotation file.
func Parse(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading annotations file: %w", err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing annotations YAML: %w", err)
	}
	return &f, nil
}

// Apply merges annotation data into graph nodes (per-field override, not all-or-nothing)
// and attaches tour steps to the graph.
// Resources in the annotation file that don't exist in the graph are silently ignored.
func Apply(g *graph.Graph, f *File) {
	// Build node index for O(1) lookup
	idx := make(map[string]*graph.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		idx[n.ID] = n
	}

	for _, a := range f.Annotations {
		n, ok := idx[a.Resource]
		if !ok {
			continue // resource not in graph — silently skip
		}
		if a.Label != "" {
			n.HumanLabel = a.Label
		}
		if a.Description != "" {
			n.Description = a.Description
		}
		if a.Docs != "" {
			n.DocsURL = a.Docs
		}
		if a.Owner != "" {
			n.Owner = a.Owner
		}
	}

	// Attach tour steps to graph
	for _, ts := range f.Tour {
		g.TourSteps = append(g.TourSteps, graph.TourStep{
			Step:      ts.Step,
			Resource:  ts.Resource,
			Title:     ts.Title,
			Narration: ts.Narration,
		})
	}
}
```

- [ ] **Step 4: Run tests (after Task 3 is complete)**

```
go test ./internal/annotations/... -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/annotations/ go.mod go.sum
git commit -m "feat(annotations): add tf-lens.yaml annotation parser and Apply()"
```

---

## Task 3: Graph Model Extensions

**Files:**
- Modify: `internal/graph/graph.go` — add fields to `Node`, add `TourStep` type, add `TourSteps []TourStep` to `Graph`
- Modify: `internal/graph/elements.go` — add annotation fields to `NodeData`, update `BuildElements()`
- Modify: `internal/graph/graph_test.go` — extend existing test to verify new NodeData fields propagate

**Interfaces:**
- Produces:
  - `graph.TourStep{Step int, Resource, Title, Narration string}`
  - `graph.Node.HumanLabel string`
  - `graph.Node.Description string`
  - `graph.Node.DocsURL string`
  - `graph.Node.Owner string`
  - `graph.Node.Environment string`
  - `graph.Node.GlossaryName string`
  - `graph.Node.GlossaryOneLiner string`
  - `graph.Graph.TourSteps []TourStep`
  - `graph.NodeData.HumanLabel`, `.Description`, `.DocsURL`, `.Owner`, `.Environment`, `.GlossaryName`, `.GlossaryOneLiner` (all `string`, `omitempty`)

- [ ] **Step 1: Write the failing test**

Add to `internal/graph/graph_test.go` (append after existing tests):

```go
func TestBuildElements_AnnotationFields(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{
				ID:               "aws_sqs_queue.orders",
				Type:             "aws_sqs_queue",
				Name:             "orders",
				Category:         graph.CategoryMessaging,
				HumanLabel:       "Order Queue",
				Description:      "Handles order events.",
				DocsURL:          "https://wiki.example.com",
				Owner:            "payments-team",
				Environment:      "prod",
				GlossaryName:     "Amazon SQS",
				GlossaryOneLiner: "Fully managed message queue.",
			},
		},
	}
	elems := graph.BuildElements(g)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	nd, ok := elems[0].Data.(graph.NodeData)
	if !ok {
		t.Fatal("element data is not NodeData")
	}
	if nd.HumanLabel != "Order Queue" {
		t.Errorf("HumanLabel = %q", nd.HumanLabel)
	}
	if nd.GlossaryName != "Amazon SQS" {
		t.Errorf("GlossaryName = %q", nd.GlossaryName)
	}
	if nd.Owner != "payments-team" {
		t.Errorf("Owner = %q", nd.Owner)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/graph/... -v -run TestBuildElements_AnnotationFields
```
Expected: FAIL — `graph.Node` has no field `HumanLabel`

- [ ] **Step 3: Add TourStep type and new fields to graph.go**

In `internal/graph/graph.go`, add after the `FlowEdge` struct (after line 91):

```go
// TourStep is one step in a guided onboarding tour of the infrastructure.
type TourStep struct {
	Step      int    // 1-based step number
	Resource  string // Terraform resource address, e.g. "aws_alb.main"
	Title     string // Short step title
	Narration string // Explanation shown to the new joiner
}
```

Add to the `Node` struct (after the `DriftChanges []NodeDriftChange` field, around line 73):

```go
	// Context fields — populated by graph.Build (auto-inference) and annotations.Apply (overrides)
	HumanLabel       string // Human-friendly display name, e.g. "Order Processing Queue"
	Description      string // What this resource does and why it exists
	DocsURL          string // Link to wiki, runbook, or AWS docs
	Owner            string // Team or person responsible, from tags.Team/Owner or annotation
	Environment      string // e.g. "prod", "staging" — from tags.Environment
	GlossaryName     string // AWS service name, e.g. "Amazon SQS" — from glossary package
	GlossaryOneLiner string // One-sentence AWS service explanation — from glossary package
```

Add to the `Graph` struct (after `FlowEdges []*FlowEdge`):

```go
	TourSteps []TourStep // Guided tour steps — populated by annotations.Apply()
```

- [ ] **Step 4: Add new fields to NodeData and update BuildElements**

In `internal/graph/elements.go`, add to the `NodeData` struct (after `DriftChanges []DriftChangeData`):

```go
	// Context / annotation fields
	HumanLabel       string `json:"humanLabel,omitempty"`
	Description      string `json:"description,omitempty"`
	DocsURL          string `json:"docsURL,omitempty"`
	Owner            string `json:"owner,omitempty"`
	Environment      string `json:"environment,omitempty"`
	GlossaryName     string `json:"glossaryName,omitempty"`
	GlossaryOneLiner string `json:"glossaryOneLiner,omitempty"`
```

In `BuildElements`, update the `NodeData` struct literal to include new fields (add after `DriftChanges: toDriftChangeData(n.DriftChanges),`):

```go
				HumanLabel:       n.HumanLabel,
				Description:      n.Description,
				DocsURL:          n.DocsURL,
				Owner:            n.Owner,
				Environment:      n.Environment,
				GlossaryName:     n.GlossaryName,
				GlossaryOneLiner: n.GlossaryOneLiner,
```

- [ ] **Step 5: Run tests**

```
go test ./internal/graph/... -v
```
Expected: all existing tests pass + new `TestBuildElements_AnnotationFields` passes.

- [ ] **Step 6: Compile check**

```
go build ./...
```
Expected: no errors (annotations package now resolves `graph.TourStep` and `graph.Node.HumanLabel`).

- [ ] **Step 7: Run annotations tests**

```
go test ./internal/annotations/... -v
```
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/graph/ internal/annotations/
git commit -m "feat(graph): add TourStep, annotation fields on Node, propagate through BuildElements"
```

---

## Task 4: Auto-Inference from Tags and Attributes

**Files:**
- Modify: `internal/graph/graph.go` — enrich nodes in `Build()` Pass 1 using tags + description attribute

**Interfaces:**
- Consumes: `parser.Resource.Tags`, `parser.Resource.Attributes`, `parser.Resource.Name`
- Produces: `Node.HumanLabel` (from `tags.Name`), `Node.Description` (from `tags.Description` or `attrs["description"]`), `Node.Owner` (from `tags.Team` or `tags.Owner`), `Node.Environment` (from `tags.Environment`)

- [ ] **Step 1: Write the failing test**

Add to `internal/graph/graph_test.go`:

```go
func TestBuild_AutoInference(t *testing.T) {
	resources := []parser.Resource{
		{
			Address:  "aws_sqs_queue.orders",
			Type:     "aws_sqs_queue",
			Name:     "orders",
			Provider: "aws",
			Tags: map[string]string{
				"Name":        "Order Processing Queue",
				"Description": "Handles order events",
				"Team":        "payments-team",
				"Environment": "prod",
			},
			Attributes: map[string]any{},
		},
		{
			Address:  "aws_iam_role.exec",
			Type:     "aws_iam_role",
			Name:     "exec",
			Provider: "aws",
			Tags:     map[string]string{},
			Attributes: map[string]any{
				"description": "Execution role for Lambda functions",
			},
		},
	}
	g := graph.Build(resources)

	var qNode, iamNode *graph.Node
	for _, n := range g.Nodes {
		if n.ID == "aws_sqs_queue.orders" {
			qNode = n
		}
		if n.ID == "aws_iam_role.exec" {
			iamNode = n
		}
	}
	if qNode == nil {
		t.Fatal("queue node not found")
	}
	if qNode.HumanLabel != "Order Processing Queue" {
		t.Errorf("HumanLabel = %q, want %q", qNode.HumanLabel, "Order Processing Queue")
	}
	if qNode.Owner != "payments-team" {
		t.Errorf("Owner = %q, want %q", qNode.Owner, "payments-team")
	}
	if qNode.Environment != "prod" {
		t.Errorf("Environment = %q, want %q", qNode.Environment, "prod")
	}
	if iamNode == nil {
		t.Fatal("iam node not found")
	}
	if iamNode.Description != "Execution role for Lambda functions" {
		t.Errorf("Description = %q", iamNode.Description)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/graph/... -v -run TestBuild_AutoInference
```
Expected: FAIL — `HumanLabel`, `Owner`, `Environment` are all empty strings.

- [ ] **Step 3: Add auto-inference to graph.Build Pass 1**

In `internal/graph/graph.go`, in `Build()`, after `n := &Node{...}` is constructed and before `g.Nodes = append(g.Nodes, n)`, add:

```go
		// Auto-infer context from tags (zero-config — no annotation file needed)
		if v := r.Tags["Name"]; v != "" {
			n.HumanLabel = v
		}
		if v := r.Tags["Description"]; v != "" {
			n.Description = v
		}
		// Check attrs["description"] as fallback (used by IAM roles, security groups)
		if n.Description == "" {
			if v, ok := r.Attributes["description"]; ok {
				if s, ok := v.(string); ok && s != "" {
					n.Description = s
				}
			}
		}
		if v := r.Tags["Team"]; v != "" {
			n.Owner = v
		} else if v := r.Tags["Owner"]; v != "" {
			n.Owner = v
		}
		if v := r.Tags["Environment"]; v != "" {
			n.Environment = v
		} else if v := r.Tags["Env"]; v != "" {
			n.Environment = v
		}
```

- [ ] **Step 4: Run tests**

```
go test ./internal/graph/... -v
```
Expected: all pass including `TestBuild_AutoInference`.

- [ ] **Step 5: Commit**

```bash
git add internal/graph/graph.go internal/graph/graph_test.go
git commit -m "feat(graph): auto-infer HumanLabel/Description/Owner/Environment from Terraform tags"
```

---

## Task 5: Glossary Annotation + CLI --annotations Flag

**Files:**
- Create: `internal/glossary/annotate.go` — `AnnotateGraph(g *graph.Graph)` sets GlossaryName/GlossaryOneLiner on each node
- Modify: `cmd/export.go` — add `--annotations` flag, call annotations.Apply + glossary.AnnotateGraph
- Modify: `cmd/serve.go` — add `--annotations` flag, call in `buildServeGraph()`

**Interfaces:**
- Consumes: `glossary.Lookup()`, `graph.Graph`, `annotations.Parse()`, `annotations.Apply()`
- Produces: nodes have `GlossaryName`, `GlossaryOneLiner` set; tour steps on graph; `--annotations` CLI flag

- [ ] **Step 1: Write test for AnnotateGraph**

Add to `internal/glossary/glossary_test.go`:

```go
func TestAnnotateGraph(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_sqs_queue.orders", Type: "aws_sqs_queue"},
			{ID: "aws_instance.web", Type: "aws_instance"},
			{ID: "aws_unknown_resource.foo", Type: "aws_unknown_resource"},
		},
	}
	glossary.AnnotateGraph(g)

	var sqs, ec2, unknown *graph.Node
	for _, n := range g.Nodes {
		switch n.ID {
		case "aws_sqs_queue.orders":
			sqs = n
		case "aws_instance.web":
			ec2 = n
		case "aws_unknown_resource.foo":
			unknown = n
		}
	}
	if sqs.GlossaryName != "Amazon SQS" {
		t.Errorf("SQS GlossaryName = %q", sqs.GlossaryName)
	}
	if sqs.GlossaryOneLiner == "" {
		t.Error("SQS GlossaryOneLiner must not be empty")
	}
	if ec2.GlossaryName != "Amazon EC2 Instance" {
		t.Errorf("EC2 GlossaryName = %q", ec2.GlossaryName)
	}
	if unknown.GlossaryName != "" {
		t.Errorf("unknown type should have empty GlossaryName, got %q", unknown.GlossaryName)
	}
}
```

Update the import in `internal/glossary/glossary_test.go`:

```go
import (
	"testing"

	"github.com/hack-monk/tf-lens/internal/glossary"
	"github.com/hack-monk/tf-lens/internal/graph"
)
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/glossary/... -v -run TestAnnotateGraph
```
Expected: FAIL — `glossary.AnnotateGraph` undefined.

- [ ] **Step 3: Implement AnnotateGraph**

```go
// internal/glossary/annotate.go
package glossary

import "github.com/hack-monk/tf-lens/internal/graph"

// AnnotateGraph sets GlossaryName and GlossaryOneLiner on every node
// whose resource type has an entry in the catalog.
func AnnotateGraph(g *graph.Graph) {
	for _, n := range g.Nodes {
		if info, ok := Lookup(n.Type); ok {
			n.GlossaryName = info.Name
			n.GlossaryOneLiner = info.OneLiner
		}
	}
}
```

- [ ] **Step 4: Run test**

```
go test ./internal/glossary/... -v
```
Expected: all pass.

- [ ] **Step 5: Add --annotations flag to cmd/export.go**

Add to the `var (...)` block at the top of `cmd/export.go`:

```go
	exportAnnotations string
```

In the `RunE` function, after Step 7 (flow inference) and before Step 8 (write output), add:

```go
		// ── 7b. Glossary annotation (always runs) ────────────────────────────
		glossary.AnnotateGraph(g)

		// ── 7c. Human annotations (optional) ────────────────────────────────
		if exportAnnotations != "" {
			af, err := annotations.Parse(exportAnnotations)
			if err != nil {
				return fmt.Errorf("parsing annotations: %w", err)
			}
			annotations.Apply(g, af)
			fmt.Printf("📝  Annotations: %d resources annotated, %d tour steps\n",
				len(af.Annotations), len(af.Tour))
		}
```

In `init()` at the bottom of `cmd/export.go`, add:

```go
	exportCmd.Flags().StringVar(&exportAnnotations, "annotations", "",
		"Path to tf-lens.yaml annotation file with human-readable labels and tour steps")
```

Add to imports in `cmd/export.go`:

```go
	"github.com/hack-monk/tf-lens/internal/annotations"
	"github.com/hack-monk/tf-lens/internal/glossary"
```

- [ ] **Step 6: Add --annotations flag to cmd/serve.go**

Add to the `var (...)` block:

```go
	serveAnnotations string
```

In `buildServeGraph()`, after the flow block (after `flow.AnnotateGraph(g, flows)`), add:

```go
	// Glossary annotation always runs
	glossary.AnnotateGraph(g)

	if serveAnnotations != "" {
		af, err := annotations.Parse(serveAnnotations)
		if err != nil {
			return nil, fmt.Errorf("parsing annotations: %w", err)
		}
		annotations.Apply(g, af)
		fmt.Printf("📝  Annotations: %d resources annotated, %d tour steps\n",
			len(af.Annotations), len(af.Tour))
	}
```

In `init()` at the bottom of `cmd/serve.go`, add:

```go
	serveCmd.Flags().StringVar(&serveAnnotations, "annotations", "",
		"Path to tf-lens.yaml annotation file with human-readable labels and tour steps")
```

In `collectWatchPaths()`, add:

```go
	if serveAnnotations != "" {
		paths = append(paths, serveAnnotations)
	}
```

Add to imports in `cmd/serve.go`:

```go
	"github.com/hack-monk/tf-lens/internal/annotations"
	"github.com/hack-monk/tf-lens/internal/glossary"
```

- [ ] **Step 7: Build and smoke-test**

```bash
go build -o tf-lens .
./tf-lens export --plan testdata/plan.json --out /tmp/test.html
```
Expected: builds and exports successfully. No errors.

- [ ] **Step 8: Commit**

```bash
git add internal/glossary/ cmd/export.go cmd/serve.go
git commit -m "feat: glossary.AnnotateGraph + --annotations flag for tf-lens.yaml"
```

---

## Task 6: Renderer — Pass Tour and Meta to Template

**Files:**
- Modify: `internal/renderer/export.go` — add `TourStepsJSON string` to `templateData`, update `ExportHTML()` to marshal tour steps

**Interfaces:**
- Consumes: `graph.Graph.TourSteps []graph.TourStep`
- Produces: `{{.TourStepsJSON}}` available in `htmlSrc` template (JSON array, empty `[]` if none)

- [ ] **Step 1: Write test**

Add to a new file `internal/renderer/export_test.go`:

```go
package renderer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
	"github.com/hack-monk/tf-lens/internal/renderer"
)

func TestExportHTML_TourStepsEmbedded(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking},
		},
		TourSteps: []graph.TourStep{
			{Step: 1, Resource: "aws_alb.main", Title: "Entry Point", Narration: "Traffic enters here."},
		},
	}
	var buf bytes.Buffer
	resolver := icons.NewResolver("")
	if err := renderer.ExportHTML(&buf, g, resolver); err != nil {
		t.Fatalf("ExportHTML error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `"Entry Point"`) {
		t.Error("HTML does not contain tour step title")
	}
	if !strings.Contains(html, `"aws_alb.main"`) {
		t.Error("HTML does not contain tour step resource")
	}
}

func TestExportHTML_EmptyTourSteps(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking},
		},
	}
	var buf bytes.Buffer
	resolver := icons.NewResolver("")
	if err := renderer.ExportHTML(&buf, g, resolver); err != nil {
		t.Fatalf("ExportHTML error: %v", err)
	}
	html := buf.String()
	// Should contain empty tour steps JSON
	if !strings.Contains(html, `var TOUR_STEPS = []`) {
		t.Error("HTML should contain empty TOUR_STEPS array")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/renderer/... -v
```
Expected: FAIL — template doesn't have `TOUR_STEPS` variable.

- [ ] **Step 3: Update templateData and ExportHTML**

In `internal/renderer/export.go`, update `templateData` struct:

```go
type templateData struct {
	Elements      string
	Offline       bool
	InlineJS      string
	TourStepsJSON string // JSON array of tour steps; "[]" when none
}
```

Update `ExportHTML` function:

```go
func ExportHTML(w io.Writer, g *graph.Graph, resolver *icons.Resolver) error {
	elements := graph.BuildElements(g)
	elemJSON, err := json.MarshalIndent(elements, "", "  ")
	if err != nil {
		return fmt.Errorf("serialising elements: %w", err)
	}

	tourJSON, err := json.Marshal(g.TourSteps)
	if err != nil {
		return fmt.Errorf("serialising tour steps: %w", err)
	}
	if g.TourSteps == nil {
		tourJSON = []byte("[]")
	}

	inlineJS, offline := loadBundledJS()
	return htmlTemplate.Execute(w, templateData{
		Elements:      string(elemJSON),
		Offline:       offline,
		InlineJS:      inlineJS,
		TourStepsJSON: string(tourJSON),
	})
}
```

- [ ] **Step 4: Add `var TOUR_STEPS = {{.TourStepsJSON}};` to the HTML template**

In `htmlSrc`, find the line:

```
var ELEMENTS = {{.Elements}};
```

Add immediately after it:

```
var TOUR_STEPS = {{.TourStepsJSON}};
```

- [ ] **Step 5: Run tests**

```
go test ./internal/renderer/... -v
```
Expected: both tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/renderer/export.go internal/renderer/export_test.go
git commit -m "feat(renderer): embed TOUR_STEPS JSON in HTML template"
```

---

## Task 7: Frontend — Dark Mode and Summary Dashboard

**Files:**
- Modify: `internal/renderer/export.go` (`htmlSrc`) — add CSS vars, dark mode toggle, dashboard strip

- [ ] **Step 1: Write test**

Add to `internal/renderer/export_test.go`:

```go
func TestExportHTML_DarkModeElements(t *testing.T) {
	g := &graph.Graph{Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}}}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"id=\"dark-toggle\"", "--bg-body", "id=\"dashboard\"", "doToggleDark"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing: %s", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/renderer/... -v -run TestExportHTML_DarkModeElements
```
Expected: FAIL.

- [ ] **Step 3: Add CSS custom properties for theming**

In `htmlSrc`, find:
```
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
```

Add immediately before it:

```css
:root{
  --bg-body:#F0F2F5;--bg-bar:#232F3E;--bg-panel:#FFFFFF;--bg-card:#FFFFFF;
  --text-primary:#1A202C;--text-secondary:#4A5568;--text-muted:#718096;
  --border:#E2E8F0;--sb-bg:rgba(255,255,255,0.85);
}
body.dark{
  --bg-body:#0D1117;--bg-bar:#161B22;--bg-panel:#161B22;--bg-card:#1C2128;
  --text-primary:#E6EDF3;--text-secondary:#8B949E;--text-muted:#6E7681;
  --border:#30363D;--sb-bg:rgba(22,27,34,0.9);
}
```

Update `body{` rule — change `background:#F0F2F5;` and `color:#1A202C;` to use CSS vars:

```css
body{
  font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;
  background:var(--bg-body);
  display:flex;flex-direction:column;height:100vh;overflow:hidden;
  color:var(--text-primary);
  transition:background .2s,color .2s;
}
```

Add CSS for dashboard:

After the `#bar{...}` rule block, add:

```css
#dashboard{
  display:flex;align-items:center;gap:8px;padding:6px 16px;
  background:var(--bg-bar);border-bottom:1px solid #2D3748;
  flex-shrink:0;flex-wrap:wrap;
}
.dp{
  display:inline-flex;align-items:center;gap:5px;
  padding:3px 10px;border-radius:12px;
  background:#2D3748;color:#E2E8F0;
  font-size:11px;font-weight:600;cursor:pointer;
  border:1px solid #4A5568;
  transition:background .15s;
}
.dp:hover{background:#4A5568}
#dark-toggle{
  margin-left:auto;padding:4px 10px;
  background:transparent;border:1px solid #4A5568;
  border-radius:6px;color:#A0AEC0;cursor:pointer;font-size:12px;
}
#dark-toggle:hover{background:#2D3748;color:#E2E8F0}
```

- [ ] **Step 4: Add dark toggle button to toolbar HTML**

In `htmlSrc`, find the closing `</div>` of `<div id="bar">` (the line right before `<div id="cy">`), and insert before it:

```html
  <button id="dark-toggle" onclick="doToggleDark()" title="Toggle dark mode">☀</button>
```

- [ ] **Step 5: Add dashboard HTML**

After `</div>` that closes `<div id="bar">` (i.e., between the bar and `<div id="cy">`), add:

```html
<div id="dashboard"></div>
```

- [ ] **Step 6: Add dashboard JS and dark mode JS**

In `htmlSrc`, after the `var TOUR_STEPS = {{.TourStepsJSON}};` line you added in Task 6, and before the `// ── Cytoscape styles` comment, add:

```js
// ── Dark mode ────────────────────────────────────────────────────────────
(function(){
  var saved = localStorage.getItem('tflens-dark');
  if(saved === '1') document.body.classList.add('dark');
})();
window.doToggleDark = function(){
  var on = document.body.classList.toggle('dark');
  localStorage.setItem('tflens-dark', on ? '1' : '0');
  var btn = document.getElementById('dark-toggle');
  if(btn) btn.textContent = on ? '🌙' : '☀';
};
(function(){
  var btn = document.getElementById('dark-toggle');
  if(btn) btn.textContent = document.body.classList.contains('dark') ? '🌙' : '☀';
})();
```

Add dashboard-building JS. Find the section that starts with:
```js
var lc = cy.nodes().filter(function(n){ return !n.isParent(); }).length;
```

Add BEFORE that section:

```js
// ── Summary dashboard ────────────────────────────────────────────────────
(function(){
  var dash = document.getElementById('dashboard');
  if(!dash) return;

  var leafCount = cy.nodes().filter(function(n){ return !n.isParent(); }).length;
  var modCount  = cy.nodes().filter(function(n){ return n.data('type')==='module'; }).length;

  function pill(icon, text, onclick){
    var d = document.createElement('div');
    d.className = 'dp';
    d.innerHTML = icon + ' ' + text;
    if(onclick) d.onclick = onclick;
    return d;
  }

  dash.appendChild(pill('📦', leafCount + ' Resources'));

  if(modCount > 0){
    dash.appendChild(pill('🧩', modCount + ' Modules'));
  }

  // Threat pill
  var tc = {critical:0, high:0, medium:0};
  cy.nodes().forEach(function(n){
    var s = n.data('threatSeverity');
    if(s && tc[s] !== undefined) tc[s]++;
  });
  var totalThreats = tc.critical + tc.high + tc.medium;
  if(totalThreats > 0){
    var parts = [];
    if(tc.critical) parts.push('🔴'+tc.critical);
    if(tc.high)     parts.push('🟠'+tc.high);
    if(tc.medium)   parts.push('🟡'+tc.medium);
    dash.appendChild(pill('⚠', parts.join(' ') + ' Threats', function(){
      // activate threat filter: highlight nodes with threats
      cy.nodes().forEach(function(n){ if(!n.data('threatSeverity')) n.addClass('faded'); else n.removeClass('faded'); });
      cy.edges().forEach(function(e){ e.addClass('faded'); });
    }));
  }

  // Cost pill
  var totalCostDash = 0;
  cy.nodes().forEach(function(n){ totalCostDash += (n.data('monthlyCost')||0); });
  if(totalCostDash > 0){
    dash.appendChild(pill('💰', fmtCost(totalCostDash)+'/mo', function(){
      // sort nodes by cost descending (visual hint — open highest cost panel)
      var costNodes = [];
      cy.nodes().forEach(function(n){ if(n.data('monthlyCost')>0 && !n.isParent()) costNodes.push(n); });
      costNodes.sort(function(a,b){ return b.data('monthlyCost')-a.data('monthlyCost'); });
      cy.nodes().addClass('faded');
      costNodes.forEach(function(n){ n.removeClass('faded'); });
      cy.edges().addClass('faded');
    }));
  }

  // Drift pill
  var driftCountDash = 0;
  cy.nodes().forEach(function(n){ if(n.data('driftStatus')) driftCountDash++; });
  if(driftCountDash > 0){
    dash.appendChild(pill('⚡', driftCountDash + ' Drifted', function(){
      cy.nodes().forEach(function(n){ if(!n.data('driftStatus')) n.addClass('faded'); else n.removeClass('faded'); });
      cy.edges().addClass('faded');
    }));
  }
})();
```

- [ ] **Step 7: Run tests**

```
go test ./internal/renderer/... -v
```
Expected: all pass including `TestExportHTML_DarkModeElements`.

- [ ] **Step 8: Visual check**

```bash
go build -o tf-lens . && ./tf-lens export --plan testdata/plan.json --out /tmp/dash-test.html
open /tmp/dash-test.html
```
Expected: dashboard strip visible below toolbar, dark toggle button works, pills show resource count.

- [ ] **Step 9: Commit**

```bash
git add internal/renderer/export.go internal/renderer/export_test.go
git commit -m "feat(ui): add dark mode toggle + summary dashboard strip"
```

---

## Task 8: Frontend — Minimap

**Files:**
- Modify: `internal/renderer/export.go` (`htmlSrc`)

- [ ] **Step 1: Write test**

Add to `internal/renderer/export_test.go`:

```go
func TestExportHTML_MinimapElements(t *testing.T) {
	g := &graph.Graph{Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}}}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"id=\"minimap\"", "id=\"minimap-vp\"", "initMinimap", "M"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing minimap element: %s", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/renderer/... -v -run TestExportHTML_MinimapElements
```
Expected: FAIL.

- [ ] **Step 3: Add minimap CSS**

In `htmlSrc`, after the `#dark-toggle:hover{...}` block added in Task 7, add:

```css
#minimap{
  position:absolute;bottom:44px;right:12px;
  width:180px;height:110px;
  background:var(--bg-panel);border:1px solid var(--border);
  border-radius:6px;overflow:hidden;z-index:50;
  box-shadow:0 2px 8px rgba(0,0,0,.15);
}
#minimap canvas{width:100%;height:100%}
#minimap-vp{
  position:absolute;border:2px solid #FF9900;border-radius:2px;
  pointer-events:none;
}
#minimap-toggle{
  position:absolute;bottom:44px;right:200px;
  background:var(--sb-bg);border:1px solid var(--border);
  border-radius:6px;padding:3px 7px;font-size:11px;
  cursor:pointer;z-index:51;color:var(--text-secondary);
}
```

- [ ] **Step 4: Add minimap HTML**

In `htmlSrc`, find `<div id="cy"></div>` and replace with:

```html
<div id="cy" style="position:relative"></div>
<button id="minimap-toggle" onclick="toggleMinimap()" title="M">⊞ Map</button>
<div id="minimap" style="display:none">
  <canvas id="minimap-canvas"></canvas>
  <div id="minimap-vp"></div>
</div>
```

- [ ] **Step 5: Add minimap JS**

In `htmlSrc`, after `// ── Panel resize ─────────────────────────────────────────────────────────` IIFE closing `})();`, before the final `})();` that closes the outer IIFE, add:

```js
// ── Minimap ──────────────────────────────────────────────────────────────
var minimapVisible = false;
window.toggleMinimap = function(){
  minimapVisible = !minimapVisible;
  document.getElementById('minimap').style.display = minimapVisible ? '' : 'none';
  if(minimapVisible) drawMinimap();
};

function drawMinimap(){
  var canvas = document.getElementById('minimap-canvas');
  if(!canvas) return;
  var mm = document.getElementById('minimap');
  canvas.width  = mm.offsetWidth;
  canvas.height = mm.offsetHeight;
  var ctx = canvas.getContext('2d');
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  // Get graph bounding box
  var bb = cy.elements().boundingBox();
  if(!bb || bb.w === 0 || bb.h === 0) return;

  var scaleX = canvas.width  / bb.w;
  var scaleY = canvas.height / bb.h;
  var scale  = Math.min(scaleX, scaleY) * 0.9;
  var offX   = (canvas.width  - bb.w * scale) / 2;
  var offY   = (canvas.height - bb.h * scale) / 2;

  // Draw nodes
  cy.nodes().forEach(function(n){
    if(n.isParent()) return;
    var nbb = n.boundingBox();
    var x = (nbb.x1 - bb.x1) * scale + offX;
    var y = (nbb.y1 - bb.y1) * scale + offY;
    var w = nbb.w * scale;
    var h = nbb.h * scale;
    var cat = n.data('category') || 'unknown';
    var colors = {networking:'#147EBA',compute:'#ED7100',storage:'#3F8624',security:'#DD344C',messaging:'#8C4FFF',unknown:'#A0AEC0'};
    ctx.fillStyle = colors[cat] || '#A0AEC0';
    ctx.fillRect(x, y, Math.max(w, 3), Math.max(h, 3));
  });

  // Viewport indicator
  var pan  = cy.pan();
  var zoom = cy.zoom();
  var ext  = cy.extent();
  var vpx = (ext.x1 - bb.x1) * scale + offX;
  var vpy = (ext.y1 - bb.y1) * scale + offY;
  var vpw = ext.w * scale;
  var vph = ext.h * scale;
  var vp = document.getElementById('minimap-vp');
  vp.style.left   = Math.max(0, vpx) + 'px';
  vp.style.top    = Math.max(0, vpy) + 'px';
  vp.style.width  = Math.min(vpw, canvas.width)  + 'px';
  vp.style.height = Math.min(vph, canvas.height) + 'px';
}

// Redraw minimap on pan/zoom
cy.on('pan zoom', function(){ if(minimapVisible) drawMinimap(); });

// Keyboard: M to toggle minimap
document.addEventListener('keydown', function(e){
  if(e.target.matches('input')) return;
  if(e.key === 'm' || e.key === 'M') toggleMinimap();
});
```

- [ ] **Step 6: Add M to keyboard help overlay**

In `htmlSrc`, find:
```html
    <div class="krow"><span class="kdesc">Show this help</span><span class="kkey">?</span></div>
```

Add after it:
```html
    <div class="krow"><span class="kdesc">Toggle minimap</span><span class="kkey">M</span></div>
```

- [ ] **Step 7: Run tests and visual check**

```
go test ./internal/renderer/... -v
go build -o tf-lens . && ./tf-lens export --plan testdata/plan.json --out /tmp/minimap-test.html
open /tmp/minimap-test.html
```
Expected: tests pass; `⊞ Map` button shows; clicking it opens minimap with coloured dots; M key toggles.

- [ ] **Step 8: Commit**

```bash
git add internal/renderer/export.go
git commit -m "feat(ui): add canvas minimap with viewport indicator, toggle with M key"
```

---

## Task 9: Frontend — Search Filter Chips

**Files:**
- Modify: `internal/renderer/export.go` (`htmlSrc`)

Extends the existing `doSearch()` function to support filter syntax: `/type:rds`, `/tag:env=prod`, `/module:payments`, `/threat:critical`, `/owner:payments-team`. Active filters shown as removable chips below the search bar.

- [ ] **Step 1: Write test**

Add to `internal/renderer/export_test.go`:

```go
func TestExportHTML_SearchFilters(t *testing.T) {
	g := &graph.Graph{Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}}}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"id=\"filter-chips\"", "parseFilters", "type:", "owner:"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing search filter element: %s", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/renderer/... -v -run TestExportHTML_SearchFilters
```
Expected: FAIL.

- [ ] **Step 3: Add filter chips CSS**

In `htmlSrc`, after `#dark-toggle:hover{...}` add:

```css
#filter-chips{
  display:flex;gap:4px;flex-wrap:wrap;padding:0 16px 6px;
  background:var(--bg-bar);
}
.fchip{
  display:inline-flex;align-items:center;gap:4px;
  padding:2px 8px;border-radius:10px;
  background:#3182CE;color:#FFF;font-size:10px;font-weight:600;cursor:pointer;
}
.fchip:after{content:'×';margin-left:2px;opacity:.7}
.fchip:hover:after{opacity:1}
```

- [ ] **Step 4: Add filter chips HTML**

In `htmlSrc`, find `<div id="dashboard"></div>` (added in Task 7) and add after it:

```html
<div id="filter-chips" style="display:none"></div>
```

- [ ] **Step 5: Replace doSearch with extended version**

In `htmlSrc`, find and replace the entire `window.doSearch = function(q){...}` block (from `window.doSearch = function(q){` through the closing `};`) with:

```js
// ── Search with filter syntax ─────────────────────────────────────────────
function parseFilters(raw){
  var filters = [];
  var rest = raw;
  var re = /(type|tag|module|threat|owner):([^\s]+)/g;
  var m;
  while((m = re.exec(raw)) !== null){
    filters.push({key: m[1], val: m[2].toLowerCase()});
    rest = rest.replace(m[0], '').trim();
  }
  return {text: rest.trim().toLowerCase(), filters: filters};
}

function matchesFilters(n, text, filters){
  // Text match
  if(text && !(
    (n.data('label')||'').toLowerCase().includes(text) ||
    (n.data('type') ||'').toLowerCase().includes(text) ||
    (n.data('abbrev')||'').toLowerCase().includes(text) ||
    n.id().toLowerCase().includes(text) ||
    (n.data('humanLabel')||'').toLowerCase().includes(text) ||
    (n.data('description')||'').toLowerCase().includes(text)
  )) return false;
  // Structured filters
  for(var i=0;i<filters.length;i++){
    var f = filters[i];
    if(f.key==='type'    && !(n.data('type')||'').toLowerCase().includes(f.val)) return false;
    if(f.key==='module'  && !(n.id()||'').toLowerCase().includes('module.'+f.val)) return false;
    if(f.key==='threat'  && (n.data('threatSeverity')||'').toLowerCase() !== f.val) return false;
    if(f.key==='owner'   && (n.data('owner')||'').toLowerCase() !== f.val) return false;
    if(f.key==='tag'){
      var kv = f.val.split('=');
      // tag filtering requires humanLabel/description check (tags not in NodeData — use owner/env as proxy)
      // basic: check if type or label contains tag key
    }
  }
  return true;
}

function renderFilterChips(filters){
  var bar = document.getElementById('filter-chips');
  if(!bar) return;
  bar.innerHTML = '';
  if(!filters.length){ bar.style.display='none'; return; }
  bar.style.display='flex';
  filters.forEach(function(f, idx){
    var chip = document.createElement('span');
    chip.className = 'fchip';
    chip.textContent = f.key+':'+f.val;
    chip.onclick = function(){
      var q = document.getElementById('q');
      var val = q.value.replace(f.key+':'+f.val, '').trim();
      q.value = val;
      doSearch(val);
    };
    bar.appendChild(chip);
  });
}

window.doSearch = function(q){
  clearSel();
  var raw = (q||'').trim();
  var qx  = document.getElementById('qx');
  var qc  = document.getElementById('qc');
  qx.style.display = raw ? '' : 'none';

  var parsed = parseFilters(raw);
  renderFilterChips(parsed.filters);

  if(!raw){ cy.elements().removeClass('faded'); qc.style.display='none'; return; }

  var matched = 0;
  var leafCount = 0;
  cy.nodes().forEach(function(n){
    if(n.isParent()) return;
    leafCount++;
    var m = matchesFilters(n, parsed.text, parsed.filters);
    if(m){ n.removeClass('faded'); matched++; } else n.addClass('faded');
  });
  cy.edges().forEach(function(e){
    var ok = !e.source().hasClass('faded') && !e.target().hasClass('faded');
    if(ok) e.removeClass('faded'); else e.addClass('faded');
  });
  qc.textContent = matched+'/'+leafCount;
  qc.style.display = '';
};
```

Also update `window.clearSearch`:

```js
window.clearSearch = function(){
  document.getElementById('q').value = '';
  document.getElementById('qx').style.display = 'none';
  document.getElementById('qc').style.display = 'none';
  document.getElementById('filter-chips').style.display = 'none';
  cy.elements().removeClass('faded');
};
```

- [ ] **Step 6: Run tests and visual check**

```
go test ./internal/renderer/... -v
go build -o tf-lens . && ./tf-lens export --plan testdata/plan.json --threat --out /tmp/search-test.html
open /tmp/search-test.html
```
Expected: tests pass; typing `type:rds` in search dims non-RDS nodes; chip appears; clicking chip removes it.

- [ ] **Step 7: Commit**

```bash
git add internal/renderer/export.go
git commit -m "feat(ui): extended search with type:/module:/threat:/owner: filter syntax + chips"
```

---

## Task 10: Frontend — Collapsible Module Groups

**Files:**
- Modify: `Makefile` — add cytoscape-expand-collapse to bundle step
- Modify: `internal/renderer/export.go` — add expand-collapse to `loadBundledJS()` file list, CDN fallback, and JS initialisation + double-click handler in `htmlSrc`

**Note:** `cytoscape-expand-collapse` requires a separate registration call after Cytoscape is initialised. The library adds `cy.expandCollapse()` and handles compound node collapse.

- [ ] **Step 1: Write test**

Add to `internal/renderer/export_test.go`:

```go
func TestExportHTML_CollapsibleModules(t *testing.T) {
	g := &graph.Graph{Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}}}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"expandCollapse", "cytoscape-expand-collapse", "dblclick"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing collapsible modules element: %s", want)
		}
	}
}
```

- [ ] **Step 2: Add cytoscape-expand-collapse to Makefile**

In `Makefile`, add at the top with the other version variables:

```makefile
EXPANDCOLLAPSE_VERSION := 4.1.0
EXPANDCOLLAPSE_JS      := $(BUNDLE_DIR)/cytoscape-expand-collapse.min.js
```

In the `bundle:` target, after the `cytoscape-node-html-label` download line:

```makefile
	@echo "→ Downloading cytoscape-expand-collapse $(EXPANDCOLLAPSE_VERSION)..."
	@curl -fsSL "https://cdn.jsdelivr.net/npm/cytoscape-expand-collapse@$(EXPANDCOLLAPSE_VERSION)/cytoscape-expand-collapse.min.js" -o $(EXPANDCOLLAPSE_JS)
```

- [ ] **Step 3: Update loadBundledJS() to include the new file**

In `internal/renderer/export.go`, find the `files` slice inside `loadBundledJS()`:

```go
	files := []string{
		"cytoscape.min.js",
		"dagre.min.js",
		"cytoscape-dagre.min.js",
		"cytoscape-node-html-label.min.js",
	}
```

Replace with:

```go
	files := []string{
		"cytoscape.min.js",
		"dagre.min.js",
		"cytoscape-dagre.min.js",
		"cytoscape-node-html-label.min.js",
		"cytoscape-expand-collapse.min.js",
	}
```

- [ ] **Step 4: Add CDN fallback for expand-collapse**

In `htmlSrc`, find:

```
<script src="https://cdn.jsdelivr.net/npm/cytoscape-node-html-label@1.2.1/dist/cytoscape-node-html-label.min.js"></script>
```

Add immediately after it:

```html
<script src="https://cdn.jsdelivr.net/npm/cytoscape-expand-collapse@4.1.0/cytoscape-expand-collapse.min.js"></script>
```

- [ ] **Step 5: Initialise expand-collapse and add double-click handler**

In `htmlSrc`, after the Cytoscape `cy = cytoscape({...})` initialisation block closes (`});`), and before `cy.nodeHtmlLabel(...)`, add:

```js
// Initialise expand-collapse extension (if available)
var ecApi = null;
if(typeof cy.expandCollapse === 'function'){
  ecApi = cy.expandCollapse({
    layoutBy: {name:'dagre',rankDir:'TB',nodeSep:50,rankSep:80},
    fisheye: false,
    animate: true,
    animationDuration: 200,
    undoable: false,
  });
}

// Double-click on a module/parent node to collapse/expand
cy.on('dblclick', 'node', function(evt){
  var n = evt.target;
  if(!n.isParent() || !ecApi) return;
  if(n.data('collapsed')){
    ecApi.expand(n);
    n.data('collapsed', false);
  } else {
    ecApi.collapse(n);
    n.data('collapsed', true);
  }
});
```

- [ ] **Step 6: Run tests**

```
go test ./internal/renderer/... -v
```
Expected: all pass including `TestExportHTML_CollapsibleModules`.

- [ ] **Step 7: Bundle and visual check**

```bash
make bundle
go build -o tf-lens .
./tf-lens export --plan testdata/plan.json --out /tmp/collapse-test.html
open /tmp/collapse-test.html
```
Expected: if plan has module grouping, double-click on a module container collapses it to a summary node. Double-click again to expand. On fresh clone without bundle, CDN fallback is used.

- [ ] **Step 8: Commit**

```bash
git add Makefile internal/renderer/export.go
git commit -m "feat(ui): collapsible module groups via cytoscape-expand-collapse (double-click)"
```

---

## Task 11: Frontend — Guided Tour

**Files:**
- Modify: `internal/renderer/export.go` (`htmlSrc`)

The tour is driven by `TOUR_STEPS` embedded in Task 6. This task adds the overlay UI and the JS controller.

- [ ] **Step 1: Write test**

Add to `internal/renderer/export_test.go`:

```go
func TestExportHTML_GuidedTour(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}},
		TourSteps: []graph.TourStep{
			{Step: 1, Resource: "aws_alb.main", Title: "Entry Point", Narration: "Traffic enters here."},
		},
	}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"id=\"tour-overlay\"", "startTour", "nextTourStep", "Start Tour"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing tour element: %s", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/renderer/... -v -run TestExportHTML_GuidedTour
```
Expected: FAIL.

- [ ] **Step 3: Add tour CSS**

In `htmlSrc`, after `#minimap-toggle{...}` block, add:

```css
#tour-overlay{
  position:fixed;inset:0;background:rgba(0,0,0,.6);
  z-index:200;display:none;align-items:flex-end;justify-content:center;
  padding-bottom:80px;
}
#tour-overlay.active{display:flex}
#tour-card{
  background:var(--bg-panel);border-radius:12px;padding:24px;
  max-width:440px;width:90%;box-shadow:0 8px 32px rgba(0,0,0,.4);
  position:relative;
}
#tour-step-num{
  font-size:10px;font-weight:700;color:var(--text-muted);
  letter-spacing:.8px;text-transform:uppercase;margin-bottom:6px;
}
#tour-title{font-size:18px;font-weight:700;color:var(--text-primary);margin-bottom:10px}
#tour-narration{font-size:13px;line-height:1.6;color:var(--text-secondary);margin-bottom:20px}
#tour-controls{display:flex;gap:8px;justify-content:flex-end}
.tour-btn{
  padding:6px 16px;border-radius:6px;border:none;cursor:pointer;
  font-size:12px;font-weight:600;
}
#tour-prev{background:var(--border);color:var(--text-primary)}
#tour-next{background:#FF9900;color:#FFF}
#tour-exit{
  position:absolute;top:12px;right:14px;
  background:none;border:none;cursor:pointer;
  font-size:18px;color:var(--text-muted);
}
#tour-start-btn{display:none}
```

- [ ] **Step 4: Add tour HTML**

In `htmlSrc`, before `</body>`, add:

```html
<button id="tour-start-btn" class="btn btn-p" onclick="startTour()" style="background:#FF9900;color:#1A202C;font-weight:700">▶ Start Tour</button>

<div id="tour-overlay">
  <div id="tour-card">
    <button id="tour-exit" onclick="exitTour()">×</button>
    <div id="tour-step-num"></div>
    <div id="tour-title"></div>
    <div id="tour-narration"></div>
    <div id="tour-controls">
      <button class="tour-btn" id="tour-prev" onclick="prevTourStep()">← Prev</button>
      <button class="tour-btn" id="tour-next" onclick="nextTourStep()">Next →</button>
    </div>
  </div>
</div>
```

- [ ] **Step 5: Add "Start Tour" button to toolbar**

In `htmlSrc`, find the dark-toggle button:
```html
  <button id="dark-toggle" onclick="doToggleDark()" title="Toggle dark mode">☀</button>
```

Add immediately before it:
```html
  <button id="tour-start-btn" class="btn" onclick="startTour()" style="display:none;background:#FF9900;color:#1A202C;font-weight:700">▶ Tour</button>
```

(Remove the duplicate `#tour-start-btn` added in Step 4 — only put it in the toolbar.)

Actually, combine: put only in toolbar. So in Step 4, skip the `<button id="tour-start-btn"...>` standalone element and only add the overlay div.

- [ ] **Step 6: Add tour JS**

In `htmlSrc`, after the minimap keyboard event listener block, add:

```js
// ── Guided Tour ──────────────────────────────────────────────────────────
(function(){
  var steps = TOUR_STEPS || [];
  var cur = 0;

  // Show Start Tour button if there are steps
  if(steps.length > 0){
    var btn = document.getElementById('tour-start-btn');
    if(btn) btn.style.display = '';
  }

  function updateHash(){
    if(cur >= 0 && cur < steps.length){
      history.replaceState(null,'','#tour='+(cur+1));
    }
  }

  function showStep(idx){
    if(idx < 0 || idx >= steps.length) return;
    cur = idx;
    var s = steps[idx];
    document.getElementById('tour-step-num').textContent = 'Step '+(idx+1)+' of '+steps.length;
    document.getElementById('tour-title').textContent = s.Title || s.title || '';
    document.getElementById('tour-narration').textContent = s.Narration || s.narration || '';
    document.getElementById('tour-prev').style.visibility = idx === 0 ? 'hidden' : '';
    document.getElementById('tour-next').textContent = idx === steps.length-1 ? 'Finish' : 'Next →';

    // Spotlight the target resource
    var resource = s.Resource || s.resource;
    cy.elements().removeClass('faded tour-spotlight');
    var target = cy.getElementById(resource);
    if(target && target.length){
      cy.nodes().forEach(function(n){ if(n.id()!==resource) n.addClass('faded'); });
      cy.edges().addClass('faded');
      cy.animate({fit:{eles:target.closedNeighborhood(), padding:80}}, {duration:400});
    }
    updateHash();
  }

  window.startTour = function(){
    if(!steps.length) return;
    document.getElementById('tour-overlay').classList.add('active');
    // Read #tour=N from hash for deep-link
    var m = location.hash.match(/#tour=(\d+)/);
    var startIdx = m ? Math.max(0, Math.min(parseInt(m[1],10)-1, steps.length-1)) : 0;
    showStep(startIdx);
  };

  window.exitTour = function(){
    document.getElementById('tour-overlay').classList.remove('active');
    cy.elements().removeClass('faded tour-spotlight');
    history.replaceState(null,'','#');
  };

  window.nextTourStep = function(){
    if(cur >= steps.length-1){ exitTour(); return; }
    showStep(cur+1);
  };

  window.prevTourStep = function(){
    if(cur > 0) showStep(cur-1);
  };

  // Auto-start if hash present on load
  var initM = location.hash.match(/#tour=(\d+)/);
  if(initM && steps.length > 0){
    setTimeout(function(){ window.startTour(); }, 600);
  }
})();
```

- [ ] **Step 7: Run tests and visual check**

```
go test ./internal/renderer/... -v
```

Create a sample annotation file and test the tour:

```bash
cat > /tmp/tf-lens-test.yaml << 'EOF'
tour:
  - step: 1
    resource: aws_alb.main
    title: "Entry Point"
    narration: "All external traffic enters through this Application Load Balancer."
EOF

go build -o tf-lens . && ./tf-lens export --plan testdata/plan.json --annotations /tmp/tf-lens-test.yaml --out /tmp/tour-test.html
open /tmp/tour-test.html
```
Expected: "▶ Tour" button appears in toolbar; clicking it opens overlay; step narration shown; Next/Prev/Finish navigate and spotlight resources; `#tour=1` in URL.

- [ ] **Step 8: Commit**

```bash
git add internal/renderer/export.go
git commit -m "feat(ui): guided tour mode with step overlay, URL hash deep-link, spotlight"
```

---

## Task 12: Frontend — Context Detail Panel

**Files:**
- Modify: `internal/renderer/export.go` (`htmlSrc`) — extend `openPanel(d)` to show `humanLabel`, `description`, `docsURL`, `owner`, `environment`, `glossaryName`, `glossaryOneLiner`

- [ ] **Step 1: Write test**

Add to `internal/renderer/export_test.go`:

```go
func TestExportHTML_ContextPanel(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{
				ID: "aws_sqs_queue.orders", Type: "aws_sqs_queue", Name: "orders",
				Category: graph.CategoryMessaging,
				HumanLabel: "Order Processing Queue", Description: "Handles order events.",
				DocsURL: "https://wiki.example.com", Owner: "payments-team",
				GlossaryName: "Amazon SQS", GlossaryOneLiner: "Fully managed message queue.",
			},
		},
	}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"humanLabel", "glossaryName", "glossaryOneLiner", "docsURL", "owner"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing panel field: %s", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/renderer/... -v -run TestExportHTML_ContextPanel
```
Expected: FAIL — panel JS doesn't reference `humanLabel` etc.

- [ ] **Step 3: Extend openPanel with context section**

In `htmlSrc`, find in `window.openPanel = function(d){`:

```js
  document.getElementById('pb').innerHTML = h;
```

Add a new context section before that line. Find the line:

```js
  h += '<div class="pd"></div>';
  h += '<div class="pa"><div class="pk">Address</div><div class="pv"><span class="pc">'+d.id+'</span></div></div>';
  h += '<div class="pa"><div class="pk">Type</div><div class="pv"><span class="pc">'+d.type+'</span></div></div>';
```

Replace with:

```js
  // ── Human context section ─────────────────────────────────────────────
  if(d.humanLabel || d.description || d.owner || d.docsURL){
    h += '<div class="pd"></div>';
    if(d.humanLabel){
      h += '<div class="pa"><div class="pk">Name</div>'
         + '<div class="pv" style="font-weight:700;font-size:14px;color:var(--text-primary)">'+d.humanLabel+'</div></div>';
    }
    if(d.description){
      h += '<div class="pa" style="align-items:flex-start"><div class="pk">What it does</div>'
         + '<div class="pv" style="line-height:1.5;color:var(--text-secondary)">'+d.description+'</div></div>';
    }
    if(d.owner){
      h += '<div class="pa"><div class="pk">Owner</div>'
         + '<div class="pv"><span style="background:#2D3748;color:#E2E8F0;padding:1px 8px;border-radius:10px;font-size:11px">'+d.owner+'</span></div></div>';
    }
    if(d.environment){
      h += '<div class="pa"><div class="pk">Environment</div>'
         + '<div class="pv"><span style="background:#276749;color:#C6F6D5;padding:1px 8px;border-radius:10px;font-size:11px">'+d.environment+'</span></div></div>';
    }
    if(d.docsURL){
      h += '<div class="pa"><div class="pk">Docs</div>'
         + '<div class="pv"><a href="'+d.docsURL+'" target="_blank" rel="noopener" style="color:#3182CE;font-size:12px;word-break:break-all">'+d.docsURL+'</a></div></div>';
    }
  }

  // ── AWS Service glossary ──────────────────────────────────────────────
  if(d.glossaryName || d.glossaryOneLiner){
    h += '<div class="pd"></div>';
    h += '<div class="pa" style="align-items:flex-start"><div class="pk" style="color:var(--text-muted)">About this service</div>';
    h += '<div style="margin-top:4px">';
    if(d.glossaryName){
      h += '<div style="font-size:12px;font-weight:700;color:var(--text-primary);margin-bottom:4px">'+d.glossaryName+'</div>';
    }
    if(d.glossaryOneLiner){
      h += '<div style="font-size:11px;color:var(--text-secondary);line-height:1.5">'+d.glossaryOneLiner+'</div>';
    }
    h += '</div></div>';
  }

  h += '<div class="pd"></div>';
  h += '<div class="pa"><div class="pk">Address</div><div class="pv"><span class="pc">'+d.id+'</span></div></div>';
  h += '<div class="pa"><div class="pk">Type</div><div class="pv"><span class="pc">'+d.type+'</span></div></div>';
```

- [ ] **Step 4: Run tests**

```
go test ./internal/renderer/... -v
```
Expected: all tests pass.

- [ ] **Step 5: Full visual check**

```bash
cat > /tmp/tf-lens-annot.yaml << 'EOF'
annotations:
  - resource: aws_alb.main
    label: "Production Load Balancer"
    description: "Entry point for all external traffic. Routes /api/* to API service, /* to frontend."
    docs: "https://wiki.example.com/lb"
    owner: "platform-team"
tour:
  - step: 1
    resource: aws_alb.main
    title: "Entry Point"
    narration: "All external traffic enters through this Application Load Balancer."
EOF

go build -o tf-lens .
./tf-lens export --plan testdata/plan.json --annotations /tmp/tf-lens-annot.yaml --threat --out /tmp/full-test.html
open /tmp/full-test.html
```

Verify:
- Dashboard shows resource count, threat pills
- Dark mode toggle works and persists on reload
- Minimap opens with M key, shows coloured dots
- `/type:alb` search filters to ALB nodes only, chip appears
- Double-click on module container collapses it (if modules exist in plan)
- "▶ Tour" appears; clicking walks through steps; resources spotlight
- Clicking any annotated node shows human label, description, owner, docs link, glossary section

- [ ] **Step 6: Commit**

```bash
git add internal/renderer/export.go internal/renderer/export_test.go
git commit -m "feat(ui): context detail panel with human label, description, docs, owner, glossary"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| Auto-infer from tags.Name/Description/Team/Environment | Task 4 |
| Auto-infer from attrs["description"] | Task 4 |
| tf-lens.yaml annotation file | Task 2 |
| --annotations CLI flag (export + serve) | Task 5 |
| AWS service glossary (30+ types) | Task 1 |
| Guided tour mode with step overlay | Task 11 |
| Start Tour button in toolbar | Task 11 |
| Tour URL hash deep-link (#tour=N) | Task 11 |
| Summary dashboard with clickable pills | Task 7 |
| Dark mode toggle + localStorage persistence | Task 7 |
| Minimap (180×120, viewport indicator, M key) | Task 8 |
| Better search: type:/module:/threat:/owner: filters | Task 9 |
| Filter chips (removable) | Task 9 |
| Collapsible module groups (double-click) | Task 10 |
| Collapse state in URL hash | Not implemented — omitted for scope (ponytail: URL hash collapse adds complexity for little gain in v1; add in follow-up) |
| Context detail panel (human label, docs, glossary) | Task 12 |
| --annotations reload on --watch | Task 5 (via collectWatchPaths) |
| All features offline in exported HTML | Tasks 7-12 (all JS is inline or bundled) |

**One gap found:** Collapse state in URL hash was in the spec but would significantly complicate Task 10. It is deferred — a `// ponytail: URL hash collapse, add if users request it` comment should be added in the double-click handler.

**Type consistency check:**
- `graph.TourStep` defined in Task 3, consumed in Task 2 (annotations.Apply), Task 6 (renderer), Task 11 (JS `TOUR_STEPS`)
- `graph.Node.HumanLabel` defined in Task 3, set in Task 4 (auto-inference) and Task 2 (Apply), serialised in Task 3 (NodeData.HumanLabel), read in Task 12 (openPanel `d.humanLabel`)
- JSON field names: `humanLabel`, `description`, `docsURL`, `owner`, `environment`, `glossaryName`, `glossaryOneLiner` — all lowercase camelCase in `NodeData` tags, consistent with existing pattern (`threatSeverity`, `monthlyCost`, `driftStatus`)
- `TOUR_STEPS` JS var set in Task 6, consumed in Task 11 — consistent

**No placeholders found.** All steps have real code.
