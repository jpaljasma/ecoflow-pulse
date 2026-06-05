#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

mkdir -p "$tmpdir/bin"
credentials_file="$tmpdir/gcs-service-account.json"
kubectl_log="$tmpdir/kubectl.log"

printf '{"type":"service_account"}\n' >"$credentials_file"

cat >"$tmpdir/bin/kubectl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

log_file="${PULSE_FAKE_KUBECTL_LOG:?PULSE_FAKE_KUBECTL_LOG is required}"
args=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --kubeconfig|--context)
      shift 2
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done
printf '%q ' "${args[@]}" >>"$log_file"
printf '\n' >>"$log_file"
set -- "${args[@]}"

if [ "$1" = "create" ] && [ "$2" = "namespace" ]; then
  printf 'kind: Namespace\n'
  exit 0
fi
if [ "$1" = "apply" ] && [ "$2" = "-f" ] && [ "$3" = "-" ]; then
  cat >/dev/null
  exit 0
fi
if [ "$1" = "-n" ] && [ "$2" = "pulse-platform" ] && [ "$3" = "get" ] && [ "$4" = "secret" ]; then
  case "$*" in
    *'.data.username'*)
      printf 'cHVsc2U='
      exit 0
      ;;
    *'.data.password'*)
      printf 'cGFzcw=='
      exit 0
      ;;
  esac
fi
if [ "$1" = "-n" ] && [ "$2" = "pulse-services" ] && [ "$3" = "create" ] && [ "$4" = "secret" ]; then
  printf 'kind: Secret\n'
  exit 0
fi

printf 'unexpected kubectl args: %s\n' "$*" >&2
exit 1
SH

chmod +x "$tmpdir/bin/kubectl"

output="$(PULSE_FAKE_KUBECTL_LOG="$kubectl_log" \
  KUBECTL="$tmpdir/bin/kubectl" \
  "$script_dir/pulse-appliance-create-runtime-secret.sh" \
    --gcs-credentials "$credentials_file" \
    --gcs-secret-key source-key.json \
    --gcs-file-name mounted-credentials.json \
    --gcs-mount-path /custom/gcs \
    --gcs-project-id test-project)"

if ! grep -Fq "created or updated pulse-services/pulse-services-runtime-secret" <<<"$output"; then
  echo "expected runtime secret success output" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi
if ! grep -Fq -- "--from-file=source-key.json=$credentials_file" "$kubectl_log"; then
  echo "expected GCS secret to use the configured secret key" >&2
  cat "$kubectl_log" >&2
  exit 1
fi
if ! grep -Fq -- "--from-literal=GOOGLE_APPLICATION_CREDENTIALS=/custom/gcs/mounted-credentials.json" "$kubectl_log"; then
  echo "expected GOOGLE_APPLICATION_CREDENTIALS to use the mounted fileName" >&2
  cat "$kubectl_log" >&2
  exit 1
fi
if grep -Fq -- "--from-literal=GOOGLE_APPLICATION_CREDENTIALS=/custom/gcs/source-key.json" "$kubectl_log"; then
  echo "GOOGLE_APPLICATION_CREDENTIALS incorrectly used the secret key as the mounted filename" >&2
  cat "$kubectl_log" >&2
  exit 1
fi

echo "pulse appliance create runtime secret test passed"
