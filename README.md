# 🔭 TF-Lens

**Terraform infrastructure visualisation — CLI-first, offline-capable, open source.**

TF-Lens parses Terraform plan and state files and renders them as clean, interactive AWS architecture diagrams. Single statically-linked Go binary. No cloud account. No runtime dependencies.

[![CI](https://github.com/hack-monk/tf-lens/actions/workflows/ci.yml/badge.svg)](https://github.com/hack-monk/tf-lens/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hack-monk/tf-lens)](https://goreportcard.com/report/github.com/hack-monk/tf-lens)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Binary Size](https://img.shields.io/badge/binary-~5.5%20MB-green.svg)](https://github.com/hack-monk/tf-lens/releases)

---

![TF-Lens diagram](docs/Screenshot.png)

## Why TF-Lens?

| Problem | TF-Lens |
|---|---|
| `terraform graph` is unreadable dependency spaghetti | VPC → Subnet → Instance compound nesting |
| Lucidchart / Visio diagrams go stale instantly | Generated directly from your plan or state file |
| Pluralith requires a cloud account and SaaS setup | Fully offline, single binary, no account |
| No free diff view for PR reviews | Built-in green/red/amber diff mode |
| No free security overlay for Terraform | Threat modelling: SG exposure, unencrypted storage, public RDS, IAM wildcards |
| Cost visibility requires separate tooling | Infracost-powered cost overlay — per-resource and total monthly estimates |

---

## Features

**`tf-lens export`** — self-contained HTML file, works offline, shareable via Slack/email/PR attachment

**`tf-lens serve`** — local HTTP server, opens in browser automatically, Refresh button for live reload

**Diff mode** — compare two plans or plan vs state, node cards show what changed at a glance

**Threat modelling** — detects 20+ security misconfigurations across SGs, S3, RDS, IAM, Lambda, EKS, ElastiCache, SQS, SNS, CloudFront

**Cost overlay** — Infracost integration shows per-resource monthly cost on node cards and total estimate in the statusbar

**AWS-style diagram** — category-coloured cards, dashed VPC/subnet containers with labels on the border line, right-angle edge routing

---

## Quick Start

```bash
# 1. Generate a plan file
terraform plan -out=plan.bin
terraform show -json plan.bin > plan.json

# 2. Export a self-contained diagram (works offline)
tf-lens export --plan plan.json --out diagram.html
open diagram.html

# 3. Or start a live interactive server
tf-lens serve --plan plan.json
# Opens http://localhost:7777 automatically
```

---

## Installation

### Build from source

```bash
git clone https://github.com/hack-monk/tf-lens.git
cd tf-lens
make bundle   # downloads Cytoscape.js + Dagre (~900KB total)
make build
./tf-lens version
```

### Download a release binary

```bash
# macOS arm64 (Apple Silicon)
curl -fsSL https://github.com/hack-monk/tf-lens/releases/latest/download/tf-lens_darwin_arm64.tar.gz \
  | tar -xz && sudo mv tf-lens /usr/local/bin/

# Linux amd64
curl -fsSL https://github.com/hack-monk/tf-lens/releases/latest/download/tf-lens_linux_amd64.tar.gz \
  | tar -xz && sudo mv tf-lens /usr/local/bin/
```

---

## CLI Reference

### Export mode

```
tf-lens export [flags]

  --plan        Path to terraform show -json output (plan JSON)
  --state       Path to terraform.tfstate file
  --out         Output HTML file path (default: diagram.html)
  --diff        Base plan/state to diff against — enables diff mode
  --threat      Run threat modelling and overlay findings on diagram
  --cost        Cost overlay: Infracost JSON file, or Terraform dir to auto-run infracost
  --icon-dir    Directory with custom SVG icons (optional)
```

### Serve mode

```
tf-lens serve [flags]

  --plan        Path to terraform show -json output
  --state       Path to terraform.tfstate file
  --port        HTTP port (default: 7777)
  --diff        Base plan/state to diff against
  --threat      Run threat modelling overlay
  --cost        Cost overlay: Infracost JSON file, or Terraform dir to auto-run infracost
  --no-open     Don't open browser automatically
```

---

## Diff Mode

Compare two plans, or a plan against a saved state, to see exactly what Terraform will change:

```bash
tf-lens export \
  --plan new_plan.json \
  --diff old_plan.json \
  --out changes.html
```

The CLI prints a summary:
```
📊  Diff summary:
    ✅  Added:     1
    ❌  Removed:   4
    🔄  Updated:   1
    ─   Unchanged: 7
```

Node cards show coloured outlines: **green** (added) · **red dashed** (removed) · **amber** (changed).

---

## Threat Modelling

Run security analysis directly on your Terraform plan:

```bash
tf-lens export --plan plan.json --threat --out security.html
```

The CLI prints a severity-sorted summary:
```
🔒  Threat model summary:
    🔴 Critical: 2
    🟠 High:     3
    🟡 Medium:   4
    🔵 Info:     3
```

Affected node cards show a severity badge. Click any node to open the detail panel with full findings:

- **Severity badge** — colour-coded label (CRITICAL / HIGH / MEDIUM / INFO)
- **Finding code** — e.g. `SG002`, `RDS001`
- **Title** — short description of what was detected
- **Detail** — explains exactly what was found and why it matters
- **Fix** — concrete remediation steps in a highlighted box

**Detection rules across 10 resource types:**

| Resource | Checks |
|---|---|
| `aws_security_group` | Open to internet (0.0.0.0/0), all-ports exposure, unrestricted egress |
| `aws_s3_bucket` | Public ACL, website hosting, no encryption, no versioning |
| `aws_db_instance` | Unencrypted storage, publicly accessible, no backups, no deletion protection |
| `aws_iam_role` | Wildcard trust policy, wildcard Action/Resource in inline policies |
| `aws_lambda_function` | Not in VPC, possible hardcoded secrets in environment variables |
| `aws_eks_cluster` | Public API endpoint, no KMS secrets encryption |
| `aws_elasticache_cluster` | No at-rest encryption, no in-transit encryption |
| `aws_sqs_queue` | No encryption |
| `aws_sns_topic` | No KMS encryption |
| `aws_cloudfront_distribution` | Outdated TLS version, no WAF |

---

## Cost Overlay

Visualise per-resource cloud costs directly on your architecture diagram, powered by [Infracost](https://www.infracost.io/).

### Option A: Auto-run Infracost against your Terraform directory

```bash
# Requires: infracost CLI installed + API key configured
tf-lens export --plan plan.json --cost /path/to/terraform/dir --out costs.html
```

### Option B: Use a pre-generated Infracost JSON file

```bash
infracost breakdown --path . --format json > cost.json
tf-lens export --plan plan.json --cost cost.json --out costs.html
```

The CLI prints a summary:
```
💰  Cost estimate:
    Monthly total: $234.60/mo
    Resources with cost: 3
```

In the diagram:
- Each node with cost shows a **green badge** (top-left corner) with the monthly amount
- A **cost summary pill** in the bottom statusbar shows the total monthly estimate
- Click any node to see its cost in the **detail panel**

---

## Icon System

TF-Lens ships with 25+ custom SVGs, colour-coded by AWS service category:

| Colour | Category | Examples |
|---|---|---|
| 🔵 Blue | Networking | VPC, Subnet, IGW, NAT, ALB, Route53 |
| 🟠 Orange | Compute | EC2, Lambda, ECS, EKS, ASG, CloudFront |
| 🟢 Green | Storage & DB | S3, RDS, DynamoDB, ElastiCache, EBS, EFS |
| 🔴 Red | Security & IAM | Security Group, IAM Role, KMS, Secrets Manager |
| 🟣 Purple | Messaging | SNS, SQS, API Gateway, CloudWatch |

Want to use the official AWS architecture icons? See [docs/icon-dir.md](docs/icon-dir.md).

---

## Architecture

```
tf-lens/
├── cmd/               # Cobra CLI commands (export, serve, version)
├── internal/
│   ├── parser/        # Terraform plan + state JSON parsing
│   ├── graph/         # Node/edge model, VPC→Subnet nesting logic
│   ├── diff/          # Plan comparison, change classification
│   ├── threat/        # Security misconfiguration detection (20+ rules)
│   ├── cost/          # Infracost integration — parse JSON or auto-run CLI
│   ├── icons/         # SVG resolver (user dir → embed → prefix → fallback)
│   ├── renderer/      # Self-contained HTML export with Cytoscape.js
│   └── server/        # HTTP server for serve mode
├── testdata/          # Synthetic Terraform plan fixtures
└── docs/              # Icon system documentation
```

Two-mode design:
- **Export** → single offline HTML file, Cytoscape.js embedded via `go:embed`
- **Serve** → local HTTP server, graph data served as JSON from `/api/graph`

---

## Roadmap

**Shipped**
- [x] `tf-lens export` — offline single-file HTML diagram
- [x] `tf-lens serve` — local HTTP server with live refresh
- [x] VPC → Subnet → Instance compound nesting
- [x] Diff mode (added / removed / changed overlays)
- [x] Threat modelling overlay (20+ detection rules)
- [x] 25+ AWS service icons
- [x] Search / filter
- [x] Click-to-inspect detail panel
- [x] Keyboard shortcuts (F, R, Esc, +/-)
- [x] Cost overlay (Infracost integration — file or auto-run)
- [x] Detailed threat findings panel (title, detail, remediation per finding)

**Up next**
- [ ] Azure support (community contribution path)
- [ ] GitHub Actions / GitLab CI PR attachment templates
- [ ] `--watch` flag for automatic plan reload

---

## Why not Pluralith?

- TF-Lens is fully offline, no account required. Pluralith requires cloud account registration.
- Single 5.5MB binary with `make build`. No complex setup.
- Threat modelling and cost overlays are not available as free features in any competing tool.
- Apache 2.0 — extend it, self-host it, contribute to it.

---

## Contributing

The icon resolver and graph engine are provider-agnostic. Adding Azure or GCP support:

1. Add SVG icons to `internal/icons/svg/` using the `<resource_type>.svg` naming convention
2. Add nesting rules to `internal/graph/graph.go` for the provider's container hierarchy
3. Add detection rules to `internal/threat/detector.go`

Please open an issue before starting significant work.

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
