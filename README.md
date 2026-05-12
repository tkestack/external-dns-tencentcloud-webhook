# external-dns-tencentcloud-webhook

[![License](https://img.shields.io/github/license/tkestack/external-dns-tencentcloud-webhook)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/tkestack/external-dns-tencentcloud-webhook)](https://goreportcard.com/report/github.com/tkestack/external-dns-tencentcloud-webhook)

[ExternalDNS](https://github.com/kubernetes-sigs/external-dns) webhook provider for Tencent Cloud DNS services (DNSPod & PrivateDNS).

## Overview

ExternalDNS upstream [removed the in-tree tencentcloud provider](https://github.com/kubernetes-sigs/external-dns/commit/640e593f) and recommends the [webhook approach](https://kubernetes-sigs.github.io/external-dns/latest/docs/tutorials/webhook-provider/). This project implements the webhook protocol as a standalone sidecar.

| Service | Zone Type | Description |
|:--------|:----------|:------------|
| [DNSPod](https://cloud.tencent.com/product/cns) | `public` | Public DNS resolution |
| [PrivateDNS](https://cloud.tencent.com/product/privatedns) | `private` | VPC-scoped internal DNS |

## Architecture

```
┌──────────────────────────────── Kubernetes Cluster ──────────────────────────────────┐
│                                                                                      │
│  ┌── external-dns-public ──────────┐      ┌── external-dns-private ──────────────┐   │
│  │                                 │      │                                      │   │
│  │  external-dns ──▶ webhook       │      │  external-dns ──▶ webhook            │   │
│  │                   --zone=public │      │                   --zone=private     │   │
│  │                   --domain=     │      │                   --domain=          │   │
│  │                    example.com  │      │                    internal.example  │   │
│  │                                 │      │                   --vpc-id=vpc-xxx   │   │
│  └───────────────┬─────────────────┘      └───────────────┬──────────────────────┘   │
│                  │                                        │                          │
└──────────────────┼────────────────────────────────────────┼──────────────────────────┘
                   ▼                                        ▼
            DNSPod API                               PrivateDNS API
         (public resolution)                      (VPC-scoped resolution)
```

Each zone type is a **separate Helm release** with independent RBAC and credentials. This follows the [ExternalDNS recommended pattern](https://kubernetes-sigs.github.io/external-dns/v0.12.0/tutorials/public-private-route53/) — separate instances per zone type for security isolation (least-privilege CAM roles) and operational safety (prevent internal services from modifying public DNS).

> Only need one zone type? Just deploy one release. The second is optional.

## Installation

```bash
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
```

### Credential mode A: OIDC (recommended)

No static keys needed. Requires:
- TKE **managed cluster** with version `>= v1.20.6-tke.27` or `>= v1.22.5-tke.1`
- OIDC enabled (TKE Console → Cluster → Basic Info → ServiceAccountIssuerDiscovery)

One-command setup (creates CAM role + generates values):

```bash
# Public zone
curl -sSL https://raw.githubusercontent.com/tkestack/external-dns-tencentcloud-webhook/master/scripts/setup.sh \
  | bash -s -- --auth oidc --zone public --domain example.com

# Private zone
curl -sSL https://raw.githubusercontent.com/tkestack/external-dns-tencentcloud-webhook/master/scripts/setup.sh \
  | bash -s -- --auth oidc --zone private --domain internal.example.com
```

Then install:

```bash
helm install external-dns-public external-dns/external-dns \
  -n external-dns --create-namespace \
  -f https://raw.githubusercontent.com/tkestack/external-dns-tencentcloud-webhook/master/deploy/helm/values-base.yaml \
  -f values-public.local.yaml

# China mainland: add values-china.yaml for CCR-hosted images
helm install external-dns-public external-dns/external-dns \
  -n external-dns --create-namespace \
  -f https://raw.githubusercontent.com/tkestack/external-dns-tencentcloud-webhook/master/deploy/helm/values-base.yaml \
  -f https://raw.githubusercontent.com/tkestack/external-dns-tencentcloud-webhook/master/deploy/helm/values-china.yaml \
  -f values-public.local.yaml
```

<details>
<summary>Manual values — public zone (without setup script)</summary>

Requires CAM role created beforehand. See [Prerequisites — OIDC setup](docs/getting-started/prerequisites.md#oidc-setup) for detailed steps.

```yaml
# values-public.yaml
serviceAccount:
  annotations:
    tke.cloud.tencent.com/role-arn: "qcs::cam::uin/<UIN>:roleName/external-dns-<CLUSTER_ID>-public"
    tke.cloud.tencent.com/audience: "sts.cloud.tencent.com"
    tke.cloud.tencent.com/token-expiration: "86400"

domainFilters:
  - example.com

txtOwnerId: <CLUSTER_ID>-public

provider:
  webhook:
    args:
      - --credential-mode=oidc
      - --zone-type=public
      - --domain-filter=example.com
```

</details>

<details>
<summary>Manual values — private zone (without setup script)</summary>

Requires CAM role with `QcloudPrivateDNSFullAccess` policy. See [Prerequisites — OIDC setup](docs/getting-started/prerequisites.md#oidc-setup).

```yaml
# values-private.yaml
serviceAccount:
  annotations:
    tke.cloud.tencent.com/role-arn: "qcs::cam::uin/<UIN>:roleName/external-dns-<CLUSTER_ID>-private"
    tke.cloud.tencent.com/audience: "sts.cloud.tencent.com"
    tke.cloud.tencent.com/token-expiration: "86400"

domainFilters:
  - internal.example.com

txtOwnerId: <CLUSTER_ID>-private

extraArgs:
  - --publish-internal-services

provider:
  webhook:
    args:
      - --credential-mode=oidc
      - --zone-type=private
      - --domain-filter=internal.example.com
      - --vpc-id=vpc-xxxxxxxx
```

> Uses `QcloudPrivateDNSFullAccess` policy. `--publish-internal-services` enables ClusterIP Services to create DNS records.

</details>

Details: [OIDC guide](docs/guides/credential-oidc.md) | [Prerequisites](docs/getting-started/prerequisites.md)

### Credential mode B: Static credentials

Works on **any Kubernetes cluster** (TKE, self-managed, EKS, etc). No cluster type or version restriction.

Using the setup script (creates Secret + generates values):

```bash
curl -sSL https://raw.githubusercontent.com/tkestack/external-dns-tencentcloud-webhook/master/scripts/setup.sh \
  | bash -s -- --auth static --zone public --domain example.com
```

Or manually:

```bash
# 1. Create Secret
kubectl create namespace external-dns
kubectl -n external-dns create secret generic tencentcloud-credentials \
  --from-literal=TENCENTCLOUD_SECRET_ID=<ID> \
  --from-literal=TENCENTCLOUD_SECRET_KEY=<KEY>

# 2. Install
helm install external-dns-public external-dns/external-dns \
  -n external-dns \
  -f https://raw.githubusercontent.com/tkestack/external-dns-tencentcloud-webhook/master/deploy/helm/values-base.yaml \
  -f values-static.yaml
```

<details>
<summary>values-static.yaml — public zone</summary>

```yaml
domainFilters:
  - example.com

txtOwnerId: <CLUSTER_ID>-public

provider:
  webhook:
    args:
      - --zone-type=public
      - --domain-filter=example.com
    env:
      - name: TENCENTCLOUD_SECRET_ID
        valueFrom:
          secretKeyRef:
            name: tencentcloud-credentials
            key: TENCENTCLOUD_SECRET_ID
      - name: TENCENTCLOUD_SECRET_KEY
        valueFrom:
          secretKeyRef:
            name: tencentcloud-credentials
            key: TENCENTCLOUD_SECRET_KEY
      - name: TENCENTCLOUD_REGION
        value: "ap-guangzhou"
```

</details>

<details>
<summary>values-static.yaml — private zone</summary>

```yaml
domainFilters:
  - internal.example.com

txtOwnerId: <CLUSTER_ID>-private

extraArgs:
  - --publish-internal-services

provider:
  webhook:
    args:
      - --zone-type=private
      - --domain-filter=internal.example.com
      - --vpc-id=vpc-xxxxxxxx
    env:
      - name: TENCENTCLOUD_SECRET_ID
        valueFrom:
          secretKeyRef:
            name: tencentcloud-credentials
            key: TENCENTCLOUD_SECRET_ID
      - name: TENCENTCLOUD_SECRET_KEY
        valueFrom:
          secretKeyRef:
            name: tencentcloud-credentials
            key: TENCENTCLOUD_SECRET_KEY
      - name: TENCENTCLOUD_REGION
        value: "ap-guangzhou"
```

</details>

Details: [Static credentials guide](docs/guides/credential-static.md)

### Managing both public and private zones

Deploy two separate Helm releases (`external-dns-public` + `external-dns-private`), each with its own permissions and values. See [public-private-zones guide](docs/guides/public-private-zones.md).

### Verify

```bash
kubectl -n external-dns get pods
# external-dns-public-xxx   2/2   Running

kubectl -n external-dns logs -l app.kubernetes.io/instance=external-dns-public -c webhook
# "Starting webhook provider server on localhost:8888"
```

### Uninstall

```bash
helm uninstall external-dns-public -n external-dns
```

## Configuration

| Webhook arg | Default | Description |
|:------------|:--------|:------------|
| `--credential-mode` | `static` | `static` or `oidc` (TKE RRSA) |
| `--zone-type` | `public` | `public` (DNSPod) or `private` (PrivateDNS) |
| `--domain-filter` | | Limit to specific domain (repeatable) |
| `--vpc-id` | | VPC ID (required for private zone) |
| `--dry-run` | `false` | Log changes without applying |

Full reference: [docs/reference/configuration.md](docs/reference/configuration.md)

## Guides

| Topic | Link |
|:------|:-----|
| Prerequisites (OIDC vs Static) | [docs/getting-started/prerequisites.md](docs/getting-started/prerequisites.md) |
| OIDC mode: how it works + troubleshooting | [docs/guides/credential-oidc.md](docs/guides/credential-oidc.md) |
| Static mode: config file migration | [docs/guides/credential-static.md](docs/guides/credential-static.md) |
| Managing public + private zones | [docs/guides/public-private-zones.md](docs/guides/public-private-zones.md) |
| Migrating from in-tree provider | [docs/guides/migration.md](docs/guides/migration.md) |
| Provider-specific annotations | [docs/reference/annotations.md](docs/reference/annotations.md) |
| FAQ | [external-dns official FAQ](https://kubernetes-sigs.github.io/external-dns/latest/docs/faq/) |

## Development

See [docs/development/](docs/development/) for building, releasing, and Taskfile reference.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
