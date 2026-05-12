# OIDC Credential Mode (TKE RRSA)

No static keys needed — uses Kubernetes ServiceAccount identity to obtain temporary credentials via STS.

## How it works

```
Pod starts
  → pod-identity-webhook (MutatingWebhook) injects:
    - env: TKE_ROLE_ARN, TKE_REGION, TKE_PROVIDER_ID, TKE_WEB_IDENTITY_TOKEN_FILE
    - volume: projected token (K8s-signed JWT)
  → Webhook reads env vars + JWT
  → Calls STS AssumeRoleWithWebIdentity
  → Gets temporary SecretId + SecretKey + Token (2h TTL, auto-renew)
  → Uses temp credentials to call DNSPod / PrivateDNS API
```

## Setup

See [prerequisites](../getting-started/prerequisites.md#oidc-setup) for enabling OIDC and creating the CAM role.

## Verify injection

```bash
kubectl -n external-dns get pod -o jsonpath='{.items[0].spec.containers[?(@.name=="webhook")].env[*].name}' | tr ' ' '\n'
# TKE_DEFAULT_REGION
# TKE_REGION
# TKE_PROVIDER_ID
# TKE_ROLE_ARN
# TKE_WEB_IDENTITY_TOKEN_FILE
```

## Troubleshooting

| Symptom | Cause | Fix |
|:--------|:------|:----|
| No TKE_* env vars | pod-identity-webhook not running | Enable OIDC in TKE console |
| `AssumeRoleWithWebIdentity` failed | Trust policy mismatch | Check OIDC provider matches cluster ID |
| `UnauthorizedOperation` | Missing policy | Attach `QcloudDNSPodFullAccess` / `QcloudPrivateDNSFullAccess` |
| Cluster type error | Not managed cluster | OIDC only supports TKE managed clusters, use static mode |
