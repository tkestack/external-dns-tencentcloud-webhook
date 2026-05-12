#!/usr/bin/env bash
# Setup external-dns-tencentcloud-webhook: configure credentials and generate Helm values.
#
# Usage:
#   ./scripts/setup.sh --auth static --zone private --domain example.com
#   ./scripts/setup.sh --auth oidc --zone public --domain example.com
#   ./scripts/setup.sh --auth static  # interactive prompts
#
# Prerequisites: kubectl (+ tccli, python3 for OIDC mode)

set -euo pipefail

# ── Defaults ──────────────────────────────────────────
AUTH_MODE=""
ZONE_TYPE="public"
DOMAINS=()
SECRET_ID=""
SECRET_KEY=""
REGION=""
OUTPUT_DIR="${OUTPUT_DIR:-.}"
NAMESPACE="${NAMESPACE:-external-dns}"

# ── Parse args ────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --auth)       AUTH_MODE="$2"; shift 2 ;;
    --zone)       ZONE_TYPE="$2"; shift 2 ;;
    --domain)     DOMAINS+=("$2"); shift 2 ;;
    --secret-id)  SECRET_ID="$2"; shift 2 ;;
    --secret-key) SECRET_KEY="$2"; shift 2 ;;
    --region)     REGION="$2"; shift 2 ;;
    --output)     OUTPUT_DIR="$2"; shift 2 ;;
    --namespace)  NAMESPACE="$2"; shift 2 ;;
    --help|-h)
      cat <<'USAGE'
Usage: setup.sh --auth MODE [OPTIONS]

Configure credentials and generate Helm values for external-dns-tencentcloud-webhook.

Auth modes:
  static    Use SecretId/SecretKey stored in a Kubernetes Secret
  oidc      Use TKE RRSA (requires managed cluster with OIDC enabled)

Options:
  --auth MODE        Auth mode: static or oidc (required)
  --zone TYPE        Zone type: public (default) or private
  --domain NAME      Domain to manage (repeatable, prompted if not provided, optional)
  --secret-id ID     Tencent Cloud SecretId (static mode, prompted if not provided)
  --secret-key KEY   Tencent Cloud SecretKey (static mode, prompted if not provided)
  --region REGION    Tencent Cloud region (auto-detected or prompted)
  --output DIR       Output directory for values file (default: current directory)
  --namespace NS     Kubernetes namespace (default: external-dns)
  --help, -h         Show this help

Examples:
  ./scripts/setup.sh --auth static --zone public --domain example.com
  ./scripts/setup.sh --auth oidc --zone private --domain internal.example.com
  ./scripts/setup.sh --auth static  # fully interactive
USAGE
      exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Validate auth mode ────────────────────────────────
if [[ -z "$AUTH_MODE" ]]; then
  echo "Error: --auth is required (static or oidc)" >&2
  exit 1
fi
if [[ "$AUTH_MODE" != "static" && "$AUTH_MODE" != "oidc" ]]; then
  echo "Error: --auth must be 'static' or 'oidc'" >&2
  exit 1
fi

# ── Validate prerequisites ────────────────────────────
if ! command -v kubectl >/dev/null 2>&1; then
  echo "Error: kubectl is not installed" >&2
  exit 1
fi
if [[ "$AUTH_MODE" == "oidc" ]]; then
  for cmd in tccli python3; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "Error: $cmd is required for OIDC mode" >&2
      exit 1
    fi
  done
fi

# ── Auto-detect cluster info ──────────────────────────
TKE_NETWORK_CONF=$(kubectl -n kube-system get cm tke-network-conf \
  -o jsonpath='{.data.tke-network-conf\.yaml}' 2>/dev/null || echo "")

# Cluster ID
CLUSTER_ID=$(echo "$TKE_NETWORK_CONF" | grep -oE 'cls-[a-z0-9]+' | head -1 || echo "")
if [ -z "$CLUSTER_ID" ]; then
  printf "Could not auto-detect cluster ID.\nEnter cluster ID (e.g. cls-xxxxxxxx): "
  read -r CLUSTER_ID
  if [[ ! "$CLUSTER_ID" =~ ^cls-[a-z0-9]+$ ]]; then
    echo "Error: invalid cluster ID format" >&2
    exit 1
  fi
fi

# Region
if [ -z "$REGION" ]; then
  REGION=$(echo "$TKE_NETWORK_CONF" | sed -n 's/.*api-env\.region: *"\([^"]*\)".*/\1/p' || echo "")
fi
if [ -z "$REGION" ]; then
  printf "Could not auto-detect region.\nEnter region (e.g. ap-guangzhou): "
  read -r REGION
  if [ -z "$REGION" ]; then
    echo "Error: region is required" >&2
    exit 1
  fi
