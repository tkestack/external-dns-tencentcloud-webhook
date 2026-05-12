# Changelog

## [Unreleased]

## [0.1.0](https://github.com/tkestack/external-dns-tencentcloud-webhook/commits/v0.1.0) (2026-05-12)

### Added

- ExternalDNS webhook provider for Tencent Cloud (DNSPod public zone & PrivateDNS)
- OIDC credential mode (TKE RRSA) — no static keys needed
- Static credential mode — works on any Kubernetes cluster
- Record line support — records on different lines are treated as separate endpoints, preventing interference between lines
- Multi-line support — same hostname on different lines via `set-identifier` annotation (e.g., 电信 + 联通)
- Provider-specific annotations: record-line, record-line-id, weight, MX, remark, status
- Line-aware delete — only removes records matching the same line, preserving manually-created records on other lines (e.g., geo-blocking)
- Idempotent create — `DomainRecordExist` errors are treated as success for safe retries
- AdjustEndpoints normalization aligned with Records() output to prevent spurious update loops
- `scripts/setup.sh` — automated credential setup and Helm values generation
- Helm-based deployment using official external-dns chart with preset values files
- Support for public and private zones via separate Helm releases
- Taskfile for one-command operations (setup/build/deploy/logs)
- Ginkgo e2e acceptance tests (OIDC + static, public + private, record-line)
- Unit tests for DNSPod provider (line grouping, set-identifier, line-aware delete, weight, status round-trip)
- Structured documentation (getting-started, guides, reference, development)

### Compatibility

- ExternalDNS v0.20.0+ (webhook provider protocol)
- Replaces removed in-tree `--provider=tencentcloud` (removed in v0.18.0)
