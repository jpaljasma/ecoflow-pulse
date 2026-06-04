#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

mkdir -p "$tmpdir/bin"

cat >"$tmpdir/bin/systemctl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = "is-active" ] && [ "$2" = "--quiet" ] && [ "$3" = "k3s" ]; then
  exit 0
fi
exit 1
SH

cat >"$tmpdir/bin/helm" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
args=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --kubeconfig|--kube-context|--namespace)
      shift 2
      ;;
    status)
      shift
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done
exit 0
SH

cat >"$tmpdir/bin/kubectl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
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
set -- "${args[@]}"
if [ "$1" = "get" ] && [ "$2" = "nodes" ]; then
  exit 0
fi
if [ "$1" = "wait" ]; then
  exit 0
fi
if [ "$1" = "-n" ] && [ "$2" = "pulse-services" ] && [ "$3" = "get" ] && [ "$4" = "pod" ]; then
  printf 'pulse-services-go-archive-0'
  exit 0
fi
if [ "$1" = "-n" ] && [ "$2" = "pulse-services" ] && [ "$3" = "exec" ]; then
  printf 'pending=2 dir=/var/lib/pulse-archive/outbox\n'
  exit 2
fi
printf 'unexpected kubectl args: %s\n' "$*" >&2
exit 1
SH

cat >"$tmpdir/bin/nc" <<'SH'
#!/usr/bin/env bash
exit 0
SH

chmod +x "$tmpdir/bin/"*

set +e
output="$(PATH="$tmpdir/bin:$PATH" "$script_dir/pulse-appliance-status.sh" --kubeconfig /tmp/k3s.yaml 2>&1)"
status=$?
set -e

if [ "$status" -eq 0 ]; then
  echo "expected appliance status to fail when archive upload outbox has pending entries" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi
if ! grep -Fq "fail: archive upload outbox has pending local entries" <<<"$output"; then
  echo "expected pending archive outbox failure in status output" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi
if ! grep -Fq "pending=2 dir=/var/lib/pulse-archive/outbox" <<<"$output"; then
  echo "expected archive outbox status detail in output" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi

echo "pulse appliance status outbox test passed"
