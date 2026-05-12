# Migrating from In-tree Provider

The `--provider=tencentcloud` was removed in external-dns v0.18.0. This guide helps you migrate to the webhook provider.

## What changed

| Before (in-tree) | After (webhook) |
|:---|:---|
| Single binary, built-in provider | Two containers in one Pod (sidecar) |
| `--provider=tencentcloud` | `--provider=webhook --webhook-provider-url=http://localhost:8888` |
| `--tencent-cloud-config-file` | `--config-file` (same JSON format) |
| `--tencent-cloud-zone-type` | `--zone-type` |
| Credentials in JSON file only | JSON file, env vars, flags, or OIDC |

## Migration steps

### 1. Keep your existing config file

The JSON format is 100% compatible:

```json
{
  "regionId": "ap-guangzhou",
  "secretId": "AKIDxxxxxxxx",
  "secretKey": "xxxxxxxx",
  "vpcId": "vpc-xxxxxxxx",
  "internetEndpoint": true
}
```

### 2. Update the Deployment

**Before:**

```yaml
containers:
- name: external-dns
  image: registry.k8s.io/external-dns/external-dns:v0.14.2
  args:
  - --provider=tencentcloud
  - --tencent-cloud-config-file=/etc/kubernetes/tencent-cloud.json
  - --tencent-cloud-zone-type=public
  - --domain-filter=example.com
  volumeMounts:
  - name: config
    mountPath: /etc/kubernetes
    readOnly: true
```

**After:**

```yaml
containers:
- name: external-dns
  image: registry.k8s.io/external-dns/external-dns:v0.21.0
  args:
  - --provider=webhook
  - --webhook-provider-url=http://localhost:8888
  - --domain-filter=example.com
- name: tencentcloud-webhook
  image: <your-registry>/external-dns-tencentcloud-webhook:latest
  args:
  - --config-file=/etc/kubernetes/tencent-cloud.json
  - --zone-type=public
  - --domain-filter=example.com
  volumeMounts:
  - name: config
    mountPath: /etc/kubernetes
    readOnly: true
  ports:
  - containerPort: 8888
  - containerPort: 8080
```

### 3. Flag mapping

| In-tree flag | Webhook flag |
|:---|:---|
| `--tencent-cloud-config-file` | `--config-file` |
| `--tencent-cloud-zone-type=public` | `--zone-type=public` |
| `--domain-filter` | `--domain-filter` (on both containers) |
| `--zone-id-filter` | `--zone-id-filter` |
| `--dry-run` | `--dry-run` |

### 4. Optional: switch to OIDC

After migration, consider switching to [OIDC mode](credential-oidc.md) to eliminate static keys entirely.
