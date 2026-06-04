#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

mkdir -p "$tmpdir/etc"
cat >"$tmpdir/etc/fstab" <<'FSTAB'
proc /proc proc defaults 0 0
UUID=root-test / ext4 defaults,discard 0 1
UUID=data-test /var/lib/pulse ext4 defaults 0 2
FSTAB

"$script_dir/pulse-appliance-host-prepare.sh" --root "$tmpdir" --no-packages >/dev/null
"$script_dir/pulse-appliance-host-prepare.sh" --root "$tmpdir" --no-packages >/dev/null

root_line="$(awk '$2 == "/" {print}' "$tmpdir/etc/fstab")"
case "$root_line" in
  "UUID=root-test / ext4 defaults,noatime,errors=remount-ro 0 1") ;;
  *)
    echo "unexpected root fstab line: $root_line" >&2
    exit 1
    ;;
esac

if [ "$(grep -o 'noatime' "$tmpdir/etc/fstab" | wc -l | tr -d ' ')" != "1" ]; then
  echo "root fstab options are not idempotent" >&2
  exit 1
fi

test -f "$tmpdir/etc/default/zramswap"
test -f "$tmpdir/etc/sysctl.d/90-pulse-appliance.conf"
test -f "$tmpdir/etc/systemd/journald.conf.d/90-pulse.conf"
test -f "$tmpdir/etc/rancher/k3s/config.yaml"
grep -q 'disable:' "$tmpdir/etc/rancher/k3s/config.yaml"

echo "pulse appliance host prepare test passed"
