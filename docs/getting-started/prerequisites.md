# Prerequisites

## Requirements by credential mode

| | OIDC (recommended) | Static credentials |
|:--|:-------------------|:-------------------|
| **Cluster type** | TKE managed cluster only | Any Kubernetes cluster |
| **Cluster version** | `>= v1.20.6-tke.27` or `>= v1.22.5-tke.1` | No restriction |
| **OIDC enabled** | Required | Not needed |
| **Static keys** | Not needed | Required (SecretId/SecretKey) |

## OIDC setup

### 1. Enable OIDC on cluster

TKE Console → Cluster → Basic Info → ServiceAccountIssuerDiscovery → Edit:

- [x] Create CAM OIDC Provider
- [x] Create webhook component
- Client ID: `sts.cloud.tencent.com`

Ref: [TKE RRSA Documentation](https://cloud.tencent.com/document/product/457/81989)

Verify:

```bash
tccli tke DescribeClusterAuthenticationOptions \
  --ClusterId <CLUSTER_ID> --region <REGION> --output json
# OIDCConfig.AutoCreateOIDCConfig = true
# ServiceAccounts.Issuer is not empty
```

### 2. Create CAM role

#### Automated (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/tkestack/external-dns-tencentcloud-webhook/master/scripts/setup.sh \
  | bash -s -- --auth oidc --zone public --domain example.com
```

The script checks cluster type/version/OIDC status, creates role, attaches policy, generates Helm values.

#### Manual

1. [CAM Console](https://console.cloud.tencent.com/cam/role) → New Role → Identity Provider
2. Select the OIDC provider for your cluster
3. Condition: `oidc:aud` = `sts.cloud.tencent.com`
4. Attach policy: `QcloudDNSPodFullAccess` (public) or `QcloudPrivateDNSFullAccess` (private)

Role naming convention: `external-dns-<cluster-id>-<zone-type>`

## Static credentials setup

No cluster type or version restriction. Just create a Secret:

```bash
kubectl create namespace external-dns
kubectl -n external-dns create secret generic tencentcloud-credentials \
  --from-literal=TENCENTCLOUD_SECRET_ID=<ID> \
  --from-literal=TENCENTCLOUD_SECRET_KEY=<KEY>
```
