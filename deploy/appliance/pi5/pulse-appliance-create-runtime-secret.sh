#!/usr/bin/env bash
set -euo pipefail

kubeconfig="${PULSE_APPLIANCE_KUBECONFIG:-${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}}"
kube_context="${KUBECONFIG_CONTEXT:-}"
kubectl_bin="${KUBECTL:-kubectl}"
services_namespace="${SERVICES_NAMESPACE:-pulse-services}"
platform_namespace="${PLATFORM_NAMESPACE:-pulse-platform}"
runtime_secret="${PULSE_SERVICES_RUNTIME_SECRET:-pulse-services-runtime-secret}"
gcs_secret="${PULSE_SERVICES_GCS_SECRET:-pulse-services-gcs-credentials}"
gcs_secret_key="${PULSE_SERVICES_GCS_SECRET_KEY:-key.json}"
gcs_mount_path="${PULSE_SERVICES_GCS_MOUNT_PATH:-/var/run/pulse-gcs}"
db_secret="${PULSE_PLATFORM_DB_SECRET:-pulse-platform-core-app}"
db_service="${PULSE_PLATFORM_DB_SERVICE:-pulse-platform-core-rw}"
db_name="${PULSE_PLATFORM_DB_NAME:-pulse}"
db_port="${PULSE_PLATFORM_DB_PORT:-5432}"
archive_writer_id="${ARCHIVE_WRITER_ID:-pulse-pi5}"
archive_bucket="${ARCHIVE_OBJECT_BUCKET:-pulse-telemetry-raw}"
archive_prefix="${ARCHIVE_OBJECT_PREFIX:-raw}"
archive_region="${ARCHIVE_OBJECT_REGION:-us-east1}"
archive_project_id="${ARCHIVE_OBJECT_GCS_PROJECT_ID:-}"
gcs_credentials_file=""

usage() {
  cat <<'USAGE'
Usage: pulse-appliance-create-runtime-secret.sh --gcs-credentials FILE [options]

Creates the local-only Kubernetes secrets required by the Pi services chart.
Run after the platform chart has created the CNPG app secret and before
applying pulse-services.

Options:
  --gcs-credentials FILE  Service-account JSON file for GCS archive access.
  --gcs-project-id ID     GCS project id to expose to services.
  --archive-writer-id ID  Archive writer id, default pulse-pi5.
  --archive-bucket NAME   Archive bucket, default pulse-telemetry-raw.
  --archive-prefix PREFIX Archive object prefix, default raw.
  --archive-region REGION
                          Archive object region, default us-east1.
  --namespace NAME        Services namespace, default pulse-services.
  --platform-namespace NAME
                          Platform namespace, default pulse-platform.
  --runtime-secret NAME   Runtime secret name, default pulse-services-runtime-secret.
  --gcs-secret NAME       GCS credentials secret name, default pulse-services-gcs-credentials.
  --kubeconfig PATH       Kubeconfig path, default /etc/rancher/k3s/k3s.yaml.
  --kube-context NAME     Explicit kube context.
  -h, --help              Show this help.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --gcs-credentials)
      gcs_credentials_file="${2:?--gcs-credentials requires a file}"
      shift
      ;;
    --gcs-project-id)
      archive_project_id="${2:?--gcs-project-id requires a value}"
      shift
      ;;
    --archive-writer-id)
      archive_writer_id="${2:?--archive-writer-id requires a value}"
      shift
      ;;
    --archive-bucket)
      archive_bucket="${2:?--archive-bucket requires a value}"
      shift
      ;;
    --archive-prefix)
      archive_prefix="${2:?--archive-prefix requires a value}"
      shift
      ;;
    --archive-region)
      archive_region="${2:?--archive-region requires a value}"
      shift
      ;;
    --namespace)
      services_namespace="${2:?--namespace requires a value}"
      shift
      ;;
    --platform-namespace)
      platform_namespace="${2:?--platform-namespace requires a value}"
      shift
      ;;
    --runtime-secret)
      runtime_secret="${2:?--runtime-secret requires a value}"
      shift
      ;;
    --gcs-secret)
      gcs_secret="${2:?--gcs-secret requires a value}"
      shift
      ;;
    --kubeconfig)
      kubeconfig="${2:?--kubeconfig requires a path}"
      shift
      ;;
    --kube-context)
      kube_context="${2:?--kube-context requires a value}"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

if [ -z "$gcs_credentials_file" ]; then
  echo "--gcs-credentials is required" >&2
  usage >&2
  exit 2
fi
if [ ! -f "$gcs_credentials_file" ]; then
  echo "GCS credentials file not found: $gcs_credentials_file" >&2
  exit 1
fi

kubectl_base=("$kubectl_bin")
if [ -n "$kubeconfig" ]; then
  kubectl_base+=(--kubeconfig "$kubeconfig")
fi
if [ -n "$kube_context" ]; then
  kubectl_base+=(--context "$kube_context")
fi

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 not found; install it before creating appliance runtime secrets" >&2
    exit 1
  fi
}

jsonpath_b64() {
  local namespace="$1"
  local secret="$2"
  local key="$3"
  "${kubectl_base[@]}" -n "$namespace" get secret "$secret" -o "jsonpath={.data.$key}" | base64 -d
}

urlencode() {
  jq -rn --arg value "$1" '$value|@uri'
}

require_command "$kubectl_bin"
require_command jq

"${kubectl_base[@]}" create namespace "$services_namespace" --dry-run=client -o yaml \
  | "${kubectl_base[@]}" apply -f -

db_user="$(jsonpath_b64 "$platform_namespace" "$db_secret" username)"
db_password="$(jsonpath_b64 "$platform_namespace" "$db_secret" password)"
db_user_encoded="$(urlencode "$db_user")"
db_password_encoded="$(urlencode "$db_password")"
db_dsn="postgres://${db_user_encoded}:${db_password_encoded}@${db_service}.${platform_namespace}.svc.cluster.local:${db_port}/${db_name}?sslmode=disable"
gcs_credentials_path="${gcs_mount_path%/}/$gcs_secret_key"

"${kubectl_base[@]}" -n "$services_namespace" create secret generic "$gcs_secret" \
  --from-file="$gcs_secret_key=$gcs_credentials_file" \
  --dry-run=client -o yaml \
  | "${kubectl_base[@]}" apply -f -

"${kubectl_base[@]}" -n "$services_namespace" create secret generic "$runtime_secret" \
  --from-literal=CONTROL_PLANE_DB_DSN="$db_dsn" \
  --from-literal=ARCHIVE_MANIFEST_DB_DSN="$db_dsn" \
  --from-literal=ARCHIVE_OBJECT_GCS_PROJECT_ID="$archive_project_id" \
  --from-literal=ARCHIVE_OBJECT_BUCKET="$archive_bucket" \
  --from-literal=ARCHIVE_OBJECT_PREFIX="$archive_prefix" \
  --from-literal=ARCHIVE_OBJECT_REGION="$archive_region" \
  --from-literal=ARCHIVE_WRITER_ID="$archive_writer_id" \
  --from-literal=GOOGLE_APPLICATION_CREDENTIALS="$gcs_credentials_path" \
  --dry-run=client -o yaml \
  | "${kubectl_base[@]}" apply -f -

echo "created or updated $services_namespace/$runtime_secret and $services_namespace/$gcs_secret"
