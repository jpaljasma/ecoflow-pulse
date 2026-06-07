#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

mkdir -p "$tmpdir/bin" "$tmpdir/dev"

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
  printf 'pending=0 dir=/var/lib/pulse-archive/outbox\n'
  exit 0
fi
printf 'unexpected kubectl args: %s\n' "$*" >&2
exit 1
SH

cat >"$tmpdir/bin/nc" <<'SH'
#!/usr/bin/env bash
exit 0
SH

cat >"$tmpdir/bin/nvme" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
echo "mock controller error" >&2
exit 5
SH

cat >"$tmpdir/bin/id" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = "-u" ]; then
  echo 1000
  exit 0
fi
exit 1
SH

chmod +x "$tmpdir/bin/"*
: >"$tmpdir/dev/nvme0n1"

set +e
output="$(PATH="$tmpdir/bin:$PATH" PULSE_APPLIANCE_NVME_DEVICE="$tmpdir/dev/nvme0n1" "$script_dir/pulse-appliance-status.sh" --kubeconfig /tmp/k3s.yaml 2>&1)"
status=$?
set -e

if [ "$status" -eq 0 ]; then
  echo "expected appliance status to fail for non-permission NVMe SMART errors" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi
if ! grep -Fq "fail: unable to read NVMe SMART log" <<<"$output"; then
  echo "expected NVMe SMART failure in status output" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi
if grep -Fq "requires root privileges" <<<"$output"; then
  echo "non-permission NVMe errors must not be downgraded to permission warnings" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi

echo "pulse appliance status NVMe failure test passed"
