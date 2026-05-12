# Configuration Reference

## Command-Line Flags

| Flag | Env Variable | Default | Description |
|:-----|:-------------|:--------|:------------|
| `--credential-mode` | | `static` | `static` (SecretId/SecretKey) or `oidc` (TKE RRSA) |
| `--config-file` | | | JSON config file (compatible with in-tree `--tencent-cloud-config-file`) |
| `--region` | `TENCENTCLOUD_REGION` | | Tencent Cloud region (e.g., `ap-guangzhou`) |
| `--secret-id` | `TENCENTCLOUD_SECRET_ID` | | API SecretId (static mode only) |
| `--secret-key` | `TENCENTCLOUD_SECRET_KEY` | | API SecretKey (static mode only) |
| `--zone-type` | | `public` | `public` (DNSPod) or `private` (PrivateDNS). See [public-private-zones](../guides/public-private-zones.md) for managing both. |
| `--vpc-id` | | | VPC ID (required for `--zone-type=private`) |
| `--domain-filter` | | | Limit to specific domains (repeatable, optional — omit to manage all) |
| `--zone-id-filter` | | | Limit to specific zone IDs (repeatable, private only) |
| `--dry-run` | | `false` | Log changes without applying |
| `--internet-endpoint` | | `true` | Use public API endpoint (`false` for VPC internal) |
| `--api-rate` | | `9` | API rate limit per second |
| `--provider-port` | | `localhost:8888` | Webhook API listen address |
| `--health-port` | | `:8080` | Health check listen address |
| `--log-level` | | `info` | `debug`, `info`, `warn`, `error` |

## Credential Resolution Order

### Static mode (default)

Priority: flag → env → config file

1. `--secret-id` / `--secret-key`
2. `TENCENTCLOUD_SECRET_ID` / `TENCENTCLOUD_SECRET_KEY`
3. `--config-file` (JSON with `secretId` / `secretKey` fields)

### OIDC mode

No static keys needed. Reads from env vars injected by `pod-identity-webhook`:

- `TKE_ROLE_ARN`
- `TKE_REGION`
- `TKE_PROVIDER_ID`
- `TKE_WEB_IDENTITY_TOKEN_FILE`

## Config File Format

Compatible with the former in-tree provider's `--tencent-cloud-config-file`:

```json
{
  "regionId": "ap-guangzhou",
  "secretId": "AKIDxxxxxxxx",
  "secretKey": "xxxxxxxx",
  "vpcId": "vpc-xxxxxxxx",
  "internetEndpoint": true
}
```

All fields are optional — flags and env vars override config file values.

## Required Permissions

| Service | Actions |
|:--------|:--------|
| DNSPod (public) | `dnspod:DescribeDomainList`, `dnspod:DescribeRecordList`, `dnspod:CreateRecord`, `dnspod:DeleteRecord`, `dnspod:ModifyRecord` |
| PrivateDNS (private) | `privatedns:DescribePrivateZoneList`, `privatedns:DescribePrivateZoneRecordList`, `privatedns:CreatePrivateZoneRecord`, `privatedns:DeletePrivateZoneRecord`, `privatedns:ModifyPrivateZoneRecord` |

## Internal API Endpoint

Set `--internet-endpoint=false` for workloads inside Tencent Cloud VPC. Uses `*.internal.tencentcloudapi.com` endpoints, avoiding public network traffic.

## Webhook API

| Endpoint | Method | Description |
|:---------|:-------|:------------|
| `/` | GET | Negotiate — returns DomainFilter |
| `/records` | GET | List current DNS records |
| `/records` | POST | Apply changes (create/update/delete) |
| `/adjustendpoints` | POST | Adjust endpoints (pass-through) |
| `/healthz` | GET | Health check |

Content-Type: `application/external.dns.webhook+json;version=1`
