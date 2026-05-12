# Building

## Prerequisites

- Go 1.24+
- [ko](https://ko.build/) (for container images)
- Access to `ccr.ccs.tencentyun.com/tkeimages` registry

## Binary

```bash
go build -o external-dns-tencentcloud-webhook .
```

## Container image (ko)

```bash
# Dev image (latest tag)
task build

# Or manually:
KO_DOCKER_REPO=ccr.ccs.tencentyun.com/tkeimages/external-dns-tencentcloud-webhook \
  ko build --bare --tags latest --platform=linux/amd64 --sbom=none .
```

## Container image (Docker)

```bash
docker build -t ccr.ccs.tencentyun.com/tkeimages/external-dns-tencentcloud-webhook:latest .
```

## Multi-platform

```bash
KO_DOCKER_REPO=ccr.ccs.tencentyun.com/tkeimages/external-dns-tencentcloud-webhook \
  ko build --bare --tags latest --platform=linux/amd64,linux/arm64 --sbom=none .
```
