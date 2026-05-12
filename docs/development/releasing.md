# Releasing

## Version scheme

Follows [semver](https://semver.org/): `v<major>.<minor>.<patch>`, starting at `v0.1.0`.

## Release process

```bash
# 1. Tag the release
task release TAG=v0.1.0

# This will:
#   - Verify working tree is clean
#   - Build and push image with tags: v0.1.0, latest
#   - Create git tag v0.1.0
```

## Manual release

```bash
# Tag
git tag v0.1.0

# Build and push
KO_DOCKER_REPO=ccr.ccs.tencentyun.com/tkeimages/external-dns-tencentcloud-webhook \
  ko build --bare --tags v0.1.0,latest --platform=linux/amd64 --sbom=none .

# Push tag
git push origin v0.1.0
```

## Image registry

| Registry | Image |
|:---------|:------|
| CCR | `ccr.ccs.tencentyun.com/tkeimages/external-dns-tencentcloud-webhook` |

## Updating external-dns base image

When a new external-dns version is released:

1. Pull and push to CCR:
   ```bash
   podman pull --platform linux/amd64 registry.k8s.io/external-dns/external-dns:<new-version>
   podman tag registry.k8s.io/external-dns/external-dns:<new-version> ccr.ccs.tencentyun.com/tkeimages/external-dns:<new-version>
   podman push ccr.ccs.tencentyun.com/tkeimages/external-dns:<new-version>
   ```

2. Update `deploy/helm/values-base.yaml`:
   ```yaml
   image:
     tag: <new-version>
   ```
