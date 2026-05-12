# Static Credential Mode

Use permanent SecretId/SecretKey. Works on **any Kubernetes cluster** — no TKE version or OIDC requirements.

## Setup

See [prerequisites](../getting-started/prerequisites.md#static-credentials-setup) for creating the Secret.

## Config file (migration from in-tree provider)

If migrating from the in-tree provider's `--tencent-cloud-config-file`, mount the existing JSON config:

```yaml
# In your Helm values
provider:
  webhook:
    args:
      - --config-file=/etc/kubernetes/tencent-cloud.json
      - --zone-type=public
    extraVolumeMounts:
      - name: config
        mountPath: /etc/kubernetes
        readOnly: true

extraVolumes:
  - name: config
    secret:
      secretName: tencent-cloud-config
```

Config file format:

```json
{
  "regionId": "ap-guangzhou",
  "secretId": "AKIDxxxxxxxx",
  "secretKey": "xxxxxxxx",
  "vpcId": "vpc-xxxxxxxx",
  "internetEndpoint": true
}
```

All fields are optional — flags and env vars override config file values. See [configuration reference](../reference/configuration.md).
