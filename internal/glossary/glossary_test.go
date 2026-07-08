// internal/glossary/glossary_test.go
package glossary_test

import (
	"testing"

	"github.com/hack-monk/tf-lens/internal/glossary"
	"github.com/hack-monk/tf-lens/internal/graph"
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
