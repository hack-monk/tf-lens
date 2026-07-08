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
