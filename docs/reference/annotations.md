# Provider-Specific Annotations

Fine-grained DNS record control via annotations on Service/Ingress. These map to Tencent Cloud API fields that external-dns doesn't natively support.

## Supported Annotations

| Annotation | DNSPod | PrivateDNS | Description |
|:-----------|:------:|:----------:|:------------|
| `external-dns.alpha.kubernetes.io/webhook-record-line` | ✅ | — | DNS record line (`默认`, `电信`, `联通`, `移动`) |
| `external-dns.alpha.kubernetes.io/webhook-record-line-id` | ✅ | — | Record line ID (overrides `record-line`) |
| `external-dns.alpha.kubernetes.io/webhook-weight` | ✅ | ✅ | Weight for load balancing (0-100) |
| `external-dns.alpha.kubernetes.io/webhook-mx` | ✅ | ✅ | MX priority (required for MX records) |
| `external-dns.alpha.kubernetes.io/webhook-remark` | ✅ | ✅ | Record remark/comment |
| `external-dns.alpha.kubernetes.io/webhook-status` | ✅ | — | Initial status (`ENABLE` or `DISABLE`) |

## How it works

ExternalDNS natively converts `external-dns.alpha.kubernetes.io/webhook-*` annotations into `webhook/*` ProviderSpecific properties on Endpoint objects. The webhook provider reads these properties when creating DNS records.

## Record Line (线路分组)

DNSPod supports DNS resolution by carrier/region line. Records on different lines are treated as separate endpoints — external-dns will **not** merge or interfere with records on other lines.

### Single line

For a single non-default line:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: app
  annotations:
    external-dns.alpha.kubernetes.io/hostname: app.example.com
    external-dns.alpha.kubernetes.io/set-identifier: "电信"
    external-dns.alpha.kubernetes.io/webhook-record-line: "电信"
spec:
  type: LoadBalancer
  ports:
  - port: 80
```

> **Note:** `set-identifier` is recommended even for single-line usage. It ensures correct behavior if you later add more lines for the same hostname.

### Multi-line (same hostname, different lines)

To have the same hostname resolve to different IPs per line, use `set-identifier` with **the same value as `record-line`**:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: app-dianxin
  annotations:
    external-dns.alpha.kubernetes.io/hostname: app.example.com
    external-dns.alpha.kubernetes.io/set-identifier: "电信"
    external-dns.alpha.kubernetes.io/webhook-record-line: "电信"
spec:
  type: LoadBalancer
  ports:
  - port: 80
---
apiVersion: v1
kind: Service
metadata:
  name: app-liantong
  annotations:
    external-dns.alpha.kubernetes.io/hostname: app.example.com
    external-dns.alpha.kubernetes.io/set-identifier: "联通"
    external-dns.alpha.kubernetes.io/webhook-record-line: "联通"
spec:
  type: LoadBalancer
  ports:
  - port: 80
```

> **Why `set-identifier`?** ExternalDNS uses `set-identifier` to distinguish multiple record sets with the same hostname. Without it, targets from both Services would be merged into one record. The value must match `record-line` so the webhook can correctly associate records when reading them back from DNSPod.

### Coexistence with manually-created records

Records created manually in DNSPod (e.g., geo-blocking lines) are **not affected** by external-dns. The webhook only deletes records that match both the same hostname **and** the same line. This means you can safely:

1. Let external-dns manage `默认` line records automatically
2. Manually add blocking rules on custom lines (e.g., `境外封禁` → `0.0.0.0`)

External-dns will not touch your manually-created records on other lines.

## Other Examples

> **Note:** The following examples use ClusterIP Services with `external-dns.alpha.kubernetes.io/target` to specify a fixed record value. This requires `--publish-internal-services` enabled on the external-dns instance (set via `extraArgs` in Helm values).

### MX record

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mail
  annotations:
    external-dns.alpha.kubernetes.io/hostname: example.com
    external-dns.alpha.kubernetes.io/target: "mail.example.com"
    external-dns.alpha.kubernetes.io/webhook-mx: "10"
spec:
  type: ClusterIP
  ports:
  - port: 25
```

### Disabled record (created but not resolving)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: staging
  annotations:
    external-dns.alpha.kubernetes.io/hostname: staging.example.com
    external-dns.alpha.kubernetes.io/target: "10.0.0.1"
    external-dns.alpha.kubernetes.io/webhook-status: "DISABLE"
    external-dns.alpha.kubernetes.io/webhook-remark: "not ready for production"
spec:
  type: ClusterIP
  ports:
  - port: 80
```
