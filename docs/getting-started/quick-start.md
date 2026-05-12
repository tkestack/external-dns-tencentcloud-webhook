# Quick Start (Taskfile)

Developer workflow using Taskfile. Automates credential setup, configuration, and Helm deployment.

## Prerequisites

- [task](https://taskfile.dev/) (v3.x)
- [helm](https://helm.sh/) (v3.x)
- kubectl connected to target cluster
- For OIDC mode: [tccli](https://cloud.tencent.com/document/product/440) (configured), TKE **managed cluster** with version `>= v1.20.6-tke.27` or `>= v1.22.5-tke.1`
- For static mode: any Kubernetes cluster

## Usage

```bash
git clone https://github.com/tkestack/external-dns-tencentcloud-webhook.git
cd external-dns-tencentcloud-webhook

# 1. Setup (generates values file, creates CAM role / Secret)
task setup ZONE_TYPE=public DOMAIN=example.com
task setup ZONE_TYPE=private DOMAIN=internal.example.com

# Or with static credentials
task setup AUTH=static ZONE_TYPE=public DOMAIN=example.com

# 2. Deploy (all zones with existing values files)
task deploy
```

`task setup` performs:
1. Detects cluster ID and region automatically
2. Sets up credentials (OIDC: creates CAM role; static: creates Secret)
3. Generates `deploy/helm/values-<zone>.local.yaml`

`task deploy` performs:
1. Deploys all zones with existing values files (or specific zone if `ZONE_TYPE` specified)
2. Uses dev image (`paas/latest`) with `imagePullPolicy=Always`
3. Waits for pod ready

## Daily development

```bash
# Code change → build → deploy
task build && task deploy

# Deploy only public zone
task build && task deploy ZONE_TYPE=public
```

## Other commands

```bash
task status                    # Show all instances
task logs                      # Webhook logs (public)
task logs ZONE_TYPE=private    # Webhook logs (private)
task examples                  # Deploy example Services
task examples:clean            # Remove examples
task down                      # Uninstall all
task down ZONE_TYPE=private    # Uninstall private only
```

See [taskfile.md](../development/taskfile.md) for full reference.
