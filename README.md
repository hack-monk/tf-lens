# 🔭 TF-Lens

**Terraform infrastructure visualisation — CLI-first, open source.**

TF-Lens parses Terraform plan and state files and renders them as
interactive infrastructure diagrams. It ships as a single statically-linked
Go binary with no runtime dependencies and no cloud account required.

[![CI](https://github.com/hack-monk/tf-lens/actions/workflows/ci.yml/badge.svg)](https://github.com/hack-monk/tf-lens/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hack-monk/tf-lens)](https://goreportcard.com/report/github.com/hack-monk/tf-lens)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

---

## Why TF-Lens?

| Problem | TF-Lens solution |
|---|---|
| `terraform graph` produces unreadable spaghetti | Human-readable VPC → Subnet → Instance grouping |
| Lucidchart/Visio diagrams go stale instantly | Diagrams generated directly from plan/state |
| Pluralith requires a cloud account | Fully offline, no account, no SaaS |
| No free diff view for PR reviews | Built-in green/red/amber diff mode |

---

## Quick Start

```bash
# Generate a plan file
terraform plan -out=plan.bin
terraform show -json plan.bin > plan.json

# Export a diagram
tf-lens export --plan plan.json --out diagram.html

# Open it — works offline, no server needed
open diagram.html
```

### Diff Mode (great for PR reviews)

```bash
# Compare new plan against previous state
tf-lens export --plan new_plan.json --diff old_plan.json --out changes.html
```

Nodes are colour-coded: 🟢 added · 🔴 removed · 🟡 changed

---

## Installation

### Download a pre-built binary

```bash
# Linux amd64
curl -fsSL https://github.com/hack-monk/tf-lens/releases/latest/download/tf-lens_linux_amd64.tar.gz \
  | tar -xz && sudo mv tf-lens /usr/local/bin/

# macOS arm64 (Apple Silicon)
curl -fsSL https://github.com/hack-monk/tf-lens/releases/latest/download/tf-lens_darwin_arm64.tar.gz \
  | tar -xz && sudo mv tf-lens /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/hack-monk/tf-lens.git
cd tf-lens
make bundle   # downloads Cytoscape.js + Dagre for offline export
make build
./tf-lens version
```

---

## CLI Reference

```
tf-lens export [flags]
  --plan      Path to terraform show -json output
  --state     Path to terraform.tfstate file
  --out       Output HTML file (default: diagram.html)
  --icon-dir  Directory with custom SVG icons (optional)
  --diff      Second plan/state to diff against

tf-lens serve [flags]          # Post-MVP — coming soon
  --plan      Path to plan JSON
  --state     Path to state file
  --port      HTTP port (default: 7777)

tf-lens version
```

---

## Icon System

TF-Lens ships with 25 custom SVGs covering the most common AWS resources,
colour-coded by service category:

| Colour | Category | Examples |
|---|---|---|
| 🔵 Blue | Networking | VPC, Subnet, IGW, NAT, ALB |
| 🟢 Green | Compute | EC2, Lambda, ECS, EKS, ASG |
| 🟠 Amber | Storage & DB | S3, RDS, DynamoDB, ElastiCache |
| 🟣 Purple | Security & IAM | Security Group, IAM Role, KMS |
| 🔴 Coral | Messaging | SNS, SQS, API Gateway, CloudWatch |

Want to use the official AWS architecture icons? See [docs/icon-dir.md](docs/icon-dir.md).

---

## Roadmap

**MVP (current)**
- [x] `tf-lens export` — self-contained HTML with Cytoscape.js
- [x] VPC → Subnet → Instance nesting
- [x] Diff mode (green/red/amber overlay)
- [x] 25 core AWS icons
- [x] Search bar

**Phase 1 (next)**
- [ ] `tf-lens serve` — local HTTP server with React + React Flow
- [ ] Threat modelling overlay (0.0.0.0/0 exposure, unencrypted storage, public S3)
- [ ] Resource detail panel with full attributes

**Phase 2**
- [ ] Cost overlay (Infracost-compatible)
- [ ] Azure support (community contribution path)
- [ ] GitHub Actions / GitLab CI templates

---

## Contributing

Contributions welcome! The icon resolver and graph engine are
provider-agnostic. Adding Azure support means:
1. Adding SVG icons to `internal/icons/svg/` following the naming convention
2. Adding nesting rules for the Azure resource group / VNet / subnet hierarchy

Please open an issue first to discuss significant changes.

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
