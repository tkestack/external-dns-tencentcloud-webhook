# Managing Both Public and Private Zones

Deploy **two Helm releases** to manage public and private DNS simultaneously.

## Why two instances

Follows the [ExternalDNS recommended pattern](https://kubernetes-sigs.github.io/external-dns/v0.12.0/tutorials/public-private-route53/):

- **Security isolation** — Separate CAM roles with least-privilege (DNSPod vs PrivateDNS)
- **Operational safety** — Internal services can't accidentally modify public DNS
- **Independent lifecycle** — Upgrade, rollback, or scale each independently

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
```

## Key differences

| Setting | Public | Private |
|:--------|:-------|:--------|
| `--zone-type` | `public` | `private` |
| `--vpc-id` | _(not needed)_ | **required** |
| `--publish-internal-services` | _(not needed)_ | **recommended** (allows ClusterIP Services) |
| Service type | LoadBalancer | LoadBalancer or ClusterIP |
| `txtOwnerId` | `<cluster-id>-public` | `<cluster-id>-private` |
| CAM role | `external-dns-<cluster-id>-public` | `external-dns-<cluster-id>-private` |
| CAM policy | `QcloudDNSPodFullAccess` | `QcloudPrivateDNSFullAccess` |
| Helm release | `external-dns-public` | `external-dns-private` |

> Use different `txtOwnerId` to prevent TXT ownership record conflicts.

## Deploy

Install each zone as shown in the [README Installation section](../../README.md#installation). The Helm release name (`external-dns-public` / `external-dns-private`) ensures all resources are isolated.

## Deploy with Taskfile

```bash
task setup ZONE_TYPE=public DOMAIN=example.com
task setup ZONE_TYPE=private DOMAIN=internal.example.com
task deploy                    # deploy both

task status                    # show both
task logs                      # public logs
task logs ZONE_TYPE=private    # private logs
task down ZONE_TYPE=private    # uninstall private only
```