fi

# VPC ID (for private zone)
VPC_ID=""
if [ "$ZONE_TYPE" = "private" ]; then
  VPC_ID=$(echo "$TKE_NETWORK_CONF" | sed -n 's/.*vpc-cni\.vpc-id: *"\([^"]*\)".*/\1/p' || echo "")
  if [ -z "$VPC_ID" ]; then
    printf "Enter VPC ID (for private zone binding): "
    read -r VPC_ID
  fi
fi

# ══════════════════════════════════════════════════════
#  Auth-specific setup
# ══════════════════════════════════════════════════════

if [[ "$AUTH_MODE" == "oidc" ]]; then
  # ── OIDC: Owner UIN ─────────────────────────────────
  OWNER_UIN=$(tccli cam GetUserAppId --output json 2>/dev/null \
    | python3 -c "import json,sys;print(json.load(sys.stdin)['OwnerUin'])" 2>/dev/null || echo "")
  if [ -z "$OWNER_UIN" ]; then
    printf "Could not auto-detect owner UIN.\nEnter owner UIN: "
    read -r OWNER_UIN
    if [ -z "$OWNER_UIN" ]; then
      echo "Error: owner UIN is required" >&2
      exit 1
    fi
  fi

  # ── OIDC: Check cluster type and version ────────────
  CLUSTER_INFO=$(tccli tke DescribeClusters --ClusterIds "[\"$CLUSTER_ID\"]" --region "$REGION" --output json 2>/dev/null || echo "")
  if [ -z "$CLUSTER_INFO" ]; then
    echo "Error: Failed to query cluster info" >&2
    exit 1
  fi

  CLUSTER_CHECK=$(echo "$CLUSTER_INFO" | python3 -c "
import json, sys

d = json.load(sys.stdin)
clusters = d.get('Clusters', [])
if not clusters:
    print('error:Cluster not found')
    sys.exit(0)

c = clusters[0]
cluster_type = c.get('ClusterType', '')
cluster_version = c.get('ClusterVersion', '0')

if cluster_type != 'MANAGED_CLUSTER':
    print(f'error:Cluster type is {cluster_type}, but OIDC (RRSA) only supports MANAGED_CLUSTER')
    sys.exit(0)

try:
    base = cluster_version.split('-')[0]
    parts = tuple(int(x) for x in base.split('.'))
    if parts < (1, 20, 6):
        print(f'error:Cluster version {cluster_version} too old (requires >= 1.20.6-tke.27 or >= 1.22.5-tke.1)')
        sys.exit(0)
    if parts[:2] == (1, 21):
        print(f'error:Cluster version 1.21.x does not support OIDC (requires >= 1.22.5-tke.1)')
        sys.exit(0)
    if parts[:2] == (1, 22) and parts[2] < 5:
        print(f'error:Cluster version {cluster_version} too old (requires >= 1.22.5-tke.1)')
        sys.exit(0)
except Exception:
    pass

print(f'ok:{cluster_type} v{cluster_version}')
" 2>/dev/null || echo "error:Failed to parse cluster info")

  if [[ "$CLUSTER_CHECK" == error:* ]]; then
    echo "Error: ${CLUSTER_CHECK#error:}" >&2
    exit 1
  fi
  echo "  Cluster:    ${CLUSTER_CHECK#ok:}"

  # ── OIDC: Check OIDC status ─────────────────────────
  OIDC_STATUS=$(tccli tke DescribeClusterAuthenticationOptions \
    --ClusterId "$CLUSTER_ID" --region "$REGION" --output json 2>/dev/null || echo "")
  if [ -z "$OIDC_STATUS" ]; then
    echo "Error: Failed to query cluster OIDC status" >&2
    exit 1
  fi

  OIDC_ENABLED=$(echo "$OIDC_STATUS" | python3 -c "
import json,sys
d=json.load(sys.stdin)
oidc = d.get('OIDCConfig', {})
sa = d.get('ServiceAccounts', {})
ok = oidc.get('AutoCreateOIDCConfig') and oidc.get('AutoInstallPodIdentityWebhookAddon') and sa.get('Issuer')
print('true' if ok else 'false')
" 2>/dev/null || echo "false")

  if [ "$OIDC_ENABLED" != "true" ]; then
    echo "Error: OIDC not enabled on cluster $CLUSTER_ID" >&2
    echo "Enable it in TKE console: Cluster → Basic Info → ServiceAccountIssuerDiscovery" >&2
    exit 1
  fi

  # ── OIDC: Create CAM role ───────────────────────────
  ROLE_NAME="external-dns-${CLUSTER_ID}-${ZONE_TYPE}"
  if [ "$ZONE_TYPE" = "private" ]; then
    POLICY_NAME="QcloudPrivateDNSFullAccess"
  else
    POLICY_NAME="QcloudDNSPodFullAccess"
  fi

  POLICY_DOC="{\"version\":\"2.0\",\"statement\":[{\"effect\":\"allow\",\"action\":\"name/sts:AssumeRoleWithWebIdentity\",\"principal\":{\"federated\":[\"qcs::cam::uin/${OWNER_UIN}:oidc-provider/${CLUSTER_ID}\"]},\"condition\":{\"string_equal\":{\"oidc:aud\":\"sts.cloud.tencent.com\"}}}]}"

  if tccli cam GetRole --RoleName "$ROLE_NAME" >/dev/null 2>&1; then
    echo "Role $ROLE_NAME already exists, skipping"
  else
    echo "Creating CAM role: $ROLE_NAME"
    tccli cam CreateRole \
      --RoleName "$ROLE_NAME" \
      --PolicyDocument "$POLICY_DOC" \
      --Description "external-dns webhook OIDC role for $CLUSTER_ID ($ZONE_TYPE)"
    echo "Role created"
  fi

  # ── OIDC: Attach policy ─────────────────────────────
  ATTACHED=$(tccli cam ListAttachedRolePolicies \
    --RoleName "$ROLE_NAME" --Page 1 --Rp 100 --output json 2>/dev/null \
    | python3 -c "import json,sys;d=json.load(sys.stdin);print(' '.join(p['PolicyName'] for p in d.get('List',[])))" 2>/dev/null || echo "")

  if echo "$ATTACHED" | grep -qw "$POLICY_NAME"; then
    echo "Policy $POLICY_NAME already attached, skipping"
  else
    echo "Attaching policy: $POLICY_NAME"
    tccli cam AttachRolePolicy \
      --AttachRoleName "$ROLE_NAME" \
      --PolicyName "$POLICY_NAME"
    echo "Policy attached"
  fi

elif [[ "$AUTH_MODE" == "static" ]]; then
  # ── Static: Collect credentials ─────────────────────
  TCCLI_CRED="${HOME}/.tccli/default.credential"
  if [ -z "$SECRET_ID" ] && [ -z "$SECRET_KEY" ] && [ -f "$TCCLI_CRED" ]; then
    _TCCLI_ID=$(python3 -c "import json;d=json.load(open('$TCCLI_CRED'));print(d.get('secretId',''))" 2>/dev/null || echo "")
    if [ -n "$_TCCLI_ID" ]; then
      _MASKED="${_TCCLI_ID:0:6}****${_TCCLI_ID: -4}"
      printf "Found tccli credentials (SecretId: %s). Use them? [Y/n] " "$_MASKED"
      read -r _CONFIRM
      if [[ -z "$_CONFIRM" || "$_CONFIRM" =~ ^[Yy] ]]; then
        SECRET_ID="$_TCCLI_ID"
        SECRET_KEY=$(python3 -c "import json;d=json.load(open('$TCCLI_CRED'));print(d.get('secretKey',''))" 2>/dev/null || echo "")
      fi
    fi
  fi

  if [ -z "$SECRET_ID" ]; then
    printf "Enter Tencent Cloud SecretId: "
    read -r SECRET_ID
    if [ -z "$SECRET_ID" ]; then
      echo "Error: SecretId is required" >&2
      exit 1
    fi
  fi

  if [ -z "$SECRET_KEY" ]; then
    printf "Enter Tencent Cloud SecretKey: "
    read -rs SECRET_KEY
    echo ""
    if [ -z "$SECRET_KEY" ]; then
      echo "Error: SecretKey is required" >&2
      exit 1
    fi
  fi

  # ── Static: Create Secret ───────────────────────────
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null 2>&1
  if kubectl -n "$NAMESPACE" get secret tencentcloud-credentials >/dev/null 2>&1; then
    echo "Secret tencentcloud-credentials already exists, updating..."
    kubectl -n "$NAMESPACE" delete secret tencentcloud-credentials >/dev/null 2>&1
  fi
  kubectl -n "$NAMESPACE" create secret generic tencentcloud-credentials \
    --from-literal=TENCENTCLOUD_SECRET_ID="$SECRET_ID" \
    --from-literal=TENCENTCLOUD_SECRET_KEY="$SECRET_KEY"
  echo "Secret created in namespace $NAMESPACE"
fi

# ══════════════════════════════════════════════════════
#  Common: domains & values generation
# ══════════════════════════════════════════════════════

# ── Ask for domains if not provided ───────────────────
if [ ${#DOMAINS[@]} -eq 0 ]; then
  printf "Enter domain(s) to manage, comma-separated (optional, press Enter to skip): "
  read -r _INPUT
  if [ -n "$_INPUT" ]; then
    IFS=',' read -ra DOMAINS <<< "$_INPUT"
  fi
fi

# ── Print summary ────────────────────────────────────
echo ""
echo "Detected:"
echo "  Cluster ID: $CLUSTER_ID"
echo "  Region:     $REGION"
echo "  Auth mode:  $AUTH_MODE"
echo "  Zone type:  $ZONE_TYPE"
if [ ${#DOMAINS[@]} -gt 0 ]; then
  echo "  Domains:    ${DOMAINS[*]}"
fi
[ -n "$VPC_ID" ] && echo "  VPC ID:     $VPC_ID"
echo ""

# ── Generate values file ──────────────────────────────
VALUES_FILE="${OUTPUT_DIR}/values-${ZONE_TYPE}.local.yaml"
mkdir -p "$OUTPUT_DIR"

# Build domainFilters YAML block
DOMAIN_FILTERS=""
DOMAIN_FILTER_ARGS=""
if [ ${#DOMAINS[@]} -gt 0 ]; then
  DOMAIN_FILTERS="domainFilters:"
  for d in "${DOMAINS[@]}"; do
    d=$(echo "$d" | xargs)  # trim whitespace
    DOMAIN_FILTERS="${DOMAIN_FILTERS}
  - ${d}"
    DOMAIN_FILTER_ARGS="${DOMAIN_FILTER_ARGS}
      - --domain-filter=${d}"
  done
fi

VPC_ARGS=""
if [ "$ZONE_TYPE" = "private" ] && [ -n "$VPC_ID" ]; then
  VPC_ARGS="      - --vpc-id=${VPC_ID}"
fi

if [[ "$AUTH_MODE" == "oidc" ]]; then
  ROLE_ARN="qcs::cam::uin/${OWNER_UIN}:roleName/${ROLE_NAME}"
  cat > "$VALUES_FILE" <<EOF
# Generated by: scripts/setup.sh --auth oidc --zone ${ZONE_TYPE}
# Do not commit — *.local.yaml is gitignored

serviceAccount:
  annotations:
    tke.cloud.tencent.com/role-arn: "${ROLE_ARN}"
    tke.cloud.tencent.com/audience: "sts.cloud.tencent.com"
    tke.cloud.tencent.com/token-expiration: "86400"

${DOMAIN_FILTERS}

txtOwnerId: ${CLUSTER_ID}-${ZONE_TYPE}

provider:
  webhook:
    args:
      - --credential-mode=oidc
      - --zone-type=${ZONE_TYPE}${DOMAIN_FILTER_ARGS}
${VPC_ARGS}
EOF

elif [[ "$AUTH_MODE" == "static" ]]; then
  cat > "$VALUES_FILE" <<EOF
# Generated by: scripts/setup.sh --auth static --zone ${ZONE_TYPE}
# Do not commit — *.local.yaml is gitignored

${DOMAIN_FILTERS}

txtOwnerId: ${CLUSTER_ID}-${ZONE_TYPE}

provider:
  webhook:
    env:
      - name: TENCENTCLOUD_SECRET_ID
        valueFrom:
          secretKeyRef:
            name: tencentcloud-credentials
            key: TENCENTCLOUD_SECRET_ID
      - name: TENCENTCLOUD_SECRET_KEY
        valueFrom:
          secretKeyRef:
            name: tencentcloud-credentials
            key: TENCENTCLOUD_SECRET_KEY
      - name: TENCENTCLOUD_REGION
        value: "${REGION}"
    args:
      - --zone-type=${ZONE_TYPE}${DOMAIN_FILTER_ARGS}
${VPC_ARGS}
EOF
fi

# ── Output ────────────────────────────────────────────
echo "════════════════════════════════════════════"
echo "  Setup complete"
echo "════════════════════════════════════════════"
echo ""
echo "  Values:  $VALUES_FILE"
if [[ "$AUTH_MODE" == "oidc" ]]; then
  echo "  Role:    $ROLE_NAME"
  echo "  Policy:  $POLICY_NAME"
elif [[ "$AUTH_MODE" == "static" ]]; then
  echo "  Secret:  tencentcloud-credentials (ns: $NAMESPACE)"
fi
if [ ${#DOMAINS[@]} -gt 0 ]; then
  echo "  Domains: ${DOMAINS[*]}"
else
  echo "  Domains: (all)"
fi
echo ""
echo "Deploy with:"
echo "  helm install external-dns-${ZONE_TYPE} external-dns/external-dns \\"
echo "    -n ${NAMESPACE} --create-namespace \\"
echo "    -f deploy/helm/values-base.yaml \\"
echo "    -f ${VALUES_FILE}"
